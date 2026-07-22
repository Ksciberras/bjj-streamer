package videos

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/objectstorage"
)

type fakeObjects struct {
	object  objectstorage.Object
	headErr error
}

func (fakeObjects) PresignPut(context.Context, string, string, int64) (string, error) {
	return "http://storage.example/upload", nil
}
func (f fakeObjects) Head(context.Context, string) (objectstorage.Object, error) {
	return f.object, f.headErr
}

type testIdentity struct{ id, token, csrf string }

func videoTestStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	connection, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connection.Exec(context.Background(), `SELECT pg_advisory_lock(8675309)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(8675309)`)
		connection.Release()
	})
	if _, err = pool.Exec(context.Background(), `TRUNCATE videos,audit_events,library_members,libraries,sessions,invitations,users CASCADE`); err != nil {
		t.Fatal(err)
	}
	return NewStore(pool), pool
}
func videoIdentity(t *testing.T, pool *pgxpool.Pool, email, role string) testIdentity {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	token, csrf, _, err := auth.NewStore(pool).CreateSession(context.Background(), id, "", auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return testIdentity{id: id, token: token, csrf: csrf}
}
func videoRequest(t *testing.T, method, path string, body any, who testIdentity) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Request-ID", "video-test")
	r.Header.Set("X-CSRF-Token", who.csrf)
	r.AddCookie(&http.Cookie{Name: "bjj_session", Value: who.token})
	return r
}
func serveVideo(mux *http.ServeMux, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestUploadAuthorizationCompletionVisibilityAndSearch(t *testing.T) {
	store, pool := videoTestStore(t)
	admin := videoIdentity(t, pool, "admin@example.com", "admin")
	instructor := videoIdentity(t, pool, "instructor@example.com", "instructor")
	other := videoIdentity(t, pool, "other@example.com", "instructor")
	student := videoIdentity(t, pool, "student@example.com", "student")
	authHandler, err := auth.NewHandler(auth.NewStore(pool), auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 100)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, fakeObjects{object: objectstorage.Object{Size: 1024, ContentType: "video/mp4"}}, authHandler)
	mux := http.NewServeMux()
	handler.Register(mux)
	body := map[string]any{"title": "Armbar Study", "instructor_name": "Coach", "description": "", "tags": []string{"armbar", "guard"}, "visibility": "private", "content_basis": "self_created", "filename": "armbar.mp4", "mime_type": "video/mp4", "byte_size": 1024}
	if response := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/upload-requests", body, student)); response.Code != http.StatusNotFound {
		t.Fatalf("student upload=%d", response.Code)
	}
	created := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/upload-requests", body, instructor))
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Video     Video  `json:"video"`
		UploadURL string `json:"upload_url"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.UploadURL == "" || payload.Video.ObjectKey != "" {
		t.Fatalf("payload=%+v", payload)
	}
	if response := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/"+payload.Video.ID+"/complete", map[string]any{}, other)); response.Code != http.StatusNotFound {
		t.Fatalf("other complete=%d", response.Code)
	}
	editBody := map[string]any{"title": "Stolen", "instructor_name": "Other", "description": "", "tags": []string{}, "visibility": "shared", "content_basis": "self_created", "archived": false}
	if response := serveVideo(mux, videoRequest(t, http.MethodPatch, "/api/videos/"+payload.Video.ID, editBody, other)); response.Code != http.StatusNotFound {
		t.Fatalf("other edit=%d", response.Code)
	}
	if response := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/"+payload.Video.ID+"/complete", map[string]any{}, instructor)); response.Code != http.StatusOK {
		t.Fatalf("complete=%d %s", response.Code, response.Body.String())
	}
	if response := serveVideo(mux, videoRequest(t, http.MethodGet, "/api/videos?q=armbar", nil, student)); response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte("Armbar Study")) {
		t.Fatalf("private leaked: %d %s", response.Code, response.Body.String())
	}
	if response := serveVideo(mux, videoRequest(t, http.MethodGet, "/api/videos?q=armbar", nil, admin)); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("Armbar Study")) {
		t.Fatalf("admin missing private: %d %s", response.Code, response.Body.String())
	}
	body["title"] = "Forbidden"
	body["visibility"] = "shared"
	body["content_basis"] = "personal_purchase"
	if response := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/upload-requests", body, instructor)); response.Code != http.StatusBadRequest {
		t.Fatalf("personal purchase shared=%d", response.Code)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO videos(uploaded_by_user_id,title,instructor_name,visibility,content_basis,object_key,original_filename,mime_type,byte_size)VALUES($1,'Invalid','Coach','shared','personal_purchase','invalid.mp4','invalid.mp4','video/mp4',1)`, instructor.id); err == nil {
		t.Fatal("database accepted shared personal purchase")
	}
}

func TestDirectUploadAgainstLocalObjectStorage(t *testing.T) {
	if os.Getenv("TEST_OBJECT_STORAGE") == "" {
		t.Skip("TEST_OBJECT_STORAGE is not set")
	}
	store, pool := videoTestStore(t)
	instructor := videoIdentity(t, pool, "uploader@example.com", "instructor")
	authHandler, err := auth.NewHandler(auth.NewStore(pool), auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 100)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := objectstorage.New(context.Background(), "http://localhost:9000", "http://localhost:9000", "us-east-1", "bjj-videos", "minioadmin", "minioadmin", true, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, objects, authHandler)
	mux := http.NewServeMux()
	handler.Register(mux)
	file := []byte("small direct upload integration object")
	body := map[string]any{"title": "Direct Upload", "instructor_name": "Coach", "description": "", "tags": []string{"test"}, "visibility": "shared", "content_basis": "self_created", "filename": "direct.mp4", "mime_type": "video/mp4", "byte_size": len(file)}
	created := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/upload-requests", body, instructor))
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Video     Video  `json:"video"`
		UploadURL string `json:"upload_url"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	put, err := http.NewRequest(http.MethodPut, payload.UploadURL, bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "video/mp4")
	putResponse, err := http.DefaultClient.Do(put)
	if err != nil {
		t.Fatal(err)
	}
	putResponse.Body.Close()
	if putResponse.StatusCode < 200 || putResponse.StatusCode >= 300 {
		t.Fatalf("put=%d", putResponse.StatusCode)
	}
	completed := serveVideo(mux, videoRequest(t, http.MethodPost, "/api/videos/"+payload.Video.ID+"/complete", map[string]any{}, instructor))
	if completed.Code != http.StatusOK {
		t.Fatalf("complete=%d %s", completed.Code, completed.Body.String())
	}
	listed := serveVideo(mux, videoRequest(t, http.MethodGet, "/api/videos?q=direct", nil, instructor))
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte("Direct Upload")) {
		t.Fatalf("list=%d %s", listed.Code, listed.Body.String())
	}
}
