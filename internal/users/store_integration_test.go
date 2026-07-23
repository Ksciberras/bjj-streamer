package users

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kyransciberras/bjj-streaming/internal/auth"
)

func testStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	connection, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connection.Exec(context.Background(), `SELECT pg_advisory_lock(8675309)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(8675309)`)
		connection.Release()
	})
	if _, err = pool.Exec(context.Background(), `TRUNCATE audit_events,library_members,libraries,sessions,invitations,users CASCADE`); err != nil {
		t.Fatal(err)
	}
	return NewStore(pool), pool
}
func insertUser(t *testing.T, pool *pgxpool.Pool, email, role string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFinalEnabledAdminCannotBeRemoved(t *testing.T) {
	store, pool := testStore(t)
	admin := insertUser(t, pool, "admin@example.com", "admin")
	student := "student"
	if _, err := store.Update(context.Background(), admin, admin, &student, nil, "request-1"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demotion got %v", err)
	}
	disabled := true
	if _, err := store.Update(context.Background(), admin, admin, nil, &disabled, "request-2"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable got %v", err)
	}
}

func TestAdminCreatesAccountAndResetsPassword(t *testing.T) {
	store, pool := testStore(t)
	admin := insertUser(t, pool, "admin@example.com", "admin")
	firstHash, err := auth.HashPassword("initial secure password")
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(context.Background(), admin, "Student@Example.com", "student", firstHash, "request-create")
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "student@example.com" || created.Role != "student" {
		t.Fatalf("created=%+v", created)
	}
	var libraryCount int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM libraries WHERE owner_user_id=$1`, created.ID).Scan(&libraryCount); err != nil || libraryCount != 1 {
		t.Fatalf("personal library count=%d err=%v", libraryCount, err)
	}
	token := make([]byte, 32)
	csrf := make([]byte, 32)
	if _, err = pool.Exec(context.Background(), `INSERT INTO sessions(user_id,token_hash,csrf_hash,expires_at,idle_expires_at)VALUES($1,$2,$3,$4,$4)`, created.ID, token, csrf, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	secondHash, err := auth.HashPassword("replacement secure password")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ResetPassword(context.Background(), admin, created.ID, secondHash, "request-reset"); err != nil {
		t.Fatal(err)
	}
	var storedHash string
	var revoked bool
	if err = pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id=$1`, created.ID).Scan(&storedHash); err != nil || !auth.CheckPassword(storedHash, "replacement secure password") {
		t.Fatalf("new password not stored securely: %v", err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM sessions WHERE user_id=$1`, created.ID).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
}

func TestRoleChangeDowngradesAssignmentsRevokesSessionsAndAudits(t *testing.T) {
	store, pool := testStore(t)
	admin := insertUser(t, pool, "admin@example.com", "admin")
	target := insertUser(t, pool, "instructor@example.com", "instructor")
	var libraryID string
	if err := pool.QueryRow(context.Background(), `INSERT INTO libraries(type,name,created_by)VALUES('shared','Shared',$1)RETURNING id`, admin).Scan(&libraryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO library_members(library_id,user_id,access_level)VALUES($1,$2,'instructor')`, libraryID, target); err != nil {
		t.Fatal(err)
	}
	token := make([]byte, 32)
	csrf := make([]byte, 32)
	if _, err := pool.Exec(context.Background(), `INSERT INTO sessions(user_id,token_hash,csrf_hash,expires_at,idle_expires_at)VALUES($1,$2,$3,$4,$4)`, target, token, csrf, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	role := "student"
	updated, err := store.Update(context.Background(), admin, target, &role, nil, "request-3")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "student" {
		t.Fatalf("role=%s", updated.Role)
	}
	var level string
	if err = pool.QueryRow(context.Background(), `SELECT access_level FROM library_members WHERE library_id=$1 AND user_id=$2`, libraryID, target).Scan(&level); err != nil || level != "student" {
		t.Fatalf("level=%s err=%v", level, err)
	}
	var revoked bool
	if err = pool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM sessions WHERE user_id=$1`, target).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
	var count int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE actor_user_id=$1 AND request_id='request-3'`, admin).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestPlatformOwnerMovesAccountAndRevokesSessions(t *testing.T) {
	store, pool := testStore(t)
	var ownerID, targetID, destinationID string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role,is_platform_owner)VALUES('owner@example.com','hash','admin',TRUE)RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES('moving@example.com','hash','student')RETURNING id`).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO organizations(name,slug)VALUES('Move Destination','move-destination') ON CONFLICT(slug)DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&destinationID); err != nil {
		t.Fatal(err)
	}
	token := make([]byte, 32)
	csrf := make([]byte, 32)
	if _, err := pool.Exec(context.Background(), `INSERT INTO sessions(user_id,token_hash,csrf_hash,expires_at,idle_expires_at)VALUES($1,$2,$3,$4,$4)`, targetID, token, csrf, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	moved, err := store.MoveToOrganization(context.Background(), ownerID, targetID, destinationID, "move-request")
	if err != nil {
		t.Fatal(err)
	}
	if moved.OrganizationID == nil || *moved.OrganizationID != destinationID {
		t.Fatalf("moved=%+v", moved)
	}
	var revoked bool
	if err = pool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM sessions WHERE user_id=$1`, targetID).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
	var auditCount int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE actor_user_id=$1 AND target_id=$2 AND action='user.organization_changed'`, ownerID, targetID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit count=%d err=%v", auditCount, err)
	}
}
