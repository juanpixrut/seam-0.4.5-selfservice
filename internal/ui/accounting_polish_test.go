package ui

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/accounting"
	"github.com/ha1tch/seam-ui/internal/auth"
)

var idAttrRE = regexp.MustCompile(`\sid="([^"]+)"`)

// Duplicate ids break label association and scripts. The page shell has one
// pre-existing pair (dark-mode-icon) that is not this module's.
func assertUniqueIDs(t *testing.T, name, body string) {
	t.Helper()
	seen := map[string]bool{}
	for _, m := range idAttrRE.FindAllStringSubmatch(body, -1) {
		if m[1] == "dark-mode-icon" {
			continue
		}
		if seen[m[1]] {
			t.Errorf("%s: duplicate id %q", name, m[1])
		}
		seen[m[1]] = true
	}
}

func TestAccountingScreensHaveUniqueIDs(t *testing.T) {
	e := newAcctEnv(t)
	for _, c := range []struct {
		role string
		uid  int64
		path string
	}{
		{auth.RoleAdmin, 1, "/accounting/collections"},
		{auth.RoleAdmin, 1, "/accounting/aes"},
		{auth.RoleAdmin, 1, "/accounting/ledger"},
		{auth.RoleAdmin, 1, "/accounting/entries/new"},
		{auth.RoleAdmin, 1, "/accounting/closes"},
		{auth.RoleAdmin, 1, "/accounting/settlements/" + itoa(e.setA.ID)},
		{auth.RoleAccountExecutive, acctUserB, "/accounting/portal/settle"},
		{auth.RoleAccountExecutive, acctUserA, "/accounting/portal"},
	} {
		rec := e.get(c.role, c.uid, c.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.path, rec.Code)
			continue
		}
		assertUniqueIDs(t, c.path, rec.Body.String())
	}
}

func TestAccountingCollectFormConfirmsAndGuards(t *testing.T) {
	e := newAcctEnv(t)
	ctx := context.Background()
	e.inst.snap.Items = append(e.inst.snap.Items, accounting.ItemSnapshot{
		ExternalRef: "i3", TenantRef: "t1", Period: "2026-10", Concept: accounting.ConceptSubscription, Amount: 70000, State: accounting.ItemPending})
	if _, err := e.svc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ := e.svc.ListItems(ctx, accounting.ItemFilter{State: accounting.ItemPending})
	if len(items) != 1 {
		t.Fatalf("want one pending item, got %d", len(items))
	}
	path := "/accounting/items/" + itoa(items[0].ID) + "/collect"

	rec := e.get(auth.RoleAdmin, 1, path)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	for _, want := range []string{`action="` + path + `"`, "SeamConfirm.showForm", `name="cash"`, `name="date"`, "USD 700.00", "&lt;script&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("collect page lacks %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("tenant name was not escaped")
	}
	if got := e.get(auth.RoleAccountExecutive, acctUserA, path).Code; got != http.StatusForbidden {
		t.Errorf("an AE reached the collect form: %d", got)
	}
	// an item that is already collected sends the user back with a message
	items, _ = e.svc.ListItems(ctx, accounting.ItemFilter{State: accounting.ItemCollected})
	rec = e.get(auth.RoleAdmin, 1, "/accounting/items/"+itoa(items[0].ID)+"/collect")
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "err=already_collected") {
		t.Errorf("collected item: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// the list links to the page and no longer carries an inline form per row
	list := e.get(auth.RoleAdmin, 1, "/accounting/collections").Body.String()
	if !strings.Contains(list, `href="`+path+`"`) || strings.Contains(list, `name="cash"`) {
		t.Error("collections list should link to the collect page")
	}
}

func TestAccountingSettlementDetailShowsLinesAndConfirmsPayment(t *testing.T) {
	e := newAcctEnv(t)
	body := e.get(auth.RoleAdmin, 1, "/accounting/settlements/"+itoa(e.setA.ID)).Body.String()
	for _, want := range []string{
		"accounting.settlements.lines", "accounting.concept.subscription", "&lt;script&gt;", // the included line and its tenant
		"accounting.payout.paypal", // method is translated, not the raw "paypal"
		"SeamConfirm.showForm",     // payment is confirmed
		`name="paid_date"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settlement detail lacks %q", want)
		}
	}
	if regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d`).MatchString(body) {
		t.Error("a raw timestamp is shown")
	}
	// the AE sees the lines too, but cannot pay
	own := e.get(auth.RoleAccountExecutive, acctUserA, "/accounting/settlements/"+itoa(e.setA.ID)).Body.String()
	if !strings.Contains(own, "accounting.settlements.lines") || strings.Contains(own, `name="paid_date"`) {
		t.Error("AE view of own settlement is wrong")
	}
}

func TestAccountingSettleFormTotalsAndConfirm(t *testing.T) {
	e := newAcctEnv(t)
	body := e.get(auth.RoleAccountExecutive, acctUserB, "/accounting/portal/settle").Body.String()
	for _, want := range []string{`id="settle-form"`, `data-cents="5000"`, `id="sel-all"`, `id="sel-total"`, "SeamConfirm.showForm"} {
		if !strings.Contains(body, want) {
			t.Errorf("settle form lacks %q", want)
		}
	}
}

func TestAccountingEntryFormHasRunningTotals(t *testing.T) {
	e := newAcctEnv(t)
	body := e.get(auth.RoleAdmin, 1, "/accounting/entries/new").Body.String()
	for _, want := range []string{`id="entry-form"`, `id="entry-debit"`, `id="entry-credit"`, `id="entry-diff"`, "SeamConfirm.showForm", `name="account_5"`, `name="credit_5"`} {
		if !strings.Contains(body, want) {
			t.Errorf("entry form lacks %q", want)
		}
	}
	if strings.Contains(body, `name="account_6"`) {
		t.Error("more rows than entryRows")
	}
}

func TestAccountingIrreversibleActionsAskFirst(t *testing.T) {
	e := newAcctEnv(t)
	ledger := e.get(auth.RoleAdmin, 1, "/accounting/ledger").Body.String()
	if !strings.Contains(ledger, "/reverse") || !strings.Contains(ledger, "danger:true") {
		t.Error("reversing an entry must ask, in the danger style")
	}
	closes := e.get(auth.RoleAdmin, 1, "/accounting/closes").Body.String()
	if !strings.Contains(closes, `action="/accounting/closes"`) || !strings.Contains(closes, "SeamConfirm.showForm") {
		t.Error("closing a month must ask")
	}
}

func TestAccountingBannerClearsItsOwnURL(t *testing.T) {
	e := newAcctEnv(t)
	with := e.get(auth.RoleAdmin, 1, "/accounting/ledger?ok=entry_posted").Body.String()
	if !strings.Contains(with, "history.replaceState") {
		t.Error("the message must remove itself from the address bar")
	}
	if strings.Contains(e.get(auth.RoleAdmin, 1, "/accounting/ledger").Body.String(), "history.replaceState") {
		t.Error("no message, no script")
	}
}

func TestAccountingErrorKeysForLedgerAndCompany(t *testing.T) {
	for err, want := range map[error]string{
		accounting.ErrUnbalanced:     "accounting.err.unbalanced",
		accounting.ErrTooFewLines:    "accounting.err.too_few_lines",
		accounting.ErrNoDescription:  "accounting.err.no_description",
		accounting.ErrBadCompany:     "accounting.err.bad_company",
		errors.New("something else"): "accounting.err.generic",
	} {
		if got := acctErrKey(err); got != want {
			t.Errorf("%v: %s, want %s", err, got, want)
		}
	}
}
