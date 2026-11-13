package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a per-key sliding window. A non-positive limit disables it.
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

// New returns a limiter that allows limit events per minute for each key.
func New(limit int) *Limiter {
	return &Limiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: time.Minute,
	}
}

// Allow reports whether key may proceed, recording the attempt when it may.
func (l *Limiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.hits[key][:0]
	for _, at := range l.hits[key] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}
