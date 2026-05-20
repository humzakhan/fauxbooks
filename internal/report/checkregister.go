package report

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// ComputeCheckRegister derives the check register from ds.
// One row per BillPayment + one row per ExpenseReport sub-ledger entry.
// Account code is always "1010" (the cash account credited by both templates).
// Payee is the vendor name derived from the EntryID.
// Sorted by (Date, EntryID) ascending.
func ComputeCheckRegister(ds gen.Dataset) []CheckRegisterRow {
	nVend := len(ds.Vendors)

	rows := make([]CheckRegisterRow, 0, len(ds.SubLedger.BillPayments)+len(ds.SubLedger.ExpenseReports))

	addRow := func(row gen.SubLedgerRow) {
		occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
		if err != nil {
			return
		}
		amt, err := decimal.NewFromString(row.Amount)
		if err != nil {
			return
		}
		var payee string
		if nVend > 0 {
			vendID := deriveVendorID(row.EntryID, nVend)
			payee = vendorName(vendID, ds.Vendors)
		}
		rows = append(rows, CheckRegisterRow{
			Date:        occurredAt,
			EntryID:     row.EntryID,
			AccountCode: "1010",
			Payee:       payee,
			Amount:      amt,
		})
	}

	for _, row := range ds.SubLedger.BillPayments {
		addRow(row)
	}
	for _, row := range ds.SubLedger.ExpenseReports {
		addRow(row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date.Equal(rows[j].Date) {
			return rows[i].EntryID < rows[j].EntryID
		}
		return rows[i].Date.Before(rows[j].Date)
	})

	return rows
}
