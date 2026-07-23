package videos

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/audit"
)

var ErrNotFound = errors.New("not found")

type Video struct {
	ID                 string    `json:"id"`
	UploadedByUserID   string    `json:"uploaded_by_user_id"`
	Title              string    `json:"title"`
	InstructorName     string    `json:"instructor_name"`
	InstructionalName  *string   `json:"instructional_name,omitempty"`
	ChapterName        *string   `json:"chapter_name,omitempty"`
	Description        string    `json:"description"`
	Tags               []string  `json:"tags"`
	Visibility         string    `json:"visibility"`
	ContentBasis       string    `json:"content_basis"`
	ObjectKey          string    `json:"-"`
	ThumbnailObjectKey *string   `json:"-"`
	ThumbnailReady     bool      `json:"-"`
	ThumbnailURL       string    `json:"thumbnail_url,omitempty"`
	OriginalFilename   string    `json:"original_filename"`
	MIMEType           string    `json:"mime_type"`
	ByteSize           int64     `json:"byte_size"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateInput struct {
	Title, InstructorName, Description, Visibility, ContentBasis string
	InstructionalName, ChapterName                               *string
	Tags                                                         []string
	ObjectKey, OriginalFilename, MIMEType                        string
	ByteSize                                                     int64
}

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

const columns = `id,uploaded_by_user_id,title,instructor_name,instructional_name,chapter_name,description,tags,visibility,content_basis,object_key,thumbnail_object_key,thumbnail_ready,original_filename,mime_type,byte_size,status,created_at,updated_at`

func scan(row pgx.Row) (Video, error) {
	var video Video
	err := row.Scan(&video.ID, &video.UploadedByUserID, &video.Title, &video.InstructorName, &video.InstructionalName, &video.ChapterName, &video.Description, &video.Tags, &video.Visibility, &video.ContentBasis, &video.ObjectKey, &video.ThumbnailObjectKey, &video.ThumbnailReady, &video.OriginalFilename, &video.MIMEType, &video.ByteSize, &video.Status, &video.CreatedAt, &video.UpdatedAt)
	return video, err
}

func (s *Store) SetThumbnail(ctx context.Context, actorID, id, key, requestID string) (Video, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Video{}, err
	}
	defer tx.Rollback(ctx)
	video, err := scan(tx.QueryRow(ctx, `UPDATE videos SET thumbnail_object_key=$2,thumbnail_ready=FALSE,updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING `+columns, id, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	if err != nil {
		return Video{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "video.thumbnail_requested", "video", id, requestID, map[string]any{}); err != nil {
		return Video{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Video{}, err
	}
	return video, nil
}

func (s *Store) MarkThumbnailReady(ctx context.Context, actorID, id, requestID string) (Video, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Video{}, err
	}
	defer tx.Rollback(ctx)
	video, err := scan(tx.QueryRow(ctx, `UPDATE videos SET thumbnail_ready=TRUE,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND thumbnail_object_key IS NOT NULL RETURNING `+columns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	if err != nil {
		return Video{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "video.thumbnail_completed", "video", id, requestID, map[string]any{}); err != nil {
		return Video{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Video{}, err
	}
	return video, nil
}

func (s *Store) Create(ctx context.Context, actorID string, input CreateInput, requestID string) (Video, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Video{}, err
	}
	defer tx.Rollback(ctx)
	video, err := scan(tx.QueryRow(ctx, `INSERT INTO videos(uploaded_by_user_id,title,instructor_name,instructional_name,chapter_name,description,tags,visibility,content_basis,object_key,original_filename,mime_type,byte_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+columns, actorID, input.Title, input.InstructorName, input.InstructionalName, input.ChapterName, input.Description, input.Tags, input.Visibility, input.ContentBasis, input.ObjectKey, input.OriginalFilename, input.MIMEType, input.ByteSize))
	if err != nil {
		return Video{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "video.upload_requested", "video", video.ID, requestID, map[string]any{"visibility": video.Visibility, "content_basis": video.ContentBasis, "byte_size": video.ByteSize}); err != nil {
		return Video{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Video{}, err
	}
	return video, nil
}

func (s *Store) Get(ctx context.Context, id string) (Video, error) {
	video, err := scan(s.db.QueryRow(ctx, `SELECT `+columns+` FROM videos WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	return video, err
}

func (s *Store) List(ctx context.Context, userID, role, query string) ([]Video, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(ctx, `SELECT `+columns+` FROM videos WHERE status='ready' AND (visibility='shared' OR uploaded_by_user_id=$1 OR $2='admin') AND ($3='' OR title ILIKE $4 OR instructor_name ILIKE $4 OR COALESCE(instructional_name,'') ILIKE $4 OR array_to_string(tags,',') ILIKE $4) ORDER BY created_at DESC,id`, userID, role, query, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Video{}
	for rows.Next() {
		video, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, video)
	}
	return result, rows.Err()
}

func (s *Store) MarkReady(ctx context.Context, actorID, id, requestID string) (Video, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Video{}, err
	}
	defer tx.Rollback(ctx)
	video, err := scan(tx.QueryRow(ctx, `UPDATE videos SET status='ready',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status='pending_upload' RETURNING `+columns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	if err != nil {
		return Video{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "video.upload_completed", "video", id, requestID, map[string]any{}); err != nil {
		return Video{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Video{}, err
	}
	return video, nil
}

func (s *Store) Update(ctx context.Context, actorID, id, requestID string, input CreateInput, archived bool) (Video, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Video{}, err
	}
	defer tx.Rollback(ctx)
	status := "ready"
	if archived {
		status = "archived"
	}
	video, err := scan(tx.QueryRow(ctx, `UPDATE videos SET title=$2,instructor_name=$3,instructional_name=$4,chapter_name=$5,description=$6,tags=$7,visibility=$8,content_basis=$9,status=$10,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status IN ('ready','archived') RETURNING `+columns, id, input.Title, input.InstructorName, input.InstructionalName, input.ChapterName, input.Description, input.Tags, input.Visibility, input.ContentBasis, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrNotFound
	}
	if err != nil {
		return Video{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "video.updated", "video", id, requestID, map[string]any{"visibility": video.Visibility, "content_basis": video.ContentBasis, "status": video.Status}); err != nil {
		return Video{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Video{}, err
	}
	return video, nil
}
