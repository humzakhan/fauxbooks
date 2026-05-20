package gen

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// testCoA returns a CoA index built from DefaultCoA for use in template tests.
func testCoA(t *testing.T) map[string]Account {
	t.Helper()
	return IndexCoA(DefaultCoA())
}

// testEvent builds a minimal Event with the given type and amount.
func testEvent(typ EventType, amount decimal.Decimal) Event {
	return Event{
		ID:         "evt-000001",
		Type:       typ,
		OccurredAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		Amount:     amount,
		RefID:      "ref-001",
		Metadata:   map[string]string{},
	}
}

// assertBalanced sums the debit and credit sides of lines and asserts they are
// equal. t.Helper is set so failures point to the caller.
func assertBalanced(t *testing.T, lines []JournalLine) {
	t.Helper()
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, l := range lines {
		totalDebit = totalDebit.Add(l.Debit)
		totalCredit = totalCredit.Add(l.Credit)
	}
	if !totalDebit.Equal(totalCredit) {
		t.Errorf("entry not balanced: Σ debits=%s Σ credits=%s", totalDebit, totalCredit)
	}
}

// representativeAmounts are the four amounts required by the spec.
var representativeAmounts = []string{"0.01", "1.00", "1000.00", "9999.99"}

// TestTemplateBalance verifies that every event type produces a balanced entry
// at all four representative amounts.
func TestTemplateBalance(t *testing.T) {
	coa := testCoA(t)

	type expandFunc func(entryID string, ev Event, coa map[string]Account, counter *int) (JournalEntry, []JournalLine, error)

	cases := []struct {
		name   string
		evType EventType
		fn     expandFunc
	}{
		{"sale_invoice", EventTypeSaleInvoice, expandSaleInvoice},
		{"customer_payment", EventTypeCustomerPayment, expandCustomerPayment},
		{"vendor_bill", EventTypeVendorBill, expandVendorBill},
		{"bill_payment", EventTypeBillPayment, expandBillPayment},
		{"expense_report", EventTypeExpenseReport, expandExpenseReport},
		{"payroll", EventTypePayroll, expandPayroll},
		{"depreciation", EventTypeDepreciation, expandDepreciation},
		{"inventory_purchase", EventTypeInventoryPurchase, expandInventoryPurchase},
		{"inventory_cogs", EventTypeInventoryCOGS, expandInventoryCOGS},
		{"interest_income", EventTypeInterestIncome, expandInterestIncome},
	}

	for _, tc := range cases {
		for _, amtStr := range representativeAmounts {
			amt, _ := decimal.NewFromString(amtStr)
			t.Run(tc.name+"/"+amtStr, func(t *testing.T) {
				counter := 0
				ev := testEvent(tc.evType, amt)
				_, lines, err := tc.fn("JE-00000001", ev, coa, &counter)
				if err != nil {
					t.Fatalf("expand returned error: %v", err)
				}
				assertBalanced(t, lines)
			})
		}
	}
}

// TestExpandDeterminism verifies that calling Expand twice with the same input
// produces deep-equal output (no hidden mutable state).
func TestExpandDeterminism(t *testing.T) {
	coa := testCoA(t)

	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rng := NewRNG(42)
	mix := EventMix{
		EventTypeSaleInvoice:       3,
		EventTypeCustomerPayment:   3,
		EventTypeVendorBill:        2,
		EventTypeBillPayment:       2,
		EventTypeExpenseReport:     2,
		EventTypePayroll:           1,
		EventTypeDepreciation:      1,
		EventTypeInventoryPurchase: 2,
		EventTypeInventoryCOGS:     2,
		EventTypeInterestIncome:    1,
	}
	events, err := Generate(rng, period, "small", mix)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	entries1, lines1, err := Expand(events, coa)
	if err != nil {
		t.Fatalf("Expand first call: %v", err)
	}

	entries2, lines2, err := Expand(events, coa)
	if err != nil {
		t.Fatalf("Expand second call: %v", err)
	}

	if len(entries1) != len(entries2) {
		t.Fatalf("entry count mismatch: %d vs %d", len(entries1), len(entries2))
	}
	for i := range entries1 {
		e1, e2 := entries1[i], entries2[i]
		if e1.EntryID != e2.EntryID || e1.Description != e2.Description ||
			e1.SourceEventID != e2.SourceEventID || !e1.OccurredAt.Equal(e2.OccurredAt) {
			t.Errorf("entry %d differs: %+v vs %+v", i, e1, e2)
		}
	}

	if len(lines1) != len(lines2) {
		t.Fatalf("line count mismatch: %d vs %d", len(lines1), len(lines2))
	}
	for i := range lines1 {
		l1, l2 := lines1[i], lines2[i]
		if l1.LineID != l2.LineID || l1.EntryID != l2.EntryID ||
			l1.AccountCode != l2.AccountCode ||
			!l1.Debit.Equal(l2.Debit) || !l1.Credit.Equal(l2.Credit) {
			t.Errorf("line %d differs: %+v vs %+v", i, l1, l2)
		}
	}
}

// TestExpandMissingAccount verifies that Expand returns an error (not a panic)
// when the CoA is missing an account that a template requires.
func TestExpandMissingAccount(t *testing.T) {
	// Build a stripped CoA missing account 4000 (Sales Revenue).
	accounts := DefaultCoA()
	filtered := make([]Account, 0, len(accounts))
	for _, a := range accounts {
		if a.Code != "4000" {
			filtered = append(filtered, a)
		}
	}
	strippedCoA := IndexCoA(filtered)

	ev := testEvent(EventTypeSaleInvoice, decimal.NewFromInt(100))
	_, _, err := Expand([]Event{ev}, strippedCoA)
	if err == nil {
		t.Fatal("expected error for missing account code 4000, got nil")
	}
}

// TestExpandAllCodesInDefaultCoA verifies that every AccountCode on every
// output line exists in DefaultCoA (invariant 2 pre-check).
func TestExpandAllCodesInDefaultCoA(t *testing.T) {
	coa := testCoA(t)

	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rng := NewRNG(1)
	mix := EventMix{
		EventTypeSaleInvoice:       1,
		EventTypeCustomerPayment:   1,
		EventTypeVendorBill:        1,
		EventTypeBillPayment:       1,
		EventTypeExpenseReport:     1,
		EventTypePayroll:           1,
		EventTypeDepreciation:      1,
		EventTypeInventoryPurchase: 1,
		EventTypeInventoryCOGS:     1,
		EventTypeInterestIncome:    1,
	}
	events, err := Generate(rng, period, "small", mix)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	_, lines, err := Expand(events, coa)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	for _, l := range lines {
		if _, ok := coa[l.AccountCode]; !ok {
			t.Errorf("line %s references account code %q which is not in DefaultCoA", l.LineID, l.AccountCode)
		}
	}
}

// TestExpandUnknownEventType verifies that Expand returns an error for an
// unrecognised event type rather than panicking.
func TestExpandUnknownEventType(t *testing.T) {
	coa := testCoA(t)
	ev := Event{
		ID:         "evt-bad",
		Type:       EventType("bogus_type"),
		OccurredAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		Amount:     decimal.NewFromInt(1),
		RefID:      "ref-001",
		Metadata:   map[string]string{},
	}
	_, _, err := Expand([]Event{ev}, coa)
	if err == nil {
		t.Fatal("expected error for unknown event type, got nil")
	}
}

// TestExpandFullBalance verifies the full Expand output: all entries balanced.
func TestExpandFullBalance(t *testing.T) {
	coa := testCoA(t)

	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rng := NewRNG(1)
	mix := EventMix{
		EventTypeSaleInvoice:       3,
		EventTypeCustomerPayment:   3,
		EventTypeVendorBill:        2,
		EventTypeBillPayment:       2,
		EventTypeExpenseReport:     2,
		EventTypePayroll:           1,
		EventTypeDepreciation:      1,
		EventTypeInventoryPurchase: 2,
		EventTypeInventoryCOGS:     2,
		EventTypeInterestIncome:    1,
	}
	events, err := Generate(rng, period, "small", mix)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	entries, lines, err := Expand(events, coa)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	if len(entries) != len(events) {
		t.Fatalf("expected %d entries, got %d", len(events), len(entries))
	}

	// Build a per-entry line index and check balance for each.
	entryLines := make(map[string][]JournalLine, len(entries))
	for _, l := range lines {
		entryLines[l.EntryID] = append(entryLines[l.EntryID], l)
	}
	for _, e := range entries {
		assertBalanced(t, entryLines[e.EntryID])
	}
}
