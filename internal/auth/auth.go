// Package auth provides authentication and authorisation for Seam AMS.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Common errors
var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("token expired")
	ErrInvalidAPIKey  = errors.New("invalid API key")
	ErrUnauthorised   = errors.New("unauthorised")
	ErrForbidden      = errors.New("forbidden")
	ErrTenantMismatch = errors.New("tenant mismatch")
)

// TokenType represents the type of authentication token.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeAPIKey  TokenType = "api_key"
)

// Claims represents the claims in a JWT token.
type Claims struct {
	UserID    int64     `json:"user_id"`
	TenantID  int64     `json:"tenant_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
}

// Valid checks if the claims are valid.
func (c *Claims) Valid() error {
	if c.UserID == 0 {
		return errors.New("missing user_id")
	}
	if c.TenantID == 0 {
		return errors.New("missing tenant_id")
	}
	if time.Now().After(c.ExpiresAt) {
		return ErrExpiredToken
	}
	return nil
}

// GetTenantID returns the tenant ID from the claims.
// This method allows middleware to extract tenant without knowing the concrete type.
func (c *Claims) GetTenantID() int64 {
	return c.TenantID
}

// HasRole checks if the claims have the specified role.
// An admin, or any role ranked at or above admin (the developer), passes
// every check, as admin always has.
func (c *Claims) HasRole(role string) bool {
	return c.Role == role || RoleRank(c.Role) >= RoleRank(RoleAdmin)
}

// User represents an authenticated user.
type User struct {
	ID           int64            `json:"id"`
	TenantID     int64            `json:"tenant_id"`
	Email        string           `json:"email"`
	Name         string           `json:"name"`
	Role         string           `json:"role"`
	Active       bool             `json:"active"`
	Preferences  *UserPreferences `json:"preferences,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	LastLoginAt  time.Time        `json:"last_login_at,omitempty"`
	passwordHash string           // Not exported in JSON
}

// UserPreferences holds per-user settings.
type UserPreferences struct {
	Locale      string `json:"locale"`       // en, es, pt
	Timezone    string `json:"timezone"`     // e.g., America/Montevideo
	DarkMode    string `json:"dark_mode"`    // system, light, dark
	EmailAlerts bool   `json:"email_alerts"` // receive email notifications
	CompactView bool   `json:"compact_view"` // use compact list views
}

// DefaultUserPreferences returns sensible defaults.
func DefaultUserPreferences() *UserPreferences {
	return &UserPreferences{
		Locale:      "en",
		Timezone:    "UTC",
		DarkMode:    "system",
		EmailAlerts: true,
		CompactView: false,
	}
}

// =============================================================================
// ROLES AND PERMISSIONS (RBAC foundation)
// =============================================================================

// Role constants - extensible for future use
const (
	RoleAdmin    = "admin"    // Full access to everything
	RoleManager  = "manager"  // Can manage assets, rules, users in tenant
	RoleOperator = "operator" // Can view and update assets, events
	RoleViewer   = "viewer"   // Read-only access

	// RoleDeveloper is an admin who additionally has access to the Developer
	// module (campaign DEV-6). Its permissions are derived from the admin
	// role's in roles.go, so the two cannot drift apart. It outranks admin.
	RoleDeveloper = "developer"

	// Roles of the Accounting & Admin module (Seam Admin, docs/ACCOUNTING_ADMIN_MODULE.md).
	// They are a separate family from the four roles above: none of them
	// grants anything in Assets, and the four roles above grant nothing in
	// Accounting except admin, which holds every permission everywhere.
	RoleAdminStaff       = "admin_staff"       // Seam administrative personnel: authorizes AEs, marks items Collected, loads payment proofs
	RoleAccountant       = "accountant"        // Keeps the books: ledger, reversals, monthly closes
	RoleAccountExecutive = "account_executive" // A real AE (salesperson): sees and settles only its own commissions
)

// Permission represents a specific action that can be controlled, in
// "module.view.action" form (MODULE_SPEC.md §2.1) -- e.g. "assets.asset.view".
// A granted pattern (as stored in rolePermissions) may end in "*" at the
// view or action position ("assets.*", "assets.asset.*"); HasPermission
// resolves a granted pattern against a requested, always-concrete
// permission per §2.3's matching algorithm. No implicit trailing wildcard:
// a two-segment grant with no "*" is not a valid pattern, only ever
// written that way in this file with an explicit "*" third segment.
type Permission string

const (
	// Assets module -- asset view (MODULE_SPEC.md §6.2; was asset:*)
	PermAssetsAssetCreate Permission = "assets.asset.create" // was PermAssetCreate
	PermAssetsAssetView   Permission = "assets.asset.view"   // was PermAssetRead
	PermAssetsAssetEdit   Permission = "assets.asset.edit"   // was PermAssetUpdate
	PermAssetsAssetDelete Permission = "assets.asset.delete" // was PermAssetDelete

	// Assets module -- rule view, also covers FSM (already shared PermRule*
	// before this reshape -- internal/handlers/api/router.go's FSM routes
	// were never separately permissioned; preserved as-is, not newly split)
	PermAssetsRuleCreate Permission = "assets.rule.create" // was PermRuleCreate
	PermAssetsRuleView   Permission = "assets.rule.view"   // was PermRuleRead
	PermAssetsRuleEdit   Permission = "assets.rule.edit"   // was PermRuleUpdate
	PermAssetsRuleDelete Permission = "assets.rule.delete" // was PermRuleDelete

	// Assets module -- master data (regions, currencies, brands, custodians,
	// alert types, properties, models, locations, property associations).
	// New, real permission replacing the reused PermSettingsUpdate found
	// gating all 48 of these routes in internal/ui/router.go -- confirmed
	// none of them were ever actually about tenant settings (MODULE_SPEC.md
	// §2.5). Only "edit" exists: the list/view routes for master data are
	// requireAuth-only today (unpermissioned), same as /settings itself --
	// not changed here, since adding a new gate to previously-ungated GET
	// routes would be a real behaviour change, not a reshape.
	PermAssetsMasterEdit Permission = "assets.master.edit"

	// Assets module -- webhooks. New, real permission replacing the same
	// reused PermSettingsRead/Update, found separately gating
	// internal/handlers/api/router.go's webhook routes -- a second,
	// distinct conflation under the same old permission, not the same one
	// as master data (confirmed: zero overlap, master data only appears in
	// the UI router, webhooks only in the API router).
	PermAssetsWebhookView Permission = "assets.webhook.view"
	PermAssetsWebhookEdit Permission = "assets.webhook.edit"

	// Assets module -- reports export (was PermDataExport, confirmed real:
	// gates /api/v1/reports/export/{assets,events}). PermDataImport is not
	// migrated -- confirmed dead code, zero usages anywhere outside its own
	// declaration and the rolePermissions lists below.
	PermAssetsReportExport Permission = "assets.report.export"

	// Administration module -- user/role management (was PermUser*,
	// then system.user.* under the single "System" module MOD-5/MOD-8
	// built). Renamed 2026-08-04 when Administration split out into its
	// own registered module (was T-02) -- the string value now matches
	// the module it actually gates, so modules.Registry's generic
	// HasAccess-via-HasModuleAccess derivation (§2.4) works without a
	// custom per-module closure.
	PermAdministrationUserCreate Permission = "administration.user.create"
	PermAdministrationUserView   Permission = "administration.user.view"
	PermAdministrationUserEdit   Permission = "administration.user.edit"
	PermAdministrationUserDelete Permission = "administration.user.delete"

	// Settings module -- tenant configuration. Renamed 2026-08-04
	// alongside PermAdministrationUser*, same reasoning: Settings split
	// out into its own registered module, so the permission string's
	// own module segment needs to say "settings", not "system".
	PermSettingsView Permission = "settings.view"
	PermSettingsEdit Permission = "settings.edit"

	// Documents module -- placeholder permission proving the module exists
	// and is visible under the same module-access derivation rule (§2.4)
	// every other module follows, before any real Documents functionality
	// exists (MODULE_SPEC.md §4.4, MOD-7). Granted to every role.
	PermDocumentsDocumentView Permission = "documents.document.view"

	// Accounting & Admin module (Seam Admin). One view per area of the
	// design (docs/ACCOUNTING_ADMIN_MODULE.md section 3). The "own"
	// qualifier of the AE-facing permissions is not part of the string:
	// handlers scope those routes to the AE linked to the session user,
	// so holding the permission never exposes another AE's data.
	PermAccountingPortalView        Permission = "accounting.portal.view"       // an AE sees its own commissions, account and tenants
	PermAccountingProfileEdit       Permission = "accounting.profile.edit"      // an AE loads and edits its own data
	PermAccountingSettlementCreate  Permission = "accounting.settlement.create" // an AE settles its own Collected lines and uploads its invoice
	PermAccountingSettlementView    Permission = "accounting.settlement.view"   // all settlements
	PermAccountingSettlementPay     Permission = "accounting.settlement.pay"    // load the payment proof
	PermAccountingAEView            Permission = "accounting.ae.view"
	PermAccountingAEAuthorize       Permission = "accounting.ae.authorize"
	PermAccountingAEEdit            Permission = "accounting.ae.edit" // edit an AE's profile and payout details; staff only when configured
	PermAccountingCollectionView    Permission = "accounting.collection.view"
	PermAccountingCollectionCollect Permission = "accounting.collection.collect" // mark a period Collected
	PermAccountingCollectionSync    Permission = "accounting.collection.sync"    // pull from the instance
	PermAccountingCommissionView    Permission = "accounting.commission.view"    // every AE's commission lines
	PermAccountingLedgerView        Permission = "accounting.ledger.view"
	PermAccountingEntryCreate       Permission = "accounting.entry.create"
	PermAccountingEntryReverse      Permission = "accounting.entry.reverse"
	PermAccountingCloseView         Permission = "accounting.close.view"
	PermAccountingCloseCreate       Permission = "accounting.close.create"
	PermAccountingCompanyView       Permission = "accounting.company.view"
	PermAccountingCompanyEdit       Permission = "accounting.company.edit"
	PermAccountingFormulaView       Permission = "accounting.formula.view" // see the formula rules and try them
	PermAccountingFormulaEdit       Permission = "accounting.formula.edit" // create, change, reorder and delete them
)

// rolePermissions defines what each role can do.
// This is the single source of truth for RBAC. Role grants below
// preserve exactly what each role could already do under the old flat
// model -- this migration reshapes the permission string format, it does
// not change who can do what (MODULE_SPEC.md §2.5/§8.2).
var rolePermissions = map[string][]Permission{
	RoleAdmin: {
		PermAssetsAssetCreate, PermAssetsAssetView, PermAssetsAssetEdit, PermAssetsAssetDelete,
		PermAssetsRuleCreate, PermAssetsRuleView, PermAssetsRuleEdit, PermAssetsRuleDelete,
		PermAssetsMasterEdit,
		PermAssetsWebhookView, PermAssetsWebhookEdit,
		PermAssetsReportExport,
		PermAdministrationUserCreate, PermAdministrationUserView, PermAdministrationUserEdit, PermAdministrationUserDelete,
		PermSettingsView, PermSettingsEdit,
		PermDocumentsDocumentView,
		// Every accounting duty of staff. Not the three AE-own permissions
		// (portal, own profile, own settlement): the admin is not an AE, and
		// holding them would only show an AE-only menu entry that answers 403.
		PermAccountingSettlementView, PermAccountingSettlementPay,
		PermAccountingAEView, PermAccountingAEAuthorize, PermAccountingAEEdit,
		PermAccountingCollectionView, PermAccountingCollectionCollect, PermAccountingCollectionSync,
		PermAccountingCommissionView,
		PermAccountingLedgerView, PermAccountingEntryCreate, PermAccountingEntryReverse,
		PermAccountingCloseView, PermAccountingCloseCreate,
		PermAccountingCompanyView, PermAccountingCompanyEdit,
		PermAccountingFormulaView, PermAccountingFormulaEdit,
	},
	RoleManager: {
		PermAssetsAssetCreate, PermAssetsAssetView, PermAssetsAssetEdit, PermAssetsAssetDelete,
		PermAssetsRuleCreate, PermAssetsRuleView, PermAssetsRuleEdit, PermAssetsRuleDelete,
		PermAssetsMasterEdit,
		PermAssetsWebhookView, PermAssetsWebhookEdit,
		PermAssetsReportExport,
		PermAdministrationUserView,
		PermSettingsView, PermSettingsEdit,
		PermDocumentsDocumentView,
	},
	RoleOperator: {
		PermAssetsAssetCreate, PermAssetsAssetView, PermAssetsAssetEdit,
		PermAssetsRuleView,
		PermAssetsWebhookView,
		PermSettingsView,
		PermDocumentsDocumentView,
	},
	RoleViewer: {
		PermAssetsAssetView,
		PermAssetsRuleView,
		PermAssetsWebhookView,
		PermSettingsView,
		PermDocumentsDocumentView,
	},

	// Accounting & Admin family. Granted exactly what the design gives each
	// actor (sections 4.1, 5, 6); nothing in Assets or Settings.
	RoleAdminStaff: {
		PermAccountingAEView, PermAccountingAEAuthorize,
		PermAccountingCollectionView, PermAccountingCollectionCollect, PermAccountingCollectionSync,
		PermAccountingCommissionView,
		PermAccountingSettlementView, PermAccountingSettlementPay,
		PermAccountingLedgerView, // read-only; posting is AccountingGrants.StaffPostEntries
		PermAccountingCompanyView, PermAccountingCompanyEdit,
		PermAccountingFormulaView, // editing is the admin's (AccountingGrants.AccountantEditFormulas for the accountant)
	},
	RoleAccountant: {
		PermAccountingAEView,
		PermAccountingCollectionView,
		PermAccountingCommissionView,
		PermAccountingSettlementView,
		PermAccountingLedgerView, PermAccountingEntryCreate, PermAccountingEntryReverse,
		PermAccountingCloseView, PermAccountingCloseCreate,
		PermAccountingCompanyView,
		PermAccountingFormulaView,
	},
	RoleAccountExecutive: {
		PermAccountingPortalView, PermAccountingProfileEdit, PermAccountingSettlementCreate,
	},
}

// HasPermission checks if a role holds a permission, matching a requested
// concrete "module.view.action" string against each of the role's granted
// patterns (which may end in "*" at the view or action position).
// MODULE_SPEC.md §2.3.
func HasPermission(role string, perm Permission) bool {
	perms, ok := grantsFor(role)
	if !ok {
		return false
	}
	reqSegs := strings.Split(string(perm), ".")
	for _, granted := range perms {
		if permissionMatches(string(granted), reqSegs) {
			return true
		}
	}
	return false
}

// permissionMatches implements MODULE_SPEC.md §2.3's algorithm: walk the
// granted pattern's segments against the requested (always concrete)
// segments; a "*" at any position matches everything from that position
// on; otherwise every segment must match exactly and the granted pattern
// must be fully consumed.
func permissionMatches(granted string, reqSegs []string) bool {
	gSegs := strings.Split(granted, ".")
	for i, g := range gSegs {
		if g == "*" {
			return true
		}
		if i >= len(reqSegs) || g != reqSegs[i] {
			return false
		}
	}
	return len(gSegs) == len(reqSegs)
}

// HasModuleAccess implements MODULE_SPEC.md §2.4's derivation rule: a role
// has access to module m if it holds at least one granted permission
// whose first segment is m. Used by the module registry/selector (MOD-4),
// not by individual route permission checks, which continue to call
// HasPermission with a concrete permission as they always have.
func HasModuleAccess(role, module string) bool {
	perms, ok := grantsFor(role)
	if !ok {
		return false
	}
	for _, granted := range perms {
		if seg, _, _ := strings.Cut(string(granted), "."); seg == module {
			return true
		}
	}
	return false
}

// UserHasPermission checks if a user has a specific permission.
func (u *User) HasPermission(perm Permission) bool {
	return HasPermission(u.Role, perm)
}

// Tenant represents a tenant in the system.
type Tenant struct {
	ID       int64                  `json:"id"`
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Active   bool                   `json:"active"`
	Settings map[string]interface{} `json:"settings"`
}

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	Name      string    `json:"name"`
	KeyHash   string    `json:"-"`
	Prefix    string    `json:"prefix"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// HasScope checks if the API key has the specified scope.
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// Config holds authentication configuration.
type Config struct {
	Secret        string
	TokenExpiry   time.Duration
	RefreshExpiry time.Duration
	APIKeyHeader  string
}

// DefaultConfig returns default auth configuration.
func DefaultConfig() Config {
	return Config{
		TokenExpiry:   24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
		APIKeyHeader:  "X-API-Key",
	}
}

// Service provides authentication operations.
type Service struct {
	config Config
}

// NewService creates a new auth service.
func NewService(cfg Config) *Service {
	return &Service{config: cfg}
}

// GenerateToken creates a new JWT token for the given claims.
// Note: This is a simplified implementation. In production, use a proper JWT library.
func (s *Service) GenerateToken(claims *Claims) (string, error) {
	if s.config.Secret == "" {
		return "", errors.New("secret not configured")
	}

	claims.IssuedAt = time.Now()
	if claims.ExpiresAt.IsZero() {
		if claims.TokenType == TokenTypeRefresh {
			claims.ExpiresAt = time.Now().Add(s.config.RefreshExpiry)
		} else {
			claims.ExpiresAt = time.Now().Add(s.config.TokenExpiry)
		}
	}

	// Simple token format: base64(json-like payload)|signature
	// In production, use github.com/golang-jwt/jwt/v5
	payload := fmt.Sprintf("%d|%d|%s|%s|%s|%d|%d",
		claims.UserID,
		claims.TenantID,
		claims.Email,
		claims.Role,
		claims.TokenType,
		claims.IssuedAt.Unix(),
		claims.ExpiresAt.Unix(),
	)

	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := s.sign(encoded)

	return encoded + "." + signature, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (s *Service) ValidateToken(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	encoded, providedSig := parts[0], parts[1]
	expectedSig := s.sign(encoded)

	// Use constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Parse the pipe-separated payload
	payloadParts := strings.Split(string(payload), "|")
	if len(payloadParts) != 7 {
		return nil, ErrInvalidToken
	}

	userID, err := parseInt64(payloadParts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	tenantID, err := parseInt64(payloadParts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	iat, err := parseInt64(payloadParts[5])
	if err != nil {
		return nil, ErrInvalidToken
	}

	exp, err := parseInt64(payloadParts[6])
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims := &Claims{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     payloadParts[2],
		Role:      payloadParts[3],
		TokenType: TokenType(payloadParts[4]),
		IssuedAt:  time.Unix(iat, 0),
		ExpiresAt: time.Unix(exp, 0),
	}

	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return claims, nil
}

// parseInt64 parses a string to int64.
func parseInt64(s string) (int64, error) {
	var result int64
	var negative bool

	if len(s) == 0 {
		return 0, errors.New("empty string")
	}

	start := 0
	if s[0] == '-' {
		negative = true
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("invalid character")
		}
		result = result*10 + int64(s[i]-'0')
	}

	if negative {
		result = -result
	}

	return result, nil
}

// GenerateAPIKey creates a new API key.
func (s *Service) GenerateAPIKey() (key string, prefix string, hash string, err error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}

	key = base64.RawURLEncoding.EncodeToString(b)
	prefix = key[:8]
	hash = s.sign(key)

	return key, prefix, hash, nil
}

// ValidateAPIKey validates an API key against its hash.
func (s *Service) ValidateAPIKey(key, hash string) bool {
	expectedHash := s.sign(key)
	return hmac.Equal([]byte(expectedHash), []byte(hash))
}

// sign creates an HMAC-SHA256 signature for the data.
func (s *Service) sign(data string) string {
	h := hmac.New(sha256.New, []byte(s.config.Secret))
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Context keys for auth information
type contextKey string

const (
	claimsKey contextKey = "auth_claims"
	tenantKey contextKey = "auth_tenant"
	userKey   contextKey = "auth_user"
)

// ContextWithClaims adds claims to the context.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext retrieves claims from the context.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	// A nil *Claims stored in the context is not a login: report it as absent, so
	// no caller that checks only ok can dereference it.
	return claims, ok && claims != nil
}

// ContextWithTenant adds tenant to the context.
func ContextWithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, tenantKey, tenant)
}

// TenantFromContext retrieves tenant from the context.
func TenantFromContext(ctx context.Context) (*Tenant, bool) {
	tenant, ok := ctx.Value(tenantKey).(*Tenant)
	return tenant, ok
}

// ContextWithUser adds user to the context.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UserFromContext retrieves user from the context.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey).(*User)
	return user, ok
}

// TenantIDFromContext is a convenience function to get just the tenant ID.
func TenantIDFromContext(ctx context.Context) (int64, bool) {
	if claims, ok := ClaimsFromContext(ctx); ok {
		return claims.TenantID, true
	}
	if tenant, ok := TenantFromContext(ctx); ok {
		return tenant.ID, true
	}
	return 0, false
}
