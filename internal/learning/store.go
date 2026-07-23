package learning

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Progress struct {
	PositionSeconds float64   `json:"position_seconds"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Note struct {
	ID               string    `json:"id"`
	TimestampSeconds float64   `json:"timestamp_seconds"`
	Body             string    `json:"body"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type StudyNote struct {
	Note
	VideoID        string `json:"video_id"`
	VideoTitle     string `json:"video_title"`
	InstructorName string `json:"instructor_name"`
}

type AnalyticsOverview struct {
	ActiveLearners int `json:"active_learners"`
	VideosStarted  int `json:"videos_started"`
	Resumes        int `json:"resumes"`
	NotesCreated   int `json:"notes_created"`
}

type ContentAnalytics struct {
	VideoID     string `json:"video_id"`
	Title       string `json:"title"`
	Instructor  string `json:"instructor_name"`
	Viewers     int    `json:"unique_viewers"`
	Starts      int    `json:"starts"`
	Resumes     int    `json:"resumes"`
	Completions int    `json:"completions"`
	Notes       int    `json:"notes"`
}

type MemberAnalytics struct {
	UserID        string     `json:"user_id"`
	Email         string     `json:"email"`
	LastActiveAt  *time.Time `json:"last_active_at"`
	VideosStarted int        `json:"videos_started"`
	Notes         int        `json:"notes"`
}

type AnalyticsResult struct {
	Days     int                `json:"days"`
	Overview AnalyticsOverview  `json:"overview"`
	Content  []ContentAnalytics `json:"content"`
	Members  []MemberAnalytics  `json:"members"`
}

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) RecordEvent(ctx context.Context, userID, videoID, eventType string, position float64) error {
	_, err := s.db.Exec(ctx, `INSERT INTO learning_events(organization_id,user_id,video_id,event_type,position_seconds)
		SELECT COALESCE(u.organization_id,v.organization_id),u.id,v.id,$3,$4
		FROM users u CROSS JOIN videos v WHERE u.id=$1 AND v.id=$2
		ON CONFLICT(user_id,video_id,event_type,occurred_on)
		DO UPDATE SET position_seconds=GREATEST(learning_events.position_seconds,EXCLUDED.position_seconds),occurred_at=CURRENT_TIMESTAMP`,
		userID, videoID, eventType, position)
	return err
}

func (s *Store) Analytics(ctx context.Context, organizationID *string, platformOwner bool, days int) (AnalyticsResult, error) {
	result := AnalyticsResult{Days: days, Content: []ContentAnalytics{}, Members: []MemberAnalytics{}}
	since := time.Now().AddDate(0, 0, -days)
	err := s.db.QueryRow(ctx, `SELECT
		COUNT(DISTINCT le.user_id),
		COUNT(DISTINCT (le.user_id,le.video_id)) FILTER (WHERE le.event_type IN ('started','resumed')),
		COUNT(*) FILTER (WHERE le.event_type='resumed'),
		(SELECT COUNT(*) FROM notes n JOIN users nu ON nu.id=n.user_id WHERE n.created_at >= $3 AND ($2 OR nu.organization_id=$1))
		FROM learning_events le
		WHERE le.occurred_at >= $3 AND ($2 OR le.organization_id=$1)`,
		organizationID, platformOwner, since).Scan(
		&result.Overview.ActiveLearners, &result.Overview.VideosStarted,
		&result.Overview.Resumes, &result.Overview.NotesCreated)
	if err != nil {
		return AnalyticsResult{}, err
	}
	rows, err := s.db.Query(ctx, `SELECT v.id,v.title,v.instructor_name,
		COUNT(DISTINCT le.user_id),
		COUNT(*) FILTER (WHERE le.event_type='started'),
		COUNT(*) FILTER (WHERE le.event_type='resumed'),
		COUNT(*) FILTER (WHERE le.event_type='completed'),
		(SELECT COUNT(*) FROM notes n JOIN users nu ON nu.id=n.user_id WHERE n.video_id=v.id AND n.created_at >= $3 AND ($2 OR nu.organization_id=$1))
		FROM videos v
		LEFT JOIN learning_events le ON le.video_id=v.id AND le.occurred_at >= $3 AND ($2 OR le.organization_id=$1)
		WHERE v.status='ready' AND ($2 OR EXISTS(SELECT 1 FROM video_organizations vo WHERE vo.video_id=v.id AND vo.organization_id=$1))
		GROUP BY v.id ORDER BY COUNT(DISTINCT le.user_id) DESC,v.title LIMIT 50`,
		organizationID, platformOwner, since)
	if err != nil {
		return AnalyticsResult{}, err
	}
	for rows.Next() {
		var item ContentAnalytics
		if err = rows.Scan(&item.VideoID, &item.Title, &item.Instructor, &item.Viewers, &item.Starts, &item.Resumes, &item.Completions, &item.Notes); err != nil {
			rows.Close()
			return AnalyticsResult{}, err
		}
		result.Content = append(result.Content, item)
	}
	rows.Close()
	rows, err = s.db.Query(ctx, `SELECT u.id,u.email,MAX(le.occurred_at),
		COUNT(DISTINCT le.video_id) FILTER (WHERE le.event_type IN ('started','resumed')),
		(SELECT COUNT(*) FROM notes n WHERE n.user_id=u.id AND n.created_at >= $3)
		FROM users u LEFT JOIN learning_events le ON le.user_id=u.id AND le.occurred_at >= $3
		WHERE u.disabled_at IS NULL AND NOT u.is_platform_owner AND ($2 OR u.organization_id=$1)
		GROUP BY u.id ORDER BY MAX(le.occurred_at) DESC NULLS LAST,u.email`,
		organizationID, platformOwner, since)
	if err != nil {
		return AnalyticsResult{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item MemberAnalytics
		if err = rows.Scan(&item.UserID, &item.Email, &item.LastActiveAt, &item.VideosStarted, &item.Notes); err != nil {
			return AnalyticsResult{}, err
		}
		result.Members = append(result.Members, item)
	}
	return result, rows.Err()
}

func (s *Store) GetProgress(ctx context.Context, userID, videoID string) (Progress, error) {
	var value Progress
	err := s.db.QueryRow(ctx, `SELECT position_seconds,updated_at FROM playback_progress WHERE user_id=$1 AND video_id=$2`, userID, videoID).Scan(&value.PositionSeconds, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Progress{PositionSeconds: 0}, nil
	}
	return value, err
}

func (s *Store) PutProgress(ctx context.Context, userID, videoID string, position float64) (Progress, error) {
	var value Progress
	err := s.db.QueryRow(ctx, `INSERT INTO playback_progress(user_id,video_id,position_seconds)VALUES($1,$2,$3) ON CONFLICT(user_id,video_id)DO UPDATE SET position_seconds=EXCLUDED.position_seconds,updated_at=CURRENT_TIMESTAMP RETURNING position_seconds,updated_at`, userID, videoID, position).Scan(&value.PositionSeconds, &value.UpdatedAt)
	return value, err
}

func (s *Store) ListNotes(ctx context.Context, userID, videoID string) ([]Note, error) {
	rows, err := s.db.Query(ctx, `SELECT id,timestamp_seconds,body,created_at,updated_at FROM notes WHERE user_id=$1 AND video_id=$2 ORDER BY timestamp_seconds,created_at,id`, userID, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Note{}
	for rows.Next() {
		var note Note
		if err = rows.Scan(&note.ID, &note.TimestampSeconds, &note.Body, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (s *Store) CreateNote(ctx context.Context, userID, videoID string, timestamp float64, body string) (Note, error) {
	var note Note
	err := s.db.QueryRow(ctx, `INSERT INTO notes(user_id,video_id,timestamp_seconds,body)VALUES($1,$2,$3,$4)RETURNING id,timestamp_seconds,body,created_at,updated_at`, userID, videoID, timestamp, body).Scan(&note.ID, &note.TimestampSeconds, &note.Body, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}

func (s *Store) UpdateNote(ctx context.Context, userID, videoID, noteID string, timestamp float64, body string) (Note, error) {
	var note Note
	err := s.db.QueryRow(ctx, `UPDATE notes SET timestamp_seconds=$4,body=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND user_id=$2 AND video_id=$3 RETURNING id,timestamp_seconds,body,created_at,updated_at`, noteID, userID, videoID, timestamp, body).Scan(&note.ID, &note.TimestampSeconds, &note.Body, &note.CreatedAt, &note.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return note, err
}

func (s *Store) DeleteNote(ctx context.Context, userID, videoID, noteID string) error {
	result, err := s.db.Exec(ctx, `DELETE FROM notes WHERE id=$1 AND user_id=$2 AND video_id=$3`, noteID, userID, videoID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListStudyNotes(ctx context.Context, userID string) ([]StudyNote, error) {
	rows, err := s.db.Query(ctx, `SELECT n.id,n.timestamp_seconds,n.body,n.created_at,n.updated_at,v.id,v.title,v.instructor_name
		FROM notes n JOIN videos v ON v.id=n.video_id
		WHERE n.user_id=$1 ORDER BY n.updated_at DESC,n.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StudyNote{}
	for rows.Next() {
		var note StudyNote
		if err = rows.Scan(&note.ID, &note.TimestampSeconds, &note.Body, &note.CreatedAt, &note.UpdatedAt, &note.VideoID, &note.VideoTitle, &note.InstructorName); err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (s *Store) ListWatchLaterIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT video_id FROM watch_later WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (s *Store) AddWatchLater(ctx context.Context, userID, videoID string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO watch_later(user_id,video_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, videoID)
	return err
}

func (s *Store) RemoveWatchLater(ctx context.Context, userID, videoID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM watch_later WHERE user_id=$1 AND video_id=$2`, userID, videoID)
	return err
}
