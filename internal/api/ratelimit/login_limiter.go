package ratelimit

import (
	"net"
	"sync"
	"time"
)

// LoginAttempt tracks failed login attempts
type LoginAttempt struct {
	Count       int
	LastAttempt time.Time
	BlockedUntil time.Time
}

// LoginLimiter manages rate limiting for login attempts
type LoginLimiter struct {
	attempts map[string]*LoginAttempt
	mu       sync.RWMutex

	// Configuration
	maxAttempts     int
	blockDuration   time.Duration
	cleanupInterval time.Duration
}

// NewLoginLimiter creates a new login rate limiter
func NewLoginLimiter(maxAttempts int, blockDuration time.Duration) *LoginLimiter {
	limiter := &LoginLimiter{
		attempts:        make(map[string]*LoginAttempt),
		maxAttempts:     maxAttempts,
		blockDuration:   blockDuration,
		cleanupInterval: 10 * time.Minute,
	}

	// Start cleanup routine
	go limiter.cleanup()

	return limiter
}

// RecordFailedAttempt records a failed login attempt
func (l *LoginLimiter) RecordFailedAttempt(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if attempt, exists := l.attempts[ip]; exists {
		attempt.Count++
		attempt.LastAttempt = now

		// Progressive blocking: increase block duration with more attempts
		if attempt.Count >= l.maxAttempts {
			multiplier := attempt.Count - l.maxAttempts + 1
			attempt.BlockedUntil = now.Add(l.blockDuration * time.Duration(multiplier))
		}
	} else {
		l.attempts[ip] = &LoginAttempt{
			Count:       1,
			LastAttempt: now,
		}
	}
}

// ResetAttempts resets attempts for an IP (called on successful login)
func (l *LoginLimiter) ResetAttempts(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// IsBlocked checks if an IP is currently blocked
func (l *LoginLimiter) IsBlocked(ip string) (bool, time.Time) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if attempt, exists := l.attempts[ip]; exists {
		if time.Now().Before(attempt.BlockedUntil) {
			return true, attempt.BlockedUntil
		}
	}

	return false, time.Time{}
}

// GetAttemptCount returns the number of failed attempts for an IP
func (l *LoginLimiter) GetAttemptCount(ip string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if attempt, exists := l.attempts[ip]; exists {
		return attempt.Count
	}
	return 0
}

// cleanup periodically removes old entries
func (l *LoginLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, attempt := range l.attempts {
			// Remove entries older than 1 hour with no blocking
			if now.Sub(attempt.LastAttempt) > 1*time.Hour && now.After(attempt.BlockedUntil) {
				delete(l.attempts, ip)
			}
		}
		l.mu.Unlock()
	}
}

// GetClientIP extracts the real client IP from request headers
func GetClientIP(remoteAddr string, xForwardedFor string, xRealIP string) string {
	// Try X-Real-IP first (set by Traefik)
	if xRealIP != "" {
		ip := net.ParseIP(xRealIP)
		if ip != nil {
			return ip.String()
		}
	}

	// Try X-Forwarded-For
	if xForwardedFor != "" {
		// Take the first IP in the chain
		ips := parseForwardedFor(xForwardedFor)
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

// parseForwardedFor parses X-Forwarded-For header
func parseForwardedFor(header string) []string {
	var ips []string
	for i := 0; i < len(header); {
		// Skip spaces
		for i < len(header) && (header[i] == ' ' || header[i] == ',') {
			i++
		}

		// Find end of IP
		start := i
		for i < len(header) && header[i] != ',' {
			i++
		}

		if start < i {
			ip := header[start:i]
			// Trim spaces
			for len(ip) > 0 && ip[0] == ' ' {
				ip = ip[1:]
			}
			for len(ip) > 0 && ip[len(ip)-1] == ' ' {
				ip = ip[:len(ip)-1]
			}
			if len(ip) > 0 {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}
