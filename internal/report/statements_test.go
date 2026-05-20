package report_test

import (
	"reflect"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// TestCustomerStatementIdentity asserts opening + invoiced - payments = closing
// for every customer statement derived from the canonical golden dataset.
func TestCustomerStatementIdentity(t *testing.T) {
	ds, _ := buildReports(t)
	stmts := report.ComputeStatements(ds)

	if len(stmts) == 0 {
		t.Fatal("expected at least one customer statement")
	}

	for _, s := range stmts {
		want := s.Opening.Add(s.Invoiced).Sub(s.Payments)
		if !s.Closing.Equal(want) {
			t.Errorf("customer %s: closing = %s; want opening + invoiced - payments = %s",
				s.CustomerID, s.Closing, want)
		}
	}
}

// TestCustomerStatementOpeningZero asserts opening balance is always zero.
func TestCustomerStatementOpeningZero(t *testing.T) {
	ds, _ := buildReports(t)
	stmts := report.ComputeStatements(ds)

	for _, s := range stmts {
		if !s.Opening.IsZero() {
			t.Errorf("customer %s: opening = %s; want 0", s.CustomerID, s.Opening)
		}
	}
}

// TestCustomerStatementSortedByID asserts the statements are sorted by CustomerID.
func TestCustomerStatementSortedByID(t *testing.T) {
	ds, _ := buildReports(t)
	stmts := report.ComputeStatements(ds)

	for i := 1; i < len(stmts); i++ {
		if stmts[i].CustomerID < stmts[i-1].CustomerID {
			t.Errorf("statements not sorted: %s before %s", stmts[i-1].CustomerID, stmts[i].CustomerID)
		}
	}
}

// TestCustomerStatementAmountsNonNegative asserts invoiced and payments are non-negative.
func TestCustomerStatementAmountsNonNegative(t *testing.T) {
	ds, _ := buildReports(t)
	stmts := report.ComputeStatements(ds)

	for _, s := range stmts {
		if s.Invoiced.LessThan(decimal.Zero) {
			t.Errorf("customer %s: invoiced = %s; want >= 0", s.CustomerID, s.Invoiced)
		}
		if s.Payments.LessThan(decimal.Zero) {
			t.Errorf("customer %s: payments = %s; want >= 0", s.CustomerID, s.Payments)
		}
	}
}

// TestCustomerStatementDeterministic asserts ComputeStatements is a pure function.
func TestCustomerStatementDeterministic(t *testing.T) {
	ds, _ := buildReports(t)
	s1 := report.ComputeStatements(ds)
	s2 := report.ComputeStatements(ds)
	if !reflect.DeepEqual(s1, s2) {
		t.Fatal("ComputeStatements produced different results on identical input")
	}
}
