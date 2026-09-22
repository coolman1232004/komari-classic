package public

import (
	"crypto/sha256"
	"net"
	"sync"
	"time"
)

type loginWindow struct {
	count   int
	expires time.Time
}
type loginLimiter struct {
	mu      sync.Mutex
	windows map[[32]byte]loginWindow
}

var passwordLoginLimiter = &loginLimiter{windows: make(map[[32]byte]loginWindow)}

// The caller supplies the socket peer or an address validated against the
// explicitly configured proxy list. Account/global budgets always apply.
func (l *loginLimiter) allow(remoteAddr, username string, now time.Time) bool {
	peer, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		peer = remoteAddr
	}
	rules := []struct {
		key      string
		maximum  int
		duration time.Duration
	}{
		{"global", 60, time.Minute},
		{"peer:" + peer, 20, time.Minute},
		{"account:" + username, 10, 5 * time.Minute},
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, value := range l.windows {
		if !now.Before(value.expires) {
			delete(l.windows, key)
		}
	}
	missing := 0
	for _, rule := range rules {
		key := sha256.Sum256([]byte(rule.key))
		value, exists := l.windows[key]
		if !exists {
			missing++
		}
		if value.count >= rule.maximum {
			return false
		}
	}
	// Fail closed instead of evicting an active limit under username/IP churn.
	if len(l.windows)+missing > 4096 {
		return false
	}
	for _, rule := range rules {
		key := sha256.Sum256([]byte(rule.key))
		value := l.windows[key]
		if value.count == 0 {
			value.expires = now.Add(rule.duration)
		}
		value.count++
		l.windows[key] = value
	}
	return true
}
