package courses

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("course not found")

type Course struct {
	ID              string    `json:"id"`
	CreatedByUserID string    `json:"created_by_user_id"`
	Title           string    `json:"title"`
	InstructorName  string    `json:"instructor_name"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Membership struct {
	VideoID      string
	Sequence     int
	ChapterTitle *string
}

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) List(ctx context.Context) ([]Course, error) {
	rows, err := s.db.Query(ctx, `SELECT id,created_by_user_id,title,instructor_name,created_at,updated_at FROM courses ORDER BY updated_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Course{}
	for rows.Next() {
		var course Course
		if err := rows.Scan(&course.ID, &course.CreatedByUserID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, course)
	}
	return result, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (Course, error) {
	var course Course
	err := s.db.QueryRow(ctx, `SELECT id,created_by_user_id,title,instructor_name,created_at,updated_at FROM courses WHERE id=$1`, id).
		Scan(&course.ID, &course.CreatedByUserID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Course{}, ErrNotFound
	}
	return course, err
}

func (s *Store) Memberships(ctx context.Context, id string) ([]Membership, error) {
	rows, err := s.db.Query(ctx, `SELECT video_id,sequence_number,chapter_title FROM course_videos WHERE course_id=$1 ORDER BY sequence_number`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Membership{}
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(&membership.VideoID, &membership.Sequence, &membership.ChapterTitle); err != nil {
			return nil, err
		}
		result = append(result, membership)
	}
	return result, rows.Err()
}

func (s *Store) Create(ctx context.Context, actorID, title, instructor string, members []Membership) (Course, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Course{}, err
	}
	defer tx.Rollback(ctx)
	var course Course
	err = tx.QueryRow(ctx, `INSERT INTO courses(created_by_user_id,title,instructor_name) VALUES($1,$2,$3) RETURNING id,created_by_user_id,title,instructor_name,created_at,updated_at`, actorID, title, instructor).
		Scan(&course.ID, &course.CreatedByUserID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt)
	if err != nil {
		return Course{}, err
	}
	for _, member := range members {
		if _, err = tx.Exec(ctx, `INSERT INTO course_videos(course_id,video_id,sequence_number,chapter_title) VALUES($1,$2,$3,$4)`, course.ID, member.VideoID, member.Sequence, member.ChapterTitle); err != nil {
			return Course{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Course{}, err
	}
	return course, nil
}
