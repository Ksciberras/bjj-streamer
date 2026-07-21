package auth

import (
	"sync"
	"time"
)

type limiterEntry struct {
	count int
	reset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string]limiterEntry
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, now: time.Now, entries: make(map[string]limiterEntry)}
}

func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.entries) >= 10000 {
		for existing, candidate := range l.entries {
			if !now.Before(candidate.reset) {
				delete(l.entries, existing)
			}
		}
	}
	entry := l.entries[key]
	if !now.Before(entry.reset) {
		entry = limiterEntry{reset: now.Add(l.window)}
	}
	if entry.count >= l.limit {
		l.entries[key] = entry
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}
