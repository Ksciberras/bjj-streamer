package auth

import "time"

type User struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Role             string  `json:"role"`
	OrganizationID   *string `json:"organization_id,omitempty"`
	OrganizationName string  `json:"organization_name,omitempty"`
	IsPlatformOwner  bool    `json:"is_platform_owner"`
}

type Session struct {
	ID        string
	User      User
	CSRFHash  []byte
	ExpiresAt time.Time
}

type Settings struct {
	CookieSecure       bool
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
}
