package courses

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/authorization"
	"github.com/kyransciberras/bjj-streaming/internal/videos"
)

type Handler struct {
	store  *Store
	videos *videos.Store
	auth   *auth.Handler
	policy authorization.Policy
}

func NewHandler(store *Store, videoStore *videos.Store, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, videos: videoStore, auth: authHandler}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/courses", h.list)
	mux.HandleFunc("POST /api/courses", h.create)
	mux.HandleFunc("GET /api/courses/{id}", h.get)
}

type courseVideo struct {
	videos.Video
	SequenceNumber    int     `json:"sequence_number"`
	CourseChapterName *string `json:"course_chapter_name,omitempty"`
}

type courseResponse struct {
	Course
	Videos []courseVideo `json:"videos"`
}

func actor(session auth.Session) authorization.Actor {
	return authorization.Actor{ID: session.User.ID, Role: authorization.Role(session.User.Role)}
}

func policyVideo(video videos.Video) authorization.Video {
	return authorization.Video{UploaderID: video.UploadedByUserID, Visibility: authorization.Visibility(video.Visibility), Ready: video.Status == "ready"}
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request, csrf bool) (auth.Session, bool) {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok || (csrf && !h.auth.RequireCSRF(w, r, session)) {
		return auth.Session{}, false
	}
	return session, true
}

func (h *Handler) visibleCourse(r *http.Request, session auth.Session, course Course) (courseResponse, bool, error) {
	members, err := h.store.Memberships(r.Context(), course.ID)
	if err != nil {
		return courseResponse{}, false, err
	}
	result := courseResponse{Course: course, Videos: []courseVideo{}}
	for _, member := range members {
		video, getErr := h.videos.Get(r.Context(), member.VideoID)
		if getErr != nil {
			continue
		}
		if h.videos.CanView(r.Context(), video, session.User.ID, session.User.Role, session.User.OrganizationID, session.User.IsPlatformOwner) {
			if video.ThumbnailReady && video.ThumbnailObjectKey != nil {
				video.ThumbnailURL = "/api/videos/" + video.ID + "/thumbnail"
			}
			result.Videos = append(result.Videos, courseVideo{Video: video, SequenceNumber: member.Sequence, CourseChapterName: member.ChapterTitle})
		}
	}
	canManage := session.User.IsPlatformOwner || (session.User.OrganizationID != nil && course.OrganizationID == *session.User.OrganizationID && (session.User.Role == "admin" || (session.User.Role == "instructor" && course.CreatedByUserID == session.User.ID)))
	return result, canManage || len(result.Videos) > 0, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	values, err := h.store.List(r.Context(), session.User.OrganizationID, session.User.IsPlatformOwner)
	if err != nil {
		serverError(w)
		return
	}
	result := []map[string]any{}
	for _, course := range values {
		visible, allowed, visibleErr := h.visibleCourse(r, session, course)
		if visibleErr != nil {
			serverError(w)
			return
		}
		if allowed {
			result = append(result, map[string]any{
				"id": course.ID, "created_by_user_id": course.CreatedByUserID, "title": course.Title,
				"instructor_name": course.InstructorName, "organization_id": course.OrganizationID, "video_count": len(visible.Videos),
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"courses": result})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, false)
	if !ok {
		return
	}
	course, err := h.store.Get(r.Context(), r.PathValue("id"))
	if err != nil || !h.store.Available(r.Context(), course.ID, session.User.OrganizationID, session.User.IsPlatformOwner) {
		notFound(w)
		return
	}
	result, allowed, err := h.visibleCourse(r, session, course)
	if err != nil {
		serverError(w)
		return
	}
	if !allowed {
		notFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"course": result})
}

type createInput struct {
	Title          string `json:"title"`
	InstructorName string `json:"instructor_name"`
	Videos         []struct {
		VideoID     string  `json:"video_id"`
		ChapterName *string `json:"chapter_name"`
	} `json:"videos"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	if session.User.Role != "admin" && session.User.Role != "instructor" {
		notFound(w)
		return
	}
	var input createInput
	if decode(r, &input) != nil {
		badRequest(w)
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.InstructorName = strings.TrimSpace(input.InstructorName)
	if len(input.Title) == 0 || len(input.Title) > 200 || len(input.InstructorName) == 0 || len(input.InstructorName) > 200 || len(input.Videos) == 0 || len(input.Videos) > 500 {
		badRequest(w)
		return
	}
	seen := map[string]bool{}
	members := make([]Membership, 0, len(input.Videos))
	for index, item := range input.Videos {
		if item.VideoID == "" || seen[item.VideoID] {
			badRequest(w)
			return
		}
		seen[item.VideoID] = true
		video, err := h.videos.Get(r.Context(), item.VideoID)
		if err != nil || video.Status != "ready" || !h.videos.CanManage(video, session.User.ID, session.User.Role, session.User.OrganizationID, session.User.IsPlatformOwner) {
			notFound(w)
			return
		}
		if item.ChapterName != nil {
			value := strings.TrimSpace(*item.ChapterName)
			if len(value) == 0 || len(value) > 200 {
				badRequest(w)
				return
			}
			item.ChapterName = &value
		}
		members = append(members, Membership{VideoID: item.VideoID, Sequence: index + 1, ChapterTitle: item.ChapterName})
	}
	course, err := h.store.Create(r.Context(), session.User.ID, input.Title, input.InstructorName, members)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"course": course})
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
