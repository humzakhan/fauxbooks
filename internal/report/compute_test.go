package report_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/report"
)

// canonicalCfg matches the fixture config used by the gen pipeline tests so
// reports computed here are derived from the committed golden dataset.
var canonicalCfg = gen.Config{
	Seed:   1,
	Period: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume: "small",
	Bundle: "monthly-close",
	Format: "csv",
}

func buildReports(t *testing.T) (gen.Dataset, report.Reports) {
	t.Helper()
	ds, err := gen.Run(canonicalCfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}
	return ds, report.Compute(ds)
}

// TestComputeDeterministic asserts Compute is a pure function: two calls with
// the same Dataset return structurally identical Reports.
func TestComputeDeterministic(t *testing.T) {
	ds, r1 := buildReports(t)
	r2 := report.Compute(ds)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("Compute produced different Reports on identical input")
	}
}

// TestTrialBalanceBalances asserts the sum of the debit column equals the sum
// of the credit column across all trial balance rows. This is guaranteed by
// invariant 1 (Σ debits = Σ credits across the dataset).
func TestTrialBalanceBalances(t *testing.T) {
	_, r := buildReports(t)

	var debits, credits decimal.Decimal
	for _, row := range r.TrialBalance.Rows {
		debits = debits.Add(row.Debit)
		credits = credits.Add(row.Credit)
	}
	if !debits.Equal(credits) {
		t.Errorf("trial balance: Σ debit = %s, Σ credit = %s", debits, credits)
	}
}

// TestProfitLossIdentity asserts NetIncome = TotalRevenue − TotalExpense.
func TestProfitLossIdentity(t *testing.T) {
	_, r := buildReports(t)
	pl := r.ProfitLoss
	want := pl.TotalRevenue.Sub(pl.TotalExpense)
	if !pl.NetIncome.Equal(want) {
		t.Errorf("NetIncome = %s; want TotalRevenue - TotalExpense = %s", pl.NetIncome, want)
	}
}

// TestBalanceSheetEquation asserts TotalAssets = TotalLiabilities + TotalEquity
// (where TotalEquity already includes NetIncome for the period). This holds by
// invariant 1: Σ debits = Σ credits implies the fundamental accounting equation.
func TestBalanceSheetEquation(t *testing.T) {
	_, r := buildReports(t)
	bs := r.BalanceSheet
	rhs := bs.TotalLiabilities.Add(bs.TotalEquity)
	if !bs.TotalAssets.Equal(rhs) {
		t.Errorf("balance sheet: TotalAssets = %s; want TotalLiabilities + TotalEquity = %s",
			bs.TotalAssets, rhs)
	}
	if !bs.TotalLiabilitiesAndEquity.Equal(rhs) {
		t.Errorf("TotalLiabilitiesAndEquity = %s; want %s", bs.TotalLiabilitiesAndEquity, rhs)
	}
}

// TestCashFlowMatchesBalanceSheetCash asserts the ending cash from the cash
// flow statement equals the sum of cash-account balances on the balance sheet.
// Both derive from the same Lines, so any discrepancy indicates a bug in the
// account-type classification or the cash-account predicate.
func TestCashFlowMatchesBalanceSheetCash(t *testing.T) {
	_, r := buildReports(t)

	var bsCash decimal.Decimal
	for _, row := range r.BalanceSheet.Assets {
		if gen.IsCashAccount(row.AccountCode) {
			bsCash = bsCash.Add(row.Amount)
		}
	}
	if !r.CashFlow.EndingCash.Equal(bsCash) {
		t.Errorf("cash flow EndingCash = %s; balance-sheet cash total = %s",
			r.CashFlow.EndingCash, bsCash)
	}
}
