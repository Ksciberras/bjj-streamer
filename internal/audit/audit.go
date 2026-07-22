package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type Event struct {
	ID          int64          `json:"id"`
	ActorUserID string         `json:"actor_user_id"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	RequestID   string         `json:"request_id"`
	Details     map[string]any `json:"details"`
	OccurredAt  time.Time      `json:"occurred_at"`
}

func Record(ctx context.Context, exec Execer, actor, action, targetType, targetID, requestID string, details map[string]any) error {
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `INSERT INTO audit_events (actor_user_id,action,target_type,target_id,request_id,details) VALUES ($1,$2,$3,$4,$5,$6)`, actor, action, targetType, targetID, requestID, encoded)
	return err
}
