package writer

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

const csvBufSize = 64 * 1024 // 64 KiB; adequate for small/medium volume

// newCSVWriter creates a csv.Writer configured for fauxbooks output:
// UTF-8, \n line endings (no CRLF), writing to w.
func newCSVWriter(w io.Writer) *csv.Writer {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	return cw
}

// writeCSV writes header then rows via cw, flushes, and checks for errors.
func writeCSV(cw *csv.Writer, header []string, rows [][]string) error {
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header: %w", err)
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csv row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteCoA writes the chart of accounts to w as CSV.
// Columns: code, name, type, normal_balance.
// Rows are sorted by account code for deterministic output.
func WriteCoA(w io.Writer, coa map[string]gen.Account) (n int, err error) {
	cw := newCSVWriter(w)

	codes := make([]string, 0, len(coa))
	for code := range coa {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	rows := make([][]string, len(codes))
	for i, code := range codes {
		a := coa[code]
		rows[i] = []string{a.Code, a.Name, string(a.Type), a.NormalBalance}
	}

	if err = writeCSV(cw, []string{"code", "name", "type", "normal_balance"}, rows); err != nil {
		return 0, fmt.Errorf("WriteCoA: %w", err)
	}
	return len(rows), nil
}

// WriteJournalEntries writes journal entry headers to w as CSV.
// Columns: row_num, entry_id, occurred_at, description, source_event_id.
func WriteJournalEntries(w io.Writer, entries []gen.JournalEntry) (n int, err error) {
	cw := newCSVWriter(w)

	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			strconv.Itoa(i + 1),
			e.EntryID,
			e.OccurredAt.Format("2006-01-02T15:04:05.999999999Z"),
			e.Description,
			e.SourceEventID,
		}
	}

	if err = writeCSV(cw, []string{"row_num", "entry_id", "occurred_at", "description", "source_event_id"}, rows); err != nil {
		return 0, fmt.Errorf("WriteJournalEntries: %w", err)
	}
	return len(rows), nil
}

// WriteGL writes general ledger rows to w as CSV.
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
func WriteGL(w io.Writer, gl []gen.GLRow) (n int, err error) {
	cw := newCSVWriter(w)

	rows := make([][]string, len(gl))
	for i, r := range gl {
		rows[i] = []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		}
	}

	if err = writeCSV(cw, []string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"}, rows); err != nil {
		return 0, fmt.Errorf("WriteGL: %w", err)
	}
	return len(rows), nil
}

// WriteBankStatement writes bank statement rows to w as CSV.
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
func WriteBankStatement(w io.Writer, bank []gen.BankRow) (n int, err error) {
	cw := newCSVWriter(w)

	rows := make([][]string, len(bank))
	for i, r := range bank {
		rows[i] = []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		}
	}

	if err = writeCSV(cw, []string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"}, rows); err != nil {
		return 0, fmt.Errorf("WriteBankStatement: %w", err)
	}
	return len(rows), nil
}

// writeSubLedger is the common implementation for all sub-ledger CSV writers.
// Columns: row_num, entry_id, ref_id, occurred_at, amount, sub_ledger_type.
func writeSubLedger(w io.Writer, rows []gen.SubLedgerRow, label string) (n int, err error) {
	cw := newCSVWriter(w)

	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.RefID,
			r.OccurredAt,
			r.Amount,
			r.SubLedgerType,
		}
	}

	if err = writeCSV(cw, []string{"row_num", "entry_id", "ref_id", "occurred_at", "amount", "sub_ledger_type"}, out); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return len(out), nil
}

// WriteInvoices writes sale invoice sub-ledger rows to w as CSV.
func WriteInvoices(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedger(w, rows, "WriteInvoices")
}

// WriteCustomerPayments writes customer payment sub-ledger rows to w as CSV.
func WriteCustomerPayments(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedger(w, rows, "WriteCustomerPayments")
}

// WriteBills writes vendor bill sub-ledger rows to w as CSV.
func WriteBills(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedger(w, rows, "WriteBills")
}

// WriteBillPayments writes bill payment sub-ledger rows to w as CSV.
func WriteBillPayments(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedger(w, rows, "WriteBillPayments")
}

// WriteExpenseReports writes expense report sub-ledger rows to w as CSV.
func WriteExpenseReports(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedger(w, rows, "WriteExpenseReports")
}
