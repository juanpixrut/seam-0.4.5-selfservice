package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Common user management errors
var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrAccountDisabled  = errors.New("account is disabled")
	ErrInvalidRole      = errors.New("invalid role")
)

// UserStore defines the interface for user persistence.
// This allows the auth package to work with any storage backend.
type UserStore interface {
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id int64) error
	ListUsers(ctx context.Context, filter UserFilter) ([]*User, error)
}

// UserFilter for listing users.
type UserFilter struct {
	TenantID int64
	Role     string
	// Roles restricts the result to users whose role is one of the listed
	// values (OR between them, AND with the other conditions). nil means
	// no restriction; a non-nil EMPTY slice means "match nothing" (the
	// store returns no rows without querying), so an unknown caller role
	// from RolesAtOrBelow can never widen into "list everybody".
	Roles []string
	// Search is a case-sensitive substring match on name or email
	// (LIKE '%q%', same semantics as the asset picker). Empty or
	// whitespace-only means no search condition.
	Search string
	Active *bool
	Limit  int
	Offset int
}

// UserService provides user management operations.
type UserService struct {
	store  UserStore
	config Config
}

// NewUserService creates a new user service.
func NewUserService(store UserStore, cfg Config) *UserService {
	return &UserService{
		store:  store,
		config: cfg,
	}
}

// CreateUser creates a new user account.
func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	// Validate role
	if !IsValidRole(input.Role) {
		return nil, ErrInvalidRole
	}

	// Validate password
	if len(input.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// Check if user exists
	existing, err := s.store.GetUserByEmail(ctx, input.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	// Hash password
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now().UTC()
	user := &User{
		TenantID:    input.TenantID,
		Email:       input.Email,
		Name:        input.Name,
		Role:        input.Role,
		Active:      true,
		Preferences: DefaultUserPreferences(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Store password hash in a way that can be retrieved
	// We'll use a custom field that gets persisted
	user.passwordHash = passwordHash

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

// CreateUserInput holds data for creating a user.
type CreateUserInput struct {
	TenantID int64
	Email    string
	Name     string
	Password string
	Role     string
}

// Authenticate validates credentials and returns the user.
func (s *UserService) Authenticate(ctx context.Context, email, password string) (*User, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidPassword // Don't reveal if user exists
	}

	if !user.Active {
		return nil, ErrAccountDisabled
	}

	if !verifyPassword(password, user.passwordHash) {
		return nil, ErrInvalidPassword
	}

	// Update last login
	user.LastLoginAt = time.Now().UTC()
	_ = s.store.UpdateUser(ctx, user)

	return user, nil
}

// SetPassword changes a user's password.
func (s *UserService) SetPassword(ctx context.Context, userID int64, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user.passwordHash = passwordHash
	user.UpdatedAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// ChangePassword changes password with old password verification.
func (s *UserService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if !verifyPassword(oldPassword, user.passwordHash) {
		return ErrInvalidPassword
	}

	return s.SetPassword(ctx, userID, newPassword)
}

// SetRole changes a user's role.
func (s *UserService) SetRole(ctx context.Context, userID int64, role string) error {
	if !IsValidRole(role) {
		return ErrInvalidRole
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.Role = role
	user.UpdatedAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// EnableUser enables a user account.
func (s *UserService) EnableUser(ctx context.Context, userID int64) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.Active = true
	user.UpdatedAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// DisableUser disables a user account.
func (s *UserService) DisableUser(ctx context.Context, userID int64) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.Active = false
	user.UpdatedAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// UpdateLastLogin updates the user's last login timestamp.
func (s *UserService) UpdateLastLogin(ctx context.Context, userID int64) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.LastLoginAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, userID int64) (*User, error) {
	return s.store.GetUserByID(ctx, userID)
}

// GetUserByEmail retrieves a user by email.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.store.GetUserByEmail(ctx, email)
}

// ListUsers lists users with optional filtering.
func (s *UserService) ListUsers(ctx context.Context, filter UserFilter) ([]*User, error) {
	return s.store.ListUsers(ctx, filter)
}

// UpdatePreferences updates a user's preferences.
func (s *UserService) UpdatePreferences(ctx context.Context, userID int64, prefs *UserPreferences) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.Preferences = prefs
	user.UpdatedAt = time.Now().UTC()

	return s.store.UpdateUser(ctx, user)
}

// IsValidRole checks if a role is one of the known roles (developer, admin,
// manager, operator, viewer, and the Accounting & Admin family:
// admin_staff, accountant, account_executive). Exported 2026-08-04 so callers outside
// this package can use the same check UserService.CreateUser/UpdateRole
// already enforce internally, rather than duplicating the role list --
// found necessary when internal/ui's AdminUserCreate handler was found,
// via adversarial testing, to have a fallback code path (h.userService
// == nil) that wrote a submitted role value straight to storage with no
// validation at all, unlike every other path to a user's role.
func IsValidRole(role string) bool {
	switch role {
	case RoleDeveloper, RoleAdmin, RoleManager, RoleOperator, RoleViewer,
		RoleAdminStaff, RoleAccountant, RoleAccountExecutive:
		return true
	default:
		return false
	}
}

// ValidRoles returns the list of valid roles.
func ValidRoles() []string {
	return []string{RoleDeveloper, RoleAdmin, RoleManager, RoleOperator, RoleViewer,
		RoleAdminStaff, RoleAccountant, RoleAccountExecutive}
}

// BcryptCost defines the bcrypt cost factor.
// 10 is the default, higher is more secure but slower.
const BcryptCost = 10

// hashPassword creates a bcrypt hash of the password.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// verifyPassword checks if password matches the bcrypt hash.
func verifyPassword(password, storedHash string) bool {
	if storedHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	return err == nil
}

// SetPasswordHash sets the password hash for a user.
func (u *User) SetPasswordHash(hash string) {
	u.passwordHash = hash
}

// GetPasswordHash gets the password hash for a user.
func (u *User) GetPasswordHash() string {
	return u.passwordHash
}

// Add passwordHash field to User struct via embedding
// (This is added to the User type in auth.go)
