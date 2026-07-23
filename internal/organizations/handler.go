package organizations

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/auth"
)

type Handler struct {
	db   *pgxpool.Pool
	auth *auth.Handler
}

func NewHandler(db *pgxpool.Pool, authHandler *auth.Handler) *Handler {
	return &Handler{db: db, auth: authHandler}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/platform/organizations", h.list)
	mux.HandleFunc("POST /api/platform/organizations", h.create)
	mux.HandleFunc("GET /api/platform/availability", h.listAvailability)
	mux.HandleFunc("PUT /api/platform/videos/{id}/organizations/{organization_id}", h.putVideo)
	mux.HandleFunc("DELETE /api/platform/videos/{id}/organizations/{organization_id}", h.deleteVideo)
	mux.HandleFunc("PUT /api/platform/courses/{id}/organizations/{organization_id}", h.putCourse)
	mux.HandleFunc("DELETE /api/platform/courses/{id}/organizations/{organization_id}", h.deleteCourse)
}

func (h *Handler) listAvailability(w http.ResponseWriter, r *http.Request) {
	if !h.owner(w, r, false) {
		return
	}
	videoRows, err := h.db.Query(r.Context(), `SELECT video_id,organization_id FROM video_organizations`)
	if err != nil {
		serverError(w)
		return
	}
	defer videoRows.Close()
	videos := []map[string]string{}
	for videoRows.Next() {
		var asset, org string
		if videoRows.Scan(&asset, &org) != nil {
			serverError(w)
			return
		}
		videos = append(videos, map[string]string{"asset_id": asset, "organization_id": org})
	}
	courseRows, err := h.db.Query(r.Context(), `SELECT course_id,organization_id FROM course_organizations`)
	if err != nil {
		serverError(w)
		return
	}
	defer courseRows.Close()
	courses := []map[string]string{}
	for courseRows.Next() {
		var asset, org string
		if courseRows.Scan(&asset, &org) != nil {
			serverError(w)
			return
		}
		courses = append(courses, map[string]string{"asset_id": asset, "organization_id": org})
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": videos, "courses": courses})
}

func (h *Handler) owner(w http.ResponseWriter, r *http.Request, csrf bool) bool {
	_, session, ok := h.auth.Authenticate(w, r)
	if !ok || !session.User.IsPlatformOwner {
		if ok {
			http.NotFound(w, r)
		}
		return false
	}
	return !csrf || h.auth.RequireCSRF(w, r, session)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	if !h.owner(w, r, false) {
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT id,name,slug,created_at FROM organizations ORDER BY name,id`)
	if err != nil {
		serverError(w)
		return
	}
	defer rows.Close()
	values := []map[string]any{}
	for rows.Next() {
		var id, name, slug string
		var created any
		if err = rows.Scan(&id, &name, &slug, &created); err != nil {
			serverError(w)
			return
		}
		values = append(values, map[string]any{"id": id, "name": name, "slug": slug, "created_at": created})
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": values})
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if !h.owner(w, r, true) {
		return
	}
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if decode(r, &input) != nil {
		badRequest(w)
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(input.Slug)
	if len(input.Name) < 1 || len(input.Name) > 120 || !slugPattern.MatchString(input.Slug) {
		badRequest(w)
		return
	}
	var id string
	if err := h.db.QueryRow(r.Context(), `INSERT INTO organizations(name,slug)VALUES($1,$2)RETURNING id`, input.Name, input.Slug).Scan(&id); err != nil {
		badRequest(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}
func (h *Handler) putVideo(w http.ResponseWriter, r *http.Request) {
	h.availability(w, r, true, "video_organizations", "video_id")
}
func (h *Handler) deleteVideo(w http.ResponseWriter, r *http.Request) {
	h.availability(w, r, false, "video_organizations", "video_id")
}
func (h *Handler) putCourse(w http.ResponseWriter, r *http.Request) {
	h.availability(w, r, true, "course_organizations", "course_id")
}
func (h *Handler) deleteCourse(w http.ResponseWriter, r *http.Request) {
	h.availability(w, r, false, "course_organizations", "course_id")
}
func (h *Handler) availability(w http.ResponseWriter, r *http.Request, add bool, table, idColumn string) {
	if !h.owner(w, r, true) {
		return
	}
	var err error
	if add {
		var containsPersonalPurchase bool
		if table == "video_organizations" {
			err = h.db.QueryRow(r.Context(), `SELECT content_basis='personal_purchase' FROM videos WHERE id=$1`, r.PathValue("id")).Scan(&containsPersonalPurchase)
		} else {
			err = h.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM course_videos cv JOIN videos v ON v.id=cv.video_id WHERE cv.course_id=$1 AND v.content_basis='personal_purchase')`, r.PathValue("id")).Scan(&containsPersonalPurchase)
		}
		if err != nil || containsPersonalPurchase {
			badRequest(w)
			return
		}
		_, err = h.db.Exec(r.Context(), `INSERT INTO `+table+`(`+idColumn+`,organization_id)VALUES($1,$2)ON CONFLICT DO NOTHING`, r.PathValue("id"), r.PathValue("organization_id"))
	} else {
		_, err = h.db.Exec(r.Context(), `DELETE FROM `+table+` WHERE `+idColumn+`=$1 AND organization_id=$2`, r.PathValue("id"), r.PathValue("organization_id"))
	}
	if err != nil {
		badRequest(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decode(r *http.Request, target any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func badRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
}
func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
