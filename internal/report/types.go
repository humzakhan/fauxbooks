// Package report computes period-derived financial statements (Profit & Loss,
// Balance Sheet, Cash Flow, Trial Balance) as typed structs from an in-memory
// gen.Dataset. All monetary values are decimal.Decimal; no float64 anywhere.
//
// The package is pure: no file I/O, no RNG, no wall-clock reads. Writers in
// internal/writer/ serialise the typed structs to CSV and JSONL.
package report

import "github.com/shopspring/decimal"

// Reports bundles the four period-derived financial reports produced from one
// gen.Dataset. Each field is independently consumable; the writer emits one
// output file pair (.csv + .jsonl) per field.
type Reports struct {
	ProfitLoss   ProfitLoss
	BalanceSheet BalanceSheet
	CashFlow     CashFlow
	TrialBalance TrialBalance
}

// ProfitLoss summarises revenue minus expense for the period.
// Amounts on Revenue rows are signed in the credit-natural direction
// (contra-revenue accounts therefore appear with a negative Amount).
// Amounts on Expense rows are signed in the debit-natural direction.
type ProfitLoss struct {
	Revenue      []ProfitLossRow
	Expense      []ProfitLossRow
	TotalRevenue decimal.Decimal
	TotalExpense decimal.Decimal
	NetIncome    decimal.Decimal
}

// ProfitLossRow is one account's contribution to a P&L section.
type ProfitLossRow struct {
	AccountCode string
	AccountName string
	Amount      decimal.Decimal
}

// BalanceSheet snapshots account balances at period end.
// Asset amounts are signed debit-natural (contra-assets appear negative).
// Liability and Equity amounts are signed credit-natural.
// TotalEquity already includes NetIncome (period earnings flowing to equity);
// NetIncome is also surfaced separately for transparency.
// The fundamental accounting equation TotalAssets = TotalLiabilities + TotalEquity
// is guaranteed by invariant 1 (Σ debits = Σ credits across the dataset).
type BalanceSheet struct {
	Assets                    []BalanceSheetRow
	Liabilities               []BalanceSheetRow
	Equity                    []BalanceSheetRow
	TotalAssets               decimal.Decimal
	TotalLiabilities          decimal.Decimal
	TotalEquity               decimal.Decimal
	NetIncome                 decimal.Decimal
	TotalLiabilitiesAndEquity decimal.Decimal
}

// BalanceSheetRow is one account's contribution to a balance sheet section.
type BalanceSheetRow struct {
	AccountCode string
	AccountName string
	Amount      decimal.Decimal
}

// CashFlow summarises the net change in cash across the period.
// OpeningCash is always zero in v0.2 because the run has no opening balances.
// Inflows enumerate per-cash-account debit totals; Outflows enumerate credits.
// EndingCash = OpeningCash + NetChange and equals the balance-sheet cash total.
type CashFlow struct {
	OpeningCash decimal.Decimal
	Inflows     []CashFlowRow
	Outflows    []CashFlowRow
	TotalIn     decimal.Decimal
	TotalOut    decimal.Decimal
	NetChange   decimal.Decimal
	EndingCash  decimal.Decimal
}

// CashFlowRow is one cash account's inflow or outflow total.
type CashFlowRow struct {
	AccountCode string
	AccountName string
	Amount      decimal.Decimal
}

// TrialBalance lists every account whose period activity is non-zero, with
// the balance reported in its natural column. By invariant 1 the sum of the
// Debit column equals the sum of the Credit column across all rows.
type TrialBalance struct {
	Rows []TrialBalanceRow
}

// TrialBalanceRow is one account's net activity. Exactly one of Debit or
// Credit is non-zero (the side matching the account's net direction).
type TrialBalanceRow struct {
	AccountCode string
	AccountName string
	AccountType string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
}
