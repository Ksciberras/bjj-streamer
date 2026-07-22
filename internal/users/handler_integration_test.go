package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
)

type identity struct{ token, csrf string }

func userIdentity(t *testing.T, store *Store, email, role string) identity {
	t.Helper()
	var id string
	if err := store.db.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	token, csrf, _, err := auth.NewStore(store.db).CreateSession(context.Background(), id, "", auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return identity{token: token, csrf: csrf}
}

func userRequest(t *testing.T, method, path string, body any, who identity, includeCSRF bool) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "user-handler-test")
	request.AddCookie(&http.Cookie{Name: "bjj_session", Value: who.token})
	if includeCSRF {
		request.Header.Set("X-CSRF-Token", who.csrf)
	}
	return request
}

func TestAdminAccountManagementRequiresAuthorizationAndCSRF(t *testing.T) {
	store, pool := testStore(t)
	admin := userIdentity(t, store, "admin@example.com", "admin")
	student := userIdentity(t, store, "student@example.com", "student")
	authHandler, err := auth.NewHandler(auth.NewStore(pool), auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 100)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, authHandler)
	mux := http.NewServeMux()
	handler.Register(mux)
	body := map[string]string{"email": "new@example.com", "role": "student", "password": "initial secure password"}

	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, userRequest(t, http.MethodPost, "/api/admin/users", body, student, true))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("student status=%d", denied.Code)
	}

	missingCSRF := httptest.NewRecorder()
	mux.ServeHTTP(missingCSRF, userRequest(t, http.MethodPost, "/api/admin/users", body, admin, false))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d", missingCSRF.Code)
	}

	created := httptest.NewRecorder()
	mux.ServeHTTP(created, userRequest(t, http.MethodPost, "/api/admin/users", body, admin, true))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
}
