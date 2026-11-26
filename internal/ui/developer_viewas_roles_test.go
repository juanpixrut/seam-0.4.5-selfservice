package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/ha1tch/seam-ui/internal/auth"
	developerschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/developer"
	"github.com/ha1tch/seam-ui/internal/i18n"
)

// The roles a developer may view the application as are the platform's, and
// those of Accounting where that module is on; the form offers exactly those,
// by the names the rest of the application gives them.
func TestViewAs_OffersTheRolesOfAccountingWhereItIsOn(t *testing.T) {
	platform := []string{auth.RoleAdmin, auth.RoleManager, auth.RoleOperator, auth.RoleViewer}
	accounting := []string{auth.RoleAdminStaff, auth.RoleAccountant, auth.RoleAccountExecutive}
	if got := (Config{}).viewAsRoles(); strings.Join(got, ",") != strings.Join(platform, ",") {
		t.Errorf("without Accounting: %v", got)
	}
	with := Config{EnableAccounting: true}.viewAsRoles()
	if strings.Join(with, ",") != strings.Join(append(platform, accounting...), ",") {
		t.Errorf("with Accounting: %v", with)
	}
	if !strings.Contains(string(developerschemas.Schema("view_as")), `"x-seam-source": "`+viewAsSource+`"`) {
		t.Fatalf("view_as.json does not take its roles from %s", viewAsSource)
	}
	for _, locale := range []string{"en", "es", "pt", "ja"} {
		tr, err := i18n.New(locale)
		if err != nil {
			t.Fatal(err)
		}
		opts, err := viewAsSources{roles: with, t: tr.ForLocale(locale)}.Resolve(context.Background(), viewAsSource)
		if err != nil || len(opts) != len(with) {
			t.Fatalf("%s: %d options, %v", locale, len(opts), err)
		}
		for i, o := range opts {
			if o.Value != with[i] || o.Label == "" || strings.HasPrefix(o.Label, "user.role.") {
				t.Errorf("%s: role %s has no name (%q)", locale, o.Value, o.Label)
			}
		}
	}
	if opts, _ := (viewAsSources{roles: with}).Resolve(context.Background(), "entity:users"); len(opts) != 0 {
		t.Errorf("another source is not this one's to answer")
	}
}
