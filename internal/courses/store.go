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
	OrganizationID  string    `json:"organization_id"`
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

func (s *Store) List(ctx context.Context, organizationID *string, platformOwner bool) ([]Course, error) {
	rows, err := s.db.Query(ctx, `SELECT id,created_by_user_id,organization_id,title,instructor_name,created_at,updated_at FROM courses c WHERE $2 OR EXISTS(SELECT 1 FROM course_organizations co WHERE co.course_id=c.id AND co.organization_id=$1) ORDER BY updated_at DESC,id`, organizationID, platformOwner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Course{}
	for rows.Next() {
		var course Course
		if err := rows.Scan(&course.ID, &course.CreatedByUserID, &course.OrganizationID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, course)
	}
	return result, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (Course, error) {
	var course Course
	err := s.db.QueryRow(ctx, `SELECT id,created_by_user_id,organization_id,title,instructor_name,created_at,updated_at FROM courses WHERE id=$1`, id).
		Scan(&course.ID, &course.CreatedByUserID, &course.OrganizationID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Course{}, ErrNotFound
	}
	return course, err
}

func (s *Store) Available(ctx context.Context, courseID string, organizationID *string, platformOwner bool) bool {
	if platformOwner {
		return true
	}
	if organizationID == nil {
		return false
	}
	var available bool
	return s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM course_organizations WHERE course_id=$1 AND organization_id=$2)`, courseID, organizationID).Scan(&available) == nil && available
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
	err = tx.QueryRow(ctx, `INSERT INTO courses(created_by_user_id,organization_id,title,instructor_name) VALUES($1,COALESCE((SELECT organization_id FROM users WHERE id=$1),(SELECT id FROM organizations ORDER BY created_at LIMIT 1)),$2,$3) RETURNING id,created_by_user_id,organization_id,title,instructor_name,created_at,updated_at`, actorID, title, instructor).
		Scan(&course.ID, &course.CreatedByUserID, &course.OrganizationID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt)
	if err != nil {
		return Course{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO course_organizations(course_id,organization_id) VALUES($1,$2)`, course.ID, course.OrganizationID); err != nil {
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

func (s *Store) Update(ctx context.Context, id, title, instructor string, members []Membership) (Course, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Course{}, err
	}
	defer tx.Rollback(ctx)

	var course Course
	err = tx.QueryRow(ctx, `UPDATE courses SET title=$2,instructor_name=$3,updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING id,created_by_user_id,organization_id,title,instructor_name,created_at,updated_at`, id, title, instructor).
		Scan(&course.ID, &course.CreatedByUserID, &course.OrganizationID, &course.Title, &course.InstructorName, &course.CreatedAt, &course.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Course{}, ErrNotFound
	}
	if err != nil {
		return Course{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM course_videos WHERE course_id=$1`, id); err != nil {
		return Course{}, err
	}
	for _, member := range members {
		if _, err = tx.Exec(ctx, `INSERT INTO course_videos(course_id,video_id,sequence_number,chapter_title) VALUES($1,$2,$3,$4)`, id, member.VideoID, member.Sequence, member.ChapterTitle); err != nil {
			return Course{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Course{}, err
	}
	return course, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	result, err := s.db.Exec(ctx, `DELETE FROM courses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
