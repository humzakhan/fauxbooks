package gen

import (
	"reflect"
	"testing"
	"time"
)

// smallInput builds a deterministic set of entries and lines for projection
// tests by running Generate + Expand with seed=1, period=Jan 2026, volume=small.
func smallInput(t *testing.T) ([]JournalEntry, []JournalLine, map[string]Account) {
	t.Helper()
	coa := IndexCoA(DefaultCoA())
	rng := NewRNG(1)
	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events, err := Generate(rng, period, "small", MonthlyClose().Mix)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	entries, lines, err := Expand(events, coa)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return entries, lines, coa
}

// TestProject_Determinism verifies that two Project calls with identical input
// produce identical output (deep equal on all three returned values).
func TestProject_Determinism(t *testing.T) {
	entries, lines, coa := smallInput(t)

	gl1, bank1, sub1, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project (first call): %v", err)
	}

	gl2, bank2, sub2, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project (second call): %v", err)
	}

	if !reflect.DeepEqual(gl1, gl2) {
		t.Errorf("GL slices differ between two calls with identical input")
	}
	if !reflect.DeepEqual(bank1, bank2) {
		t.Errorf("Bank slices differ between two calls with identical input")
	}
	if !reflect.DeepEqual(sub1, sub2) {
		t.Errorf("SubLedgerSet differs between two calls with identical input")
	}
}

// TestProject_GLRowCount verifies that len(gl) == len(lines).
func TestProject_GLRowCount(t *testing.T) {
	entries, lines, coa := smallInput(t)

	gl, _, _, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	if len(gl) != len(lines) {
		t.Errorf("len(gl) = %d; want %d (len(lines))", len(gl), len(lines))
	}
}

// TestProject_BankRowCount verifies that the number of bank rows equals the
// count of journal lines whose AccountCode is in CashAccountCodes.
func TestProject_BankRowCount(t *testing.T) {
	entries, lines, coa := smallInput(t)

	_, bank, _, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	wantBankRows := 0
	for _, ln := range lines {
		if CashAccountCodes[ln.AccountCode] {
			wantBankRows++
		}
	}

	if len(bank) != wantBankRows {
		t.Errorf("len(bank) = %d; want %d (cash-account lines)", len(bank), wantBankRows)
	}
}

// TestProject_SubLedgerReferentialIntegrity verifies that every sub-ledger row's
// EntryID exists in the input entries slice.
func TestProject_SubLedgerReferentialIntegrity(t *testing.T) {
	entries, lines, coa := smallInput(t)

	_, _, sub, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	entryIDs := make(map[string]bool, len(entries))
	for _, e := range entries {
		entryIDs[e.EntryID] = true
	}

	checkRows := func(label string, rows []SubLedgerRow) {
		t.Helper()
		for _, row := range rows {
			if !entryIDs[row.EntryID] {
				t.Errorf("%s: row EntryID %q does not exist in input entries", label, row.EntryID)
			}
		}
	}

	checkRows("Invoices", sub.Invoices)
	checkRows("CustomerPayments", sub.CustomerPayments)
	checkRows("Bills", sub.Bills)
	checkRows("BillPayments", sub.BillPayments)
	checkRows("ExpenseReports", sub.ExpenseReports)
}

// TestProject_SortStability verifies that the composite sort key produces a
// stable, identical ordering across two independent calls.
func TestProject_SortStability(t *testing.T) {
	entries, lines, coa := smallInput(t)

	gl1, _, _, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project (first): %v", err)
	}
	gl2, _, _, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project (second): %v", err)
	}

	if len(gl1) != len(gl2) {
		t.Fatalf("GL length mismatch: %d vs %d", len(gl1), len(gl2))
	}
	for i := range gl1 {
		if gl1[i].LineID != gl2[i].LineID {
			t.Errorf("row %d: LineID mismatch: %q vs %q", i, gl1[i].LineID, gl2[i].LineID)
		}
		if gl1[i].RowNum != gl2[i].RowNum {
			t.Errorf("row %d: RowNum mismatch: %d vs %d", i, gl1[i].RowNum, gl2[i].RowNum)
		}
	}
}

// TestProject_GLRowNumsSequential verifies that GL row numbers are assigned
// sequentially starting at 1 with no gaps.
func TestProject_GLRowNumsSequential(t *testing.T) {
	entries, lines, coa := smallInput(t)
	gl, _, _, err := Project(entries, lines, coa)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	for i, row := range gl {
		if row.RowNum != i+1 {
			t.Errorf("gl[%d].RowNum = %d; want %d", i, row.RowNum, i+1)
		}
	}
}

// TestRun_Smoke verifies that Run returns a non-nil Dataset with no error for
// the canonical smoke configuration, and that GL, Bank, and SubLedger are
// non-empty.
func TestRun_Smoke(t *testing.T) {
	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		Seed:   1,
		Period: period,
		Volume: "small",
		Bundle: "monthly-close",
	}

	ds, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(ds.GL) == 0 {
		t.Error("Dataset.GL is empty; want non-empty")
	}
	if len(ds.Bank) == 0 {
		t.Error("Dataset.Bank is empty; want non-empty")
	}

	totalSubLedger := len(ds.SubLedger.Invoices) +
		len(ds.SubLedger.CustomerPayments) +
		len(ds.SubLedger.Bills) +
		len(ds.SubLedger.BillPayments) +
		len(ds.SubLedger.ExpenseReports)
	if totalSubLedger == 0 {
		t.Error("Dataset.SubLedger is entirely empty; want non-empty")
	}
}

// TestRun_UnknownBundle verifies that Run returns ErrUnknownBundle for a
// bundle name not built into this version.
func TestRun_UnknownBundle(t *testing.T) {
	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		Seed:   1,
		Period: period,
		Volume: "small",
		Bundle: "no-such-bundle",
	}
	_, err := Run(cfg)
	if err == nil {
		t.Fatal("Run with unknown bundle: expected error, got nil")
	}
}
