package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Password reset errors
var (
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
	ErrResetTokenUsed    = errors.New("reset token has already been used")
)

// ResetToken represents a password reset token.
type ResetToken struct {
	Token     string
	Email     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

// IsExpired returns true if the token has expired.
func (t *ResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid returns true if the token is valid (not expired and not used).
func (t *ResetToken) IsValid() bool {
	return !t.IsExpired() && !t.Used
}

// PasswordResetManager handles password reset token generation and validation.
type PasswordResetManager struct {
	mu       sync.RWMutex
	tokens   map[string]*ResetToken // token -> ResetToken
	byEmail  map[string]string      // email -> token (latest)
	duration time.Duration
}

// PasswordResetConfig holds configuration for password reset.
type PasswordResetConfig struct {
	// TokenDuration is how long reset tokens are valid
	TokenDuration time.Duration
}

// DefaultPasswordResetConfig returns sensible defaults.
func DefaultPasswordResetConfig() PasswordResetConfig {
	return PasswordResetConfig{
		TokenDuration: 1 * time.Hour,
	}
}

// NewPasswordResetManager creates a new password reset manager.
func NewPasswordResetManager(cfg PasswordResetConfig) *PasswordResetManager {
	if cfg.TokenDuration == 0 {
		cfg.TokenDuration = 1 * time.Hour
	}

	mgr := &PasswordResetManager{
		tokens:   make(map[string]*ResetToken),
		byEmail:  make(map[string]string),
		duration: cfg.TokenDuration,
	}

	// Start cleanup goroutine
	go mgr.cleanupLoop()

	return mgr
}

// GenerateToken creates a new password reset token for a user.
// Any existing token for the same email is invalidated.
func (m *PasswordResetManager) GenerateToken(userID int64, email string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	now := time.Now()
	resetToken := &ResetToken{
		Token:     token,
		Email:     email,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(m.duration),
		Used:      false,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Invalidate any existing token for this email
	if oldToken, exists := m.byEmail[email]; exists {
		delete(m.tokens, oldToken)
	}

	// Store new token
	m.tokens[token] = resetToken
	m.byEmail[email] = token

	return token, nil
}

// ValidateToken validates a reset token and returns the associated user info.
// The token is NOT marked as used - call UseToken after successful password reset.
func (m *PasswordResetManager) ValidateToken(token string) (*ResetToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resetToken, exists := m.tokens[token]
	if !exists {
		return nil, ErrInvalidResetToken
	}

	if resetToken.Used {
		return nil, ErrResetTokenUsed
	}

	if resetToken.IsExpired() {
		return nil, ErrInvalidResetToken
	}

	return resetToken, nil
}

// UseToken marks a token as used after successful password reset.
func (m *PasswordResetManager) UseToken(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resetToken, exists := m.tokens[token]
	if !exists {
		return ErrInvalidResetToken
	}

	if resetToken.Used {
		return ErrResetTokenUsed
	}

	resetToken.Used = true
	return nil
}

// InvalidateToken removes a token (e.g., when user changes password normally).
func (m *PasswordResetManager) InvalidateToken(email string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if token, exists := m.byEmail[email]; exists {
		delete(m.tokens, token)
		delete(m.byEmail, email)
	}
}

// GetTokenInfo returns information about a token (for admin/debugging).
func (m *PasswordResetManager) GetTokenInfo(token string) *ResetToken {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if t, exists := m.tokens[token]; exists {
		// Return a copy
		return &ResetToken{
			Token:     t.Token,
			Email:     t.Email,
			UserID:    t.UserID,
			CreatedAt: t.CreatedAt,
			ExpiresAt: t.ExpiresAt,
			Used:      t.Used,
		}
	}
	return nil
}

// cleanupLoop periodically removes expired tokens.
func (m *PasswordResetManager) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup removes expired and used tokens.
func (m *PasswordResetManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, resetToken := range m.tokens {
		// Remove if expired or used (and older than 1 hour)
		if resetToken.IsExpired() || (resetToken.Used && now.Sub(resetToken.CreatedAt) > time.Hour) {
			delete(m.tokens, token)
			if m.byEmail[resetToken.Email] == token {
				delete(m.byEmail, resetToken.Email)
			}
		}
	}
}

// PasswordResetService provides high-level password reset operations.
type PasswordResetService struct {
	resetMgr    *PasswordResetManager
	userService *UserService
	auditLogger *AuditLogger
}

// NewPasswordResetService creates a new password reset service.
func NewPasswordResetService(resetMgr *PasswordResetManager, userService *UserService, auditLogger *AuditLogger) *PasswordResetService {
	return &PasswordResetService{
		resetMgr:    resetMgr,
		userService: userService,
		auditLogger: auditLogger,
	}
}

// RequestReset initiates a password reset for an email.
// Returns the reset token (for sending via email) and user info.
// Returns nil, nil if the email doesn't exist (to prevent email enumeration).
func (s *PasswordResetService) RequestReset(ctx context.Context, email string) (string, *User, error) {
	// Look up user by email
	user, err := s.userService.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal if email exists
		return "", nil, nil
	}

	if !user.Active {
		// Don't allow reset for disabled accounts
		return "", nil, nil
	}

	// Generate token
	token, err := s.resetMgr.GenerateToken(user.ID, email)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// CompleteReset validates the token and resets the password.
func (s *PasswordResetService) CompleteReset(ctx context.Context, token, newPassword string) error {
	// Validate token
	resetToken, err := s.resetMgr.ValidateToken(token)
	if err != nil {
		return err
	}

	// Update password using SetPassword (no old password required for reset)
	if err := s.userService.SetPassword(ctx, resetToken.UserID, newPassword); err != nil {
		return err
	}

	// Mark token as used
	if err := s.resetMgr.UseToken(token); err != nil {
		return err
	}

	// Invalidate any other tokens for this email
	s.resetMgr.InvalidateToken(resetToken.Email)

	return nil
}
