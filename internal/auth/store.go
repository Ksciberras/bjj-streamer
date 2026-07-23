package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthenticated = errors.New("unauthenticated")
var ErrBootstrapComplete = errors.New("bootstrap already completed")

const platformOwnerEmail = "kyranu2@gmail.com"

type Store struct {
	db  *pgxpool.Pool
	now func() time.Time
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db, now: time.Now} }

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func (s *Store) BootstrapAdmin(ctx context.Context, email, passwordHash string) (User, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(90210)`); err != nil {
		return User{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return User{}, err
	}
	if count != 0 {
		return User{}, ErrBootstrapComplete
	}
	var user User
	normalizedEmail := normalizeEmail(email)
	err = tx.QueryRow(ctx, `INSERT INTO users (email,password_hash,role,organization_id,is_platform_owner)
		VALUES ($1,$2,'admin',CASE WHEN $1=$3 THEN NULL ELSE (SELECT id FROM organizations ORDER BY created_at,id LIMIT 1) END,$1=$3)
		RETURNING id,email,role,organization_id,is_platform_owner`,
		normalizedEmail, passwordHash, platformOwnerEmail,
	).Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.IsPlatformOwner)
	if err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) PasswordHash(ctx context.Context, email string) (User, string, error) {
	var user User
	var hash string
	err := s.db.QueryRow(ctx, `SELECT u.id,u.email,u.role,u.organization_id,COALESCE(o.name,'All gyms'),u.is_platform_owner,u.password_hash FROM users u LEFT JOIN organizations o ON o.id=u.organization_id WHERE u.email=$1 AND u.disabled_at IS NULL`, normalizeEmail(email)).Scan(&user.ID, &user.Email, &user.Role, &user.OrganizationID, &user.OrganizationName, &user.IsPlatformOwner, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	return user, hash, err
}

func (s *Store) CreateSession(ctx context.Context, userID, oldToken string, settings Settings) (string, string, time.Time, error) {
	token, tokenDigest, err := newToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	csrf, csrfDigest, err := newToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := s.now()
	absolute := now.Add(settings.SessionAbsoluteTTL)
	idle := now.Add(settings.SessionIdleTTL)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if oldToken != "" {
		if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=$1 WHERE token_hash=$2 AND revoked_at IS NULL`, now, tokenHash(oldToken)); err != nil {
			return "", "", time.Time{}, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO sessions (user_id,token_hash,csrf_hash,expires_at,idle_expires_at,created_at,last_seen_at) VALUES ($1,$2,$3,$4,$5,$6,$6)`, userID, tokenDigest, csrfDigest, absolute, idle, now)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", time.Time{}, err
	}
	return token, csrf, absolute, nil
}

func (s *Store) Authenticate(ctx context.Context, token string, idleTTL time.Duration) (Session, error) {
	now := s.now()
	var session Session
	err := s.db.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=$2, idle_expires_at=LEAST(s.expires_at,$3) FROM users u LEFT JOIN organizations o ON o.id=u.organization_id WHERE s.token_hash=$1 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>$2 AND s.idle_expires_at>$2 AND u.disabled_at IS NULL RETURNING s.id,u.id,u.email,u.role,u.organization_id,COALESCE(o.name,'All gyms'),u.is_platform_owner,s.csrf_hash,s.expires_at`, tokenHash(token), now, now.Add(idleTTL)).Scan(&session.ID, &session.User.ID, &session.User.Email, &session.User.Role, &session.User.OrganizationID, &session.User.OrganizationName, &session.User.IsPlatformOwner, &session.CSRFHash, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	return session, err
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `UPDATE sessions SET revoked_at=$1 WHERE token_hash=$2 AND revoked_at IS NULL`, s.now(), tokenHash(token))
	return err
}
