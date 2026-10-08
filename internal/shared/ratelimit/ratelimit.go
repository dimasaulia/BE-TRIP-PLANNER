// Package ratelimit is a small fixed window counter kept in memory, enough for
// the single instance MVP.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count   int
	resetAt time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		buckets: map[string]*bucket{},
	}
}

// Allow reports whether key may proceed, and how long to wait when it may not.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l.limit <= 0 {
		return true, 0
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) > 10000 {
		for k, b := range l.buckets {
			if now.After(b.resetAt) {
				delete(l.buckets, k)
			}
		}
	}

	current, exists := l.buckets[key]
	if !exists || now.After(current.resetAt) {
		l.buckets[key] = &bucket{count: 1, resetAt: now.Add(l.window)}
		return true, 0
	}

	if current.count >= l.limit {
		return false, time.Until(current.resetAt)
	}

	current.count++
	return true, 0
}
