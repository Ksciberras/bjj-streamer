package learning

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/authorization"
	"github.com/kyransciberras/bjj-streaming/internal/videos"
)

type PlaybackObjects interface {
	PresignGet(context.Context, string) (string, error)
}

type Handler struct {
	store   *Store
	videos  *videos.Store
	objects PlaybackObjects
	auth    *auth.Handler
	policy  authorization.Policy
}

func NewHandler(store *Store, videoStore *videos.Store, objects PlaybackObjects, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, videos: videoStore, objects: objects, auth: authHandler}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/videos/{id}/playback", h.playback)
	mux.HandleFunc("GET /api/videos/{id}/progress", h.getProgress)
	mux.HandleFunc("PUT /api/videos/{id}/progress", h.putProgress)
	mux.HandleFunc("GET /api/videos/{id}/notes", h.listNotes)
	mux.HandleFunc("POST /api/videos/{id}/notes", h.createNote)
	mux.HandleFunc("PATCH /api/videos/{id}/notes/{note_id}", h.updateNote)
	mux.HandleFunc("DELETE /api/videos/{id}/notes/{note_id}", h.deleteNote)
	mux.HandleFunc("GET /api/study", h.study)
	mux.HandleFunc("PUT /api/videos/{id}/watch-later", h.addWatchLater)
	mux.HandleFunc("DELETE /api/videos/{id}/watch-later", h.removeWatchLater)
	mux.HandleFunc("POST /api/videos/{id}/learning-events", h.recordEvent)
	mux.HandleFunc("GET /api/analytics", h.analytics)
}

func (h *Handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Type            string  `json:"type"`
		PositionSeconds float64 `json:"position_seconds"`
	}
	if decode(r, &input) != nil ||
		(input.Type != "started" && input.Type != "resumed" && input.Type != "completed") ||
		invalidSeconds(input.PositionSeconds) {
		badRequest(w)
		return
	}
	if err := h.store.RecordEvent(r.Context(), session.User.ID, video.ID, input.Type, input.PositionSeconds); err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return
	}
	if session.User.Role != "admin" && session.User.Role != "instructor" {
		notFound(w)
		return
	}
	days := 30
	if r.URL.Query().Get("period") == "7" {
		days = 7
	}
	result, err := h.store.Analytics(r.Context(), session.User.OrganizationID, session.User.IsPlatformOwner, days)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analytics": result})
}

func (h *Handler) study(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return
	}
	ids, err := h.store.ListWatchLaterIDs(r.Context(), session.User.ID)
	if err != nil {
		serverError(w)
		return
	}
	watchLater := []videos.Video{}
	for _, id := range ids {
		video, getErr := h.videos.Get(r.Context(), id)
		if getErr == nil && h.videos.CanView(r.Context(), video, session.User.ID, session.User.Role, session.User.OrganizationID, session.User.IsPlatformOwner) {
			if video.ThumbnailReady && video.ThumbnailObjectKey != nil {
				video.ThumbnailURL = "/api/videos/" + video.ID + "/thumbnail"
			}
			watchLater = append(watchLater, video)
		}
	}
	notes, err := h.store.ListStudyNotes(r.Context(), session.User.ID)
	if err != nil {
		serverError(w)
		return
	}
	type studyNoteResponse struct {
		StudyNote
		Video videos.Video `json:"video"`
	}
	visibleNotes := []studyNoteResponse{}
	for _, note := range notes {
		video, getErr := h.videos.Get(r.Context(), note.VideoID)
		if getErr == nil && h.videos.CanView(r.Context(), video, session.User.ID, session.User.Role, session.User.OrganizationID, session.User.IsPlatformOwner) {
			if video.ThumbnailReady && video.ThumbnailObjectKey != nil {
				video.ThumbnailURL = "/api/videos/" + video.ID + "/thumbnail"
			}
			visibleNotes = append(visibleNotes, studyNoteResponse{StudyNote: note, Video: video})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"watch_later": watchLater, "notes": visibleNotes})
}

func (h *Handler) addWatchLater(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	if err := h.store.AddWatchLater(r.Context(), session.User.ID, video.ID); err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeWatchLater(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	if err := h.store.RemoveWatchLater(r.Context(), session.User.ID, video.ID); err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authorized(w http.ResponseWriter, r *http.Request, csrf bool) (auth.Session, videos.Video, bool) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok {
		return auth.Session{}, videos.Video{}, false
	}
	if csrf && !h.auth.RequireCSRF(w, r, session) {
		return auth.Session{}, videos.Video{}, false
	}
	video, err := h.videos.Get(r.Context(), r.PathValue("id"))
	if err != nil || !h.videos.CanView(r.Context(), video, session.User.ID, session.User.Role, session.User.OrganizationID, session.User.IsPlatformOwner) {
		notFound(w)
		return auth.Session{}, videos.Video{}, false
	}
	return session, video, true
}

func (h *Handler) playback(w http.ResponseWriter, r *http.Request) {
	_, video, ok := h.authorized(w, r, false)
	if !ok {
		return
	}
	url, err := h.objects.PresignGet(r.Context(), video.ObjectKey)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"playback_url": url})
}
func (h *Handler) getProgress(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, false)
	if !ok {
		return
	}
	value, err := h.store.GetProgress(r.Context(), session.User.ID, video.ID)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"progress": value})
}
func (h *Handler) putProgress(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	var input struct {
		PositionSeconds float64 `json:"position_seconds"`
	}
	if decode(r, &input) != nil || invalidSeconds(input.PositionSeconds) {
		badRequest(w)
		return
	}
	value, err := h.store.PutProgress(r.Context(), session.User.ID, video.ID, input.PositionSeconds)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"progress": value})
}
func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, false)
	if !ok {
		return
	}
	values, err := h.store.ListNotes(r.Context(), session.User.ID, video.ID)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": values})
}

type noteInput struct {
	TimestampSeconds float64 `json:"timestamp_seconds"`
	Body             string  `json:"body"`
}

func (h *Handler) createNote(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	var input noteInput
	if decode(r, &input) != nil || !validNote(input) {
		badRequest(w)
		return
	}
	note, err := h.store.CreateNote(r.Context(), session.User.ID, video.ID, input.TimestampSeconds, strings.TrimSpace(input.Body))
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"note": note})
}
func (h *Handler) updateNote(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	var input noteInput
	if decode(r, &input) != nil || !validNote(input) {
		badRequest(w)
		return
	}
	note, err := h.store.UpdateNote(r.Context(), session.User.ID, video.ID, r.PathValue("note_id"), input.TimestampSeconds, strings.TrimSpace(input.Body))
	if err == ErrNotFound {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": note})
}
func (h *Handler) deleteNote(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.authorized(w, r, true)
	if !ok {
		return
	}
	err := h.store.DeleteNote(r.Context(), session.User.ID, video.ID, r.PathValue("note_id"))
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
func invalidSeconds(value float64) bool {
	return value < 0 || math.IsNaN(value) || math.IsInf(value, 0)
}
func validNote(input noteInput) bool {
	length := len([]rune(strings.TrimSpace(input.Body)))
	return !invalidSeconds(input.TimestampSeconds) && length >= 1 && length <= 5000
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
func badRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
}
func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
