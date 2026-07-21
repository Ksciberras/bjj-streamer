package auth

import "time"

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Session struct {
	ID        string
	User      User
	CSRFHash  []byte
	ExpiresAt time.Time
}

type Settings struct {
	CookieSecure       bool
	InvitationTTL      time.Duration
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
}
