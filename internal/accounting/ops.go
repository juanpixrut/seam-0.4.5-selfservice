package accounting

import (
	"errors"
	"fmt"
)

// This file holds the standard operations of the plan (section 12.5): each
// builds the entry for one kind of business event, so that no caller ever
// has to remember which side of which account an operation touches.

// ErrBadOperation is returned when an operation is given accounts of the
// wrong kind (for instance a bank account where an AE account is expected).
var ErrBadOperation = errors.New("accounting: operation uses an account of the wrong kind")

func requireUnder(chart *Chart, code, ancestor, what string) error {
	a, ok := chart.Get(code)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownAccount, code)
	}
	if !Under(code, ancestor) || !a.Postable {
		return fmt.Errorf("%w: %s must be a posting account under %s (got %s %s)", ErrBadOperation, what, ancestor, a.Code, a.Name)
	}
	return nil
}

func twoLine(date, desc string, src Source, ref string, debitCode, creditCode string, amount Cents) Entry {
	return Entry{
		Date: date, Description: desc, Source: src, Reference: ref,
		Movements: []Movement{
			{AccountCode: debitCode, Side: Debit, Amount: amount},
			{AccountCode: creditCode, Side: Credit, Amount: amount},
		},
	}
}

// CustomerCollection: a customer paid. Debit the cash account where the money
// arrived (1.1.x), credit 4.1 Ingresos por servicios.
func CustomerCollection(chart *Chart, date, desc, ref, cashCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceCollection, ref, cashCode, CodeServiceInc, amount), nil
}

// CommissionAccrual: an item became Collected and the AE earned a commission.
// Debit 5.1 Comisiones de AE, credit the AE's own 2.1.x account.
func CommissionAccrual(chart *Chart, date, desc, ref, aeCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, aeCode, CodeAEPayables, "AE account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceCommission, ref, CodeAECommExp, aeCode, amount), nil
}

// SettlementPayment: Seam paid an AE. Debit the AE's 2.1.x account, credit the
// cash account the money left from (1.1.x).
func SettlementPayment(chart *Chart, date, desc, ref, aeCode, cashCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, aeCode, CodeAEPayables, "AE account"); err != nil {
		return Entry{}, err
	}
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceSettlement, ref, aeCode, cashCode, amount), nil
}

// Expense: an operating, collection or tax expense paid from a cash account.
// Debit 5.2, 5.3 or 5.4, credit 1.1.x.
func Expense(chart *Chart, date, desc, expenseCode, cashCode string, amount Cents) (Entry, error) {
	switch expenseCode {
	case CodeCollectExp, CodeOperatingEx, CodeTaxesExp:
	default:
		return Entry{}, fmt.Errorf("%w: expense must be 5.2, 5.3 or 5.4 (got %s)", ErrBadOperation, expenseCode)
	}
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceManual, "", expenseCode, cashCode, amount), nil
}

// Refund covers a refund and a chargeback: money returned to a customer.
// Debit 4.1 Ingresos por servicios, credit 1.1.x.
func Refund(chart *Chart, date, desc, cashCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceManual, "", CodeServiceInc, cashCode, amount), nil
}

// PartnerContribution: a partner put money in. Debit 1.1.x, credit 3.1.x.
func PartnerContribution(chart *Chart, date, desc, cashCode, partnerCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	if err := requireUnder(chart, partnerCode, CodePartners, "partner account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceManual, "", cashCode, partnerCode, amount), nil
}

// PartnerWithdrawal: a partner took money out. Debit 3.1.x, credit 1.1.x.
func PartnerWithdrawal(chart *Chart, date, desc, cashCode, partnerCode string, amount Cents) (Entry, error) {
	if err := requireUnder(chart, cashCode, CodeCashGroup, "cash account"); err != nil {
		return Entry{}, err
	}
	if err := requireUnder(chart, partnerCode, CodePartners, "partner account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceManual, "", partnerCode, cashCode, amount), nil
}

// Transfer moves money between two of Seam's own cash accounts. Debit the
// destination, credit the origin.
func Transfer(chart *Chart, date, desc, fromCode, toCode string, amount Cents) (Entry, error) {
	if fromCode == toCode {
		return Entry{}, fmt.Errorf("%w: origin and destination are the same account", ErrBadOperation)
	}
	if err := requireUnder(chart, fromCode, CodeCashGroup, "origin account"); err != nil {
		return Entry{}, err
	}
	if err := requireUnder(chart, toCode, CodeCashGroup, "destination account"); err != nil {
		return Entry{}, err
	}
	return twoLine(date, desc, SourceManual, "", toCode, fromCode, amount), nil
}

// Adjustment corrects the balance of one account by amount, in the direction
// given. The other side is 5.5 Otros egresos y ajustes when the correction is
// a loss and 4.2 Otros ingresos when it is a gain (section 12.5, "según el
// signo"). increase means the account's own natural-sign balance goes up.
//
//	debit-normal account (Activo, Egresos), increase:  Dr account, Cr 4.2
//	debit-normal account, decrease:                    Dr 5.5,     Cr account
//	credit-normal account (Pasivo, Patrimonio, Ingresos), increase: Dr 5.5, Cr account
//	credit-normal account, decrease:                                Dr account, Cr 4.2
func Adjustment(chart *Chart, date, desc, code string, increase bool, amount Cents) (Entry, error) {
	a, ok := chart.Get(code)
	if !ok {
		return Entry{}, fmt.Errorf("%w: %q", ErrUnknownAccount, code)
	}
	if !a.Postable {
		return Entry{}, fmt.Errorf("%w: %s %s", ErrNotPostable, a.Code, a.Name)
	}
	if code == CodeOtherExp || code == CodeOtherInc {
		return Entry{}, fmt.Errorf("%w: cannot adjust the adjustment accounts themselves", ErrBadOperation)
	}
	debitNormal := a.Group().DebitNormal()
	switch {
	case debitNormal && increase:
		return twoLine(date, desc, SourceManual, "", code, CodeOtherInc, amount), nil
	case debitNormal && !increase:
		return twoLine(date, desc, SourceManual, "", CodeOtherExp, code, amount), nil
	case !debitNormal && increase:
		return twoLine(date, desc, SourceManual, "", CodeOtherExp, code, amount), nil
	default:
		return twoLine(date, desc, SourceManual, "", code, CodeOtherInc, amount), nil
	}
}
