package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

type entry struct {
	failures  int
	lockedAt  *time.Time
	windowEnd time.Time
}

// LoginLimiter tracks failed login attempts per IP and enforces lockout.
type LoginLimiter struct {
	mu          sync.Mutex
	entries     map[string]*entry
	maxFailures int
	lockout     time.Duration
	window      time.Duration
}

func NewLoginLimiter() *LoginLimiter {
	l := &LoginLimiter{
		entries:     make(map[string]*entry),
		maxFailures: 5,
		lockout:     15 * time.Minute,
		window:      15 * time.Minute,
	}
	go l.janitor()
	return l
}

// Middleware blocks requests from locked-out IPs before the handler runs.
func (l *LoginLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.isLocked(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, response.Error(
				"too many failed login attempts; try again in 15 minutes",
			))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RecordFailure increments the failure counter for the given IP.
func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok || time.Now().After(e.windowEnd) {
		e = &entry{}
		l.entries[ip] = e
	}
	e.failures++
	e.windowEnd = time.Now().Add(l.window)
	if e.failures >= l.maxFailures {
		now := time.Now()
		e.lockedAt = &now
	}
}

// RecordSuccess clears the failure record for the given IP.
func (l *LoginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *LoginLimiter) isLocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok || e.lockedAt == nil {
		return false
	}
	if time.Now().After(e.lockedAt.Add(l.lockout)) {
		delete(l.entries, ip)
		return false
	}
	return true
}

func (l *LoginLimiter) janitor() {
	t := time.NewTicker(5 * time.Minute)
	for range t.C {
		l.mu.Lock()
		now := time.Now()
		for ip, e := range l.entries {
			if e.lockedAt == nil && now.After(e.windowEnd) {
				delete(l.entries, ip)
			}
		}
		l.mu.Unlock()
	}
}
