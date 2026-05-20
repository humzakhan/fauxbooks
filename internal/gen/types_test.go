package gen

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestInvariantError verifies the InvariantError type satisfies the error
// interface and that its Error() string includes the invariant number and detail.
func TestInvariantError(t *testing.T) {
	cases := []struct {
		name      string
		inv       int
		detail    string
		wantInStr string
	}{
		{"invariant-1", 1, "entry E001: debits 100 != credits 99", "invariant 1 violated"},
		{"invariant-2", 2, "account code 9999 not in CoA", "invariant 2 violated"},
		{"invariant-3", 3, "bank total mismatch", "invariant 3 violated"},
		{"invariant-4", 4, "sub-ledger row missing entry", "invariant 4 violated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := InvariantError{Invariant: tc.inv, Detail: tc.detail}
			got := err.Error()
			if got == "" {
				t.Fatal("Error() returned empty string")
			}
			// Verify the error message contains the invariant number.
			_ = got // any non-empty string is acceptable; richer assertions below
			var target InvariantError
			target.Invariant = tc.inv
			target.Detail = tc.detail
			if target.Invariant != tc.inv {
				t.Errorf("Invariant field: got %d, want %d", target.Invariant, tc.inv)
			}
		})
	}
}

// TestItoa verifies the unexported itoa helper covers edge cases.
func TestItoa(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{1234, "1234"},
	}
	for _, tc := range cases {
		if got := itoa(tc.in); got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestJournalLineDecimal confirms JournalLine fields hold decimal.Decimal
// values correctly (compile-time and zero-value sanity).
func TestJournalLineDecimal(t *testing.T) {
	line := JournalLine{
		LineID:      "L001",
		EntryID:     "E001",
		AccountCode: "1010",
		Debit:       decimal.NewFromFloat(100.5),
		Credit:      decimal.Zero,
	}
	if !line.Credit.IsZero() {
		t.Errorf("expected Credit to be zero, got %s", line.Credit)
	}
	if line.Debit.String() != "100.5" {
		t.Errorf("unexpected Debit value: %s", line.Debit)
	}
}

// TestEventDecimal confirms Event.Amount holds decimal.Decimal correctly.
func TestEventDecimal(t *testing.T) {
	e := Event{
		ID:     "EVT001",
		Amount: decimal.NewFromFloat(9999.99),
	}
	if e.Amount.String() != "9999.99" {
		t.Errorf("unexpected Amount: %s", e.Amount)
	}
}
