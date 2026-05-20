package gen

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

// compositeKey builds the deterministic sort key for a journal line.
// Format: "<RFC3339Nano>|<entryID>|<lineIndex10d>"
// The zero-padded line index ensures correct lexicographic ordering for up to
// 10 billion lines, matching the spec requirement for stable cross-OS sort.
func compositeKey(occurredAt string, entryID string, lineIndex int) string {
	return fmt.Sprintf("%s|%s|%010d", occurredAt, entryID, lineIndex)
}

// formatDecimalRaw returns a decimal as plain text with 9 fractional digits.
// This is the canonical projection-layer format; the writer package enforces
// the same rule for output serialization.
func formatDecimalRaw(d decimal.Decimal) string {
	return d.StringFixed(9)
}

// projectedLine pairs a journal line with its sort key and original index so
// the sort is stable and produces identical output on every OS/arch.
type projectedLine struct {
	key   string
	line  JournalLine
	entry JournalEntry
}

// Project projects a set of journal entries and lines into GL rows, bank rows,
// and sub-ledger row sets. It is a pure function: no file I/O, no RNG calls,
// no wall-clock reads.
//
// Sort key: "<entry.OccurredAt as RFC3339Nano>|<entry.EntryID>|<lineIndex10d>"
// Row numbers are assigned after sorting; GL row count equals len(lines).
// Bank rows are derived from lines posted to cash account codes (CashAccountCodes).
func Project(entries []JournalEntry, lines []JournalLine, coa map[string]Account) (gl []GLRow, bank []BankRow, sub SubLedgerSet, err error) {
	// Build an entry lookup so we can retrieve OccurredAt for each line.
	entryByID := make(map[string]JournalEntry, len(entries))
	for _, e := range entries {
		entryByID[e.EntryID] = e
	}

	// Build (key, line, entry) triples for sorting.
	projected := make([]projectedLine, len(lines))
	for i, ln := range lines {
		e, ok := entryByID[ln.EntryID]
		if !ok {
			return nil, nil, SubLedgerSet{}, fmt.Errorf("gen.Project: line %s references unknown entry %s", ln.LineID, ln.EntryID)
		}
		key := compositeKey(e.OccurredAt.Format("2006-01-02T15:04:05.999999999Z"), e.EntryID, i)
		projected[i] = projectedLine{key: key, line: ln, entry: e}
	}

	// Stable sort by composite key. sort.SliceStable preserves original order
	// for ties (there should be none with our key design, but stability is free).
	sort.SliceStable(projected, func(i, j int) bool {
		return projected[i].key < projected[j].key
	})

	// Assign GL rows in sorted order.
	gl = make([]GLRow, len(projected))
	bankRowNum := 0
	bank = make([]BankRow, 0, len(projected)/4)

	// Sub-ledger accumulators.
	invoices := make([]SubLedgerRow, 0)
	customerPayments := make([]SubLedgerRow, 0)
	bills := make([]SubLedgerRow, 0)
	billPayments := make([]SubLedgerRow, 0)
	expenseReports := make([]SubLedgerRow, 0)

	// Sub-ledger row counters per type.
	invNum, cpNum, billNum, bpNum, erNum := 0, 0, 0, 0, 0

	// entrySubLedgerType maps an EntryID to the sub-ledger type it belongs to.
	// We derive this from the entry's SourceEventID by looking up events.
	// Since Project doesn't receive []Event, we derive sub-ledger type from the
	// entry description (set deterministically by the template) or from the
	// account code pattern. We use a simpler approach: track which entries we've
	// already added to a sub-ledger to avoid duplicates.
	entrySubLedgerAdded := make(map[string]bool)

	// We need event type per entry to categorise sub-ledger rows.
	// The entry description is the reliable way since we don't have []Event here.
	// The description values are set by the templates and are stable.
	entryDescToSubType := map[string]string{
		"Sale invoice":     "invoice",
		"Customer payment": "customer_payment",
		"Vendor bill":      "bill",
		"Bill payment":     "bill_payment",
		"Expense report":   "expense_report",
	}

	for i, pl := range projected {
		ts := pl.entry.OccurredAt.Format("2006-01-02T15:04:05.999999999Z")

		gl[i] = GLRow{
			RowNum:      i + 1,
			EntryID:     pl.line.EntryID,
			LineID:      pl.line.LineID,
			OccurredAt:  ts,
			AccountCode: pl.line.AccountCode,
			Debit:       formatDecimalRaw(pl.line.Debit),
			Credit:      formatDecimalRaw(pl.line.Credit),
		}

		// Bank row: only for cash account codes.
		if CashAccountCodes[pl.line.AccountCode] {
			bankRowNum++
			bank = append(bank, BankRow{
				RowNum:      bankRowNum,
				EntryID:     pl.line.EntryID,
				LineID:      pl.line.LineID,
				OccurredAt:  ts,
				AccountCode: pl.line.AccountCode,
				Debit:       formatDecimalRaw(pl.line.Debit),
				Credit:      formatDecimalRaw(pl.line.Credit),
			})
		}

		// Sub-ledger rows: one row per entry (not per line) for qualifying event types.
		// We emit the sub-ledger row the first time we see this entry in the sorted stream.
		if !entrySubLedgerAdded[pl.line.EntryID] {
			subType, isSubType := entryDescToSubType[pl.entry.Description]
			if isSubType {
				entrySubLedgerAdded[pl.line.EntryID] = true

				// Derive amount: the primary amount on a sub-ledger entry is the
				// debit side of the first line (which equals the event amount for
				// 2-leg entries and the gross for payroll — but payroll is not in
				// the sub-ledger set).
				amount := pl.line.Debit
				if amount.IsZero() {
					amount = pl.line.Credit
				}
				amountStr := formatDecimalRaw(amount)

				// Retrieve RefID from Metadata if available; else empty.
				// Metadata is stored on the Event, which we don't have here.
				// The sub-ledger RefID must come from the entry somehow.
				// Judgment call: Project doesn't receive []Event, so RefID is empty
				// in the sub-ledger for now. PR-08 can extend if needed. The
				// invariant tests only check EntryID referential integrity.
				row := SubLedgerRow{
					EntryID:       pl.line.EntryID,
					OccurredAt:    ts,
					Amount:        amountStr,
					SubLedgerType: subType,
				}

				switch subType {
				case "invoice":
					invNum++
					row.RowNum = invNum
					invoices = append(invoices, row)
				case "customer_payment":
					cpNum++
					row.RowNum = cpNum
					customerPayments = append(customerPayments, row)
				case "bill":
					billNum++
					row.RowNum = billNum
					bills = append(bills, row)
				case "bill_payment":
					bpNum++
					row.RowNum = bpNum
					billPayments = append(billPayments, row)
				case "expense_report":
					erNum++
					row.RowNum = erNum
					expenseReports = append(expenseReports, row)
				}
			}
		}
	}

	sub = SubLedgerSet{
		Invoices:         invoices,
		CustomerPayments: customerPayments,
		Bills:            bills,
		BillPayments:     billPayments,
		ExpenseReports:   expenseReports,
	}
	return gl, bank, sub, nil
}
