package ui

import (
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// §6.4: what the AE sees is what it is due to receive, not what the
// customer pays.
func TestAccountingPortalHidesCustomerAmounts(t *testing.T) {
	e := newAcctEnv(t)
	// Beta's item is 500.00 and its commission 50.00 (10%).
	body := e.get(auth.RoleAccountExecutive, acctUserB, "/accounting/portal").Body.String()
	for _, leak := range []string{"USD 500.00", "USD 1,000.00", "USD 1000.00"} {
		if strings.Contains(body, leak) {
			t.Errorf("portal shows a customer amount (%s)", leak)
		}
	}
	if !strings.Contains(body, "USD 50.00") {
		t.Error("own commission missing from the portal")
	}
}

func TestAccountingPortalShowsOwnAccountOnly(t *testing.T) {
	e := newAcctEnv(t)
	b := e.get(auth.RoleAccountExecutive, acctUserB, "/accounting/portal").Body.String()
	for _, want := range []string{"accounting.portal.account", "accounting.portal.commissions", "accounting.portal.balance",
		"accounting.source.commission", "accounting.col.rubro"} {
		if !strings.Contains(b, want) {
			t.Errorf("Beta's portal lacks %s", want)
		}
	}
	// Ana's commission (100.00) and her settlement must not appear for Beta.
	if strings.Contains(b, "USD 100.00") || strings.Contains(b, "/accounting/settlements/"+itoa(e.setA.ID)) {
		t.Error("another AE's account data leaked into Beta's portal")
	}
	a := e.get(auth.RoleAccountExecutive, acctUserA, "/accounting/portal").Body.String()
	if !strings.Contains(a, "USD 100.00") || strings.Contains(a, "USD 50.00") {
		t.Error("Ana's portal must show her own commission and nobody else's")
	}
}

func TestAccountingPortalAccountNeedsAuthorization(t *testing.T) {
	e := newAcctEnv(t)
	// A newly registered AE has no account yet: no account panel, no error.
	rec := e.get(auth.RoleAccountExecutive, 799, "/accounting/portal")
	if rec.Code != 200 {
		t.Fatalf("new AE portal = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "accounting.portal.account") {
		t.Error("unauthorized AE sees an account panel")
	}
}

func TestAccountingPortalListsOwnTenantsOnly(t *testing.T) {
	e := newAcctEnv(t)
	b := e.get(auth.RoleAccountExecutive, acctUserB, "/accounting/portal").Body.String()
	if !strings.Contains(b, "accounting.portal.tenants") || !strings.Contains(b, "Beta SA") {
		t.Error("Beta's own tenant is missing from its portal")
	}
	// Ana's tenant is named with markup in the fixtures; it must not show for Beta.
	if strings.Contains(b, "&lt;script&gt;") {
		t.Error("another AE's tenant leaked into the tenants panel")
	}
	a := e.get(auth.RoleAccountExecutive, acctUserA, "/accounting/portal").Body.String()
	if !strings.Contains(a, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("Ana's tenant is missing or not escaped")
	}
	if strings.Contains(a, "Beta SA") {
		t.Error("Beta's tenant leaked into Ana's portal")
	}
}

func TestAccountingPortalTenantsAbsentUntilAuthorized(t *testing.T) {
	e := newAcctEnv(t)
	b := e.get(auth.RoleAccountExecutive, 798, "/accounting/portal").Body.String()
	if strings.Contains(b, "accounting.portal.tenants") {
		t.Error("an unauthorized AE sees a tenants panel")
	}
}
