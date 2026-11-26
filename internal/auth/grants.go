package auth

import "sync/atomic"

// Configurable accounting duties.
//
// The static rolePermissions give the conservative split (the accountant keeps
// the books, admin_staff runs collections and payments). Two settings move a
// duty across, decided once at start-up from the configuration; the extra
// grants live apart from rolePermissions so the defaults stay readable in one
// table.

// AccountingGrants selects the optional accounting duties.
type AccountingGrants struct {
	// AccountantOperate lets the accountant sync, collect and pay.
	AccountantOperate bool
	// StaffPostEntries lets admin_staff post and reverse manual entries.
	StaffPostEntries bool
	// StaffEditAEProfiles lets admin_staff edit AE profiles and payout details.
	StaffEditAEProfiles bool
	// AccountantEditFormulas lets the accountant create and change formula rules.
	AccountantEditFormulas bool
}

var extraGrants atomic.Pointer[map[string][]Permission]

// ConfigureAccounting installs the optional duties. Call it at start-up, before
// serving; calling it again replaces the previous choice.
func ConfigureAccounting(g AccountingGrants) {
	m := map[string][]Permission{}
	if g.AccountantOperate {
		m[RoleAccountant] = []Permission{
			PermAccountingCollectionSync, PermAccountingCollectionCollect, PermAccountingSettlementPay,
		}
	}
	if g.AccountantEditFormulas {
		m[RoleAccountant] = append(m[RoleAccountant], PermAccountingFormulaEdit)
	}
	if g.StaffPostEntries {
		m[RoleAdminStaff] = append(m[RoleAdminStaff], PermAccountingEntryCreate, PermAccountingEntryReverse)
	}
	if g.StaffEditAEProfiles {
		m[RoleAdminStaff] = append(m[RoleAdminStaff], PermAccountingAEEdit)
	}
	extraGrants.Store(&m)
}

// grantsFor returns the permissions of role: its static grants plus any
// optional accounting duties switched on.
func grantsFor(role string) ([]Permission, bool) {
	base, ok := rolePermissions[role]
	if !ok {
		return nil, false
	}
	if p := extraGrants.Load(); p != nil {
		if extra := (*p)[role]; len(extra) > 0 {
			out := make([]Permission, 0, len(base)+len(extra))
			out = append(out, base...)
			return append(out, extra...), true
		}
	}
	return base, true
}
