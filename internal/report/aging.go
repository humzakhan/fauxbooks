package report

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// ComputeAging derives AR aging (by invoice) and AP aging (by bill) from ds.
// periodEnd is the last instant of the period month; all OccurredAt timestamps
// are compared against it to compute age in days.
//
// Aging semantics: gross invoiced/billed amounts per party per bucket.
// No payment offsets are applied (no invoice-to-payment matching in v0.2).
//
// Day calculation: daysOld = int(periodEnd.Sub(occurredAt).Hours() / 24).
// Buckets: 0-30, 31-60, 61-90, 91+.
func ComputeAging(ds gen.Dataset, periodEnd time.Time) AgingReport {
	nCust := len(ds.Customers)
	nVend := len(ds.Vendors)

	// accum maps partyID -> mutable aging accumulator
	type agingAccum struct {
		name    string
		b0_30   decimal.Decimal
		b31_60  decimal.Decimal
		b61_90  decimal.Decimal
		b91plus decimal.Decimal
	}

	arMap := make(map[string]*agingAccum)
	apMap := make(map[string]*agingAccum)

	addToMap := func(m map[string]*agingAccum, partyID, name string, daysOld int, amt decimal.Decimal) {
		acc, ok := m[partyID]
		if !ok {
			acc = &agingAccum{name: name}
			m[partyID] = acc
		}
		switch {
		case daysOld <= 30:
			acc.b0_30 = acc.b0_30.Add(amt)
		case daysOld <= 60:
			acc.b31_60 = acc.b31_60.Add(amt)
		case daysOld <= 90:
			acc.b61_90 = acc.b61_90.Add(amt)
		default:
			acc.b91plus = acc.b91plus.Add(amt)
		}
	}

	// AR: invoices
	if nCust > 0 {
		for _, row := range ds.SubLedger.Invoices {
			occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
			if err != nil {
				continue
			}
			amt, err := decimal.NewFromString(row.Amount)
			if err != nil {
				continue
			}
			custID := deriveCustomerID(row.EntryID, nCust)
			name := customerName(custID, ds.Customers)
			daysOld := int(periodEnd.Sub(occurredAt).Hours() / 24)
			addToMap(arMap, custID, name, daysOld, amt)
		}
	}

	// AP: bills
	if nVend > 0 {
		for _, row := range ds.SubLedger.Bills {
			occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
			if err != nil {
				continue
			}
			amt, err := decimal.NewFromString(row.Amount)
			if err != nil {
				continue
			}
			vendID := deriveVendorID(row.EntryID, nVend)
			name := vendorName(vendID, ds.Vendors)
			daysOld := int(periodEnd.Sub(occurredAt).Hours() / 24)
			addToMap(apMap, vendID, name, daysOld, amt)
		}
	}

	buildRows := func(m map[string]*agingAccum) []AgingRow {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		rows := make([]AgingRow, 0, len(keys))
		for _, k := range keys {
			acc := m[k]
			total := acc.b0_30.Add(acc.b31_60).Add(acc.b61_90).Add(acc.b91plus)
			rows = append(rows, AgingRow{
				PartyID:      k,
				PartyName:    acc.name,
				Bucket0_30:   acc.b0_30,
				Bucket31_60:  acc.b31_60,
				Bucket61_90:  acc.b61_90,
				Bucket91Plus: acc.b91plus,
				Total:        total,
			})
		}
		return rows
	}

	return AgingReport{
		AR: buildRows(arMap),
		AP: buildRows(apMap),
	}
}
