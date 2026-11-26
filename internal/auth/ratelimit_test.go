package auth

import (
	"testing"
	"time"
)

func TestLoginLimiter_AllowsInitialAttempt(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      5,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour, // Long interval to avoid cleanup during test
	}
	limiter := NewLoginLimiter(cfg)

	allowed, _, _ := limiter.CheckAllowed("test@example.com")
	if !allowed {
		t.Error("Initial attempt should be allowed")
	}
}

func TestLoginLimiter_CountsFailures(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      5,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour,
	}
	limiter := NewLoginLimiter(cfg)

	email := "test@example.com"

	// First failure
	locked, _ := limiter.RecordFailure(email)
	if locked {
		t.Error("Should not lock after 1 failure")
	}

	remaining := limiter.RemainingAttempts(email)
	if remaining != 4 {
		t.Errorf("RemainingAttempts = %d, want 4", remaining)
	}

	// Second failure
	limiter.RecordFailure(email)
	remaining = limiter.RemainingAttempts(email)
	if remaining != 3 {
		t.Errorf("RemainingAttempts = %d, want 3", remaining)
	}
}

func TestLoginLimiter_LocksAfterMaxAttempts(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      3,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour,
	}
	limiter := NewLoginLimiter(cfg)

	email := "test@example.com"

	// First two failures
	limiter.RecordFailure(email)
	limiter.RecordFailure(email)

	// Third failure should trigger lockout
	locked, duration := limiter.RecordFailure(email)
	if !locked {
		t.Error("Should lock after 3 failures")
	}
	if duration != time.Minute {
		t.Errorf("Lock duration = %v, want %v", duration, time.Minute)
	}

	// Check should now deny
	allowed, _, _ := limiter.CheckAllowed(email)
	if allowed {
		t.Error("Should deny locked account")
	}
}

func TestLoginLimiter_ResetOnSuccess(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      5,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour,
	}
	limiter := NewLoginLimiter(cfg)

	email := "test@example.com"

	// Record some failures
	limiter.RecordFailure(email)
	limiter.RecordFailure(email)

	// Successful login
	limiter.RecordSuccess(email)

	// Should be back to full attempts
	remaining := limiter.RemainingAttempts(email)
	if remaining != 5 {
		t.Errorf("RemainingAttempts after success = %d, want 5", remaining)
	}
}

func TestLoginLimiter_DifferentEmails(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      3,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour,
	}
	limiter := NewLoginLimiter(cfg)

	// Lock first email
	limiter.RecordFailure("user1@example.com")
	limiter.RecordFailure("user1@example.com")
	limiter.RecordFailure("user1@example.com")

	// Second email should still be allowed
	allowed, _, _ := limiter.CheckAllowed("user2@example.com")
	if !allowed {
		t.Error("Different email should not be affected")
	}

	// First email should be locked
	allowed, _, _ = limiter.CheckAllowed("user1@example.com")
	if allowed {
		t.Error("First email should be locked")
	}
}

func TestLoginLimiter_GetAttemptInfo(t *testing.T) {
	cfg := LoginLimiterConfig{
		MaxAttempts:      5,
		LockoutDurations: []time.Duration{time.Minute},
		AttemptWindow:    15 * time.Minute,
		CleanupInterval:  time.Hour,
	}
	limiter := NewLoginLimiter(cfg)

	email := "test@example.com"

	// No info for unknown email
	info := limiter.GetAttemptInfo(email)
	if info != nil {
		t.Error("Should return nil for unknown email")
	}

	// Record failure and check info
	limiter.RecordFailure(email)
	info = limiter.GetAttemptInfo(email)
	if info == nil {
		t.Fatal("Should return info after failure")
	}
	if info.Count != 1 {
		t.Errorf("Count = %d, want 1", info.Count)
	}
}

func TestLoginAttempt_IsLocked(t *testing.T) {
	// Not locked
	attempt := &LoginAttempt{}
	if attempt.IsLocked() {
		t.Error("Should not be locked with zero LockExpiry")
	}

	// Locked (future expiry)
	attempt.LockExpiry = time.Now().Add(time.Hour)
	if !attempt.IsLocked() {
		t.Error("Should be locked with future expiry")
	}

	// Expired lock
	attempt.LockExpiry = time.Now().Add(-time.Hour)
	if attempt.IsLocked() {
		t.Error("Should not be locked with past expiry")
	}
}

func TestLoginAttempt_LockDuration(t *testing.T) {
	attempt := &LoginAttempt{}

	// No lock
	if attempt.LockDuration() != 0 {
		t.Error("LockDuration should be 0 when not locked")
	}

	// Active lock
	attempt.LockExpiry = time.Now().Add(30 * time.Second)
	duration := attempt.LockDuration()
	if duration < 25*time.Second || duration > 35*time.Second {
		t.Errorf("LockDuration = %v, expected ~30s", duration)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{1 * time.Second, "1 second"},
		{30 * time.Second, "30 seconds"},
		{1 * time.Minute, "1 minute"},
		{5 * time.Minute, "05 minutes"},
		{1 * time.Hour, "1 hour"},
		{2 * time.Hour, "02 hours"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.duration)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}
