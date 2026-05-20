package gen

// cashAccountCodes is the canonical set of account codes that represent cash
// accounts. Unexported to prevent external mutation; use IsCashAccount to query.
var cashAccountCodes = map[string]bool{
	"1010": true, // Checking Account
	"1020": true, // Savings Account
	"1030": true, // Petty Cash
}

// IsCashAccount reports whether code is one of the recognised cash account
// codes. It is the single source of truth used by both the projection engine
// and the invariant checker so the two sets can never diverge.
func IsCashAccount(code string) bool {
	return cashAccountCodes[code]
}

// defaultCoA is the hardcoded chart of accounts returned by DefaultCoA.
// Defined as a package-level slice so that DefaultCoA is O(1) and returns
// the same backing data on every call (callers must not mutate it).
var defaultCoA = []Account{
	// ── Assets (1000–1999) ─────────────────────────────────────────────────
	{Code: "1000", Name: "Cash and Cash Equivalents", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1010", Name: "Checking Account", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1020", Name: "Savings Account", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1030", Name: "Petty Cash", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1100", Name: "Accounts Receivable", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1110", Name: "Allowance for Doubtful Accounts", Type: AccountTypeAsset, NormalBalance: "credit"},
	{Code: "1200", Name: "Inventory", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1300", Name: "Prepaid Expenses", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1400", Name: "Other Current Assets", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1500", Name: "Property Plant and Equipment", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1510", Name: "Accumulated Depreciation", Type: AccountTypeAsset, NormalBalance: "credit"},
	{Code: "1600", Name: "Intangible Assets", Type: AccountTypeAsset, NormalBalance: "debit"},
	{Code: "1610", Name: "Accumulated Amortization", Type: AccountTypeAsset, NormalBalance: "credit"},
	{Code: "1700", Name: "Other Non-Current Assets", Type: AccountTypeAsset, NormalBalance: "debit"},

	// ── Liabilities (2000–2999) ────────────────────────────────────────────
	{Code: "2000", Name: "Accounts Payable", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2100", Name: "Accrued Liabilities", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2110", Name: "Accrued Salaries and Wages", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2120", Name: "Accrued Interest Payable", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2200", Name: "Short-Term Notes Payable", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2300", Name: "Deferred Revenue", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2400", Name: "Sales Tax Payable", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2500", Name: "Income Tax Payable", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2600", Name: "Long-Term Debt", Type: AccountTypeLiability, NormalBalance: "credit"},
	{Code: "2700", Name: "Other Non-Current Liabilities", Type: AccountTypeLiability, NormalBalance: "credit"},

	// ── Equity (3000–3999) ─────────────────────────────────────────────────
	{Code: "3000", Name: "Common Stock", Type: AccountTypeEquity, NormalBalance: "credit"},
	{Code: "3100", Name: "Additional Paid-In Capital", Type: AccountTypeEquity, NormalBalance: "credit"},
	{Code: "3200", Name: "Retained Earnings", Type: AccountTypeEquity, NormalBalance: "credit"},
	{Code: "3300", Name: "Dividends Paid", Type: AccountTypeEquity, NormalBalance: "debit"},
	{Code: "3400", Name: "Treasury Stock", Type: AccountTypeEquity, NormalBalance: "debit"},
	{Code: "3500", Name: "Accumulated Other Comprehensive Income", Type: AccountTypeEquity, NormalBalance: "credit"},

	// ── Revenue (4000–4999) ────────────────────────────────────────────────
	{Code: "4000", Name: "Sales Revenue", Type: AccountTypeRevenue, NormalBalance: "credit"},
	{Code: "4100", Name: "Service Revenue", Type: AccountTypeRevenue, NormalBalance: "credit"},
	{Code: "4200", Name: "Interest Income", Type: AccountTypeRevenue, NormalBalance: "credit"},
	{Code: "4300", Name: "Other Income", Type: AccountTypeRevenue, NormalBalance: "credit"},
	{Code: "4400", Name: "Sales Returns and Allowances", Type: AccountTypeRevenue, NormalBalance: "debit"},
	{Code: "4500", Name: "Sales Discounts", Type: AccountTypeRevenue, NormalBalance: "debit"},

	// ── Expenses (5000–5999) ───────────────────────────────────────────────
	{Code: "5000", Name: "Cost of Goods Sold", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5100", Name: "Salaries and Wages Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5110", Name: "Payroll Tax Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5200", Name: "Rent Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5210", Name: "Utilities Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5220", Name: "Office Supplies Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5300", Name: "Travel and Entertainment Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5310", Name: "Meals and Entertainment Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5400", Name: "Depreciation Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5410", Name: "Amortization Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5500", Name: "Insurance Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5600", Name: "Professional Fees Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5700", Name: "Advertising and Marketing Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5800", Name: "Interest Expense", Type: AccountTypeExpense, NormalBalance: "debit"},
	{Code: "5900", Name: "Other Operating Expenses", Type: AccountTypeExpense, NormalBalance: "debit"},
}

// DefaultCoACount is the exact number of accounts in DefaultCoA. Tests assert
// this value to confirm the CoA is neither truncated nor accidentally extended.
const DefaultCoACount = 51

// DefaultCoA returns the hardcoded chart of accounts for v0.1.
// The returned slice is a copy; callers may not mutate the backing array.
func DefaultCoA() []Account {
	out := make([]Account, len(defaultCoA))
	copy(out, defaultCoA)
	return out
}

// IndexCoA builds a lookup map from account code to Account.
// It does not validate for duplicates; use DefaultCoA (which is duplicate-free
// by construction) as input in production paths.
func IndexCoA(accounts []Account) map[string]Account {
	m := make(map[string]Account, len(accounts))
	for _, a := range accounts {
		m[a.Code] = a
	}
	return m
}
