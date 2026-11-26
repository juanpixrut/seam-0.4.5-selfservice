package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// XOLUClient defines the interface for xolu operations needed by the user store.
type XOLUClient interface {
	Get(ctx context.Context, entityType string, id int64) (*XOLUEntity, error)
	Create(ctx context.Context, entityType string, data map[string]interface{}) (*XOLUEntity, error)
	Update(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error)
	Delete(ctx context.Context, entityType string, id int64) error
	OQL(ctx context.Context, query string) (*OQLResult, error)
}

// XOLUEntity represents an entity from xolu.
type XOLUEntity struct {
	ID        int64
	Data      map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OQLResult represents a query result from xolu.
// Data matches the "data" field in xolu's OQL response envelope.
type OQLResult struct {
	Data []map[string]interface{}
}

// XOLUUserStore implements UserStore using xolu as the backend.
type XOLUUserStore struct {
	client XOLUClient
}

// NewXOLUUserStore creates a new xolu-backed user store.
func NewXOLUUserStore(client XOLUClient) *XOLUUserStore {
	return &XOLUUserStore{client: client}
}

// GetUserByID retrieves a user by ID.
func (s *XOLUUserStore) GetUserByID(ctx context.Context, id int64) (*User, error) {
	entity, err := s.client.Get(ctx, "users", id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return entityToUser(entity), nil
}

// GetUserByEmail retrieves a user by email.
func (s *XOLUUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	result, err := s.client.OQL(ctx, fmt.Sprintf("SELECT TOP 1 * FROM users WHERE email = '%s'", escapeOQL(email)))
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, ErrUserNotFound
	}
	return rowToUser(result.Data[0]), nil
}

// CreateUser creates a new user.
func (s *XOLUUserStore) CreateUser(ctx context.Context, user *User) error {
	data := userToData(user)
	entity, err := s.client.Create(ctx, "users", data)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	user.ID = entity.ID
	user.CreatedAt = entity.CreatedAt
	user.UpdatedAt = entity.UpdatedAt
	return nil
}

// xoluPatcher is what a client offers when it can merge fields into a record
// instead of replacing it.
type xoluPatcher interface {
	Patch(ctx context.Context, entityType string, id int64, data map[string]interface{}) (*XOLUEntity, error)
}

// UpdateUser updates an existing user. The fields the store knows are merged
// into the record where the client can merge: a user record also carries
// fields this store does not know (what other parts of the application keep
// with the account), and replacing the record -- which every sign-in did,
// to note its time -- removed them.
func (s *XOLUUserStore) UpdateUser(ctx context.Context, user *User) error {
	data := userToData(user)
	var err error
	if p, ok := s.client.(xoluPatcher); ok {
		_, err = p.Patch(ctx, "users", user.ID, data)
	} else {
		_, err = s.client.Update(ctx, "users", user.ID, data)
	}
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// DeleteUser deletes a user.
func (s *XOLUUserStore) DeleteUser(ctx context.Context, id int64) error {
	return s.client.Delete(ctx, "users", id)
}

// ListUsers lists users with filtering.
func (s *XOLUUserStore) ListUsers(ctx context.Context, filter UserFilter) ([]*User, error) {
	// A non-nil empty Roles slice means "no role is allowed": answer with
	// no rows instead of building a query without a role condition.
	if filter.Roles != nil && len(filter.Roles) == 0 {
		return []*User{}, nil
	}

	// OQL parses T-SQL: a row cap is TOP n in the SELECT clause, and there is
	// no OFFSET. With a Limit set, fetch TOP (limit+offset) rows and drop the
	// first Offset of them after the query (same approach as alerts.List and
	// the audit store).
	fetchN := 0
	if filter.Limit > 0 {
		fetchN = filter.Limit + filter.Offset
	}
	query := "SELECT * FROM users"
	if fetchN > 0 {
		query = fmt.Sprintf("SELECT TOP %d * FROM users", fetchN)
	}
	var conditions []string

	if filter.TenantID > 0 {
		conditions = append(conditions, fmt.Sprintf("tenant_id = %d", filter.TenantID))
	}
	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = '%s'", escapeOQL(filter.Role)))
	}
	if len(filter.Roles) > 0 {
		parts := make([]string, len(filter.Roles))
		for i, r := range filter.Roles {
			parts[i] = fmt.Sprintf("role = '%s'", escapeOQL(r))
		}
		conditions = append(conditions, "("+strings.Join(parts, " OR ")+")")
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		e := escapeOQL(q)
		conditions = append(conditions, fmt.Sprintf("(name LIKE '%%%s%%' OR email LIKE '%%%s%%')", e, e))
	}
	if filter.Active != nil {
		conditions = append(conditions, fmt.Sprintf("active = %t", *filter.Active))
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, c := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	query += " ORDER BY name ASC"

	result, err := s.client.OQL(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	// OQL has no OFFSET: drop the first filter.Offset rows here instead.
	data := result.Data
	if filter.Offset > 0 {
		if filter.Offset >= len(data) {
			data = nil
		} else {
			data = data[filter.Offset:]
		}
	}
	users := make([]*User, len(data))
	for i, row := range data {
		users[i] = rowToUser(row)
	}
	return users, nil
}

// entityToUser converts an xolu entity to a User.
func entityToUser(entity *XOLUEntity) *User {
	if entity == nil {
		return nil
	}
	user := rowToUser(entity.Data)
	user.ID = entity.ID
	user.CreatedAt = entity.CreatedAt
	user.UpdatedAt = entity.UpdatedAt
	return user
}

// rowToUser converts a query row to a User.
func rowToUser(data map[string]interface{}) *User {
	user := &User{
		ID:       toInt64Val(data["id"]),
		TenantID: toInt64Val(data["tenant_id"]),
		Email:    toStringVal(data["email"]),
		Name:     toStringVal(data["name"]),
		Role:     toStringVal(data["role"]),
		Active:   toBoolVal(data["active"]),
	}

	// Parse password hash
	if hash, ok := data["password_hash"].(string); ok {
		user.passwordHash = hash
	}

	// Parse preferences
	if prefs, ok := data["preferences"].(map[string]interface{}); ok {
		user.Preferences = &UserPreferences{
			Locale:      toStringVal(prefs["locale"]),
			Timezone:    toStringVal(prefs["timezone"]),
			DarkMode:    toStringVal(prefs["dark_mode"]),
			EmailAlerts: toBoolVal(prefs["email_alerts"]),
			CompactView: toBoolVal(prefs["compact_view"]),
		}
	}

	// Parse timestamps
	if ts, ok := data["created_at"].(string); ok {
		user.CreatedAt, _ = time.Parse(time.RFC3339, ts)
	}
	if ts, ok := data["updated_at"].(string); ok {
		user.UpdatedAt, _ = time.Parse(time.RFC3339, ts)
	}
	if ts, ok := data["last_login_at"].(string); ok {
		user.LastLoginAt, _ = time.Parse(time.RFC3339, ts)
	}

	return user
}

// userToData converts a User to xolu data map.
func userToData(user *User) map[string]interface{} {
	data := map[string]interface{}{
		"tenant_id":  user.TenantID,
		"email":      user.Email,
		"name":       user.Name,
		"role":       user.Role,
		"active":     user.Active,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	if user.passwordHash != "" {
		data["password_hash"] = user.passwordHash
	}

	if user.Preferences != nil {
		data["preferences"] = map[string]interface{}{
			"locale":       user.Preferences.Locale,
			"timezone":     user.Preferences.Timezone,
			"dark_mode":    user.Preferences.DarkMode,
			"email_alerts": user.Preferences.EmailAlerts,
			"compact_view": user.Preferences.CompactView,
		}
	}

	if !user.CreatedAt.IsZero() {
		data["created_at"] = user.CreatedAt.Format(time.RFC3339)
	}

	if !user.LastLoginAt.IsZero() {
		data["last_login_at"] = user.LastLoginAt.Format(time.RFC3339)
	}

	return data
}

// escapeOQL escapes a string for OQL queries.
func escapeOQL(s string) string {
	// Simple escape - replace single quotes with doubled quotes
	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// Helper conversion functions
func toInt64Val(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func toStringVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toBoolVal(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// Ensure XOLUUserStore implements UserStore
var _ UserStore = (*XOLUUserStore)(nil)

// ErrNoStore is returned when no user store is configured.
var ErrNoStore = errors.New("no user store configured")
