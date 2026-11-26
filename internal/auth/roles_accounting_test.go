package auth

import "testing"

// The permission matrix of the Accounting & Admin family. Each case is a
// sentence of the design (docs/ACCOUNTING_ADMIN_MODULE.md section 3): who may
// do what, and, as important, who may not.
func TestAccountingRoles_Matrix(t *testing.T) {
	type tc struct {
		role string
		perm Permission
		want bool
	}
	cases := []tc{
		// An AE: its own portal, profile and settlements, and nothing else.
		{RoleAccountExecutive, PermAccountingPortalView, true},
		{RoleAccountExecutive, PermAccountingProfileEdit, true},
		{RoleAccountExecutive, PermAccountingSettlementCreate, true},
		{RoleAccountExecutive, PermAccountingSettlementPay, false},
		{RoleAccountExecutive, PermAccountingSettlementView, false}, // all settlements
		{RoleAccountExecutive, PermAccountingAEAuthorize, false},    // an AE cannot authorize itself
		{RoleAccountExecutive, PermAccountingCollectionCollect, false},
		{RoleAccountExecutive, PermAccountingLedgerView, false},
		{RoleAccountExecutive, PermAccountingCommissionView, false}, // all AEs' commissions
		{RoleAccountExecutive, PermAssetsAssetView, false},

		// Seam administrative personnel: authorize, collect, pay.
		{RoleAdminStaff, PermAccountingAEAuthorize, true},
		{RoleAdminStaff, PermAccountingCollectionCollect, true},
		{RoleAdminStaff, PermAccountingCollectionSync, true},
		{RoleAdminStaff, PermAccountingSettlementPay, true},
		{RoleAdminStaff, PermAccountingCompanyEdit, true},
		{RoleAdminStaff, PermAccountingEntryCreate, false}, // default: free entries are the accountant's
		{RoleAdminStaff, PermAccountingLedgerView, true},
		{RoleAdminStaff, PermAccountingEntryReverse, false},
		{RoleAdminStaff, PermAccountingCloseCreate, false},
		{RoleAdminStaff, PermAccountingSettlementCreate, false}, // that is the AE's act
		{RoleAdminStaff, PermAccountingPortalView, false},
		{RoleAdminStaff, PermAssetsAssetView, false},

		// Accountants: the books.
		{RoleAccountant, PermAccountingLedgerView, true},
		{RoleAccountant, PermAccountingEntryCreate, true},
		{RoleAccountant, PermAccountingEntryReverse, true},
		{RoleAccountant, PermAccountingCloseCreate, true},
		{RoleAccountant, PermAccountingCloseView, true},
		{RoleAccountant, PermAccountingCompanyView, true},
		{RoleAccountant, PermAccountingCompanyEdit, false},
		{RoleAccountant, PermAccountingAEAuthorize, false},
		{RoleAccountant, PermAccountingCollectionCollect, false},
		{RoleAccountant, PermAccountingSettlementPay, false},
		{RoleAccountant, PermAssetsAssetView, false},

		// Admin holds the whole module; the other core roles hold none of it.
		{RoleAdmin, PermAccountingAEAuthorize, true},
		{RoleAdmin, PermAccountingCloseCreate, true},
		{RoleAdmin, PermAccountingPortalView, false}, // the admin is not an AE
		{RoleAdmin, PermAccountingProfileEdit, false},
		{RoleAdmin, PermAccountingSettlementCreate, false},
		{RoleManager, PermAccountingLedgerView, false},
		{RoleOperator, PermAccountingPortalView, false},
		{RoleViewer, PermAccountingCollectionView, false},
	}
	for _, c := range cases {
		if got := HasPermission(c.role, c.perm); got != c.want {
			t.Errorf("HasPermission(%s, %s) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestAccountingRoles_ModuleAccess(t *testing.T) {
	for _, r := range []string{RoleAdmin, RoleAdminStaff, RoleAccountant, RoleAccountExecutive} {
		if !HasModuleAccess(r, "accounting") {
			t.Errorf("%s should see the accounting module", r)
		}
	}
	for _, r := range []string{RoleManager, RoleOperator, RoleViewer, "", "root"} {
		if HasModuleAccess(r, "accounting") {
			t.Errorf("%q must not see the accounting module", r)
		}
	}
	// And the new roles see none of the Assets, Settings or Administration modules.
	for _, r := range []string{RoleAdminStaff, RoleAccountant, RoleAccountExecutive} {
		for _, m := range []string{"assets", "settings", "administration", "documents"} {
			if HasModuleAccess(r, m) {
				t.Errorf("%s must not see module %s", r, m)
			}
		}
	}
}

// Every permission constant of the module is granted to at least one role, so
// a permission that nothing can ever hold (and so a route nobody can open)
// cannot be added by mistake.
func TestAccountingPermissions_EachIsGranted(t *testing.T) {
	all := []Permission{
		PermAccountingPortalView, PermAccountingProfileEdit,
		PermAccountingSettlementCreate, PermAccountingSettlementView, PermAccountingSettlementPay,
		PermAccountingAEView, PermAccountingAEAuthorize,
		PermAccountingCollectionView, PermAccountingCollectionCollect, PermAccountingCollectionSync,
		PermAccountingCommissionView, PermAccountingLedgerView,
		PermAccountingEntryCreate, PermAccountingEntryReverse,
		PermAccountingCloseView, PermAccountingCloseCreate,
		PermAccountingCompanyView, PermAccountingCompanyEdit,
		PermAccountingFormulaView,
	}
	for _, p := range all {
		held := false
		for _, r := range []string{RoleAdminStaff, RoleAccountant, RoleAccountExecutive} {
			if HasPermission(r, p) {
				held = true
			}
		}
		if !held {
			t.Errorf("no accounting role holds %s", p)
		}
	}
}

func TestIsValidRole_AccountingFamily(t *testing.T) {
	for _, r := range []string{RoleAdminStaff, RoleAccountant, RoleAccountExecutive} {
		if !IsValidRole(r) {
			t.Errorf("%s should be valid", r)
		}
	}
	if IsValidRole("ACCOUNTANT") || IsValidRole("accounting") {
		t.Error("role names are exact")
	}
}

// The two optional duty moves, and that they stay inside their role.
func TestAccountingGrants_Options(t *testing.T) {
	t.Cleanup(func() { ConfigureAccounting(AccountingGrants{}) })

	ConfigureAccounting(AccountingGrants{AccountantOperate: true})
	for _, p := range []Permission{PermAccountingCollectionCollect, PermAccountingCollectionSync, PermAccountingSettlementPay} {
		if !HasPermission(RoleAccountant, p) {
			t.Errorf("AccountantOperate: accountant lacks %s", p)
		}
	}
	if HasPermission(RoleAccountant, PermAccountingAEAuthorize) || HasPermission(RoleAccountant, PermAccountingCompanyEdit) {
		t.Error("AccountantOperate must not grant authorizing AEs or editing the company")
	}
	if HasPermission(RoleAdminStaff, PermAccountingEntryCreate) || HasPermission(RoleAccountExecutive, PermAccountingSettlementPay) {
		t.Error("AccountantOperate leaked to another role")
	}

	ConfigureAccounting(AccountingGrants{StaffPostEntries: true})
	if !HasPermission(RoleAdminStaff, PermAccountingEntryCreate) || !HasPermission(RoleAdminStaff, PermAccountingEntryReverse) {
		t.Error("StaffPostEntries: staff cannot post/reverse")
	}
	if HasPermission(RoleAdminStaff, PermAccountingCloseCreate) {
		t.Error("StaffPostEntries must not let staff close a month")
	}
	if HasPermission(RoleAccountant, PermAccountingSettlementPay) {
		t.Error("the previous option was not cleared")
	}
	if HasPermission(RoleAccountExecutive, PermAccountingEntryCreate) || HasPermission(RoleManager, PermAccountingEntryCreate) {
		t.Error("StaffPostEntries leaked to another role")
	}

	ConfigureAccounting(AccountingGrants{})
	if HasPermission(RoleAdminStaff, PermAccountingEntryCreate) {
		t.Error("defaults not restored")
	}
}

func TestStaffEditAEProfilesIsOptIn(t *testing.T) {
	defer ConfigureAccounting(AccountingGrants{})
	ConfigureAccounting(AccountingGrants{})
	if HasPermission(RoleAdminStaff, PermAccountingAEEdit) || HasPermission(RoleAccountant, PermAccountingAEEdit) || HasPermission(RoleAccountExecutive, PermAccountingAEEdit) {
		t.Error("nobody but admin edits AE profiles by default")
	}
	if !HasPermission(RoleAdmin, PermAccountingAEEdit) {
		t.Error("admin keeps full access")
	}
	ConfigureAccounting(AccountingGrants{StaffEditAEProfiles: true, StaffPostEntries: true})
	if !HasPermission(RoleAdminStaff, PermAccountingAEEdit) || !HasPermission(RoleAdminStaff, PermAccountingEntryCreate) {
		t.Error("both staff switches must stack on admin_staff")
	}
	if HasPermission(RoleAccountant, PermAccountingAEEdit) || HasPermission(RoleAccountExecutive, PermAccountingAEEdit) {
		t.Error("the staff switch leaked to another role")
	}
}

// Every accounting permission is held by the admin, except the three that
// belong to an AE acting for itself. A permission added later without a grant
// would otherwise be silently unusable by anyone.
func TestAdminHoldsEveryStaffAccountingPermission(t *testing.T) {
	ownAE := map[Permission]bool{PermAccountingPortalView: true, PermAccountingProfileEdit: true, PermAccountingSettlementCreate: true}
	for _, p := range allAccountingPerms {
		if got := HasPermission(RoleAdmin, p); got == ownAE[p] {
			t.Errorf("admin holds %s = %v", p, got)
		}
	}
}

var allAccountingPerms = []Permission{
	PermAccountingPortalView, PermAccountingProfileEdit, PermAccountingSettlementCreate,
	PermAccountingSettlementView, PermAccountingSettlementPay,
	PermAccountingAEView, PermAccountingAEAuthorize, PermAccountingAEEdit,
	PermAccountingCollectionView, PermAccountingCollectionCollect, PermAccountingCollectionSync,
	PermAccountingCommissionView, PermAccountingLedgerView, PermAccountingEntryCreate, PermAccountingEntryReverse,
	PermAccountingCloseView, PermAccountingCloseCreate, PermAccountingCompanyView, PermAccountingCompanyEdit,
	PermAccountingFormulaView, PermAccountingFormulaEdit,
}

// Formula rules decide what AEs are paid: everyone in the back office may look,
// only the admin changes them unless the accountant is given the duty.
func TestFormulaPermissions(t *testing.T) {
	defer ConfigureAccounting(AccountingGrants{})
	ConfigureAccounting(AccountingGrants{})
	for _, r := range []string{RoleAdmin, RoleAdminStaff, RoleAccountant} {
		if !HasPermission(r, PermAccountingFormulaView) {
			t.Errorf("%s must see formulas", r)
		}
	}
	if HasPermission(RoleAccountExecutive, PermAccountingFormulaView) || HasPermission(RoleAccountExecutive, PermAccountingFormulaEdit) {
		t.Error("an AE must not see or edit formulas")
	}
	for _, r := range []string{RoleAdminStaff, RoleAccountant} {
		if HasPermission(r, PermAccountingFormulaEdit) {
			t.Errorf("%s must not edit formulas by default", r)
		}
	}
	if !HasPermission(RoleAdmin, PermAccountingFormulaEdit) {
		t.Error("admin edits formulas")
	}
	ConfigureAccounting(AccountingGrants{AccountantEditFormulas: true})
	if !HasPermission(RoleAccountant, PermAccountingFormulaEdit) {
		t.Error("the accountant switch must grant formula editing")
	}
	if HasPermission(RoleAdminStaff, PermAccountingFormulaEdit) || HasPermission(RoleAccountExecutive, PermAccountingFormulaEdit) {
		t.Error("the accountant switch leaked to another role")
	}
}
