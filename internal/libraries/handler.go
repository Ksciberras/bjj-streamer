package libraries

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/authorization"
	"github.com/kyransciberras/bjj-streaming/internal/users"
)

type Handler struct {
	store  *Store
	users  *users.Store
	auth   *auth.Handler
	policy authorization.Policy
}

func NewHandler(store *Store, userStore *users.Store, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, users: userStore, auth: authHandler}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/libraries", h.list)
	mux.HandleFunc("POST /api/libraries", h.create)
	mux.HandleFunc("GET /api/libraries/{id}", h.get)
	mux.HandleFunc("PATCH /api/libraries/{id}", h.update)
	mux.HandleFunc("GET /api/libraries/{id}/members", h.members)
	mux.HandleFunc("PUT /api/libraries/{id}/members/{user_id}", h.putMember)
	mux.HandleFunc("DELETE /api/libraries/{id}/members/{user_id}", h.removeMember)
}
func (h *Handler) session(w http.ResponseWriter, r *http.Request, csrf bool) (auth.Session, bool) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return auth.Session{}, false
	}
	if csrf && !h.auth.RequireCSRF(w, r, session) {
		return auth.Session{}, false
	}
	return session, true
}
func actor(session auth.Session) authorization.Actor {
	return authorization.Actor{ID: session.User.ID, Role: authorization.Role(session.User.Role)}
}
func policyLibrary(value Library) authorization.Library {
	return authorization.Library{ID: value.ID, Type: authorization.LibraryType(value.Type), OwnerID: valueString(value.OwnerUserID), Archived: value.Archived}
}
func membership(value Library) *authorization.MembershipLevel {
	if value.Membership == nil {
		return nil
	}
	level := authorization.MembershipLevel(*value.Membership)
	return &level
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	values, err := h.store.List(r.Context(), session.User.ID)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": values})
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	if !h.policy.CreateSharedLibrary(actor(session)) {
		notFound(w)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if decode(r, &input) != nil || !validName(input.Name) {
		badRequest(w)
		return
	}
	value, err := h.store.CreateShared(r.Context(), session.User.ID, strings.TrimSpace(input.Name), r.Header.Get("X-Request-ID"))
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"library": value})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	value, err := h.store.Get(r.Context(), r.PathValue("id"), session.User.ID)
	if err != nil || !h.policy.ViewLibrary(actor(session), policyLibrary(value), membership(value)) {
		notFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"library": value})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	value, err := h.store.Get(r.Context(), r.PathValue("id"), session.User.ID)
	if err != nil {
		notFound(w)
		return
	}
	allowed := h.policy.EditPersonalLibrary(actor(session), policyLibrary(value)) || h.policy.ManageSharedLibrary(actor(session), policyLibrary(value))
	if !allowed {
		notFound(w)
		return
	}
	var input struct {
		Name     *string `json:"name"`
		Archived *bool   `json:"archived"`
	}
	if decode(r, &input) != nil || (input.Name == nil && input.Archived == nil) || (input.Name != nil && !validName(*input.Name)) || (value.Type == "personal" && input.Archived != nil) {
		badRequest(w)
		return
	}
	updated, err := h.store.Update(r.Context(), session.User.ID, value.ID, session.User.ID, r.Header.Get("X-Request-ID"), trimmed(input.Name), input.Archived)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"library": updated})
}
func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	value, err := h.store.Get(r.Context(), r.PathValue("id"), session.User.ID)
	if err != nil || !h.policy.ViewMembers(actor(session), policyLibrary(value), membership(value)) {
		notFound(w)
		return
	}
	if session.User.Role == "admin" {
		members, queryErr := h.store.ListMembers(r.Context(), value.ID)
		if queryErr != nil {
			serverError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
		return
	}
	member, err := h.store.GetMember(r.Context(), value.ID, session.User.ID)
	if err != nil {
		notFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": []Member{member}})
}
func (h *Handler) putMember(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	value, err := h.store.Get(r.Context(), r.PathValue("id"), session.User.ID)
	if err != nil || !h.policy.ManageMembership(actor(session), policyLibrary(value)) {
		notFound(w)
		return
	}
	target, err := h.users.Get(r.Context(), r.PathValue("user_id"))
	if err != nil {
		notFound(w)
		return
	}
	var input struct {
		AccessLevel string `json:"access_level"`
	}
	if decode(r, &input) != nil {
		badRequest(w)
		return
	}
	targetActor := authorization.Actor{ID: target.ID, Role: authorization.Role(target.Role), Disabled: target.Disabled}
	level := authorization.MembershipLevel(input.AccessLevel)
	if !h.policy.AssignMembership(actor(session), policyLibrary(value), targetActor, level) {
		notFound(w)
		return
	}
	if err = h.store.PutMember(r.Context(), session.User.ID, value.ID, target.ID, input.AccessLevel, r.Header.Get("X-Request-ID")); err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	value, err := h.store.Get(r.Context(), r.PathValue("id"), session.User.ID)
	if err != nil || !h.policy.ManageMembership(actor(session), policyLibrary(value)) {
		notFound(w)
		return
	}
	if err = h.store.RemoveMember(r.Context(), session.User.ID, value.ID, r.PathValue("user_id"), r.Header.Get("X-Request-ID")); err != nil {
		notFound(w)
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
func validName(name string) bool {
	length := len([]rune(strings.TrimSpace(name)))
	return length >= 1 && length <= 120
}
func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}
func valueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
func badRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
}
func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
