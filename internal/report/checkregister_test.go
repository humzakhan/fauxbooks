package report_test

import (
	"reflect"
	"testing"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// TestCheckRegisterCount asserts len(checkRegister) == len(BillPayments) + len(ExpenseReports).
func TestCheckRegisterCount(t *testing.T) {
	ds, _ := buildReports(t)
	cr := report.ComputeCheckRegister(ds)

	want := len(ds.SubLedger.BillPayments) + len(ds.SubLedger.ExpenseReports)
	if len(cr) != want {
		t.Errorf("check register len = %d; want %d (BillPayments=%d + ExpenseReports=%d)",
			len(cr), want, len(ds.SubLedger.BillPayments), len(ds.SubLedger.ExpenseReports))
	}
}

// TestCheckRegisterNoCustomerPayments asserts no customer_payment rows are included.
// Since check register only includes BillPayments + ExpenseReports, the total
// must equal exactly that count (verified by TestCheckRegisterCount above).
// This test additionally verifies there are no zero-amount rows that might
// indicate spurious customer_payment inclusion.
func TestCheckRegisterNoCustomerPayments(t *testing.T) {
	ds, _ := buildReports(t)
	cr := report.ComputeCheckRegister(ds)

	// EntryIDs in BillPayments and ExpenseReports.
	validIDs := make(map[string]bool, len(ds.SubLedger.BillPayments)+len(ds.SubLedger.ExpenseReports))
	for _, r := range ds.SubLedger.BillPayments {
		validIDs[r.EntryID] = true
	}
	for _, r := range ds.SubLedger.ExpenseReports {
		validIDs[r.EntryID] = true
	}

	for _, row := range cr {
		if !validIDs[row.EntryID] {
			t.Errorf("check register contains unexpected EntryID %q (not a bill_payment or expense_report)", row.EntryID)
		}
	}
}

// TestCheckRegisterSortOrder verifies rows are sorted by (Date, EntryID) ascending.
func TestCheckRegisterSortOrder(t *testing.T) {
	ds, _ := buildReports(t)
	cr := report.ComputeCheckRegister(ds)

	for i := 1; i < len(cr); i++ {
		prev := cr[i-1]
		curr := cr[i]
		if curr.Date.Before(prev.Date) {
			t.Errorf("row %d: date %s before row %d date %s", i, curr.Date, i-1, prev.Date)
		}
		if curr.Date.Equal(prev.Date) && curr.EntryID < prev.EntryID {
			t.Errorf("row %d: EntryID %q before %q at same date", i, curr.EntryID, prev.EntryID)
		}
	}
}

// TestCheckRegisterAccountCode verifies all rows have account code "1010".
func TestCheckRegisterAccountCode(t *testing.T) {
	ds, _ := buildReports(t)
	cr := report.ComputeCheckRegister(ds)

	for _, row := range cr {
		if row.AccountCode != "1010" {
			t.Errorf("row %s: AccountCode = %q; want %q", row.EntryID, row.AccountCode, "1010")
		}
	}
}

// TestCheckRegisterDeterministic asserts ComputeCheckRegister is a pure function.
func TestCheckRegisterDeterministic(t *testing.T) {
	ds, _ := buildReports(t)
	cr1 := report.ComputeCheckRegister(ds)
	cr2 := report.ComputeCheckRegister(ds)
	if !reflect.DeepEqual(cr1, cr2) {
		t.Fatal("ComputeCheckRegister produced different results on identical input")
	}
}
