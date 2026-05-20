package report

import (
	"sort"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// ComputeStatements derives one CustomerStatement per active customer from ds.
// Opening balance is always zero (no prior-period carry-forward in v0.2).
// Invoiced = sum of invoice amounts attributed to this customer.
// Payments = sum of customer_payment amounts attributed to this customer.
// Closing = Invoiced - Payments.
// Only customers with at least one invoice or payment appear in the output.
// Output is sorted by CustomerID.
func ComputeStatements(ds gen.Dataset) []CustomerStatement {
	nCust := len(ds.Customers)
	if nCust == 0 {
		return nil
	}

	type accum struct {
		name     string
		invoiced decimal.Decimal
		payments decimal.Decimal
	}

	m := make(map[string]*accum)

	ensure := func(custID string) *accum {
		acc, ok := m[custID]
		if !ok {
			name := customerName(custID, ds.Customers)
			acc = &accum{name: name}
			m[custID] = acc
		}
		return acc
	}

	for _, row := range ds.SubLedger.Invoices {
		amt, err := decimal.NewFromString(row.Amount)
		if err != nil {
			continue
		}
		custID := deriveCustomerID(row.EntryID, nCust)
		ensure(custID).invoiced = ensure(custID).invoiced.Add(amt)
	}

	for _, row := range ds.SubLedger.CustomerPayments {
		amt, err := decimal.NewFromString(row.Amount)
		if err != nil {
			continue
		}
		custID := deriveCustomerID(row.EntryID, nCust)
		ensure(custID).payments = ensure(custID).payments.Add(amt)
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	stmts := make([]CustomerStatement, 0, len(keys))
	for _, k := range keys {
		acc := m[k]
		closing := acc.invoiced.Sub(acc.payments)
		stmts = append(stmts, CustomerStatement{
			CustomerID:   k,
			CustomerName: acc.name,
			Opening:      decimal.Zero,
			Invoiced:     acc.invoiced,
			Payments:     acc.payments,
			Closing:      closing,
		})
	}
	return stmts
}
