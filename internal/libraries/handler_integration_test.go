package libraries

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/users"
)

type testIdentity struct{ id, token, csrf string }

func createIdentity(t *testing.T, poolURLStore *Store, email, role string) testIdentity {
	t.Helper()
	var id string
	if err := poolURLStore.db.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	authStore := auth.NewStore(poolURLStore.db)
	token, csrf, _, err := authStore.CreateSession(context.Background(), id, "", auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return testIdentity{id: id, token: token, csrf: csrf}
}
func request(t *testing.T, method, path string, body any, identity testIdentity) *http.Request {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Request-ID", "integration-request")
	if identity.token != "" {
		r.AddCookie(&http.Cookie{Name: "bjj_session", Value: identity.token})
		r.Header.Set("X-CSRF-Token", identity.csrf)
	}
	return r
}
func serve(mux *http.ServeMux, r *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, r)
	return response
}

func TestHTTPAuthorizationConcealsAndImmediatelyRemovesAccess(t *testing.T) {
	store, pool := libraryTestStore(t)
	admin := createIdentity(t, store, "admin@example.com", "admin")
	instructor := createIdentity(t, store, "instructor@example.com", "instructor")
	student := createIdentity(t, store, "student@example.com", "student")
	authHandler, err := auth.NewHandler(auth.NewStore(pool), auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 100)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, users.NewStore(pool), authHandler)
	mux := http.NewServeMux()
	handler.Register(mux)
	created := serve(mux, request(t, http.MethodPost, "/api/libraries", map[string]string{"name": "Team library"}, admin))
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Library Library `json:"library"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	libraryID := payload.Library.ID
	for name, who := range map[string]testIdentity{"student": student, "unassigned instructor": instructor} {
		t.Run(name+" concealed", func(t *testing.T) {
			response := serve(mux, request(t, http.MethodGet, "/api/libraries/"+libraryID, nil, who))
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d", response.Code)
			}
		})
	}
	personal, err := store.List(context.Background(), student.id)
	if err != nil {
		t.Fatal(err)
	}
	personalID := personal[0].ID
	if response := serve(mux, request(t, http.MethodGet, "/api/libraries/"+personalID, nil, admin)); response.Code != http.StatusNotFound {
		t.Fatalf("admin personal access=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodPost, "/api/libraries", map[string]string{"name": "Forbidden"}, student)); response.Code != http.StatusNotFound {
		t.Fatalf("student create=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodPut, "/api/libraries/"+libraryID+"/members/"+student.id, map[string]string{"access_level": "instructor"}, admin)); response.Code != http.StatusNotFound {
		t.Fatalf("student instructor assignment=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodPut, "/api/libraries/"+libraryID+"/members/"+instructor.id, map[string]string{"access_level": "instructor"}, admin)); response.Code != http.StatusNoContent {
		t.Fatalf("instructor assignment=%d %s", response.Code, response.Body.String())
	}
	if response := serve(mux, request(t, http.MethodGet, "/api/libraries/"+libraryID, nil, instructor)); response.Code != http.StatusOK {
		t.Fatalf("assigned instructor access=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodPut, "/api/libraries/"+libraryID+"/members/"+student.id, map[string]string{"access_level": "student"}, admin)); response.Code != http.StatusNoContent {
		t.Fatalf("student membership=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodGet, "/api/libraries/"+libraryID, nil, student)); response.Code != http.StatusOK {
		t.Fatalf("student member access=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodDelete, "/api/libraries/"+libraryID+"/members/"+student.id, nil, admin)); response.Code != http.StatusNoContent {
		t.Fatalf("remove=%d", response.Code)
	}
	if response := serve(mux, request(t, http.MethodGet, "/api/libraries/"+libraryID, nil, student)); response.Code != http.StatusNotFound {
		t.Fatalf("removed access=%d", response.Code)
	}
}
