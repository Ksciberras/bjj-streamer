package learning

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
	"github.com/kyransciberras/bjj-streaming/internal/videos"
)

type playbackFake struct{}

func (playbackFake) PresignGet(context.Context, string) (string, error) {
	return "http://storage.example/play", nil
}

type learner struct{ id, token, csrf string }

func learningPool(t *testing.T) *pgxpool.Pool {
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
	if _, err = pool.Exec(context.Background(), `TRUNCATE notes,playback_progress,videos,audit_events,library_members,libraries,sessions,invitations,users CASCADE`); err != nil {
		t.Fatal(err)
	}
	return pool
}
func newLearner(t *testing.T, pool *pgxpool.Pool, email, role string) learner {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	token, csrf, _, err := auth.NewStore(pool).CreateSession(context.Background(), id, "", auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return learner{id: id, token: token, csrf: csrf}
}
func readyVideo(t *testing.T, pool *pgxpool.Pool, uploader learner, visibility string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO videos(uploaded_by_user_id,title,instructor_name,visibility,content_basis,object_key,original_filename,mime_type,byte_size,status)VALUES($1,$2,'Coach',$3,'self_created',$4,'video.mp4','video/mp4',10,'ready')RETURNING id`, uploader.id, "Video "+visibility, visibility, visibility+".mp4").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func learningRequest(t *testing.T, method, path string, body any, who learner) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", who.csrf)
	request.AddCookie(&http.Cookie{Name: "bjj_session", Value: who.token})
	return request
}
func learningResponse(mux *http.ServeMux, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestPlaybackAuthorizationAndLearningIsolation(t *testing.T) {
	pool := learningPool(t)
	instructor := newLearner(t, pool, "instructor@example.com", "instructor")
	first := newLearner(t, pool, "first@example.com", "student")
	second := newLearner(t, pool, "second@example.com", "student")
	sharedID := readyVideo(t, pool, instructor, "shared")
	privateID := readyVideo(t, pool, instructor, "private")
	authHandler, err := auth.NewHandler(auth.NewStore(pool), auth.Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 100)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewStore(pool), videos.NewStore(pool), playbackFake{}, authHandler)
	mux := http.NewServeMux()
	handler.Register(mux)
	if response := learningResponse(mux, learningRequest(t, http.MethodGet, "/api/videos/"+sharedID+"/playback", nil, first)); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("playback_url")) {
		t.Fatalf("shared playback=%d %s", response.Code, response.Body.String())
	}
	if response := learningResponse(mux, learningRequest(t, http.MethodGet, "/api/videos/"+privateID+"/playback", nil, first)); response.Code != http.StatusNotFound {
		t.Fatalf("private playback=%d", response.Code)
	}
	for _, item := range []struct {
		who      learner
		position float64
	}{{first, 12.5}, {second, 87.25}} {
		response := learningResponse(mux, learningRequest(t, http.MethodPut, "/api/videos/"+sharedID+"/progress", map[string]any{"position_seconds": item.position}, item.who))
		if response.Code != http.StatusOK {
			t.Fatalf("progress=%d %s", response.Code, response.Body.String())
		}
	}
	firstProgress, _ := NewStore(pool).GetProgress(context.Background(), first.id, sharedID)
	secondProgress, _ := NewStore(pool).GetProgress(context.Background(), second.id, sharedID)
	if firstProgress.PositionSeconds != 12.5 || secondProgress.PositionSeconds != 87.25 {
		t.Fatalf("progress first=%+v second=%+v", firstProgress, secondProgress)
	}
	created := learningResponse(mux, learningRequest(t, http.MethodPost, "/api/videos/"+sharedID+"/notes", map[string]any{"timestamp_seconds": 33.0, "body": "First user's note"}, first))
	if created.Code != http.StatusCreated {
		t.Fatalf("note=%d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Note Note `json:"note"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response := learningResponse(mux, learningRequest(t, http.MethodGet, "/api/videos/"+sharedID+"/notes", nil, second)); response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte("First user's note")) {
		t.Fatalf("note leaked=%d %s", response.Code, response.Body.String())
	}
	if response := learningResponse(mux, learningRequest(t, http.MethodPatch, "/api/videos/"+sharedID+"/notes/"+payload.Note.ID, map[string]any{"timestamp_seconds": 1, "body": "stolen"}, second)); response.Code != http.StatusNotFound {
		t.Fatalf("other edit=%d", response.Code)
	}
	if response := learningResponse(mux, learningRequest(t, http.MethodDelete, "/api/videos/"+sharedID+"/notes/"+payload.Note.ID, nil, second)); response.Code != http.StatusNotFound {
		t.Fatalf("other delete=%d", response.Code)
	}
	if response := learningResponse(mux, learningRequest(t, http.MethodPut, "/api/videos/"+sharedID+"/watch-later", nil, first)); response.Code != http.StatusNoContent {
		t.Fatalf("watch later=%d %s", response.Code, response.Body.String())
	}
	firstStudy := learningResponse(mux, learningRequest(t, http.MethodGet, "/api/study", nil, first))
	if firstStudy.Code != http.StatusOK || !bytes.Contains(firstStudy.Body.Bytes(), []byte("First user's note")) || !bytes.Contains(firstStudy.Body.Bytes(), []byte(sharedID)) {
		t.Fatalf("first study=%d %s", firstStudy.Code, firstStudy.Body.String())
	}
	secondStudy := learningResponse(mux, learningRequest(t, http.MethodGet, "/api/study", nil, second))
	if secondStudy.Code != http.StatusOK || bytes.Contains(secondStudy.Body.Bytes(), []byte("First user's note")) || bytes.Contains(secondStudy.Body.Bytes(), []byte(sharedID)) {
		t.Fatalf("study data leaked=%d %s", secondStudy.Code, secondStudy.Body.String())
	}
}
