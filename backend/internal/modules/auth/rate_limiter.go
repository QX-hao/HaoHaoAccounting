package auth

import (
	"strings"
	"sync"
	"time"
)

// loginLimiter 是内存级登录限流器，按“IP + 用户名”限制连续失败次数。
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

// newLoginLimiter 创建滑动窗口限流器；now 可在测试里替换，避免依赖真实时间等待。
func newLoginLimiter(maxFailures int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		maxFailures: maxFailures,
		window:      window,
		now:         time.Now,
		attempts:    map[string]loginAttempt{},
	}
}

// Allow 在窗口内失败次数未达到上限时放行登录尝试。
func (l *loginLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneExpired(l.now())
	attempt, ok := l.attempts[key]
	if !ok {
		return true
	}
	return attempt.failures < l.maxFailures
}

// RetryAfter 返回还需要等待多久；达到窗口边界时会顺手清理过期记录。
func (l *loginLimiter) RetryAfter(key string) time.Duration {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.pruneExpired(now)
	attempt, ok := l.attempts[key]
	if !ok || attempt.failures < l.maxFailures {
		return 0
	}
	remaining := l.window - now.Sub(attempt.firstFailure)
	if remaining <= 0 {
		delete(l.attempts, key)
		return 0
	}
	return remaining
}

// RecordFailure 记录一次失败，第一次失败的时间作为整个窗口的起点。
func (l *loginLimiter) RecordFailure(key string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.pruneExpired(now)
	attempt, ok := l.attempts[key]
	if !ok || now.Sub(attempt.firstFailure) >= l.window {
		l.attempts[key] = loginAttempt{failures: 1, firstFailure: now}
		return
	}
	attempt.failures++
	l.attempts[key] = attempt
}

// RecordSuccess 登录成功后清掉失败计数，避免历史失败继续影响用户。
func (l *loginLimiter) RecordSuccess(key string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// pruneExpired 清理窗口外记录，避免长期运行后 attempts map 无限制增长。
func (l *loginLimiter) pruneExpired(now time.Time) {
	for key, attempt := range l.attempts {
		if now.Sub(attempt.firstFailure) >= l.window {
			delete(l.attempts, key)
		}
	}
}

// loginLimiterKey 同时包含 IP 和用户名，降低同 IP 多用户或同用户多 IP 互相影响的概率。
func loginLimiterKey(ip, username string) string {
	return strings.ToLower(strings.TrimSpace(ip)) + "|" + strings.ToLower(strings.TrimSpace(username))
}
