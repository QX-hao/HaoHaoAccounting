package auth

import (
	"strings"
	"sync"
	"time"
)

type loginLimiter struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	now         func() time.Time
	attempts    map[string]loginAttempt
}

type loginAttempt struct {
	failures     int
	firstFailure time.Time
}

func newLoginLimiter(maxFailures int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		maxFailures: maxFailures,
		window:      window,
		now:         time.Now,
		attempts:    map[string]loginAttempt{},
	}
}

func (l *loginLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if !ok {
		return true
	}
	now := l.now()
	if now.Sub(attempt.firstFailure) > l.window {
		delete(l.attempts, key)
		return true
	}
	return attempt.failures < l.maxFailures
}

func (l *loginLimiter) RecordFailure(key string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok || now.Sub(attempt.firstFailure) > l.window {
		l.attempts[key] = loginAttempt{failures: 1, firstFailure: now}
		return
	}
	attempt.failures++
	l.attempts[key] = attempt
}

func (l *loginLimiter) RecordSuccess(key string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func loginLimiterKey(ip, username string) string {
	return strings.ToLower(strings.TrimSpace(ip)) + "|" + strings.ToLower(strings.TrimSpace(username))
}
