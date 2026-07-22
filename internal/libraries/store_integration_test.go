package libraries

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func libraryTestStore(t *testing.T) (*Store, *pgxpool.Pool) {
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
func addUser(t *testing.T, pool *pgxpool.Pool, email, role string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,role)VALUES($1,'hash',$2)RETURNING id`, email, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPersonalLibraryIsAutomaticOwnerOnlyAndCannotHaveMembers(t *testing.T) {
	store, pool := libraryTestStore(t)
	owner := addUser(t, pool, "owner@example.com", "student")
	other := addUser(t, pool, "other@example.com", "student")
	libraries, err := store.List(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 1 || libraries[0].Type != "personal" || valueString(libraries[0].OwnerUserID) != owner {
		t.Fatalf("libraries=%+v", libraries)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO library_members(library_id,user_id,access_level)VALUES($1,$2,'student')`, libraries[0].ID, other); err == nil {
		t.Fatal("membership added to personal library")
	}
	otherLibraries, err := store.List(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherLibraries) != 1 || otherLibraries[0].ID == libraries[0].ID {
		t.Fatal("personal library disclosed")
	}
}

func TestSharedMembershipAndContentBasisConstraints(t *testing.T) {
	store, pool := libraryTestStore(t)
	admin := addUser(t, pool, "admin@example.com", "admin")
	student := addUser(t, pool, "student@example.com", "student")
	shared, err := store.CreateShared(context.Background(), admin, "Shared", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PutMember(context.Background(), admin, shared.ID, student, "student", "request-2"); err != nil {
		t.Fatal(err)
	}
	studentLibraries, err := store.List(context.Background(), student)
	if err != nil || len(studentLibraries) != 2 {
		t.Fatalf("libraries=%+v err=%v", studentLibraries, err)
	}
	if _, err = pool.Exec(context.Background(), `SELECT assert_content_basis_for_library($1,'personal_purchase')`, shared.ID); err == nil {
		t.Fatal("personal purchase accepted in shared library")
	}
	if _, err = pool.Exec(context.Background(), `SELECT assert_content_basis_for_library($1,'licensed_for_group')`, shared.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.RemoveMember(context.Background(), admin, shared.ID, student, "request-3"); err != nil {
		t.Fatal(err)
	}
	studentLibraries, err = store.List(context.Background(), student)
	if err != nil || len(studentLibraries) != 1 {
		t.Fatalf("membership removal not immediate: %+v %v", studentLibraries, err)
	}
}

func TestInstructorAssignmentRequiresEligibleGlobalRoleAndAuditIsAppendOnly(t *testing.T) {
	store, pool := libraryTestStore(t)
	admin := addUser(t, pool, "admin@example.com", "admin")
	student := addUser(t, pool, "student@example.com", "student")
	shared, err := store.CreateShared(context.Background(), admin, "Shared", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PutMember(context.Background(), admin, shared.ID, student, "instructor", "request-2"); err == nil {
		t.Fatal("student received instructor assignment")
	}
	if _, err = pool.Exec(context.Background(), `UPDATE audit_events SET action='tampered'`); err == nil {
		t.Fatal("audit event update succeeded")
	}
	if _, err = pool.Exec(context.Background(), `DELETE FROM audit_events`); err == nil {
		t.Fatal("audit event delete succeeded")
	}
}
