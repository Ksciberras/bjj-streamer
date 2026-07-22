package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }
func (s *Store) List(ctx context.Context, limit int) ([]Event, error) {
	rows, err := s.db.Query(ctx, `SELECT id,actor_user_id,action,target_type,target_id,request_id,details,occurred_at FROM audit_events ORDER BY occurred_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var event Event
		var details []byte
		if err = rows.Scan(&event.ID, &event.ActorUserID, &event.Action, &event.TargetType, &event.TargetID, &event.RequestID, &details, &event.OccurredAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(details, &event.Details); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
