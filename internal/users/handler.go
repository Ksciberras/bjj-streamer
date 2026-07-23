package users

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/authorization"
)

type Handler struct {
	store  *Store
	auth   *auth.Handler
	policy authorization.Policy
}

func NewHandler(store *Store, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, auth: authHandler}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/users", h.list)
	mux.HandleFunc("POST /api/admin/users", h.create)
	mux.HandleFunc("PATCH /api/admin/users/{id}", h.update)
	mux.HandleFunc("POST /api/admin/users/{id}/password", h.resetPassword)
	mux.HandleFunc("POST /api/admin/users/{id}/sessions/revoke", h.revoke)
}

func (h *Handler) actor(w http.ResponseWriter, r *http.Request, csrf bool) (auth.Session, bool) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return auth.Session{}, false
	}
	actor := authorization.Actor{ID: session.User.ID, Role: authorization.Role(session.User.Role)}
	if !h.policy.ManageUsers(actor) {
		notFound(w)
		return auth.Session{}, false
	}
	if csrf && !h.auth.RequireCSRF(w, r, session) {
		return auth.Session{}, false
	}
	return session, true
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.actor(w, r, false)
	if !ok {
		return
	}
	users, err := h.store.ListFor(r.Context(), session.User.ID)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.actor(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Email          string  `json:"email"`
		Role           string  `json:"role"`
		Password       string  `json:"password"`
		OrganizationID *string `json:"organization_id"`
	}
	if decode(r, &input) != nil || !auth.ValidEmail(input.Email) || auth.ValidatePassword(input.Password) != nil {
		badRequest(w, "invalid user")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		badRequest(w, "invalid user")
		return
	}
	if !session.User.IsPlatformOwner {
		input.OrganizationID = nil
	}
	user, err := h.store.CreateInOrganization(r.Context(), session.User.ID, input.Email, input.Role, hash, input.OrganizationID, r.Header.Get("X-Request-ID"))
	if err == ErrConflict {
		badRequest(w, "email or role conflicts with an existing account")
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	session, ok := h.actor(w, r, true)
	if !ok {
		return
	}
	if !h.store.CanManage(r.Context(), session.User.ID, r.PathValue("id")) {
		notFound(w)
		return
	}
	var input struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
	}
	if decode(r, &input) != nil || (input.Role == nil && input.Disabled == nil) {
		badRequest(w, "invalid user update")
		return
	}
	user, err := h.store.Update(r.Context(), session.User.ID, r.PathValue("id"), input.Role, input.Disabled, r.Header.Get("X-Request-ID"))
	if err == ErrNotFound {
		notFound(w)
		return
	}
	if err == ErrLastAdmin || err == ErrConflict {
		badRequest(w, err.Error())
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}
func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	session, ok := h.actor(w, r, true)
	if !ok {
		return
	}
	if !h.store.CanManage(r.Context(), session.User.ID, r.PathValue("id")) {
		notFound(w)
		return
	}
	err := h.store.RevokeSessions(r.Context(), session.User.ID, r.PathValue("id"), r.Header.Get("X-Request-ID"))
	if err == ErrNotFound {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	session, ok := h.actor(w, r, true)
	if !ok {
		return
	}
	if !h.store.CanManage(r.Context(), session.User.ID, r.PathValue("id")) {
		notFound(w)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if decode(r, &input) != nil || auth.ValidatePassword(input.Password) != nil {
		badRequest(w, "invalid password")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		badRequest(w, "invalid password")
		return
	}
	if err = h.store.ResetPassword(r.Context(), session.User.ID, r.PathValue("id"), hash, r.Header.Get("X-Request-ID")); err == ErrNotFound {
		notFound(w)
		return
	} else if err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decode(r *http.Request, target any) error {
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
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
func badRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
}
func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
