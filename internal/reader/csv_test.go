package reader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// writeFile is a test helper that writes content to name inside dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

// minimalCSVSet writes a minimal but valid set of CSV files to dir:
// 3 journal entries, 6 lines (2 per entry), balanced debit/credit pairs.
func minimalCSVSet(t *testing.T, dir string) {
	t.Helper()

	// coa.csv — use 1010 (Checking Account) which is a recognised cash account
	// per gen.IsCashAccount; invariant 3 checks GL cash-account codes against
	// bank statement totals.
	writeFile(t, dir, "coa.csv", strings.Join([]string{
		"code,name,type,normal_balance",
		"1010,Checking Account,asset,debit",
		"4000,Sales Revenue,revenue,credit",
	}, "\n")+"\n")

	ts1 := "2026-01-02T10:00:00Z"
	ts2 := "2026-01-05T11:00:00Z"
	ts3 := "2026-01-10T12:00:00Z"

	// journal_entries.csv
	writeFile(t, dir, "journal_entries.csv", strings.Join([]string{
		"row_num,entry_id,occurred_at,description,source_event_id",
		"1,E-0001," + ts1 + ",Sale 1,EV-001",
		"2,E-0002," + ts2 + ",Sale 2,EV-002",
		"3,E-0003," + ts3 + ",Sale 3,EV-003",
	}, "\n")+"\n")

	// gl.csv — 2 lines per entry, debit 1010 (cash) / credit 4000
	writeFile(t, dir, "gl.csv", strings.Join([]string{
		"row_num,entry_id,line_id,occurred_at,account_code,debit,credit",
		"1,E-0001,L-0001," + ts1 + ",1010,100.000000000,0.000000000",
		"2,E-0001,L-0002," + ts1 + ",4000,0.000000000,100.000000000",
		"3,E-0002,L-0003," + ts2 + ",1010,200.000000000,0.000000000",
		"4,E-0002,L-0004," + ts2 + ",4000,0.000000000,200.000000000",
		"5,E-0003,L-0005," + ts3 + ",1010,150.000000000,0.000000000",
		"6,E-0003,L-0006," + ts3 + ",4000,0.000000000,150.000000000",
	}, "\n")+"\n")

	// bank_statement.csv — cash account lines only (must mirror GL cash lines)
	writeFile(t, dir, "bank_statement.csv", strings.Join([]string{
		"row_num,entry_id,line_id,occurred_at,account_code,debit,credit",
		"1,E-0001,L-0001," + ts1 + ",1010,100.000000000,0.000000000",
		"2,E-0002,L-0003," + ts2 + ",1010,200.000000000,0.000000000",
		"3,E-0003,L-0005," + ts3 + ",1010,150.000000000,0.000000000",
	}, "\n")+"\n")

	// customers.csv — empty (no customers in this minimal set)
	writeFile(t, dir, "customers.csv", "id,name,email,phone,address,city,state,zip\n")

	// vendors.csv — empty
	writeFile(t, dir, "vendors.csv", "id,name,email,phone,address,city,state,zip\n")

	// sub-ledger files — all empty
	for _, name := range []string{
		"invoices.csv",
		"customer_payments.csv",
		"bills.csv",
		"bill_payments.csv",
		"expense_reports.csv",
	} {
		writeFile(t, dir, name, "row_num,entry_id,ref_id,occurred_at,amount,sub_ledger_type\n")
	}
}

// TestLoadMinimal verifies that Load on a minimal 3-entry/6-line CSV set
// returns a Dataset with 6 Lines, balanced Σdebit == Σcredit, and passes
// invariant.Check.
func TestLoadMinimal(t *testing.T) {
	dir := t.TempDir()
	minimalCSVSet(t, dir)

	ds, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := len(ds.Lines); got != 6 {
		t.Errorf("Lines: got %d, want 6", got)
	}

	var totalDebit, totalCredit decimal.Decimal
	for _, ln := range ds.Lines {
		totalDebit = totalDebit.Add(ln.Debit)
		totalCredit = totalCredit.Add(ln.Credit)
	}
	if !totalDebit.Equal(totalCredit) {
		t.Errorf("Lines not balanced: debit %s != credit %s", totalDebit, totalCredit)
	}

	if err := invariant.Check(ds); err != nil {
		t.Errorf("invariant.Check: %v", err)
	}
}

// TestLoadBadDecimal verifies that Load returns an error (not a panic) when
// a GL CSV row contains a non-numeric debit value.
func TestLoadBadDecimal(t *testing.T) {
	dir := t.TempDir()
	minimalCSVSet(t, dir)

	// Overwrite gl.csv with one row containing a non-numeric debit.
	writeFile(t, dir, "gl.csv", strings.Join([]string{
		"row_num,entry_id,line_id,occurred_at,account_code,debit,credit",
		"1,E-0001,L-0001,2026-01-02T10:00:00Z,1010,NOT_A_NUMBER,0.000000000",
		"2,E-0001,L-0002,2026-01-02T10:00:00Z,4000,0.000000000,100.000000000",
	}, "\n")+"\n")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load: expected error for non-numeric debit, got nil")
	}
}

// TestLoadNonexistentDir verifies that Load returns an error when the directory
// does not exist.
func TestLoadNonexistentDir(t *testing.T) {
	_, err := Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("Load: expected error for nonexistent dir, got nil")
	}
}

// TestLoadCoAEquivalence generates a real dataset via gen.Run, writes it via
// writer.Write, then loads it back with Load and asserts that the reconstructed
// CoA equals gen.IndexCoA(gen.DefaultCoA()).
func TestLoadCoAEquivalence(t *testing.T) {
	cfg, err := gen.ParseConfig(1, "2026-01", "small", "monthly-close", t.TempDir(), "csv")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	cfg.Version = "test"

	ds, err := gen.Run(cfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}

	outDir := t.TempDir()
	rpts := struct { /* empty reports */
	}{}
	_ = rpts

	// We need the full report.Reports struct. Use writer.Write signature.
	// Since we don't import report here, we use a zero-value approach via
	// the writer package's exported Write function which requires report.Reports.
	// Instead, write just the CSV files we need manually to avoid the import cycle.
	// Write coa.csv, journal_entries.csv, gl.csv, bank_statement.csv, etc. directly.
	if err := writeDatasetCSVs(t, ds, outDir); err != nil {
		t.Fatalf("writeDatasetCSVs: %v", err)
	}

	loaded, err := Load(outDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := gen.IndexCoA(gen.DefaultCoA())
	if !reflect.DeepEqual(loaded.CoA, want) {
		t.Errorf("loaded CoA does not equal gen.IndexCoA(gen.DefaultCoA())")
		for code, wantAcct := range want {
			if gotAcct, ok := loaded.CoA[code]; !ok {
				t.Errorf("  code %s missing from loaded CoA", code)
			} else if gotAcct != wantAcct {
				t.Errorf("  code %s: got %+v, want %+v", code, gotAcct, wantAcct)
			}
		}
		for code := range loaded.CoA {
			if _, ok := want[code]; !ok {
				t.Errorf("  code %s present in loaded CoA but not in want", code)
			}
		}
	}
}

// writeDatasetCSVs writes the CSV files needed for Load from ds, without
// writing report files or manifest. Used in tests to avoid importing report.
func writeDatasetCSVs(t *testing.T, ds gen.Dataset, dir string) error {
	t.Helper()

	// coa.csv
	{
		f, err := os.Create(filepath.Join(dir, "coa.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteCoA(f, ds.CoA)
		f.Close()
		if err != nil {
			return err
		}
	}

	// journal_entries.csv
	{
		f, err := os.Create(filepath.Join(dir, "journal_entries.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteJournalEntries(f, ds.Entries)
		f.Close()
		if err != nil {
			return err
		}
	}

	// gl.csv
	{
		f, err := os.Create(filepath.Join(dir, "gl.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteGL(f, ds.GL)
		f.Close()
		if err != nil {
			return err
		}
	}

	// bank_statement.csv
	{
		f, err := os.Create(filepath.Join(dir, "bank_statement.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteBankStatement(f, ds.Bank)
		f.Close()
		if err != nil {
			return err
		}
	}

	// customers.csv
	{
		f, err := os.Create(filepath.Join(dir, "customers.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteCustomers(f, ds.Customers)
		f.Close()
		if err != nil {
			return err
		}
	}

	// vendors.csv
	{
		f, err := os.Create(filepath.Join(dir, "vendors.csv"))
		if err != nil {
			return err
		}
		_, err = writer.WriteVendors(f, ds.Vendors)
		f.Close()
		if err != nil {
			return err
		}
	}

	// sub-ledger files
	type slFile struct {
		name string
		rows []gen.SubLedgerRow
		fn   func(f *os.File, rows []gen.SubLedgerRow) error
	}
	slFiles := []slFile{
		{"invoices.csv", ds.SubLedger.Invoices, func(f *os.File, rows []gen.SubLedgerRow) error {
			_, err := writer.WriteInvoices(f, rows)
			return err
		}},
		{"customer_payments.csv", ds.SubLedger.CustomerPayments, func(f *os.File, rows []gen.SubLedgerRow) error {
			_, err := writer.WriteCustomerPayments(f, rows)
			return err
		}},
		{"bills.csv", ds.SubLedger.Bills, func(f *os.File, rows []gen.SubLedgerRow) error {
			_, err := writer.WriteBills(f, rows)
			return err
		}},
		{"bill_payments.csv", ds.SubLedger.BillPayments, func(f *os.File, rows []gen.SubLedgerRow) error {
			_, err := writer.WriteBillPayments(f, rows)
			return err
		}},
		{"expense_reports.csv", ds.SubLedger.ExpenseReports, func(f *os.File, rows []gen.SubLedgerRow) error {
			_, err := writer.WriteExpenseReports(f, rows)
			return err
		}},
	}
	for _, sl := range slFiles {
		f, err := os.Create(filepath.Join(dir, sl.name))
		if err != nil {
			return err
		}
		err = sl.fn(f, sl.rows)
		f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Ensure time package is used (for timestamp parsing validation).
var _ = time.RFC3339Nano
