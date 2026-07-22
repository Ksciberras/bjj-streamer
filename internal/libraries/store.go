package libraries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/audit"
)

var ErrNotFound = errors.New("not found")

type Library struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	OwnerUserID *string   `json:"owner_user_id,omitempty"`
	Archived    bool      `json:"archived"`
	Membership  *string   `json:"membership,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
type Member struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	GlobalRole  string `json:"global_role"`
	AccessLevel string `json:"access_level"`
}
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

const libraryColumns = `l.id,l.type,l.name,l.owner_user_id,l.archived_at IS NOT NULL,lm.access_level,l.created_at`

func scanLibrary(row pgx.Row) (Library, error) {
	var value Library
	err := row.Scan(&value.ID, &value.Type, &value.Name, &value.OwnerUserID, &value.Archived, &value.Membership, &value.CreatedAt)
	return value, err
}

func (s *Store) List(ctx context.Context, userID string) ([]Library, error) {
	rows, err := s.db.Query(ctx, `SELECT `+libraryColumns+` FROM libraries l LEFT JOIN library_members lm ON lm.library_id=l.id AND lm.user_id=$1 WHERE (l.type='personal' AND l.owner_user_id=$1) OR (l.type='shared' AND lm.user_id IS NOT NULL) ORDER BY l.created_at,l.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Library{}
	for rows.Next() {
		value, scanErr := scanLibrary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *Store) Get(ctx context.Context, id, userID string) (Library, error) {
	value, err := scanLibrary(s.db.QueryRow(ctx, `SELECT `+libraryColumns+` FROM libraries l LEFT JOIN library_members lm ON lm.library_id=l.id AND lm.user_id=$2 WHERE l.id=$1`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Library{}, ErrNotFound
	}
	return value, err
}

func (s *Store) CreateShared(ctx context.Context, actorID, name, requestID string) (Library, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Library{}, err
	}
	defer tx.Rollback(ctx)
	var value Library
	err = tx.QueryRow(ctx, `INSERT INTO libraries(type,name,created_by)VALUES('shared',$1,$2)RETURNING id,type,name,owner_user_id,archived_at IS NOT NULL,created_at`, name, actorID).Scan(&value.ID, &value.Type, &value.Name, &value.OwnerUserID, &value.Archived, &value.CreatedAt)
	if err != nil {
		return Library{}, err
	}
	level := "student"
	value.Membership = &level
	if _, err = tx.Exec(ctx, `INSERT INTO library_members(library_id,user_id,access_level)VALUES($1,$2,'student')`, value.ID, actorID); err != nil {
		return Library{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "library.created", "library", value.ID, requestID, map[string]any{"type": "shared", "name": name}); err != nil {
		return Library{}, err
	}
	if err = audit.Record(ctx, tx, actorID, "library.membership_added", "library", value.ID, requestID, map[string]any{"user_id": actorID, "access_level": "student"}); err != nil {
		return Library{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Library{}, err
	}
	return value, nil
}

func (s *Store) Update(ctx context.Context, actorID, id, userID, requestID string, name *string, archived *bool) (Library, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Library{}, err
	}
	defer tx.Rollback(ctx)
	current, err := scanLibrary(tx.QueryRow(ctx, `SELECT `+libraryColumns+` FROM libraries l LEFT JOIN library_members lm ON lm.library_id=l.id AND lm.user_id=$2 WHERE l.id=$1 FOR UPDATE OF l`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Library{}, ErrNotFound
	}
	if err != nil {
		return Library{}, err
	}
	nextName := current.Name
	if name != nil {
		nextName = *name
	}
	nextArchived := current.Archived
	if archived != nil {
		nextArchived = *archived
	}
	var updated Library
	err = tx.QueryRow(ctx, `UPDATE libraries SET name=$2,archived_at=CASE WHEN $3 THEN COALESCE(archived_at,CURRENT_TIMESTAMP) ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING id,type,name,owner_user_id,archived_at IS NOT NULL,created_at`, id, nextName, nextArchived).Scan(&updated.ID, &updated.Type, &updated.Name, &updated.OwnerUserID, &updated.Archived, &updated.CreatedAt)
	if err != nil {
		return Library{}, err
	}
	updated.Membership = current.Membership
	if current.Name != updated.Name || current.Archived != updated.Archived {
		if err = audit.Record(ctx, tx, actorID, "library.updated", "library", id, requestID, map[string]any{"name_before": current.Name, "name_after": updated.Name, "archived_before": current.Archived, "archived_after": updated.Archived}); err != nil {
			return Library{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Library{}, err
	}
	return updated, nil
}

func (s *Store) ListMembers(ctx context.Context, libraryID string) ([]Member, error) {
	rows, err := s.db.Query(ctx, `SELECT lm.user_id,u.email,u.role,lm.access_level FROM library_members lm JOIN users u ON u.id=lm.user_id WHERE lm.library_id=$1 ORDER BY u.email,u.id`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Member{}
	for rows.Next() {
		var member Member
		if err = rows.Scan(&member.UserID, &member.Email, &member.GlobalRole, &member.AccessLevel); err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, rows.Err()
}
func (s *Store) GetMember(ctx context.Context, libraryID, userID string) (Member, error) {
	var value Member
	err := s.db.QueryRow(ctx, `SELECT lm.user_id,u.email,u.role,lm.access_level FROM library_members lm JOIN users u ON u.id=lm.user_id WHERE lm.library_id=$1 AND lm.user_id=$2`, libraryID, userID).Scan(&value.UserID, &value.Email, &value.GlobalRole, &value.AccessLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return value, err
}

func (s *Store) PutMember(ctx context.Context, actorID, libraryID, userID, level, requestID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var before *string
	scanErr := tx.QueryRow(ctx, `SELECT access_level FROM library_members WHERE library_id=$1 AND user_id=$2 FOR UPDATE`, libraryID, userID).Scan(&before)
	if scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return scanErr
	}
	_, err = tx.Exec(ctx, `INSERT INTO library_members(library_id,user_id,access_level)VALUES($1,$2,$3) ON CONFLICT(library_id,user_id)DO UPDATE SET access_level=EXCLUDED.access_level,updated_at=CURRENT_TIMESTAMP`, libraryID, userID, level)
	if err != nil {
		return err
	}
	action := "library.membership_added"
	details := map[string]any{"user_id": userID, "access_level": level}
	if before != nil {
		action = "library.membership_changed"
		details["before"] = *before
		details["after"] = level
	}
	if err = audit.Record(ctx, tx, actorID, action, "library", libraryID, requestID, details); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) RemoveMember(ctx context.Context, actorID, libraryID, userID, requestID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `DELETE FROM library_members WHERE library_id=$1 AND user_id=$2`, libraryID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = audit.Record(ctx, tx, actorID, "library.membership_removed", "library", libraryID, requestID, map[string]any{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
