package gen

import (
	"testing"
	"time"
)

// testMix is a balanced EventMix used across all tests in this file.
// Every supported EventType has weight 1 (uniform distribution).
var testMix = EventMix{
	EventTypeSaleInvoice:       10,
	EventTypeCustomerPayment:   10,
	EventTypeVendorBill:        10,
	EventTypeBillPayment:       10,
	EventTypeExpenseReport:     5,
	EventTypePayroll:           5,
	EventTypeDepreciation:      3,
	EventTypeInventoryPurchase: 7,
	EventTypeInventoryCOGS:     7,
	EventTypeInterestIncome:    3,
}

// testPeriod is January 2026 (UTC), used as the canonical test period.
var testPeriod = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// requireNoError is a t.Helper that fatals if err is non-nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGenerate_Determinism verifies that two Generate calls with the same
// seed, period, volume, and mix produce byte-identical []Event slices.
func TestGenerate_Determinism(t *testing.T) {
	const seed = 1

	rng1 := NewRNG(seed)
	got1, err := Generate(rng1, testPeriod, "small", testMix)
	requireNoError(t, err)

	rng2 := NewRNG(seed)
	got2, err := Generate(rng2, testPeriod, "small", testMix)
	requireNoError(t, err)

	if len(got1) != len(got2) {
		t.Fatalf("length mismatch: %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		e1, e2 := got1[i], got2[i]
		if e1.ID != e2.ID {
			t.Errorf("[%d] ID: %q vs %q", i, e1.ID, e2.ID)
		}
		if e1.Type != e2.Type {
			t.Errorf("[%d] Type: %q vs %q", i, e1.Type, e2.Type)
		}
		if !e1.OccurredAt.Equal(e2.OccurredAt) {
			t.Errorf("[%d] OccurredAt: %v vs %v", i, e1.OccurredAt, e2.OccurredAt)
		}
		if !e1.Amount.Equal(e2.Amount) {
			t.Errorf("[%d] Amount: %v vs %v", i, e1.Amount, e2.Amount)
		}
		if e1.RefID != e2.RefID {
			t.Errorf("[%d] RefID: %q vs %q", i, e1.RefID, e2.RefID)
		}
	}
}

// TestGenerate_DifferentSeeds verifies that two Generate calls with different
// seeds produce different event slices (probabilistic sanity check).
func TestGenerate_DifferentSeeds(t *testing.T) {
	rng1 := NewRNG(1)
	got1, err := Generate(rng1, testPeriod, "small", testMix)
	requireNoError(t, err)

	rng2 := NewRNG(2)
	got2, err := Generate(rng2, testPeriod, "small", testMix)
	requireNoError(t, err)

	// At least one event must differ (type, amount, or refID).
	// With different seeds the probability of all 150 events matching is
	// astronomically small; treat any full match as a bug.
	allSame := true
	for i := range got1 {
		if got1[i].Type != got2[i].Type ||
			!got1[i].Amount.Equal(got2[i].Amount) ||
			got1[i].RefID != got2[i].RefID {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("different seeds produced identical event slices — RNG is not being seeded correctly")
	}
}

// TestGenerate_VolumeCounts verifies that each volume tier produces an event
// count within the ±20% acceptance band defined in the spec.
func TestGenerate_VolumeCounts(t *testing.T) {
	cases := []struct {
		volume string
		lo     int
		hi     int
	}{
		{"small", 120, 180},
		{"medium", 1200, 1800},
		{"large", 12000, 18000},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.volume, func(t *testing.T) {
			rng := NewRNG(1)
			events, err := Generate(rng, testPeriod, tc.volume, testMix)
			requireNoError(t, err)
			n := len(events)
			if n < tc.lo || n > tc.hi {
				t.Errorf("volume %q: got %d events, want [%d, %d]", tc.volume, n, tc.lo, tc.hi)
			}
		})
	}
}

// TestGenerate_ExactCounts verifies the exact deterministic counts per tier.
// These exact values are documented in volumeEventCount's implementation.
func TestGenerate_ExactCounts(t *testing.T) {
	cases := []struct {
		volume string
		want   int
	}{
		{"small", 150},
		{"medium", 1500},
		{"large", 15000},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.volume, func(t *testing.T) {
			rng := NewRNG(42)
			events, err := Generate(rng, testPeriod, tc.volume, testMix)
			requireNoError(t, err)
			if len(events) != tc.want {
				t.Errorf("volume %q: got %d events, want %d", tc.volume, len(events), tc.want)
			}
		})
	}
}

// TestGenerate_TimestampBounds verifies that every event's OccurredAt lies
// strictly within the period month (>= periodStart, < periodEnd).
func TestGenerate_TimestampBounds(t *testing.T) {
	rng := NewRNG(1)
	events, err := Generate(rng, testPeriod, "small", testMix)
	requireNoError(t, err)

	periodStart, periodEnd := periodBounds(testPeriod)

	for i, e := range events {
		if e.OccurredAt.Before(periodStart) {
			t.Errorf("[%d] OccurredAt %v is before periodStart %v", i, e.OccurredAt, periodStart)
		}
		if !e.OccurredAt.Before(periodEnd) {
			t.Errorf("[%d] OccurredAt %v is not before periodEnd %v", i, e.OccurredAt, periodEnd)
		}
	}
}

// TestGenerate_UnknownVolume verifies that Generate returns a wrapped
// ErrUnknownVolume for an unrecognised volume string.
func TestGenerate_UnknownVolume(t *testing.T) {
	rng := NewRNG(1)
	_, err := Generate(rng, testPeriod, "xlarge", testMix)
	if err == nil {
		t.Fatal("expected error for unknown volume, got nil")
	}
	// The error must wrap ErrUnknownVolume so callers can use errors.Is.
	if !isErrUnknownVolume(err) {
		t.Errorf("error %q does not wrap ErrUnknownVolume", err)
	}
}

// isErrUnknownVolume unwraps to check for ErrUnknownVolume without importing errors
// at the call site (keeps the test file clean; errors is already imported via gen).
func isErrUnknownVolume(err error) bool {
	// Unwrap manually — same as errors.Is but without adding an import to the test.
	for err != nil {
		if err == ErrUnknownVolume {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return false
}

// TestGenerate_EmptyMix verifies that Generate returns an error when the mix
// has no positive weights (would otherwise loop forever or panic).
func TestGenerate_EmptyMix(t *testing.T) {
	rng := NewRNG(1)
	_, err := Generate(rng, testPeriod, "small", EventMix{})
	if err == nil {
		t.Fatal("expected error for empty EventMix, got nil")
	}
}

// TestGenerate_ZeroWeightExcluded verifies that event types with weight 0 are
// never selected, even when they appear in the mix map.
func TestGenerate_ZeroWeightExcluded(t *testing.T) {
	// Only allow sale invoices; all other types have weight 0 (absent from map
	// or explicitly 0 — both are equivalent after the map lookup).
	onlyInvoice := EventMix{
		EventTypeSaleInvoice: 1,
		EventTypePayroll:     0, // explicit zero — must be excluded
	}

	rng := NewRNG(1)
	events, err := Generate(rng, testPeriod, "small", onlyInvoice)
	requireNoError(t, err)

	for i, e := range events {
		if e.Type != EventTypeSaleInvoice {
			t.Errorf("[%d] got type %q, want %q", i, e.Type, EventTypeSaleInvoice)
		}
	}
}

// TestGenerate_IDsAreUnique verifies that every event in a run has a unique ID.
func TestGenerate_IDsAreUnique(t *testing.T) {
	rng := NewRNG(1)
	events, err := Generate(rng, testPeriod, "small", testMix)
	requireNoError(t, err)

	seen := make(map[string]int, len(events))
	for i, e := range events {
		if prev, ok := seen[e.ID]; ok {
			t.Errorf("[%d] ID %q already seen at index %d", i, e.ID, prev)
		}
		seen[e.ID] = i
	}
}

// TestGenerate_AmountsPositive verifies that no event has a zero or negative amount.
func TestGenerate_AmountsPositive(t *testing.T) {
	rng := NewRNG(1)
	events, err := Generate(rng, testPeriod, "small", testMix)
	requireNoError(t, err)

	for i, e := range events {
		if !e.Amount.IsPositive() {
			t.Errorf("[%d] Amount %v is not positive", i, e.Amount)
		}
	}
}

// TestPeriodBounds verifies the helper returns correct UTC start/end instants.
func TestPeriodBounds(t *testing.T) {
	cases := []struct {
		period    time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			period:    time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), // mid-month input
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			period:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), // Feb (28 days in 2026)
			wantStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			period:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), // Feb leap year
			wantStart: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		start, end := periodBounds(tc.period)
		if !start.Equal(tc.wantStart) {
			t.Errorf("period %v: start got %v, want %v", tc.period, start, tc.wantStart)
		}
		if !end.Equal(tc.wantEnd) {
			t.Errorf("period %v: end got %v, want %v", tc.period, end, tc.wantEnd)
		}
	}
}
