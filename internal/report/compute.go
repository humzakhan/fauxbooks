package report

import (
	"sort"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// Compute derives the four financial reports from ds. It is a pure function:
// it does not read the wall clock, call the RNG, or touch the filesystem.
// Two calls with structurally equal Datasets return structurally equal Reports.
//
// Account amounts are signed in each account's natural direction (assets and
// expenses debit-positive; liabilities, equity, and revenue credit-positive).
// Contra-accounts therefore appear with a negative Amount on the section they
// belong to. Zero-balance accounts are excluded from every section.
func Compute(ds gen.Dataset) Reports {
	totals := perAccountTotals(ds.Lines)
	codes := sortedAccountCodes(ds.CoA)

	pl := computeProfitLoss(codes, ds.CoA, totals)
	cf := computeCashFlow(codes, ds.CoA, totals)
	bs := computeBalanceSheet(codes, ds.CoA, totals, pl.NetIncome)
	tb := computeTrialBalance(codes, ds.CoA, totals)

	return Reports{
		ProfitLoss:   pl,
		BalanceSheet: bs,
		CashFlow:     cf,
		TrialBalance: tb,
	}
}

// netSide records the per-account debit and credit accumulations.
type netSide struct {
	debit  decimal.Decimal
	credit decimal.Decimal
}

// perAccountTotals sums debits and credits per account code over all lines.
// The returned map is indexed by account code; missing keys denote no activity.
func perAccountTotals(lines []gen.JournalLine) map[string]netSide {
	out := make(map[string]netSide, len(lines))
	for _, l := range lines {
		s := out[l.AccountCode]
		s.debit = s.debit.Add(l.Debit)
		s.credit = s.credit.Add(l.Credit)
		out[l.AccountCode] = s
	}
	return out
}

// sortedAccountCodes returns CoA keys in lexicographic order for deterministic
// iteration. Iterating ds.CoA directly is forbidden by the determinism rule.
func sortedAccountCodes(coa map[string]gen.Account) []string {
	codes := make([]string, 0, len(coa))
	for code := range coa {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func computeProfitLoss(codes []string, coa map[string]gen.Account, totals map[string]netSide) ProfitLoss {
	var pl ProfitLoss
	for _, code := range codes {
		acct := coa[code]
		s, ok := totals[code]
		if !ok {
			continue
		}
		switch acct.Type {
		case gen.AccountTypeRevenue:
			amount := s.credit.Sub(s.debit)
			if amount.IsZero() {
				continue
			}
			pl.Revenue = append(pl.Revenue, ProfitLossRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      amount,
			})
			pl.TotalRevenue = pl.TotalRevenue.Add(amount)
		case gen.AccountTypeExpense:
			amount := s.debit.Sub(s.credit)
			if amount.IsZero() {
				continue
			}
			pl.Expense = append(pl.Expense, ProfitLossRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      amount,
			})
			pl.TotalExpense = pl.TotalExpense.Add(amount)
		}
	}
	pl.NetIncome = pl.TotalRevenue.Sub(pl.TotalExpense)
	return pl
}

func computeBalanceSheet(codes []string, coa map[string]gen.Account, totals map[string]netSide, netIncome decimal.Decimal) BalanceSheet {
	var bs BalanceSheet
	for _, code := range codes {
		acct := coa[code]
		s, ok := totals[code]
		if !ok {
			continue
		}
		switch acct.Type {
		case gen.AccountTypeAsset:
			amount := s.debit.Sub(s.credit)
			if amount.IsZero() {
				continue
			}
			bs.Assets = append(bs.Assets, BalanceSheetRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      amount,
			})
			bs.TotalAssets = bs.TotalAssets.Add(amount)
		case gen.AccountTypeLiability:
			amount := s.credit.Sub(s.debit)
			if amount.IsZero() {
				continue
			}
			bs.Liabilities = append(bs.Liabilities, BalanceSheetRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      amount,
			})
			bs.TotalLiabilities = bs.TotalLiabilities.Add(amount)
		case gen.AccountTypeEquity:
			amount := s.credit.Sub(s.debit)
			if amount.IsZero() {
				continue
			}
			bs.Equity = append(bs.Equity, BalanceSheetRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      amount,
			})
			bs.TotalEquity = bs.TotalEquity.Add(amount)
		}
	}
	bs.NetIncome = netIncome
	bs.TotalEquity = bs.TotalEquity.Add(netIncome)
	bs.TotalLiabilitiesAndEquity = bs.TotalLiabilities.Add(bs.TotalEquity)
	return bs
}

func computeCashFlow(codes []string, coa map[string]gen.Account, totals map[string]netSide) CashFlow {
	var cf CashFlow
	for _, code := range codes {
		if !gen.IsCashAccount(code) {
			continue
		}
		acct := coa[code]
		s, ok := totals[code]
		if !ok {
			continue
		}
		if !s.debit.IsZero() {
			cf.Inflows = append(cf.Inflows, CashFlowRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      s.debit,
			})
			cf.TotalIn = cf.TotalIn.Add(s.debit)
		}
		if !s.credit.IsZero() {
			cf.Outflows = append(cf.Outflows, CashFlowRow{
				AccountCode: acct.Code,
				AccountName: acct.Name,
				Amount:      s.credit,
			})
			cf.TotalOut = cf.TotalOut.Add(s.credit)
		}
	}
	cf.NetChange = cf.TotalIn.Sub(cf.TotalOut)
	cf.EndingCash = cf.OpeningCash.Add(cf.NetChange)
	return cf
}

func computeTrialBalance(codes []string, coa map[string]gen.Account, totals map[string]netSide) TrialBalance {
	var tb TrialBalance
	for _, code := range codes {
		acct := coa[code]
		s, ok := totals[code]
		if !ok {
			continue
		}
		net := s.debit.Sub(s.credit)
		if net.IsZero() {
			continue
		}
		row := TrialBalanceRow{
			AccountCode: acct.Code,
			AccountName: acct.Name,
			AccountType: string(acct.Type),
		}
		if net.IsPositive() {
			row.Debit = net
		} else {
			row.Credit = net.Neg()
		}
		tb.Rows = append(tb.Rows, row)
	}
	return tb
}
