// Package invariant implements the four write-time invariant checks for a
// fauxbooks Dataset. Check must be called before any output is written; a
// non-nil return means the run must exit 2 and write nothing.
package invariant

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// Check runs invariants 1–4 in order and returns the first InvariantError
// encountered, or nil when the Dataset satisfies all four invariants.
//
// Invariant 1: every journal entry's Σ debits == Σ credits.
// Invariant 2: every JournalLine.AccountCode exists in ds.CoA.
// Invariant 3: Σ BankRow amounts == Σ GLRow amounts on cash account codes.
// Invariant 4: every SubLedgerRow.EntryID exists in ds.Entries.
//
// Check is a pure function: no file I/O, no RNG, no wall-clock reads.
func Check(ds gen.Dataset) error {
	if err := checkInv1(ds); err != nil {
		return err
	}
	if err := checkInv2(ds); err != nil {
		return err
	}
	if err := checkInv3(ds); err != nil {
		return err
	}
	return checkInv4(ds)
}

// checkInv1 verifies that every journal entry's Σ debits == Σ credits.
func checkInv1(ds gen.Dataset) error {
	// Group lines by EntryID.
	linesByEntry := make(map[string][]gen.JournalLine, len(ds.Entries))
	for _, ln := range ds.Lines {
		linesByEntry[ln.EntryID] = append(linesByEntry[ln.EntryID], ln)
	}
	for _, entry := range ds.Entries {
		lines := linesByEntry[entry.EntryID]
		var totalDebit, totalCredit decimal.Decimal
		for _, ln := range lines {
			totalDebit = totalDebit.Add(ln.Debit)
			totalCredit = totalCredit.Add(ln.Credit)
		}
		if !totalDebit.Equal(totalCredit) {
			return gen.InvariantError{
				Invariant: 1,
				Detail:    fmt.Sprintf("entry %s: debits %s != credits %s", entry.EntryID, totalDebit.String(), totalCredit.String()),
			}
		}
	}
	return nil
}

// checkInv2 verifies that every JournalLine.AccountCode exists in ds.CoA.
func checkInv2(ds gen.Dataset) error {
	for _, ln := range ds.Lines {
		if _, ok := ds.CoA[ln.AccountCode]; !ok {
			return gen.InvariantError{
				Invariant: 2,
				Detail:    fmt.Sprintf("line %s references unknown account code %s", ln.LineID, ln.AccountCode),
			}
		}
	}
	return nil
}

// checkInv3 verifies that bank statement totals tie to GL cash account postings.
// Both sides sum amounts as: debit amounts minus credit amounts (net position).
// Since bank rows are derived directly from cash-account GL lines, we compare the
// sum of all BankRow.Debit - BankRow.Credit against the sum of all GLRow amounts
// on cash account codes using the same formula.
func checkInv3(ds gen.Dataset) error {
	// Sum bank rows: net = Σ Debit - Σ Credit (stored as decimal strings).
	var bankNet decimal.Decimal
	for _, br := range ds.Bank {
		d, _ := decimal.NewFromString(br.Debit)
		c, _ := decimal.NewFromString(br.Credit)
		bankNet = bankNet.Add(d).Sub(c)
	}

	// Sum GL rows on cash account codes with the same formula.
	var glCashNet decimal.Decimal
	for _, gr := range ds.GL {
		if !gen.CashAccountCodes[gr.AccountCode] {
			continue
		}
		d, _ := decimal.NewFromString(gr.Debit)
		c, _ := decimal.NewFromString(gr.Credit)
		glCashNet = glCashNet.Add(d).Sub(c)
	}

	if !bankNet.Equal(glCashNet) {
		return gen.InvariantError{
			Invariant: 3,
			Detail:    fmt.Sprintf("bank net %s != GL cash net %s", bankNet.String(), glCashNet.String()),
		}
	}
	return nil
}

// checkInv4 verifies that every SubLedgerRow.EntryID exists in ds.Entries.
func checkInv4(ds gen.Dataset) error {
	// Build an entry ID set.
	entryIDs := make(map[string]bool, len(ds.Entries))
	for _, e := range ds.Entries {
		entryIDs[e.EntryID] = true
	}

	// Collect all sub-ledger row sets in one pass.
	all := make([][]gen.SubLedgerRow, 0, 5)
	all = append(all,
		ds.SubLedger.Invoices,
		ds.SubLedger.CustomerPayments,
		ds.SubLedger.Bills,
		ds.SubLedger.BillPayments,
		ds.SubLedger.ExpenseReports,
	)
	for _, rows := range all {
		for _, row := range rows {
			if !entryIDs[row.EntryID] {
				return gen.InvariantError{
					Invariant: 4,
					Detail:    fmt.Sprintf("sub-ledger row references unknown entry %s", row.EntryID),
				}
			}
		}
	}
	return nil
}
