package audit

import (
	"encoding/json"
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
func (h *Handler) Register(mux *http.ServeMux) { mux.HandleFunc("GET /api/admin/audit-events", h.list) }
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return
	}
	if !session.User.IsPlatformOwner {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	events, err := h.store.List(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_events": events})
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
