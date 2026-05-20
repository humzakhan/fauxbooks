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

	if err = cw.Write([]string{"code", "name", "type", "normal_balance"}); err != nil {
		return 0, fmt.Errorf("WriteCoA: csv header: %w", err)
	}
	for _, code := range codes {
		a := coa[code]
		if err = cw.Write([]string{a.Code, a.Name, string(a.Type), a.NormalBalance}); err != nil {
			return 0, fmt.Errorf("WriteCoA: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteCoA: %w", err)
	}
	return n, nil
}

// WriteJournalEntries writes journal entry headers to w as CSV.
// Columns: row_num, entry_id, occurred_at, description, source_event_id.
func WriteJournalEntries(w io.Writer, entries []gen.JournalEntry) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"row_num", "entry_id", "occurred_at", "description", "source_event_id"}); err != nil {
		return 0, fmt.Errorf("WriteJournalEntries: csv header: %w", err)
	}
	for i, e := range entries {
		row := []string{
			strconv.Itoa(i + 1),
			e.EntryID,
			e.OccurredAt.Format("2006-01-02T15:04:05.999999999Z"),
			e.Description,
			e.SourceEventID,
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("WriteJournalEntries: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteJournalEntries: %w", err)
	}
	return n, nil
}

// WriteGL writes general ledger rows to w as CSV.
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
func WriteGL(w io.Writer, gl []gen.GLRow) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"}); err != nil {
		return 0, fmt.Errorf("WriteGL: csv header: %w", err)
	}
	for _, r := range gl {
		row := []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("WriteGL: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteGL: %w", err)
	}
	return n, nil
}

// WriteBankStatement writes bank statement rows to w as CSV.
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
func WriteBankStatement(w io.Writer, bank []gen.BankRow) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"}); err != nil {
		return 0, fmt.Errorf("WriteBankStatement: csv header: %w", err)
	}
	for _, r := range bank {
		row := []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("WriteBankStatement: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteBankStatement: %w", err)
	}
	return n, nil
}

// writeSubLedger is the common implementation for all sub-ledger CSV writers.
// Columns: row_num, entry_id, ref_id, occurred_at, amount, sub_ledger_type.
func writeSubLedger(w io.Writer, rows []gen.SubLedgerRow, label string) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"row_num", "entry_id", "ref_id", "occurred_at", "amount", "sub_ledger_type"}); err != nil {
		return 0, fmt.Errorf("%s: csv header: %w", label, err)
	}
	for _, r := range rows {
		row := []string{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.RefID,
			r.OccurredAt,
			r.Amount,
			r.SubLedgerType,
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("%s: csv row: %w", label, err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return n, nil
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

// writePartyMaster is the common implementation for customer and vendor CSV writers.
// Columns: id, name, email, phone, address, city, state, zip.
func writePartyMaster(w io.Writer, id, name, email, phone, address, city, state, zip []string, label string) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"id", "name", "email", "phone", "address", "city", "state", "zip"}); err != nil {
		return 0, fmt.Errorf("%s: csv header: %w", label, err)
	}
	for i := range id {
		row := []string{id[i], name[i], email[i], phone[i], address[i], city[i], state[i], zip[i]}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("%s: csv row: %w", label, err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return n, nil
}

// WriteCustomers writes the customer master to w as CSV.
func WriteCustomers(w io.Writer, customers []gen.Customer) (n int, err error) {
	id := make([]string, len(customers))
	name := make([]string, len(customers))
	email := make([]string, len(customers))
	phone := make([]string, len(customers))
	address := make([]string, len(customers))
	city := make([]string, len(customers))
	state := make([]string, len(customers))
	zip := make([]string, len(customers))
	for i, c := range customers {
		id[i] = c.ID
		name[i] = c.Name
		email[i] = c.Email
		phone[i] = c.Phone
		address[i] = c.Address
		city[i] = c.City
		state[i] = c.State
		zip[i] = c.Zip
	}
	return writePartyMaster(w, id, name, email, phone, address, city, state, zip, "WriteCustomers")
}

// WriteVendors writes the vendor master to w as CSV.
func WriteVendors(w io.Writer, vendors []gen.Vendor) (n int, err error) {
	id := make([]string, len(vendors))
	name := make([]string, len(vendors))
	email := make([]string, len(vendors))
	phone := make([]string, len(vendors))
	address := make([]string, len(vendors))
	city := make([]string, len(vendors))
	state := make([]string, len(vendors))
	zip := make([]string, len(vendors))
	for i, v := range vendors {
		id[i] = v.ID
		name[i] = v.Name
		email[i] = v.Email
		phone[i] = v.Phone
		address[i] = v.Address
		city[i] = v.City
		state[i] = v.State
		zip[i] = v.Zip
	}
	return writePartyMaster(w, id, name, email, phone, address, city, state, zip, "WriteVendors")
}
