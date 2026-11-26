package auth

import (
	"context"
	"testing"
	"time"
)

// MockUserStore implements UserStore for testing.
type MockUserStore struct {
	users     map[int64]*User
	byEmail   map[string]*User
	nextID    int64
	createErr error
	updateErr error
}

func NewMockUserStore() *MockUserStore {
	return &MockUserStore{
		users:   make(map[int64]*User),
		byEmail: make(map[string]*User),
		nextID:  1,
	}
}

func (s *MockUserStore) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MockUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user, ok := s.byEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MockUserStore) CreateUser(ctx context.Context, user *User) error {
	if s.createErr != nil {
		return s.createErr
	}
	user.ID = s.nextID
	s.nextID++
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	s.users[user.ID] = user
	s.byEmail[user.Email] = user
	return nil
}

func (s *MockUserStore) UpdateUser(ctx context.Context, user *User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.users[user.ID] = user
	s.byEmail[user.Email] = user
	return nil
}

func (s *MockUserStore) DeleteUser(ctx context.Context, id int64) error {
	user, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	delete(s.users, id)
	delete(s.byEmail, user.Email)
	return nil
}

func (s *MockUserStore) ListUsers(ctx context.Context, filter UserFilter) ([]*User, error) {
	var result []*User
	for _, user := range s.users {
		if filter.Role != "" && user.Role != filter.Role {
			continue
		}
		if filter.Active != nil && user.Active != *filter.Active {
			continue
		}
		result = append(result, user)
	}
	return result, nil
}

func TestUserService_CreateUser(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	user, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})

	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if user.ID == 0 {
		t.Error("expected user ID to be set")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", user.Email)
	}
	if user.Role != RoleOperator {
		t.Errorf("expected role 'operator', got %q", user.Role)
	}
	if !user.Active {
		t.Error("expected user to be active")
	}
}

func TestUserService_CreateUser_InvalidRole(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     "invalid_role",
	})

	if err != ErrInvalidRole {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestUserService_CreateUser_ShortPassword(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "short",
		Role:     RoleOperator,
	})

	if err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestUserService_CreateUser_Duplicate(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	// Create first user
	_, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})
	if err != nil {
		t.Fatalf("first CreateUser() error = %v", err)
	}

	// Try to create duplicate
	_, err = svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Another User",
		Password: "password456",
		Role:     RoleViewer,
	})

	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

func TestUserService_Authenticate(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	// Create user
	_, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Authenticate with correct password
	user, err := svc.Authenticate(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", user.Email)
	}

	// Authenticate with wrong password
	_, err = svc.Authenticate(ctx, "test@example.com", "wrongpassword")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestUserService_DisableEnable(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	// Create user
	user, _ := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})

	// Disable
	err := svc.DisableUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DisableUser() error = %v", err)
	}

	// Check disabled
	user, _ = svc.GetUser(ctx, user.ID)
	if user.Active {
		t.Error("expected user to be disabled")
	}

	// Try to authenticate
	_, err = svc.Authenticate(ctx, "test@example.com", "password123")
	if err != ErrAccountDisabled {
		t.Errorf("expected ErrAccountDisabled, got %v", err)
	}

	// Enable
	err = svc.EnableUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("EnableUser() error = %v", err)
	}

	// Check enabled
	user, _ = svc.GetUser(ctx, user.ID)
	if !user.Active {
		t.Error("expected user to be enabled")
	}
}

func TestUserService_SetRole(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	// Create user as operator
	user, _ := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})

	// Change to admin
	err := svc.SetRole(ctx, user.ID, RoleAdmin)
	if err != nil {
		t.Fatalf("SetRole() error = %v", err)
	}

	// Verify
	user, _ = svc.GetUser(ctx, user.ID)
	if user.Role != RoleAdmin {
		t.Errorf("expected role 'admin', got %q", user.Role)
	}

	// Try invalid role
	err = svc.SetRole(ctx, user.ID, "superuser")
	if err != ErrInvalidRole {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	store := NewMockUserStore()
	svc := NewUserService(store, DefaultConfig())
	ctx := context.Background()

	// Create user
	user, _ := svc.CreateUser(ctx, CreateUserInput{
		TenantID: 1,
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     RoleOperator,
	})

	// Change password with correct old password
	err := svc.ChangePassword(ctx, user.ID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	// Authenticate with new password
	_, err = svc.Authenticate(ctx, "test@example.com", "newpassword456")
	if err != nil {
		t.Errorf("expected successful auth with new password, got %v", err)
	}

	// Old password should fail
	_, err = svc.Authenticate(ctx, "test@example.com", "password123")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword for old password, got %v", err)
	}

	// Wrong old password should fail
	err = svc.ChangePassword(ctx, user.ID, "wrongpassword", "anotherpassword")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()
	if len(roles) != 8 {
		t.Errorf("expected 8 roles, got %d", len(roles))
	}

	expected := map[string]bool{
		RoleDeveloper:        true,
		RoleAdmin:            true,
		RoleManager:          true,
		RoleOperator:         true,
		RoleViewer:           true,
		RoleAdminStaff:       true,
		RoleAccountant:       true,
		RoleAccountExecutive: true,
	}

	for _, role := range roles {
		if !expected[role] {
			t.Errorf("unexpected role: %s", role)
		}
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "testpassword123"

	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if hash == password {
		t.Error("hash should not equal password")
	}

	// Verify correct password
	if !verifyPassword(password, hash) {
		t.Error("verifyPassword should return true for correct password")
	}

	// Verify wrong password
	if verifyPassword("wrongpassword", hash) {
		t.Error("verifyPassword should return false for wrong password")
	}

	// Verify empty hash
	if verifyPassword(password, "") {
		t.Error("verifyPassword should return false for empty hash")
	}
}
