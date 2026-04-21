package platform

import (
	"sync"
	"time"
)

const (
	invitationRateLimit  = 20
	invitationRateWindow = time.Hour
)

type invitationRateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

var invitationLimiter = &invitationRateLimiter{windows: make(map[string][]time.Time)}

func (l *invitationRateLimiter) allow(tenantID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-invitationRateWindow)

	times := l.windows[tenantID]
	fresh := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}

	if len(fresh) >= invitationRateLimit {
		l.windows[tenantID] = fresh
		return false
	}
	l.windows[tenantID] = append(fresh, now)
	return true
}
