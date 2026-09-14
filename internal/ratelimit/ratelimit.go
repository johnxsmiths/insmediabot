package ratelimit

import (
	"sync"
	"time"
)

type userHistory struct {
	timestamps []time.Time
}

// Limiter provides lightweight sliding-window rate limiting in serverless memory.
type Limiter struct {
	mu            sync.Mutex
	users         map[int64]*userHistory
	maxRequests   int
	windowSeconds time.Duration
}

// New creates a new in-memory rate limiter.
func New(maxRequests, windowSeconds int) *Limiter {
	if maxRequests <= 0 {
		maxRequests = 15
	}
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	return &Limiter{
		users:         make(map[int64]*userHistory),
		maxRequests:   maxRequests,
		windowSeconds: time.Duration(windowSeconds) * time.Second,
	}
}

// Allow checks whether a user has exceeded their request limit.
func (l *Limiter) Allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.windowSeconds)

	hist, exists := l.users[userID]
	if !exists {
		l.users[userID] = &userHistory{timestamps: []time.Time{now}}
		return true
	}

	// Filter out expired timestamps
	valid := make([]time.Time, 0, len(hist.timestamps)+1)
	for _, t := range hist.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.maxRequests {
		hist.timestamps = valid
		return false
	}

	valid = append(valid, now)
	hist.timestamps = valid
	return true
}
