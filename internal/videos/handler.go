package videos

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/authorization"
	"github.com/kyransciberras/bjj-streaming/internal/objectstorage"
)

const maxUploadSize int64 = 5 * 1024 * 1024 * 1024
const maxThumbnailSize int64 = 5 * 1024 * 1024

type Objects interface {
	PresignGet(ctx context.Context, key string) (string, error)
	PresignPut(ctx context.Context, key, contentType string, size int64) (string, error)
	Head(ctx context.Context, key string) (objectstorage.Object, error)
}

type Handler struct {
	store   *Store
	objects Objects
	auth    *auth.Handler
	policy  authorization.Policy
}

func NewHandler(store *Store, objects Objects, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, objects: objects, auth: authHandler}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/videos", h.list)
	mux.HandleFunc("POST /api/videos/upload-requests", h.createUpload)
	mux.HandleFunc("POST /api/videos/{id}/complete", h.complete)
	mux.HandleFunc("PATCH /api/videos/{id}", h.update)
	mux.HandleFunc("POST /api/videos/{id}/thumbnail-upload-request", h.createThumbnailUpload)
	mux.HandleFunc("POST /api/videos/{id}/thumbnail-complete", h.completeThumbnail)
	mux.HandleFunc("GET /api/videos/{id}/thumbnail", h.thumbnail)
}

func actor(session auth.Session) authorization.Actor {
	return authorization.Actor{ID: session.User.ID, Role: authorization.Role(session.User.Role)}
}
func policyVideo(video Video) authorization.Video {
	return authorization.Video{UploaderID: video.UploadedByUserID, Visibility: authorization.Visibility(video.Visibility), Ready: video.Status == "ready"}
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

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	values, err := h.store.List(r.Context(), session.User.ID, session.User.Role, strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		serverError(w)
		return
	}
	for index := range values {
		setThumbnailURL(&values[index])
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": values})
}

func setThumbnailURL(video *Video) {
	if video.ThumbnailObjectKey != nil {
		video.ThumbnailURL = "/api/videos/" + video.ID + "/thumbnail"
	}
}

type metadataInput struct {
	Title             string   `json:"title"`
	InstructorName    string   `json:"instructor_name"`
	InstructionalName *string  `json:"instructional_name"`
	ChapterName       *string  `json:"chapter_name"`
	Description       string   `json:"description"`
	Tags              []string `json:"tags"`
	Visibility        string   `json:"visibility"`
	ContentBasis      string   `json:"content_basis"`
}
type uploadInput struct {
	metadataInput
	Filename string `json:"filename"`
	MIMEType string `json:"mime_type"`
	ByteSize int64  `json:"byte_size"`
}

func (h *Handler) createUpload(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	if !h.policy.CreateVideo(actor(session)) {
		notFound(w)
		return
	}
	var input uploadInput
	if decode(r, &input) != nil || !validMetadata(input.metadataInput) || input.MIMEType != "video/mp4" || input.ByteSize <= 0 || input.ByteSize > maxUploadSize || !strings.EqualFold(filepath.Ext(input.Filename), ".mp4") || filepath.Base(input.Filename) != input.Filename || len(input.Filename) > 255 {
		badRequest(w)
		return
	}
	key, err := objectKey()
	if err != nil {
		serverError(w)
		return
	}
	url, err := h.objects.PresignPut(r.Context(), key, input.MIMEType, input.ByteSize)
	if err != nil {
		serverError(w)
		return
	}
	created, err := h.store.Create(r.Context(), session.User.ID, storeInput(input.metadataInput, key, input.Filename, input.MIMEType, input.ByteSize), r.Header.Get("X-Request-ID"))
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"video": created, "upload_url": url})
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	video, err := h.store.Get(r.Context(), r.PathValue("id"))
	if err != nil || !h.policy.ManageVideo(actor(session), policyVideo(video)) {
		notFound(w)
		return
	}
	if video.Status != "pending_upload" {
		badRequest(w)
		return
	}
	object, err := h.objects.Head(r.Context(), video.ObjectKey)
	if err != nil || object.Size != video.ByteSize || object.ContentType != video.MIMEType {
		badRequest(w)
		return
	}
	video, err = h.store.MarkReady(r.Context(), session.User.ID, video.ID, r.Header.Get("X-Request-ID"))
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": video})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	video, err := h.store.Get(r.Context(), r.PathValue("id"))
	if err != nil || !h.policy.ManageVideo(actor(session), policyVideo(video)) {
		notFound(w)
		return
	}
	var input struct {
		metadataInput
		Archived bool `json:"archived"`
	}
	if decode(r, &input) != nil || !validMetadata(input.metadataInput) {
		badRequest(w)
		return
	}
	updated, err := h.store.Update(r.Context(), session.User.ID, video.ID, r.Header.Get("X-Request-ID"), storeInput(input.metadataInput, video.ObjectKey, video.OriginalFilename, video.MIMEType, video.ByteSize), input.Archived)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": updated})
}

type thumbnailInput struct {
	Filename string `json:"filename"`
	MIMEType string `json:"mime_type"`
	ByteSize int64  `json:"byte_size"`
}

var thumbnailExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func (h *Handler) createThumbnailUpload(w http.ResponseWriter, r *http.Request) {
	session, video, ok := h.managedVideo(w, r)
	if !ok {
		return
	}
	var input thumbnailInput
	if decode(r, &input) != nil {
		badRequest(w)
		return
	}
	extension, valid := thumbnailExtensions[input.MIMEType]
	fileExtension := strings.ToLower(filepath.Ext(input.Filename))
	if !valid || input.ByteSize <= 0 || input.ByteSize > maxThumbnailSize ||
		!validThumbnailExtension(input.MIMEType, fileExtension) ||
		filepath.Base(input.Filename) != input.Filename || len(input.Filename) > 255 {
		badRequest(w)
		return
	}
	key, err := generatedObjectKey("thumbnails/", extension)
	if err != nil {
		serverError(w)
		return
	}
	url, err := h.objects.PresignPut(r.Context(), key, input.MIMEType, input.ByteSize)
	if err != nil {
		serverError(w)
		return
	}
	video, err = h.store.SetThumbnail(r.Context(), session.User.ID, video.ID, key, r.Header.Get("X-Request-ID"))
	if err != nil {
		serverError(w)
		return
	}
	setThumbnailURL(&video)
	writeJSON(w, http.StatusCreated, map[string]any{"video": video, "upload_url": url})
}

func (h *Handler) completeThumbnail(w http.ResponseWriter, r *http.Request) {
	_, video, ok := h.managedVideo(w, r)
	if !ok {
		return
	}
	if video.ThumbnailObjectKey == nil {
		badRequest(w)
		return
	}
	object, err := h.objects.Head(r.Context(), *video.ThumbnailObjectKey)
	if err != nil || object.Size <= 0 || object.Size > maxThumbnailSize {
		badRequest(w)
		return
	}
	if _, valid := thumbnailExtensions[object.ContentType]; !valid {
		badRequest(w)
		return
	}
	setThumbnailURL(&video)
	writeJSON(w, http.StatusOK, map[string]any{"video": video})
}

func (h *Handler) thumbnail(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	video, err := h.store.Get(r.Context(), r.PathValue("id"))
	if err != nil || video.ThumbnailObjectKey == nil || !h.policy.ViewVideo(actor(session), policyVideo(video)) {
		notFound(w)
		return
	}
	url, err := h.objects.PresignGet(r.Context(), *video.ThumbnailObjectKey)
	if err != nil {
		serverError(w)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) managedVideo(w http.ResponseWriter, r *http.Request) (auth.Session, Video, bool) {
	session, ok := h.session(w, r, true)
	if !ok {
		return auth.Session{}, Video{}, false
	}
	video, err := h.store.Get(r.Context(), r.PathValue("id"))
	if err != nil || !h.policy.ManageVideo(actor(session), policyVideo(video)) {
		notFound(w)
		return auth.Session{}, Video{}, false
	}
	return session, video, true
}

func validThumbnailExtension(mime, extension string) bool {
	if mime == "image/jpeg" {
		return extension == ".jpg" || extension == ".jpeg"
	}
	return thumbnailExtensions[mime] == extension
}

func validMetadata(input metadataInput) bool {
	if length(input.Title, 1, 200) == false || length(input.InstructorName, 1, 200) == false || len(input.Description) > 10000 || (input.Visibility != "shared" && input.Visibility != "private") || (input.ContentBasis != "self_created" && input.ContentBasis != "licensed_for_group" && input.ContentBasis != "personal_purchase") || (input.ContentBasis == "personal_purchase" && input.Visibility != "private") || len(input.Tags) > 30 {
		return false
	}
	for _, optional := range []*string{input.InstructionalName, input.ChapterName} {
		if optional != nil && !length(*optional, 1, 200) {
			return false
		}
	}
	for _, tag := range input.Tags {
		if !length(tag, 1, 50) {
			return false
		}
	}
	return true
}
func length(value string, min, max int) bool {
	n := len([]rune(strings.TrimSpace(value)))
	return n >= min && n <= max
}
func storeInput(input metadataInput, key, filename, mime string, size int64) CreateInput {
	return CreateInput{Title: strings.TrimSpace(input.Title), InstructorName: strings.TrimSpace(input.InstructorName), InstructionalName: trimmed(input.InstructionalName), ChapterName: trimmed(input.ChapterName), Description: strings.TrimSpace(input.Description), Tags: cleanTags(input.Tags), Visibility: input.Visibility, ContentBasis: input.ContentBasis, ObjectKey: key, OriginalFilename: filename, MIMEType: mime, ByteSize: size}
}
func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	if result == "" {
		return nil
	}
	return &result
}
func cleanTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		value := strings.ToLower(strings.TrimSpace(tag))
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
func objectKey() (string, error) {
	return generatedObjectKey("videos/", ".mp4")
}

func generatedObjectKey(prefix, extension string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw) + extension, nil
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
