package gen

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// ficaRate is the FICA tax rate used for payroll entries (employee and employer
// shares). Declared as a package-level var using RequireFromString to avoid any
// float64 representation ambiguity in the monetary path.
var ficaRate = decimal.RequireFromString("0.0765")

// Account code mapping per event type (all codes verified to exist in DefaultCoA):
//
//	sale_invoice:
//	  DR 1100 Accounts Receivable
//	  CR 4000 Sales Revenue
//
//	customer_payment:
//	  DR 1010 Checking Account
//	  CR 1100 Accounts Receivable
//
//	vendor_bill:
//	  DR 5900 Other Operating Expenses
//	  CR 2000 Accounts Payable
//
//	bill_payment:
//	  DR 2000 Accounts Payable
//	  CR 1010 Checking Account
//
//	expense_report:
//	  DR 5300 Travel and Entertainment Expense
//	  CR 1010 Checking Account
//
//	payroll:
//	  DR 5100 Salaries and Wages Expense  (gross wages; last-leg-takes-remainder)
//	  DR 5110 Payroll Tax Expense          (employer share = 7.65% of gross, RoundBank(9))
//	  CR 2110 Accrued Salaries and Wages   (net pay = gross - employee taxes)
//	  CR 2100 Accrued Liabilities          (employee tax withholding = 7.65% of gross, RoundBank(9))
//	  CR 1010 Checking Account             (last leg = total CR - prior CRs, exact balance)
//
//	depreciation:
//	  DR 5400 Depreciation Expense
//	  CR 1510 Accumulated Depreciation
//
//	inventory_purchase:
//	  DR 1200 Inventory
//	  CR 1010 Checking Account
//
//	inventory_cogs:
//	  DR 5000 Cost of Goods Sold
//	  CR 1200 Inventory
//
//	interest_income:
//	  DR 1010 Checking Account
//	  CR 4200 Interest Income
//
// Multi-leg rounding pattern (payroll):
//   When splitting an amount into N legs, the last leg = total minus the sum
//   of all prior legs computed via RoundBank(9). This guarantees exact balance
//   regardless of the rounding direction applied to each prior leg.

// Expand walks events and dispatches each to the matching expand* function,
// collecting the resulting JournalEntry and []JournalLine values into flat
// output slices. It returns an error if any event's type has no matching
// template or if any account code used by the template is absent from coa.
func Expand(events []Event, coa map[string]Account) ([]JournalEntry, []JournalLine, error) {
	entries := make([]JournalEntry, 0, len(events))
	lines := make([]JournalLine, 0, len(events)*2)

	lineCounter := 0
	for i, ev := range events {
		entryID := fmt.Sprintf("JE-%08d", i+1)

		var newEntry JournalEntry
		var newLines []JournalLine
		var err error

		switch ev.Type {
		case EventTypeSaleInvoice:
			newEntry, newLines, err = expandSaleInvoice(entryID, ev, coa, &lineCounter)
		case EventTypeCustomerPayment:
			newEntry, newLines, err = expandCustomerPayment(entryID, ev, coa, &lineCounter)
		case EventTypeVendorBill:
			newEntry, newLines, err = expandVendorBill(entryID, ev, coa, &lineCounter)
		case EventTypeBillPayment:
			newEntry, newLines, err = expandBillPayment(entryID, ev, coa, &lineCounter)
		case EventTypeExpenseReport:
			newEntry, newLines, err = expandExpenseReport(entryID, ev, coa, &lineCounter)
		case EventTypePayroll:
			newEntry, newLines, err = expandPayroll(entryID, ev, coa, &lineCounter)
		case EventTypeDepreciation:
			newEntry, newLines, err = expandDepreciation(entryID, ev, coa, &lineCounter)
		case EventTypeInventoryPurchase:
			newEntry, newLines, err = expandInventoryPurchase(entryID, ev, coa, &lineCounter)
		case EventTypeInventoryCOGS:
			newEntry, newLines, err = expandInventoryCOGS(entryID, ev, coa, &lineCounter)
		case EventTypeInterestIncome:
			newEntry, newLines, err = expandInterestIncome(entryID, ev, coa, &lineCounter)
		default:
			return nil, nil, fmt.Errorf("gen.Expand: unknown event type %q for event %s", ev.Type, ev.ID)
		}

		if err != nil {
			return nil, nil, err
		}

		entries = append(entries, newEntry)
		lines = append(lines, newLines...)
	}

	return entries, lines, nil
}

// requireAccount returns the Account for code from coa, or an error if absent.
func requireAccount(code string, coa map[string]Account) (Account, error) {
	a, ok := coa[code]
	if !ok {
		return Account{}, fmt.Errorf("gen.Expand: account code %q not found in CoA", code)
	}
	return a, nil
}

// nextLineID returns the next sequential line ID and increments the counter.
func nextLineID(counter *int) string {
	*counter++
	return fmt.Sprintf("JL-%08d", *counter)
}

// debitLine builds a JournalLine with the given debit amount; credit is zero.
func debitLine(lineID, entryID, accountCode string, amount decimal.Decimal) JournalLine {
	return JournalLine{
		LineID:      lineID,
		EntryID:     entryID,
		AccountCode: accountCode,
		Debit:       amount,
		Credit:      decimal.Zero,
	}
}

// creditLine builds a JournalLine with the given credit amount; debit is zero.
func creditLine(lineID, entryID, accountCode string, amount decimal.Decimal) JournalLine {
	return JournalLine{
		LineID:      lineID,
		EntryID:     entryID,
		AccountCode: accountCode,
		Debit:       decimal.Zero,
		Credit:      amount,
	}
}

// expandSaleInvoice: DR 1100 / CR 4000
func expandSaleInvoice(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("1100", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("4000", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Sale invoice",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "1100", ev.Amount),
		creditLine(nextLineID(counter), entryID, "4000", ev.Amount),
	}
	return entry, lines, nil
}

// expandCustomerPayment: DR 1010 / CR 1100
func expandCustomerPayment(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("1010", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1100", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Customer payment",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "1010", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1100", ev.Amount),
	}
	return entry, lines, nil
}

// expandVendorBill: DR 5900 / CR 2000
func expandVendorBill(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("5900", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("2000", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Vendor bill",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "5900", ev.Amount),
		creditLine(nextLineID(counter), entryID, "2000", ev.Amount),
	}
	return entry, lines, nil
}

// expandBillPayment: DR 2000 / CR 1010
func expandBillPayment(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("2000", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1010", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Bill payment",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "2000", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1010", ev.Amount),
	}
	return entry, lines, nil
}

// expandExpenseReport: DR 5300 / CR 1010
func expandExpenseReport(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("5300", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1010", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Expense report",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "5300", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1010", ev.Amount),
	}
	return entry, lines, nil
}

// expandPayroll: 5-leg entry with last-leg-takes-remainder pattern.
//
// gross = ev.Amount
// employeeTax = gross * 0.0765, RoundBank(9)   (employee FICA withholding)
// employerTax = gross * 0.0765, RoundBank(9)   (employer FICA share)
// netPay      = gross - employeeTax             (exact, no rounding needed)
// cashOut     = netPay (last CR leg = total debits - prior credits, exact)
//
// Debit  5100 gross
// Debit  5110 employerTax
// Credit 2100 employeeTax
// Credit 2110 netPay
// Credit 1010 last = (gross + employerTax) - (employeeTax + netPay)
//
// By construction: DR total = gross + employerTax
//
//	CR total = employeeTax + netPay + cashOut
//	         = employeeTax + (gross - employeeTax) + cashOut
//	         = gross + cashOut
//	cashOut  = (gross + employerTax) - gross = employerTax  → CR = DR ✓
func expandPayroll(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	for _, code := range []string{"5100", "5110", "2100", "2110", "1010"} {
		if _, err := requireAccount(code, coa); err != nil {
			return JournalEntry{}, nil, err
		}
	}

	gross := ev.Amount
	employeeTax := gross.Mul(ficaRate).RoundBank(9)
	employerTax := gross.Mul(ficaRate).RoundBank(9)
	netPay := gross.Sub(employeeTax)

	// Last credit leg = total debits - sum of prior credits (exact balance by construction).
	totalDebits := gross.Add(employerTax)
	priorCredits := employeeTax.Add(netPay)
	cashOut := totalDebits.Sub(priorCredits)

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Payroll",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "5100", gross),
		debitLine(nextLineID(counter), entryID, "5110", employerTax),
		creditLine(nextLineID(counter), entryID, "2100", employeeTax),
		creditLine(nextLineID(counter), entryID, "2110", netPay),
		creditLine(nextLineID(counter), entryID, "1010", cashOut),
	}
	return entry, lines, nil
}

// expandDepreciation: DR 5400 / CR 1510
func expandDepreciation(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("5400", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1510", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Depreciation",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "5400", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1510", ev.Amount),
	}
	return entry, lines, nil
}

// expandInventoryPurchase: DR 1200 / CR 1010
func expandInventoryPurchase(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("1200", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1010", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Inventory purchase",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "1200", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1010", ev.Amount),
	}
	return entry, lines, nil
}

// expandInventoryCOGS: DR 5000 / CR 1200
func expandInventoryCOGS(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("5000", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("1200", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Inventory COGS",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "5000", ev.Amount),
		creditLine(nextLineID(counter), entryID, "1200", ev.Amount),
	}
	return entry, lines, nil
}

// expandInterestIncome: DR 1010 / CR 4200
func expandInterestIncome(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error) {
	if _, err := requireAccount("1010", coa); err != nil {
		return JournalEntry{}, nil, err
	}
	if _, err := requireAccount("4200", coa); err != nil {
		return JournalEntry{}, nil, err
	}

	entry := JournalEntry{
		EntryID:       entryID,
		OccurredAt:    ev.OccurredAt,
		Description:   "Interest income",
		SourceEventID: ev.ID,
	}
	lines := []JournalLine{
		debitLine(nextLineID(counter), entryID, "1010", ev.Amount),
		creditLine(nextLineID(counter), entryID, "4200", ev.Amount),
	}
	return entry, lines, nil
}
