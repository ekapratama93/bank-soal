package httpapi

import (
	"sync"
	"time"
)

const (
	loginLimitMax    = 5
	loginLimitWindow = 15 * time.Minute
)

// LoginLimiter is a simple in-memory sliding-window limiter keyed by
// lowercased email — 5 attempts per 15 minutes, same as the Python
// backend's slowapi-based limiter. Like that implementation, this resets
// on process restart and isn't shared across instances; that's an
// accepted, documented tradeoff for a single-process deployment, not a
// bug.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{attempts: map[string][]time.Time{}}
}

// Allow records an attempt for key and reports whether it's within the
// limit (5 in the trailing 15 minutes).
func (l *LoginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-loginLimitWindow)

	kept := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= loginLimitMax {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, now)
	return true
}
