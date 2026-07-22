package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginSessionAndCSRFFlow(t *testing.T) {
	store, _ := integrationStore(t)
	bootstrap(t, store)
	handler, err := NewHandler(store, Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 10)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Register(mux)
	login := jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "admin@example.com", "password": "correct horse battery staple"})
	loginResponse := httptest.NewRecorder()
	mux.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	var session, csrf *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == sessionCookie {
			session = cookie
		}
		if cookie.Name == csrfCookie {
			csrf = cookie
		}
	}
	if session == nil || csrf == nil || !session.HttpOnly || csrf.HttpOnly {
		t.Fatal("secure cookie properties missing")
	}
	logout := jsonRequest(t, http.MethodPost, "/api/auth/logout", map[string]string{})
	logout.AddCookie(session)
	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, logout)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d", denied.Code)
	}
	logout = jsonRequest(t, http.MethodPost, "/api/auth/logout", map[string]string{})
	logout.AddCookie(session)
	logout.Header.Set(csrfHeader, csrf.Value)
	loggedOut := httptest.NewRecorder()
	mux.ServeHTTP(loggedOut, logout)
	if loggedOut.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d", loggedOut.Code)
	}
	current := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	current.AddCookie(session)
	after := httptest.NewRecorder()
	mux.ServeHTTP(after, current)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d", after.Code)
	}
}

func TestLoginRateLimitAndGenericFailure(t *testing.T) {
	store, _ := integrationStore(t)
	bootstrap(t, store)
	handler, _ := NewHandler(store, Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}, 1)
	mux := http.NewServeMux()
	handler.Register(mux)
	for index, want := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		request := jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "missing@example.com", "password": "incorrect password"})
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("request %d status=%d", index, response.Code)
		}
	}
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	return request
}
