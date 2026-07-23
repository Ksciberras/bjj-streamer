package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func integrationStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	connection, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connection.Exec(context.Background(), `SELECT pg_advisory_lock(8675309)`); err != nil {
		connection.Release()
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

func bootstrap(t *testing.T, s *Store) User {
	t.Helper()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.BootstrapAdmin(context.Background(), "Admin@Example.com", hash)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestBootstrapCanOnlyRunOnce(t *testing.T) {
	s, _ := integrationStore(t)
	bootstrap(t, s)
	hash, _ := HashPassword("another secure password")
	if _, err := s.BootstrapAdmin(context.Background(), "other@example.com", hash); !errors.Is(err, ErrBootstrapComplete) {
		t.Fatalf("got %v", err)
	}
}

func TestFreshBootstrapRecognizesDesignatedPlatformOwner(t *testing.T) {
	s, _ := integrationStore(t)
	hash, err := HashPassword("secure platform password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.BootstrapAdmin(context.Background(), platformOwnerEmail, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsPlatformOwner || user.OrganizationID != nil {
		t.Fatalf("platform owner bootstrap=%+v", user)
	}
}

func TestSessionsRotateRevokeExpireAndHonorDisabledUser(t *testing.T) {
	s, pool := integrationStore(t)
	user := bootstrap(t, s)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	settings := Settings{SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}
	first, _, _, err := s.CreateSession(context.Background(), user.ID, "", settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(context.Background(), first, time.Hour); err != nil {
		t.Fatal(err)
	}
	second, _, _, err := s.CreateSession(context.Background(), user.ID, first, settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(context.Background(), first, time.Hour); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("rotated session got %v", err)
	}
	if err = s.RevokeSession(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(context.Background(), second, time.Hour); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked session got %v", err)
	}
	third, _, _, err := s.CreateSession(context.Background(), user.ID, "", settings)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err = s.Authenticate(context.Background(), third, time.Hour); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired session got %v", err)
	}
	s.now = func() time.Time { return now }
	idle, _, _, err := s.CreateSession(context.Background(), user.ID, "", settings)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err = s.Authenticate(context.Background(), idle, time.Hour); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("idle session got %v", err)
	}
	s.now = func() time.Time { return now }
	fourth, _, _, err := s.CreateSession(context.Background(), user.ID, "", settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE users SET disabled_at=$1 WHERE id=$2`, now, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(context.Background(), fourth, time.Hour); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("disabled user session got %v", err)
	}
}
