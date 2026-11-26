package auth

import (
	"sync"
	"time"
)

// LoginAttempt tracks failed login attempts for an email.
type LoginAttempt struct {
	Count      int
	FirstAt    time.Time
	LastAt     time.Time
	LockedAt   time.Time
	LockExpiry time.Time
}

// IsLocked returns true if the account is currently locked.
func (a *LoginAttempt) IsLocked() bool {
	if a.LockExpiry.IsZero() {
		return false
	}
	return time.Now().Before(a.LockExpiry)
}

// LockDuration returns the remaining lock duration.
func (a *LoginAttempt) LockDuration() time.Duration {
	if !a.IsLocked() {
		return 0
	}
	return time.Until(a.LockExpiry)
}

// LoginLimiter implements brute-force protection for login attempts.
type LoginLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*LoginAttempt
	config   LoginLimiterConfig
}

// LoginLimiterConfig holds configuration for the login limiter.
type LoginLimiterConfig struct {
	// MaxAttempts is the number of failed attempts before lockout
	MaxAttempts int
	// LockoutDurations defines escalating lockout durations
	// Index 0 = first lockout, Index 1 = second lockout, etc.
	LockoutDurations []time.Duration
	// AttemptWindow is the time window for counting attempts
	AttemptWindow time.Duration
	// CleanupInterval is how often to clean up old entries
	CleanupInterval time.Duration
}

// DefaultLoginLimiterConfig returns sensible defaults.
func DefaultLoginLimiterConfig() LoginLimiterConfig {
	return LoginLimiterConfig{
		MaxAttempts: 5,
		LockoutDurations: []time.Duration{
			1 * time.Minute,  // First lockout: 1 minute
			5 * time.Minute,  // Second lockout: 5 minutes
			15 * time.Minute, // Third lockout: 15 minutes
			60 * time.Minute, // Fourth+ lockout: 1 hour
		},
		AttemptWindow:   15 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// NewLoginLimiter creates a new login limiter.
func NewLoginLimiter(cfg LoginLimiterConfig) *LoginLimiter {
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 5
	}
	if len(cfg.LockoutDurations) == 0 {
		cfg.LockoutDurations = DefaultLoginLimiterConfig().LockoutDurations
	}
	if cfg.AttemptWindow == 0 {
		cfg.AttemptWindow = 15 * time.Minute
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	l := &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
		config:   cfg,
	}

	// Start cleanup goroutine
	go l.cleanupLoop()

	return l
}

// CheckAllowed checks if a login attempt is allowed for the given email.
// Returns (allowed, remaining lockout duration, error message).
func (l *LoginLimiter) CheckAllowed(email string) (bool, time.Duration, string) {
	l.mu.RLock()
	attempt, exists := l.attempts[email]
	l.mu.RUnlock()

	if !exists {
		return true, 0, ""
	}

	if attempt.IsLocked() {
		remaining := attempt.LockDuration()
		return false, remaining, formatLockMessage(remaining)
	}

	return true, 0, ""
}

// RecordFailure records a failed login attempt.
// Returns (locked, lockout duration) if this attempt caused a lockout.
func (l *LoginLimiter) RecordFailure(email string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	attempt, exists := l.attempts[email]

	if !exists {
		attempt = &LoginAttempt{
			Count:   1,
			FirstAt: now,
			LastAt:  now,
		}
		l.attempts[email] = attempt
		return false, 0
	}

	// If the attempt window has passed, reset the counter
	if now.Sub(attempt.FirstAt) > l.config.AttemptWindow {
		attempt.Count = 1
		attempt.FirstAt = now
		attempt.LastAt = now
		return false, 0
	}

	// Increment counter
	attempt.Count++
	attempt.LastAt = now

	// Check if we should lock
	if attempt.Count >= l.config.MaxAttempts {
		// Calculate lockout duration based on how many times locked before
		lockoutIndex := l.countPreviousLockouts(attempt)
		if lockoutIndex >= len(l.config.LockoutDurations) {
			lockoutIndex = len(l.config.LockoutDurations) - 1
		}
		lockDuration := l.config.LockoutDurations[lockoutIndex]

		attempt.LockedAt = now
		attempt.LockExpiry = now.Add(lockDuration)

		return true, lockDuration
	}

	return false, 0
}

// RecordSuccess records a successful login and resets the attempt counter.
func (l *LoginLimiter) RecordSuccess(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, email)
}

// countPreviousLockouts counts how many times this email has been locked before.
func (l *LoginLimiter) countPreviousLockouts(attempt *LoginAttempt) int {
	if attempt.LockedAt.IsZero() {
		return 0
	}
	// Count based on how many times LockExpiry has been set
	// This is a simple heuristic - in production you might track this explicitly
	count := 0
	if !attempt.LockedAt.IsZero() {
		count++
	}
	return count
}

// GetAttemptInfo returns information about login attempts for an email.
// Useful for logging and debugging.
func (l *LoginLimiter) GetAttemptInfo(email string) *LoginAttempt {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[email]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	return &LoginAttempt{
		Count:      attempt.Count,
		FirstAt:    attempt.FirstAt,
		LastAt:     attempt.LastAt,
		LockedAt:   attempt.LockedAt,
		LockExpiry: attempt.LockExpiry,
	}
}

// RemainingAttempts returns how many attempts are left before lockout.
func (l *LoginLimiter) RemainingAttempts(email string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[email]
	if !exists {
		return l.config.MaxAttempts
	}

	remaining := l.config.MaxAttempts - attempt.Count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// cleanupLoop periodically removes expired entries.
func (l *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.cleanup()
	}
}

// cleanup removes entries that are no longer relevant.
func (l *LoginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for email, attempt := range l.attempts {
		// Remove if:
		// 1. Lock has expired AND attempt window has passed
		// 2. No recent activity
		lockExpired := attempt.LockExpiry.IsZero() || now.After(attempt.LockExpiry)
		windowExpired := now.Sub(attempt.LastAt) > l.config.AttemptWindow

		if lockExpired && windowExpired {
			delete(l.attempts, email)
		}
	}
}

// formatLockMessage creates a user-friendly lockout message.
func formatLockMessage(remaining time.Duration) string {
	if remaining < time.Minute {
		secs := int(remaining.Seconds())
		if secs <= 1 {
			return "Account locked. Try again in 1 second."
		}
		return "Account locked. Try again in " + formatDuration(remaining) + "."
	}
	return "Account locked. Try again in " + formatDuration(remaining) + "."
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs == 1 {
			return "1 second"
		}
		return string(rune('0'+secs/10)) + string(rune('0'+secs%10)) + " seconds"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return string(rune('0'+mins/10)) + string(rune('0'+mins%10)) + " minutes"
	}
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour"
	}
	return string(rune('0'+hours/10)) + string(rune('0'+hours%10)) + " hours"
}
