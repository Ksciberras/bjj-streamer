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

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

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
