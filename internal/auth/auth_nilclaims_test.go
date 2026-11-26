package auth

import (
	"context"
	"testing"
)

// A nil *Claims in the context must read as "no claims", never as a login
// whose fields can be dereferenced (found by an adversarial test of the
// Developer module's tenant lookups, 2026-10-12).
func TestClaimsFromContext_NilClaimsAreAbsent(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), nil)
	if c, ok := ClaimsFromContext(ctx); ok || c != nil {
		t.Errorf("a stored nil must read as absent, got %v %v", c, ok)
	}
	if c, ok := ClaimsFromContext(context.Background()); ok || c != nil {
		t.Errorf("no claims at all: got %v %v", c, ok)
	}
	want := &Claims{TenantID: 4}
	if c, ok := ClaimsFromContext(ContextWithClaims(context.Background(), want)); !ok || c != want {
		t.Errorf("real claims must still come back")
	}
}
