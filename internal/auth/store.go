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
var ErrInvalidInvitation = errors.New("invalid invitation")
var ErrUnauthenticated = errors.New("unauthenticated")
var ErrBootstrapComplete = errors.New("bootstrap already completed")

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
	err = tx.QueryRow(ctx, `INSERT INTO users (email,password_hash,role) VALUES ($1,$2,'admin') RETURNING id,email,role`, normalizeEmail(email), passwordHash).Scan(&user.ID, &user.Email, &user.Role)
	if err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateInvitation(ctx context.Context, email, role string, invitedBy string, ttl time.Duration) (string, time.Time, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().Add(ttl)
	_, err = s.db.Exec(ctx, `INSERT INTO invitations (email,role,token_hash,invited_by,expires_at,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, normalizeEmail(email), role, hash, invitedBy, expires, s.now())
	return token, expires, err
}

func (s *Store) AcceptInvitation(ctx context.Context, token, passwordHash string) (User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var id, email, role string
	err = tx.QueryRow(ctx, `SELECT id,email,role FROM invitations WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>$2 FOR UPDATE`, tokenHash(token), s.now()).Scan(&id, &email, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidInvitation
	}
	if err != nil {
		return User{}, err
	}
	var user User
	err = tx.QueryRow(ctx, `INSERT INTO users (email,password_hash,role) VALUES ($1,$2,$3) RETURNING id,email,role`, email, passwordHash, role).Scan(&user.ID, &user.Email, &user.Role)
	if err != nil {
		return User{}, ErrInvalidInvitation
	}
	if _, err = tx.Exec(ctx, `UPDATE invitations SET consumed_at=$1 WHERE id=$2`, s.now(), id); err != nil {
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
	err := s.db.QueryRow(ctx, `SELECT id,email,role,password_hash FROM users WHERE email=$1 AND disabled_at IS NULL`, normalizeEmail(email)).Scan(&user.ID, &user.Email, &user.Role, &hash)
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
	err := s.db.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=$2, idle_expires_at=LEAST(s.expires_at,$3) FROM users u WHERE s.token_hash=$1 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>$2 AND s.idle_expires_at>$2 AND u.disabled_at IS NULL RETURNING s.id,u.id,u.email,u.role,s.csrf_hash,s.expires_at`, tokenHash(token), now, now.Add(idleTTL)).Scan(&session.ID, &session.User.ID, &session.User.Email, &session.User.Role, &session.CSRFHash, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	return session, err
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `UPDATE sessions SET revoked_at=$1 WHERE token_hash=$2 AND revoked_at IS NULL`, s.now(), tokenHash(token))
	return err
}
