package writer

import (
	"fmt"
	"io"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// reportRowJSONL is the JSONL representation of a section-tagged report row
// (Profit & Loss, Balance Sheet, Cash Flow). Account code and name are empty
// strings for total or summary rows. Amount is decimal text with 9 fractional
// digits, never a JSON number.
type reportRowJSONL struct {
	Section     string `json:"section"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Amount      string `json:"amount"`
}

// trialBalanceRowJSONL is the JSONL representation of one trial balance row.
type trialBalanceRowJSONL struct {
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
}

// WriteProfitLoss writes a P&L report to w as CSV.
// Columns: section, account_code, account_name, amount.
// Sections in order: revenue (per-account), total_revenue, expense (per-account),
// total_expense, net_income.
func WriteProfitLoss(w io.Writer, pl report.ProfitLoss) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"section", "account_code", "account_name", "amount"}); err != nil {
		return 0, fmt.Errorf("WriteProfitLoss: csv header: %w", err)
	}

	for _, r := range pl.Revenue {
		if err = cw.Write([]string{"revenue", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteProfitLoss: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_revenue", "", "", FormatDecimal(pl.TotalRevenue)}); err != nil {
		return 0, fmt.Errorf("WriteProfitLoss: csv row: %w", err)
	}
	n++
	for _, r := range pl.Expense {
		if err = cw.Write([]string{"expense", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteProfitLoss: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_expense", "", "", FormatDecimal(pl.TotalExpense)}); err != nil {
		return 0, fmt.Errorf("WriteProfitLoss: csv row: %w", err)
	}
	n++
	if err = cw.Write([]string{"net_income", "", "", FormatDecimal(pl.NetIncome)}); err != nil {
		return 0, fmt.Errorf("WriteProfitLoss: csv row: %w", err)
	}
	n++

	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteProfitLoss: %w", err)
	}
	return n, nil
}

// WriteProfitLossJSONL writes a P&L report to w as JSONL, one object per row.
func WriteProfitLossJSONL(w io.Writer, pl report.ProfitLoss) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range pl.Revenue {
		if err = enc.Encode(reportRowJSONL{Section: "revenue", AccountCode: r.AccountCode, AccountName: r.AccountName, Amount: FormatDecimal(r.Amount)}); err != nil {
			return n, fmt.Errorf("WriteProfitLossJSONL: %w", err)
		}
		n++
	}
	if err = enc.Encode(reportRowJSONL{Section: "total_revenue", Amount: FormatDecimal(pl.TotalRevenue)}); err != nil {
		return n, fmt.Errorf("WriteProfitLossJSONL: %w", err)
	}
	n++
	for _, r := range pl.Expense {
		if err = enc.Encode(reportRowJSONL{Section: "expense", AccountCode: r.AccountCode, AccountName: r.AccountName, Amount: FormatDecimal(r.Amount)}); err != nil {
			return n, fmt.Errorf("WriteProfitLossJSONL: %w", err)
		}
		n++
	}
	if err = enc.Encode(reportRowJSONL{Section: "total_expense", Amount: FormatDecimal(pl.TotalExpense)}); err != nil {
		return n, fmt.Errorf("WriteProfitLossJSONL: %w", err)
	}
	n++
	if err = enc.Encode(reportRowJSONL{Section: "net_income", Amount: FormatDecimal(pl.NetIncome)}); err != nil {
		return n, fmt.Errorf("WriteProfitLossJSONL: %w", err)
	}
	n++
	return n, nil
}

// WriteBalanceSheet writes a balance sheet to w as CSV.
// Columns: section, account_code, account_name, amount.
// Sections in order: asset, total_assets, liability, total_liabilities,
// equity (per-account), net_income, total_equity, total_liabilities_and_equity.
func WriteBalanceSheet(w io.Writer, bs report.BalanceSheet) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"section", "account_code", "account_name", "amount"}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv header: %w", err)
	}

	for _, r := range bs.Assets {
		if err = cw.Write([]string{"asset", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_assets", "", "", FormatDecimal(bs.TotalAssets)}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
	}
	n++
	for _, r := range bs.Liabilities {
		if err = cw.Write([]string{"liability", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_liabilities", "", "", FormatDecimal(bs.TotalLiabilities)}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
	}
	n++
	for _, r := range bs.Equity {
		if err = cw.Write([]string{"equity", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"net_income", "", "", FormatDecimal(bs.NetIncome)}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
	}
	n++
	if err = cw.Write([]string{"total_equity", "", "", FormatDecimal(bs.TotalEquity)}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
	}
	n++
	if err = cw.Write([]string{"total_liabilities_and_equity", "", "", FormatDecimal(bs.TotalLiabilitiesAndEquity)}); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: csv row: %w", err)
	}
	n++

	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteBalanceSheet: %w", err)
	}
	return n, nil
}

// WriteBalanceSheetJSONL writes a balance sheet to w as JSONL, one object per row.
func WriteBalanceSheetJSONL(w io.Writer, bs report.BalanceSheet) (n int, err error) {
	enc := newJSONLEncoder(w)

	emit := func(section, code, name, amount string) error {
		return enc.Encode(reportRowJSONL{Section: section, AccountCode: code, AccountName: name, Amount: amount})
	}

	for _, r := range bs.Assets {
		if err = emit("asset", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)); err != nil {
			return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
		}
		n++
	}
	if err = emit("total_assets", "", "", FormatDecimal(bs.TotalAssets)); err != nil {
		return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
	}
	n++
	for _, r := range bs.Liabilities {
		if err = emit("liability", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)); err != nil {
			return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
		}
		n++
	}
	if err = emit("total_liabilities", "", "", FormatDecimal(bs.TotalLiabilities)); err != nil {
		return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
	}
	n++
	for _, r := range bs.Equity {
		if err = emit("equity", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)); err != nil {
			return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
		}
		n++
	}
	if err = emit("net_income", "", "", FormatDecimal(bs.NetIncome)); err != nil {
		return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
	}
	n++
	if err = emit("total_equity", "", "", FormatDecimal(bs.TotalEquity)); err != nil {
		return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
	}
	n++
	if err = emit("total_liabilities_and_equity", "", "", FormatDecimal(bs.TotalLiabilitiesAndEquity)); err != nil {
		return n, fmt.Errorf("WriteBalanceSheetJSONL: %w", err)
	}
	n++
	return n, nil
}

// WriteCashFlow writes a cash flow statement to w as CSV.
// Columns: section, account_code, account_name, amount.
// Sections in order: opening_cash, inflow (per cash account), total_inflows,
// outflow (per cash account), total_outflows, net_change, ending_cash.
func WriteCashFlow(w io.Writer, cf report.CashFlow) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"section", "account_code", "account_name", "amount"}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv header: %w", err)
	}

	if err = cw.Write([]string{"opening_cash", "", "", FormatDecimal(cf.OpeningCash)}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
	}
	n++
	for _, r := range cf.Inflows {
		if err = cw.Write([]string{"inflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_inflows", "", "", FormatDecimal(cf.TotalIn)}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
	}
	n++
	for _, r := range cf.Outflows {
		if err = cw.Write([]string{"outflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)}); err != nil {
			return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
		}
		n++
	}
	if err = cw.Write([]string{"total_outflows", "", "", FormatDecimal(cf.TotalOut)}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
	}
	n++
	if err = cw.Write([]string{"net_change", "", "", FormatDecimal(cf.NetChange)}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
	}
	n++
	if err = cw.Write([]string{"ending_cash", "", "", FormatDecimal(cf.EndingCash)}); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: csv row: %w", err)
	}
	n++

	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteCashFlow: %w", err)
	}
	return n, nil
}

// WriteCashFlowJSONL writes a cash flow statement to w as JSONL, one object per row.
func WriteCashFlowJSONL(w io.Writer, cf report.CashFlow) (n int, err error) {
	enc := newJSONLEncoder(w)

	emit := func(section, code, name, amount string) error {
		return enc.Encode(reportRowJSONL{Section: section, AccountCode: code, AccountName: name, Amount: amount})
	}

	if err = emit("opening_cash", "", "", FormatDecimal(cf.OpeningCash)); err != nil {
		return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
	}
	n++
	for _, r := range cf.Inflows {
		if err = emit("inflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)); err != nil {
			return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
		}
		n++
	}
	if err = emit("total_inflows", "", "", FormatDecimal(cf.TotalIn)); err != nil {
		return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
	}
	n++
	for _, r := range cf.Outflows {
		if err = emit("outflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)); err != nil {
			return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
		}
		n++
	}
	if err = emit("total_outflows", "", "", FormatDecimal(cf.TotalOut)); err != nil {
		return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
	}
	n++
	if err = emit("net_change", "", "", FormatDecimal(cf.NetChange)); err != nil {
		return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
	}
	n++
	if err = emit("ending_cash", "", "", FormatDecimal(cf.EndingCash)); err != nil {
		return n, fmt.Errorf("WriteCashFlowJSONL: %w", err)
	}
	n++
	return n, nil
}

// WriteTrialBalance writes a trial balance to w as CSV.
// Columns: account_code, account_name, account_type, debit, credit.
// One row per account with non-zero period activity, in account-code order.
// By invariant 1, the sum of the debit column equals the sum of the credit column.
func WriteTrialBalance(w io.Writer, tb report.TrialBalance) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"account_code", "account_name", "account_type", "debit", "credit"}); err != nil {
		return 0, fmt.Errorf("WriteTrialBalance: csv header: %w", err)
	}
	for _, r := range tb.Rows {
		if err = cw.Write([]string{r.AccountCode, r.AccountName, r.AccountType, FormatDecimal(r.Debit), FormatDecimal(r.Credit)}); err != nil {
			return 0, fmt.Errorf("WriteTrialBalance: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteTrialBalance: %w", err)
	}
	return n, nil
}

// WriteTrialBalanceJSONL writes a trial balance to w as JSONL, one object per row.
func WriteTrialBalanceJSONL(w io.Writer, tb report.TrialBalance) (n int, err error) {
	enc := newJSONLEncoder(w)
	for _, r := range tb.Rows {
		row := trialBalanceRowJSONL{
			AccountCode: r.AccountCode,
			AccountName: r.AccountName,
			AccountType: r.AccountType,
			Debit:       FormatDecimal(r.Debit),
			Credit:      FormatDecimal(r.Credit),
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteTrialBalanceJSONL: %w", err)
		}
		n++
	}
	return n, nil
}
