// Package ratelimit is a fixed-window in-process limiter — correct for the
// single-instance alpha (architecture: no cache tier yet; a shared store is
// a later, measured knob). Injected, never global.
package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	count int
	reset time.Time
}

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	m      map[string]*entry
	now    func() time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, m: map[string]*entry{}, now: time.Now}
}

// Allow reports whether the key may proceed; when denied it returns the
// seconds until the window resets (the Retry-After value).
func (l *Limiter) Allow(key string) (ok bool, retryAfterS int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e := l.m[key]
	if e == nil || now.After(e.reset) {
		l.m[key] = &entry{count: 1, reset: now.Add(l.window)}
		if len(l.m) > 4096 { // lazy prune
			for k, v := range l.m {
				if now.After(v.reset) {
					delete(l.m, k)
				}
			}
		}
		return true, 0
	}
	if e.count >= l.limit {
		return false, int(time.Until(e.reset).Seconds()) + 1
	}
	e.count++
	return true, 0
}
