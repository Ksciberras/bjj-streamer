package auth

import (
	"testing"
	"time"
)

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("key") || !limiter.Allow("key") || limiter.Allow("key") {
		t.Fatal("limit not enforced")
	}
	if !limiter.Allow("other") {
		t.Fatal("keys should be isolated")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow("key") {
		t.Fatal("limit did not reset")
	}
}
