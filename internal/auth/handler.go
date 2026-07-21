package auth

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const sessionCookie = "bjj_session"
const csrfCookie = "bjj_csrf"
const csrfHeader = "X-CSRF-Token"

type Handler struct {
	store             *Store
	settings          Settings
	loginLimiter      *RateLimiter
	invitationLimiter *RateLimiter
	dummyHash         string
}

func NewHandler(store *Store, settings Settings, loginLimit, invitationLimit int) (*Handler, error) {
	dummy, err := HashPassword("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Handler{store: store, settings: settings, loginLimiter: NewRateLimiter(loginLimit, time.Minute), invitationLimiter: NewRateLimiter(invitationLimit, time.Minute), dummyHash: dummy}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/auth/login", noStore(http.HandlerFunc(h.login)))
	mux.Handle("POST /api/auth/logout", noStore(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/auth/session", noStore(http.HandlerFunc(h.currentSession)))
	mux.Handle("POST /api/auth/invitations", noStore(http.HandlerFunc(h.createInvitation)))
	mux.Handle("POST /api/auth/invitations/accept", noStore(http.HandlerFunc(h.acceptInvitation)))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decodeJSON(r, &input) != nil {
		invalidCredentials(w)
		return
	}
	allowedIP := h.loginLimiter.Allow("ip:" + clientIP(r))
	allowedIdentity := h.loginLimiter.Allow("email:" + normalizeEmail(input.Email))
	if !allowedIP || !allowedIdentity {
		rateLimited(w)
		return
	}
	if input.Email == "" {
		invalidCredentials(w)
		return
	}
	if len(input.Password) > 1024 {
		CheckPassword(h.dummyHash, input.Password)
		invalidCredentials(w)
		return
	}
	user, encoded, err := h.store.PasswordHash(r.Context(), input.Email)
	if err != nil {
		CheckPassword(h.dummyHash, input.Password)
		invalidCredentials(w)
		return
	}
	if !CheckPassword(encoded, input.Password) {
		invalidCredentials(w)
		return
	}
	old := cookieValue(r, sessionCookie)
	token, csrf, expires, err := h.store.CreateSession(r.Context(), user.ID, old, h.settings)
	if err != nil {
		serverError(w)
		return
	}
	h.setCookies(w, token, csrf, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token, session, ok := h.authenticated(w, r)
	if !ok {
		return
	}
	if !validCSRF(r, session.CSRFHash) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid csrf token"})
		return
	}
	if err := h.store.RevokeSession(r.Context(), token); err != nil {
		serverError(w)
		return
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) currentSession(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.authenticated(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": session.User})
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	if !h.invitationLimiter.Allow(r.URL.Path + ":" + clientIP(r)) {
		rateLimited(w)
		return
	}
	_, session, ok := h.authenticated(w, r)
	if !ok {
		return
	}
	if !validCSRF(r, session.CSRFHash) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid csrf token"})
		return
	}
	if session.User.Role != "admin" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var input struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if decodeJSON(r, &input) != nil || !ValidEmail(input.Email) || !validRole(input.Role) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid invitation"})
		return
	}
	token, expires, err := h.store.CreateInvitation(r.Context(), input.Email, input.Role, session.User.ID, h.settings.InvitationTTL)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "expires_at": expires})
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if !h.invitationLimiter.Allow(r.URL.Path + ":" + clientIP(r)) {
		rateLimited(w)
		return
	}
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if decodeJSON(r, &input) != nil || input.Token == "" || ValidatePassword(input.Password) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired invitation"})
		return
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		serverError(w)
		return
	}
	user, err := h.store.AcceptInvitation(r.Context(), input.Token, hash)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired invitation"})
		return
	}
	token, csrf, expires, err := h.store.CreateSession(r.Context(), user.ID, cookieValue(r, sessionCookie), h.settings)
	if err != nil {
		serverError(w)
		return
	}
	h.setCookies(w, token, csrf, expires)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *Handler) authenticated(w http.ResponseWriter, r *http.Request) (string, Session, bool) {
	token := cookieValue(r, sessionCookie)
	if token == "" {
		unauthenticated(w)
		return "", Session{}, false
	}
	session, err := h.store.Authenticate(r.Context(), token, h.settings.SessionIdleTTL)
	if err != nil {
		h.clearCookies(w)
		unauthenticated(w)
		return "", Session{}, false
	}
	return token, session, true
}

func (h *Handler) setCookies(w http.ResponseWriter, token, csrf string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: h.settings.CookieSecure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: csrf, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: false, Secure: h.settings.CookieSecure, SameSite: http.SameSiteStrictMode})
}

func (h *Handler) clearCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: name == sessionCookie, Secure: h.settings.CookieSecure, SameSite: http.SameSiteStrictMode})
	}
}

func validCSRF(r *http.Request, want []byte) bool {
	got := r.Header.Get(csrfHeader)
	return got != "" && subtle.ConstantTimeCompare(tokenHash(got), want) == 1
}
func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func validRole(role string) bool { return role == "admin" || role == "instructor" || role == "student" }
func ValidEmail(email string) bool {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil && strings.EqualFold(parsed.Address, strings.TrimSpace(email))
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer != nil && (peer.IsPrivate() || peer.IsLoopback()) {
		if forwarded := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); forwarded != nil {
			return forwarded.String()
		}
	}
	return host
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func invalidCredentials(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
}
func unauthenticated(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
}
func rateLimited(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
}
func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
