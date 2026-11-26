package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	xoluClient *xolu.Client
	auth       *auth.Service
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(xoluClient *xolu.Client, authService *auth.Service) *AuthHandler {
	return &AuthHandler{
		xoluClient: xoluClient,
		auth:       authService,
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TenantID int64  `json:"tenant_id,omitempty"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

// UserInfo represents user information in responses.
type UserInfo struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	TenantID int64  `json:"tenant_id"`
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "email and password required")
		return
	}

	// Find user by email
	ctx := r.Context()
	query := "SELECT TOP 1 * FROM users WHERE email = '" + req.Email + "'"
	if req.TenantID > 0 {
		query = "SELECT TOP 1 * FROM users WHERE email = '" + req.Email + "' AND tenant_id = " + formatInt64(req.TenantID)
	}

	result, err := h.xoluClient.OQL(ctx, query)
	if err != nil || len(result.Data) == 0 {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	userData := result.Data[0]

	// Check password (in production, use bcrypt)
	storedHash, _ := userData["password_hash"].(string)
	if !verifyPassword(req.Password, storedHash) {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	// Check if user is active
	if active, ok := userData["active"].(bool); ok && !active {
		WriteError(w, http.StatusForbidden, "forbidden", "account disabled")
		return
	}

	// Extract user info
	userID := int64(userData["id"].(float64))
	tenantID := int64(userData["tenant_id"].(float64))
	email, _ := userData["email"].(string)
	name, _ := userData["name"].(string)
	role, _ := userData["role"].(string)

	// Generate tokens
	accessClaims := &auth.Claims{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		TokenType: auth.TokenTypeAccess,
	}

	accessToken, err := h.auth.GenerateToken(accessClaims)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}

	refreshClaims := &auth.Claims{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		TokenType: auth.TokenTypeRefresh,
	}

	refreshToken, err := h.auth.GenerateToken(refreshClaims)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}

	// Update last login
	h.xoluClient.Update(ctx, "users", userID, map[string]interface{}{
		"last_login": time.Now(),
	})

	response := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessClaims.ExpiresAt,
		User: UserInfo{
			ID:       userID,
			Email:    email,
			Name:     name,
			Role:     role,
			TenantID: tenantID,
		},
	}

	WriteJSON(w, http.StatusOK, response)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "refresh_token required")
		return
	}

	// Validate refresh token
	claims, err := h.auth.ValidateToken(req.RefreshToken)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}

	if claims.TokenType != auth.TokenTypeRefresh {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid token type")
		return
	}

	// Generate new access token
	accessClaims := &auth.Claims{
		UserID:    claims.UserID,
		TenantID:  claims.TenantID,
		Email:     claims.Email,
		Role:      claims.Role,
		TokenType: auth.TokenTypeAccess,
	}

	accessToken, err := h.auth.GenerateToken(accessClaims)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"expires_at":   accessClaims.ExpiresAt,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// In a production system, you would:
	// 1. Add the token to a blacklist
	// 2. Clear any server-side sessions
	// For now, just return success (client should discard tokens)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "logged out successfully",
	})
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	// Fetch full user data
	ctx := r.Context()
	entity, err := h.xoluClient.Get(ctx, "users", claims.UserID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	// Remove sensitive fields
	delete(entity.Data, "password_hash")

	WriteJSON(w, http.StatusOK, entity.Data)
}

// ChangePassword handles POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "current_password and new_password required")
		return
	}

	if len(req.NewPassword) < 8 {
		WriteError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		return
	}

	// Get current user
	ctx := r.Context()
	entity, err := h.xoluClient.Get(ctx, "users", claims.UserID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	// Verify current password
	storedHash, _ := entity.Data["password_hash"].(string)
	if !verifyPassword(req.CurrentPassword, storedHash) {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "current password incorrect")
		return
	}

	// Update password
	newHash := hashPassword(req.NewPassword)
	_, err = h.xoluClient.Update(ctx, "users", claims.UserID, map[string]interface{}{
		"password_hash": newHash,
		"updated_at":    time.Now(),
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update password")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "password changed successfully",
	})
}

// hashPassword creates a password hash (simplified - use bcrypt in production).
func hashPassword(password string) string {
	// In production: bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return "hash:" + password
}

// verifyPassword verifies a password against a hash (simplified - use bcrypt in production).
func verifyPassword(password, hash string) bool {
	// In production: bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return hash == "hash:"+password
}

// formatInt64 formats an int64 as a string.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}
