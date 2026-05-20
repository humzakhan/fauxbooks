// Package reader implements the CSV reader for fauxbooks output directories.
// It reconstructs a gen.Dataset from the CSV files produced by a gen run,
// suitable for passing to invariant.Check.
//
// Pure I/O: no RNG, no time.Now(), no file writes.
//
// Note on truth.json: recon-stress-test bundles write a truth.json file that
// is NOT a CSV and is intentionally ignored by Load. ds.Truth is left nil.
// Verifying recon-stress-test output will cause invariant.Check to fire on the
// deliberately injected mismatches, producing exit 2. This is expected behavior;
// do not special-case it.
package reader

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// Load reads the standard fauxbooks CSV output files from dir and reconstructs
// a gen.Dataset suitable for invariant.Check. Pure I/O — no RNG, no time.Now,
// no file writes. Returns wrapped errors with "reader:" prefix at I/O boundaries.
func Load(dir string) (gen.Dataset, error) {
	var ds gen.Dataset

	// coa.csv → ds.CoA
	coa, err := loadCoA(dir)
	if err != nil {
		return ds, err
	}
	ds.CoA = coa

	// journal_entries.csv → ds.Entries
	entries, err := loadJournalEntries(dir)
	if err != nil {
		return ds, err
	}
	ds.Entries = entries

	// gl.csv → ds.GL and ds.Lines (Lines reconstructed from GL rows)
	gl, lines, err := loadGL(dir)
	if err != nil {
		return ds, err
	}
	ds.GL = gl
	ds.Lines = lines

	// bank_statement.csv → ds.Bank
	bank, err := loadBankStatement(dir)
	if err != nil {
		return ds, err
	}
	ds.Bank = bank

	// customers.csv → ds.Customers
	customers, err := loadCustomers(dir)
	if err != nil {
		return ds, err
	}
	ds.Customers = customers

	// vendors.csv → ds.Vendors
	vendors, err := loadVendors(dir)
	if err != nil {
		return ds, err
	}
	ds.Vendors = vendors

	// sub-ledger files
	invoices, err := loadSubLedger(dir, "invoices.csv")
	if err != nil {
		return ds, err
	}
	ds.SubLedger.Invoices = invoices

	customerPayments, err := loadSubLedger(dir, "customer_payments.csv")
	if err != nil {
		return ds, err
	}
	ds.SubLedger.CustomerPayments = customerPayments

	bills, err := loadSubLedger(dir, "bills.csv")
	if err != nil {
		return ds, err
	}
	ds.SubLedger.Bills = bills

	billPayments, err := loadSubLedger(dir, "bill_payments.csv")
	if err != nil {
		return ds, err
	}
	ds.SubLedger.BillPayments = billPayments

	expenseReports, err := loadSubLedger(dir, "expense_reports.csv")
	if err != nil {
		return ds, err
	}
	ds.SubLedger.ExpenseReports = expenseReports

	return ds, nil
}

// openCSV opens the named file in dir and returns a csv.Reader configured for
// fauxbooks format (UTF-8, LF line endings). The caller is responsible for
// closing the returned *os.File. If the file cannot be opened, returns a
// wrapped error with the "reader:" prefix.
func openCSV(dir, name string) (*os.File, *csv.Reader, error) {
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reader: open %s: %w", name, err)
	}
	br := bufio.NewReaderSize(f, 64*1024)
	cr := csv.NewReader(br)
	cr.ReuseRecord = true
	return f, cr, nil
}

// readHeader reads the header row from cr and returns it, or an error if none
// can be read. The name parameter is used for error context.
func readHeader(cr *csv.Reader, name string) ([]string, error) {
	hdr, err := cr.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("reader: %s: empty file", name)
	}
	if err != nil {
		return nil, fmt.Errorf("reader: read %s header: %w", name, err)
	}
	// ReuseRecord: make a copy of the header before reading more rows.
	out := make([]string, len(hdr))
	copy(out, hdr)
	return out, nil
}

// loadCoA reads coa.csv and returns a map[string]gen.Account.
// Columns: code, name, type, normal_balance.
func loadCoA(dir string) (map[string]gen.Account, error) {
	const name = "coa.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	coa := make(map[string]gen.Account)
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 4 {
			return nil, fmt.Errorf("reader: %s row %d: expected 4 columns, got %d", name, i, len(rec))
		}
		code := rec[0]
		acct := gen.Account{
			Code:          code,
			Name:          rec[1],
			Type:          gen.AccountType(rec[2]),
			NormalBalance: rec[3],
		}
		coa[code] = acct
	}
	return coa, nil
}

// loadJournalEntries reads journal_entries.csv and returns []gen.JournalEntry.
// Columns: row_num, entry_id, occurred_at, description, source_event_id.
func loadJournalEntries(dir string) ([]gen.JournalEntry, error) {
	const name = "journal_entries.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	var entries []gen.JournalEntry
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 5 {
			return nil, fmt.Errorf("reader: %s row %d: expected 5 columns, got %d", name, i, len(rec))
		}
		// rec[0] is row_num; skip it.
		occurredAt, err := time.Parse("2006-01-02T15:04:05.999999999Z", rec[2])
		if err != nil {
			return nil, fmt.Errorf("reader: parse %s row %d column occurred_at: %w", name, i, err)
		}
		entries = append(entries, gen.JournalEntry{
			EntryID:       rec[1],
			OccurredAt:    occurredAt,
			Description:   rec[3],
			SourceEventID: rec[4],
		})
	}
	return entries, nil
}

// loadGL reads gl.csv and returns ([]gen.GLRow, []gen.JournalLine, error).
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
// ds.Lines is reconstructed here because GL rows contain all the necessary data.
func loadGL(dir string) ([]gen.GLRow, []gen.JournalLine, error) {
	const name = "gl.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, nil, err
	}

	var gl []gen.GLRow
	var lines []gen.JournalLine
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 7 {
			return nil, nil, fmt.Errorf("reader: %s row %d: expected 7 columns, got %d", name, i, len(rec))
		}

		rowNum, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, nil, fmt.Errorf("reader: parse %s row %d column row_num: %w", name, i, err)
		}

		debitStr := rec[5]
		creditStr := rec[6]

		debit, err := decimal.NewFromString(debitStr)
		if err != nil {
			return nil, nil, fmt.Errorf("reader: parse %s row %d column debit: %w", name, i, err)
		}
		credit, err := decimal.NewFromString(creditStr)
		if err != nil {
			return nil, nil, fmt.Errorf("reader: parse %s row %d column credit: %w", name, i, err)
		}

		entryID := rec[1]
		lineID := rec[2]
		occurredAt := rec[3]
		accountCode := rec[4]

		gl = append(gl, gen.GLRow{
			RowNum:      rowNum,
			EntryID:     entryID,
			LineID:      lineID,
			OccurredAt:  occurredAt,
			AccountCode: accountCode,
			Debit:       debitStr,
			Credit:      creditStr,
		})

		lines = append(lines, gen.JournalLine{
			LineID:      lineID,
			EntryID:     entryID,
			AccountCode: accountCode,
			Debit:       debit,
			Credit:      credit,
		})
	}
	return gl, lines, nil
}

// loadBankStatement reads bank_statement.csv and returns []gen.BankRow.
// Columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit.
func loadBankStatement(dir string) ([]gen.BankRow, error) {
	const name = "bank_statement.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	var rows []gen.BankRow
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 7 {
			return nil, fmt.Errorf("reader: %s row %d: expected 7 columns, got %d", name, i, len(rec))
		}

		rowNum, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, fmt.Errorf("reader: parse %s row %d column row_num: %w", name, i, err)
		}

		rows = append(rows, gen.BankRow{
			RowNum:      rowNum,
			EntryID:     rec[1],
			LineID:      rec[2],
			OccurredAt:  rec[3],
			AccountCode: rec[4],
			Debit:       rec[5],
			Credit:      rec[6],
		})
	}
	return rows, nil
}

// loadCustomers reads customers.csv and returns []gen.Customer.
// Columns: id, name, email, phone, address, city, state, zip.
func loadCustomers(dir string) ([]gen.Customer, error) {
	const name = "customers.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	var customers []gen.Customer
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 8 {
			return nil, fmt.Errorf("reader: %s row %d: expected 8 columns, got %d", name, i, len(rec))
		}
		customers = append(customers, gen.Customer{
			ID:      rec[0],
			Name:    rec[1],
			Email:   rec[2],
			Phone:   rec[3],
			Address: rec[4],
			City:    rec[5],
			State:   rec[6],
			Zip:     rec[7],
		})
	}
	return customers, nil
}

// loadVendors reads vendors.csv and returns []gen.Vendor.
// Columns: id, name, email, phone, address, city, state, zip.
func loadVendors(dir string) ([]gen.Vendor, error) {
	const name = "vendors.csv"
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	var vendors []gen.Vendor
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 8 {
			return nil, fmt.Errorf("reader: %s row %d: expected 8 columns, got %d", name, i, len(rec))
		}
		vendors = append(vendors, gen.Vendor{
			ID:      rec[0],
			Name:    rec[1],
			Email:   rec[2],
			Phone:   rec[3],
			Address: rec[4],
			City:    rec[5],
			State:   rec[6],
			Zip:     rec[7],
		})
	}
	return vendors, nil
}

// loadSubLedger reads a sub-ledger CSV file and returns []gen.SubLedgerRow.
// Columns: row_num, entry_id, ref_id, occurred_at, amount, sub_ledger_type.
func loadSubLedger(dir, name string) ([]gen.SubLedgerRow, error) {
	f, cr, err := openCSV(dir, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := readHeader(cr, name); err != nil {
		return nil, err
	}

	var rows []gen.SubLedgerRow
	for i := 1; ; i++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reader: read %s row %d: %w", name, i, err)
		}
		if len(rec) != 6 {
			return nil, fmt.Errorf("reader: %s row %d: expected 6 columns, got %d", name, i, len(rec))
		}

		rowNum, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, fmt.Errorf("reader: parse %s row %d column row_num: %w", name, i, err)
		}

		rows = append(rows, gen.SubLedgerRow{
			RowNum:        rowNum,
			EntryID:       rec[1],
			RefID:         rec[2],
			OccurredAt:    rec[3],
			Amount:        rec[4],
			SubLedgerType: rec[5],
		})
	}
	return rows, nil
}
