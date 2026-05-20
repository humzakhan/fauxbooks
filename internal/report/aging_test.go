package report_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/report"
)

// periodEnd for January 2026.
var jan2026End = time.Date(2026, 1, 31, 23, 59, 59, 999999999, time.UTC)

// buildMinimalDS returns a Dataset with one customer and one vendor, with
// provided invoice/bill OccurredAt strings and amounts.
func buildMinimalDS(
	invoiceAt, invoiceAmt string,
	billAt, billAmt string,
) gen.Dataset {
	cust := gen.Customer{ID: "cust-001", Name: "Acme Corp"}
	vend := gen.Vendor{ID: "vend-001", Name: "Globex Ltd"}

	// We need to choose EntryIDs that hash to index 0 (cust-001 / vend-001).
	// deriveCustomerID(entryID, 1) always returns cust-001 when n=1.
	// deriveVendorID(entryID, 1) always returns vend-001 when n=1.
	ds := gen.Dataset{
		Customers: []gen.Customer{cust},
		Vendors:   []gen.Vendor{vend},
	}
	if invoiceAt != "" {
		ds.SubLedger.Invoices = []gen.SubLedgerRow{
			{EntryID: "e-inv-1", OccurredAt: invoiceAt, Amount: invoiceAmt, SubLedgerType: "invoice"},
		}
	}
	if billAt != "" {
		ds.SubLedger.Bills = []gen.SubLedgerRow{
			{EntryID: "e-bill-1", OccurredAt: billAt, Amount: billAmt, SubLedgerType: "bill"},
		}
	}
	return ds
}

// rfc3339 formats a time.Time as RFC3339Nano string.
func rfc3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// TestAgingBucketBoundary30Days verifies an invoice exactly 30 days before
// periodEnd lands in the 0-30 bucket.
func TestAgingBucketBoundary30Days(t *testing.T) {
	// Exactly 30 days before periodEnd (noon on the 30th day before).
	occurredAt := jan2026End.Add(-30 * 24 * time.Hour)
	ds := buildMinimalDS(rfc3339(occurredAt), "100.000000000", "", "")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AR) == 0 {
		t.Fatal("expected at least one AR row")
	}
	row := aging.AR[0]
	want, _ := decimal.NewFromString("100.000000000")
	if !row.Bucket0_30.Equal(want) {
		t.Errorf("30-day invoice: Bucket0_30 = %s; want %s", row.Bucket0_30, want)
	}
	if !row.Bucket31_60.IsZero() {
		t.Errorf("30-day invoice: Bucket31_60 should be zero, got %s", row.Bucket31_60)
	}
}

// TestAgingBucketBoundary31Days verifies an invoice exactly 31 days before
// periodEnd lands in the 31-60 bucket.
func TestAgingBucketBoundary31Days(t *testing.T) {
	occurredAt := jan2026End.Add(-31 * 24 * time.Hour)
	ds := buildMinimalDS(rfc3339(occurredAt), "200.000000000", "", "")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AR) == 0 {
		t.Fatal("expected at least one AR row")
	}
	row := aging.AR[0]
	want, _ := decimal.NewFromString("200.000000000")
	if !row.Bucket31_60.Equal(want) {
		t.Errorf("31-day invoice: Bucket31_60 = %s; want %s", row.Bucket31_60, want)
	}
	if !row.Bucket0_30.IsZero() {
		t.Errorf("31-day invoice: Bucket0_30 should be zero, got %s", row.Bucket0_30)
	}
}

// TestAgingBucketBoundary61Days verifies an invoice exactly 61 days before
// periodEnd lands in the 61-90 bucket.
func TestAgingBucketBoundary61Days(t *testing.T) {
	occurredAt := jan2026End.Add(-61 * 24 * time.Hour)
	ds := buildMinimalDS(rfc3339(occurredAt), "300.000000000", "", "")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AR) == 0 {
		t.Fatal("expected at least one AR row")
	}
	row := aging.AR[0]
	want, _ := decimal.NewFromString("300.000000000")
	if !row.Bucket61_90.Equal(want) {
		t.Errorf("61-day invoice: Bucket61_90 = %s; want %s", row.Bucket61_90, want)
	}
}

// TestAgingBucketBoundary91Days verifies an invoice exactly 91 days before
// periodEnd lands in the 91+ bucket.
func TestAgingBucketBoundary91Days(t *testing.T) {
	occurredAt := jan2026End.Add(-91 * 24 * time.Hour)
	ds := buildMinimalDS(rfc3339(occurredAt), "400.000000000", "", "")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AR) == 0 {
		t.Fatal("expected at least one AR row")
	}
	row := aging.AR[0]
	want, _ := decimal.NewFromString("400.000000000")
	if !row.Bucket91Plus.Equal(want) {
		t.Errorf("91-day invoice: Bucket91Plus = %s; want %s", row.Bucket91Plus, want)
	}
}

// TestAgingAPBucketBoundary30Days verifies a bill exactly 30 days before
// periodEnd lands in the 0-30 bucket.
func TestAgingAPBucketBoundary30Days(t *testing.T) {
	occurredAt := jan2026End.Add(-30 * 24 * time.Hour)
	ds := buildMinimalDS("", "", rfc3339(occurredAt), "150.000000000")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AP) == 0 {
		t.Fatal("expected at least one AP row")
	}
	row := aging.AP[0]
	want, _ := decimal.NewFromString("150.000000000")
	if !row.Bucket0_30.Equal(want) {
		t.Errorf("30-day bill: Bucket0_30 = %s; want %s", row.Bucket0_30, want)
	}
}

// TestAgingAPBucketBoundary31Days verifies a bill exactly 31 days before
// periodEnd lands in the 31-60 bucket.
func TestAgingAPBucketBoundary31Days(t *testing.T) {
	occurredAt := jan2026End.Add(-31 * 24 * time.Hour)
	ds := buildMinimalDS("", "", rfc3339(occurredAt), "250.000000000")

	aging := report.ComputeAging(ds, jan2026End)
	if len(aging.AP) == 0 {
		t.Fatal("expected at least one AP row")
	}
	row := aging.AP[0]
	want, _ := decimal.NewFromString("250.000000000")
	if !row.Bucket31_60.Equal(want) {
		t.Errorf("31-day bill: Bucket31_60 = %s; want %s", row.Bucket31_60, want)
	}
}

// TestAgingTotalEqualsSum verifies Total = sum of all buckets.
func TestAgingTotalEqualsSum(t *testing.T) {
	// Two invoices: one in 0-30, one in 31-60 bucket.
	ds := gen.Dataset{
		Customers: []gen.Customer{{ID: "cust-001", Name: "Acme"}},
	}
	now := jan2026End
	ds.SubLedger.Invoices = []gen.SubLedgerRow{
		{EntryID: "e1", OccurredAt: rfc3339(now.Add(-10 * 24 * time.Hour)), Amount: "100.000000000"},
		{EntryID: "e2", OccurredAt: rfc3339(now.Add(-40 * 24 * time.Hour)), Amount: "200.000000000"},
	}

	aging := report.ComputeAging(ds, now)
	for _, row := range aging.AR {
		bucketSum := row.Bucket0_30.Add(row.Bucket31_60).Add(row.Bucket61_90).Add(row.Bucket91Plus)
		if !bucketSum.Equal(row.Total) {
			t.Errorf("Total %s != bucket sum %s for party %s", row.Total, bucketSum, row.PartyID)
		}
	}
}

// TestAgingDeterministic verifies ComputeAging returns identical output on two calls.
func TestAgingDeterministic(t *testing.T) {
	ds, _ := buildReports(t)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)
	a1 := report.ComputeAging(ds, periodEnd)
	a2 := report.ComputeAging(ds, periodEnd)
	if !reflect.DeepEqual(a1, a2) {
		t.Fatal("ComputeAging produced different results on identical input")
	}
}
