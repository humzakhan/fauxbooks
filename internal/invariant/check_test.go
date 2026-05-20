package invariant_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
)

// minimalDataset returns the smallest valid Dataset: one balanced entry with two
// lines, a matching GL row and bank row (on a cash account), and one sub-ledger
// row pointing to the entry. All four invariants are satisfied.
func minimalDataset() gen.Dataset {
	coa := gen.IndexCoA(gen.DefaultCoA())

	entry := gen.JournalEntry{
		EntryID:     "e-001",
		OccurredAt:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "Sale invoice",
	}

	hundred := decimal.NewFromInt(100)

	lines := []gen.JournalLine{
		{LineID: "l-001", EntryID: "e-001", AccountCode: "1010", Debit: hundred, Credit: decimal.Zero},
		{LineID: "l-002", EntryID: "e-001", AccountCode: "4000", Debit: decimal.Zero, Credit: hundred},
	}

	gl := []gen.GLRow{
		{RowNum: 1, EntryID: "e-001", LineID: "l-001", AccountCode: "1010",
			Debit: "100.000000000", Credit: "0.000000000"},
		{RowNum: 2, EntryID: "e-001", LineID: "l-002", AccountCode: "4000",
			Debit: "0.000000000", Credit: "100.000000000"},
	}

	bank := []gen.BankRow{
		{RowNum: 1, EntryID: "e-001", LineID: "l-001", AccountCode: "1010",
			Debit: "100.000000000", Credit: "0.000000000"},
	}

	sub := gen.SubLedgerSet{
		Invoices: []gen.SubLedgerRow{
			{RowNum: 1, EntryID: "e-001", Amount: "100.000000000", SubLedgerType: "invoice"},
		},
	}

	return gen.Dataset{
		CoA:       coa,
		Entries:   []gen.JournalEntry{entry},
		Lines:     lines,
		GL:        gl,
		Bank:      bank,
		SubLedger: sub,
	}
}

// extractInvariantError asserts that err is non-nil and that errors.As extracts
// an InvariantError with the expected invariant number.
func extractInvariantError(t *testing.T, err error, wantInvariant int) invariant.InvariantError {
	t.Helper()
	if err == nil {
		t.Fatalf("Check: want error (invariant %d), got nil", wantInvariant)
	}
	var ie invariant.InvariantError
	if !errors.As(err, &ie) {
		t.Fatalf("errors.As: want InvariantError, got %T: %v", err, err)
	}
	if ie.Invariant != wantInvariant {
		t.Errorf("InvariantError.Invariant: got %d, want %d (detail: %s)", ie.Invariant, wantInvariant, ie.Detail)
	}
	return ie
}

// TestInv1_Unbalanced verifies that a journal entry with Σ debits ≠ Σ credits
// returns InvariantError{Invariant: 1}.
func TestInv1_Unbalanced(t *testing.T) {
	ds := minimalDataset()
	// Break invariant 1: credit is one cent less than debit.
	ds.Lines[1].Credit = decimal.NewFromFloat(99.99)

	err := invariant.Check(ds)
	extractInvariantError(t, err, 1)
}

// TestInv2_UnknownAccountCode verifies that a journal line referencing a code
// absent from ds.CoA returns InvariantError{Invariant: 2}.
func TestInv2_UnknownAccountCode(t *testing.T) {
	ds := minimalDataset()
	// Break invariant 2: replace a known code with a nonexistent one.
	ds.Lines[1].AccountCode = "9999"

	err := invariant.Check(ds)
	extractInvariantError(t, err, 2)
}

// TestInv3_BankGLMismatch verifies that a one-cent discrepancy between bank
// totals and GL cash postings returns InvariantError{Invariant: 3}.
func TestInv3_BankGLMismatch(t *testing.T) {
	ds := minimalDataset()
	// Break invariant 3: inflate bank debit by one cent.
	ds.Bank[0].Debit = "100.010000000"

	err := invariant.Check(ds)
	extractInvariantError(t, err, 3)
}

// TestInv4_DanglingSubLedgerEntryID verifies that a sub-ledger row whose
// EntryID does not exist in ds.Entries returns InvariantError{Invariant: 4}.
func TestInv4_DanglingSubLedgerEntryID(t *testing.T) {
	ds := minimalDataset()
	// Break invariant 4: point the invoice sub-ledger row at a nonexistent entry.
	ds.SubLedger.Invoices[0].EntryID = "does-not-exist"

	err := invariant.Check(ds)
	extractInvariantError(t, err, 4)
}

// TestCheckOrdering verifies that Check returns invariants in order: a Dataset
// that violates both inv-1 and inv-2 should return inv-1 first.
func TestCheckOrdering(t *testing.T) {
	ds := minimalDataset()
	// Break both inv-1 and inv-2.
	ds.Lines[1].Credit = decimal.NewFromFloat(99.99) // inv-1
	ds.Lines[1].AccountCode = "9999"                 // inv-2 (same line)

	err := invariant.Check(ds)
	extractInvariantError(t, err, 1) // must be inv-1, not inv-2
}

// TestErrorsAs explicitly verifies that errors.As works with a pointer to
// invariant.InvariantError for callers using the aliased type.
func TestErrorsAs(t *testing.T) {
	ds := minimalDataset()
	ds.Lines[1].Credit = decimal.NewFromFloat(99.99)

	err := invariant.Check(ds)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	var ie invariant.InvariantError
	if !errors.As(err, &ie) {
		t.Fatalf("errors.As(err, &invariant.InvariantError{}) returned false; err=%v", err)
	}
	if ie.Invariant != 1 {
		t.Errorf("got Invariant=%d, want 1", ie.Invariant)
	}
}

// TestPositive_RunProducesValidDataset verifies that a Dataset produced by
// gen.Run with Seed=1, Volume=small returns nil from Check.
func TestPositive_RunProducesValidDataset(t *testing.T) {
	cfg := gen.Config{
		Seed:   1,
		Period: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Volume: "small",
		Bundle: "monthly-close",
	}
	ds, err := gen.Run(cfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}
	if err := invariant.Check(ds); err != nil {
		t.Errorf("Check(Run(...)): want nil, got %v", err)
	}
}
