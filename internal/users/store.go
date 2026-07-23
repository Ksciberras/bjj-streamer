package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/audit"
)

var ErrNotFound = errors.New("not found")
var ErrLastAdmin = errors.New("cannot remove final enabled administrator")
var ErrConflict = errors.New("conflict")

type User struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	Role            string    `json:"role"`
	OrganizationID  *string   `json:"organization_id,omitempty"`
	IsPlatformOwner bool      `json:"is_platform_owner"`
	Disabled        bool      `json:"disabled"`
	CreatedAt       time.Time `json:"created_at"`
}
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) Create(ctx context.Context, actorID, email, role, passwordHash, requestID string) (User, error) {
	return s.CreateInOrganization(ctx, actorID, email, role, passwordHash, nil, requestID)
}

func (s *Store) CreateInOrganization(ctx context.Context, actorID, email, role, passwordHash string, organizationID *string, requestID string) (User, error) {
	if role != "admin" && role != "instructor" && role != "student" {
		return User{}, ErrConflict
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var user User
	err = tx.QueryRow(ctx, `INSERT INTO users(email,password_hash,role,organization_id)VALUES($1,$2,$3,COALESCE($5,(SELECT organization_id FROM users WHERE id=$4)))RETURNING id,email,role,organization_id,is_platform_owner,disabled_at IS NOT NULL,created_at`, strings.ToLower(strings.TrimSpace(email)), passwordHash, role, actorID, organizationID).Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.IsPlatformOwner, &user.Disabled, &user.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "user.created", "user", user.ID, requestID, map[string]any{"email": user.Email, "role": user.Role}); err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) ResetPassword(ctx context.Context, actorID, targetID, passwordHash, requestID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, targetID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND revoked_at IS NULL`, targetID); err != nil {
		return err
	}
	if err = audit.Record(ctx, tx, actorID, "user.password_reset", "user", targetID, requestID, map[string]any{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, `SELECT id,email,role,organization_id,is_platform_owner,disabled_at IS NOT NULL,created_at FROM users ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []User{}
	for rows.Next() {
		var user User
		if err = rows.Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.IsPlatformOwner, &user.Disabled, &user.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (s *Store) ListFor(ctx context.Context, actorID string) ([]User, error) {
	rows, err := s.db.Query(ctx, `SELECT u.id,u.email,u.role,u.organization_id,u.is_platform_owner,u.disabled_at IS NOT NULL,u.created_at FROM users u JOIN users a ON a.id=$1 WHERE a.is_platform_owner OR (NOT u.is_platform_owner AND u.organization_id=a.organization_id) ORDER BY u.created_at,u.id`, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []User{}
	for rows.Next() {
		var user User
		if err = rows.Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.IsPlatformOwner, &user.Disabled, &user.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (User, error) {
	var user User
	err := s.db.QueryRow(ctx, `SELECT id,email,role,organization_id,is_platform_owner,disabled_at IS NOT NULL,created_at FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.IsPlatformOwner, &user.Disabled, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) CanManage(ctx context.Context, actorID, targetID string) bool {
	var allowed bool
	err := s.db.QueryRow(ctx, `SELECT a.is_platform_owner OR (NOT t.is_platform_owner AND a.organization_id=t.organization_id) FROM users a JOIN users t ON t.id=$2 WHERE a.id=$1`, actorID, targetID).Scan(&allowed)
	return err == nil && allowed
}

func (s *Store) Update(ctx context.Context, actorID, targetID string, role *string, disabled *bool, requestID string) (User, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var current User
	err = tx.QueryRow(ctx, `SELECT id,email,role,disabled_at IS NOT NULL,created_at FROM users WHERE id=$1 FOR UPDATE`, targetID).Scan(&current.ID, &current.Email, &current.Role, &current.Disabled, &current.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	nextRole := current.Role
	if role != nil {
		nextRole = *role
	}
	nextDisabled := current.Disabled
	if disabled != nil {
		nextDisabled = *disabled
	}
	if nextRole != "admin" && nextRole != "instructor" && nextRole != "student" {
		return User{}, ErrConflict
	}
	removingAdmin := current.Role == "admin" && !current.Disabled && (nextRole != "admin" || nextDisabled)
	if removingAdmin {
		var count int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT id FROM users WHERE role='admin' AND disabled_at IS NULL AND organization_id=(SELECT organization_id FROM users WHERE id=$1) FOR UPDATE) enabled_admins`, targetID).Scan(&count)
		if err != nil {
			return User{}, err
		}
		if count <= 1 {
			return User{}, ErrLastAdmin
		}
	}
	if current.Role != nextRole && nextRole == "student" {
		rows, queryErr := tx.Query(ctx, `UPDATE library_members SET access_level='student',updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND access_level='instructor' RETURNING library_id`, targetID)
		if queryErr != nil {
			return User{}, queryErr
		}
		var libraryIDs []string
		for rows.Next() {
			var id string
			if queryErr = rows.Scan(&id); queryErr != nil {
				rows.Close()
				return User{}, queryErr
			}
			libraryIDs = append(libraryIDs, id)
		}
		queryErr = rows.Err()
		rows.Close()
		if queryErr != nil {
			return User{}, queryErr
		}
		for _, id := range libraryIDs {
			if err = audit.Record(ctx, tx, actorID, "library.membership_changed", "library", id, requestID, map[string]any{"user_id": targetID, "before": "instructor", "after": "student"}); err != nil {
				return User{}, err
			}
		}
	}
	var updated User
	err = tx.QueryRow(ctx, `UPDATE users SET role=$2,disabled_at=CASE WHEN $3 THEN COALESCE(disabled_at,CURRENT_TIMESTAMP) ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING id,email,role,disabled_at IS NOT NULL,created_at`, targetID, nextRole, nextDisabled).Scan(&updated.ID, &updated.Email, &updated.Role, &updated.Disabled, &updated.CreatedAt)
	if err != nil {
		return User{}, err
	}
	changed := current.Role != updated.Role || current.Disabled != updated.Disabled
	if changed {
		if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND revoked_at IS NULL`, targetID); err != nil {
			return User{}, err
		}
		if err = audit.Record(ctx, tx, actorID, "user.authorization_changed", "user", targetID, requestID, map[string]any{"role_before": current.Role, "role_after": updated.Role, "disabled_before": current.Disabled, "disabled_after": updated.Disabled}); err != nil {
			return User{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return updated, nil
}

func (s *Store) RevokeSessions(ctx context.Context, actorID, targetID, requestID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, targetID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	result, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND revoked_at IS NULL`, targetID)
	if err != nil {
		return err
	}
	if err = audit.Record(ctx, tx, actorID, "user.sessions_revoked", "user", targetID, requestID, map[string]any{"session_count": result.RowsAffected()}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
