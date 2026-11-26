package auth

import (
	"testing"
	"time"
)

func TestPasswordResetManager_GenerateToken(t *testing.T) {
	mgr := NewPasswordResetManager(PasswordResetConfig{
		TokenDuration: time.Hour,
	})

	token, err := mgr.GenerateToken(123, "test@example.com")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}

	// Token should be valid
	resetToken, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if resetToken.UserID != 123 {
		t.Errorf("UserID = %d, want 123", resetToken.UserID)
	}
	if resetToken.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", resetToken.Email)
	}
}

func TestPasswordResetManager_ValidateToken_Invalid(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	_, err := mgr.ValidateToken("nonexistent-token")
	if err != ErrInvalidResetToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidResetToken)
	}
}

func TestPasswordResetManager_TokenExpiry(t *testing.T) {
	mgr := NewPasswordResetManager(PasswordResetConfig{
		TokenDuration: 1 * time.Millisecond, // Very short for testing
	})

	token, _ := mgr.GenerateToken(123, "test@example.com")

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	_, err := mgr.ValidateToken(token)
	if err != ErrInvalidResetToken {
		t.Errorf("ValidateToken() error = %v, want %v (expired)", err, ErrInvalidResetToken)
	}
}

func TestPasswordResetManager_UseToken(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	token, _ := mgr.GenerateToken(123, "test@example.com")

	// First use should succeed
	err := mgr.UseToken(token)
	if err != nil {
		t.Fatalf("UseToken() error = %v", err)
	}

	// Second use should fail
	err = mgr.UseToken(token)
	if err != ErrResetTokenUsed {
		t.Errorf("UseToken() second call error = %v, want %v", err, ErrResetTokenUsed)
	}

	// Validation should also fail
	_, err = mgr.ValidateToken(token)
	if err != ErrResetTokenUsed {
		t.Errorf("ValidateToken() after use error = %v, want %v", err, ErrResetTokenUsed)
	}
}

func TestPasswordResetManager_InvalidatesPreviousToken(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	// Generate first token
	token1, _ := mgr.GenerateToken(123, "test@example.com")

	// Generate second token for same email
	token2, _ := mgr.GenerateToken(123, "test@example.com")

	// First token should be invalid
	_, err := mgr.ValidateToken(token1)
	if err != ErrInvalidResetToken {
		t.Errorf("First token should be invalidated, error = %v", err)
	}

	// Second token should be valid
	_, err = mgr.ValidateToken(token2)
	if err != nil {
		t.Errorf("Second token should be valid, error = %v", err)
	}
}

func TestPasswordResetManager_InvalidateToken(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	token, _ := mgr.GenerateToken(123, "test@example.com")

	// Invalidate by email
	mgr.InvalidateToken("test@example.com")

	// Token should now be invalid
	_, err := mgr.ValidateToken(token)
	if err != ErrInvalidResetToken {
		t.Errorf("ValidateToken() after invalidate error = %v, want %v", err, ErrInvalidResetToken)
	}
}

func TestPasswordResetManager_DifferentEmails(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	token1, _ := mgr.GenerateToken(1, "user1@example.com")
	token2, _ := mgr.GenerateToken(2, "user2@example.com")

	// Both should be valid
	rt1, err := mgr.ValidateToken(token1)
	if err != nil {
		t.Errorf("Token1 error = %v", err)
	}
	if rt1.UserID != 1 {
		t.Errorf("Token1 UserID = %d, want 1", rt1.UserID)
	}

	rt2, err := mgr.ValidateToken(token2)
	if err != nil {
		t.Errorf("Token2 error = %v", err)
	}
	if rt2.UserID != 2 {
		t.Errorf("Token2 UserID = %d, want 2", rt2.UserID)
	}
}

func TestPasswordResetManager_GetTokenInfo(t *testing.T) {
	mgr := NewPasswordResetManager(DefaultPasswordResetConfig())

	token, _ := mgr.GenerateToken(123, "test@example.com")

	info := mgr.GetTokenInfo(token)
	if info == nil {
		t.Fatal("GetTokenInfo() returned nil")
	}

	if info.UserID != 123 {
		t.Errorf("UserID = %d, want 123", info.UserID)
	}

	// Nonexistent token
	info = mgr.GetTokenInfo("nonexistent")
	if info != nil {
		t.Error("GetTokenInfo() should return nil for nonexistent token")
	}
}

func TestResetToken_IsValid(t *testing.T) {
	// Valid token
	token := &ResetToken{
		ExpiresAt: time.Now().Add(time.Hour),
		Used:      false,
	}
	if !token.IsValid() {
		t.Error("Token should be valid")
	}

	// Expired token
	token.ExpiresAt = time.Now().Add(-time.Hour)
	if token.IsValid() {
		t.Error("Expired token should not be valid")
	}

	// Used token
	token.ExpiresAt = time.Now().Add(time.Hour)
	token.Used = true
	if token.IsValid() {
		t.Error("Used token should not be valid")
	}
}
