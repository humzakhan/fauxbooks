package gen

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// EventType discriminates the template used to expand an Event into journal lines.
// Every constant below corresponds to one business event supported in v0.1.
// The string value is stable and used as a display/serialization key.
const (
	// EventTypeSaleInvoice records a customer invoice for goods or services sold.
	EventTypeSaleInvoice EventType = "sale_invoice"

	// EventTypeCustomerPayment records a cash receipt applied against an invoice.
	EventTypeCustomerPayment EventType = "customer_payment"

	// EventTypeVendorBill records a vendor invoice received for goods or services.
	EventTypeVendorBill EventType = "vendor_bill"

	// EventTypeBillPayment records a cash disbursement to pay a vendor bill.
	EventTypeBillPayment EventType = "bill_payment"

	// EventTypeExpenseReport records an employee expense report reimbursement.
	EventTypeExpenseReport EventType = "expense_report"

	// EventTypePayroll records a payroll run (salaries, wages, payroll taxes).
	EventTypePayroll EventType = "payroll"

	// EventTypeDepreciation records periodic depreciation of fixed assets.
	EventTypeDepreciation EventType = "depreciation"

	// EventTypeInventoryPurchase records a purchase of inventory for resale.
	EventTypeInventoryPurchase EventType = "inventory_purchase"

	// EventTypeInventoryCOGS records the cost of goods sold for an inventory sale.
	EventTypeInventoryCOGS EventType = "inventory_cogs"

	// EventTypeInterestIncome records interest earned on cash accounts.
	EventTypeInterestIncome EventType = "interest_income"
)

// allEventTypes lists every valid EventType in a fixed order used by weight
// table construction. Order is stable so the weighted-selection loop is
// deterministic across builds.
var allEventTypes = []EventType{
	EventTypeSaleInvoice,
	EventTypeCustomerPayment,
	EventTypeVendorBill,
	EventTypeBillPayment,
	EventTypeExpenseReport,
	EventTypePayroll,
	EventTypeDepreciation,
	EventTypeInventoryPurchase,
	EventTypeInventoryCOGS,
	EventTypeInterestIncome,
}

// EventMix maps each EventType to a non-negative integer weight.
// Weight 0 excludes that type. The generator selects types proportionally
// to their weights using integer arithmetic (no float64 probabilities).
// Callers supply the mix; no default mix is exported — the bundle (PR-08)
// is the single place that defines the production weights.
type EventMix map[EventType]int

// volumeEventCount returns the exact event count for a volume tier.
//
// Tier targets: small = 150, medium = 1500, large = 15000.
// Exact counts are used (not a jitter formula) to keep tests simple and the
// expected-count assertion unambiguous. These values sit squarely within the
// ±20% acceptance band required by the spec (120–180, 1200–1800, 12000–18000).
func volumeEventCount(volume string) (int, error) {
	switch volume {
	case "small":
		return 150, nil
	case "medium":
		return 1500, nil
	case "large":
		return 15000, nil
	default:
		return 0, fmt.Errorf("gen.Generate: unknown volume tier %q", volume)
	}
}

// periodBounds returns [start, end) for the calendar month containing period.
// start is the first instant of the month (00:00:00 UTC);
// end is the first instant of the following month (exclusive upper bound).
func periodBounds(period time.Time) (start, end time.Time) {
	start = time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}

// deterministicOffset returns the nanosecond duration to add to periodStart
// for event i out of total events in a period of daysInPeriod calendar days.
//
// The offset spreads events evenly across the period by dividing the period's
// nanosecond span into (total + 1) equal slots and placing event i at slot
// (i + 1). This guarantees:
//   - All offsets are strictly within [0, periodNanos).
//   - No two events at identical positions (they are equally spaced).
//   - The result is a pure arithmetic function of (i, total, daysInPeriod) —
//     no RNG draw, no wall-clock read.
func deterministicOffset(i, total, daysInPeriod int) time.Duration {
	periodNanos := int64(daysInPeriod) * 24 * int64(time.Hour)
	// slot = periodNanos / (total + 1) nanoseconds per slot
	slotNanos := periodNanos / int64(total+1)
	return time.Duration(int64(i+1) * slotNanos)
}

// Generate produces a deterministic slice of Event values.
//
// Parameters:
//   - rng: seeded RNG; all randomness flows through it.
//   - period: any time.Time whose year and month identify the target period.
//     The day and sub-day components are ignored.
//   - volume: "small" (150), "medium" (1500), or "large" (15000).
//   - mix: EventMix mapping EventType → integer weight. Types with weight 0
//     are excluded. At least one type must have weight > 0.
//
// All timestamps in the returned events derive from period arithmetic; no
// time.Now() is called. Event type selection uses integer weights and IntN
// (no float64 probabilities). The returned slice is ordered by OccurredAt.
//
// Returns ErrUnknownVolume (wrapped) for an unrecognised volume tier.
func Generate(rng *RNG, period time.Time, volume string, mix EventMix) ([]Event, error) {
	count, err := volumeEventCount(volume)
	if err != nil {
		// ErrUnknownVolume is declared in config.go; reuse it here so callers
		// can use errors.Is(err, ErrUnknownVolume) regardless of call site.
		return nil, fmt.Errorf("%w: %s", ErrUnknownVolume, volume)
	}
	return generateWithCount(rng, period, count, mix)
}

// generateWithCount produces exactly count events using the given mix.
// It is the shared core of Generate (volume-tier path) and the YAML-bundle
// volume-scale path in runWithBundle. RNG call order is identical in both paths.
func generateWithCount(rng *RNG, period time.Time, count int, mix EventMix) ([]Event, error) {
	// Build a parallel pair of (type, weight) slices from allEventTypes so the
	// iteration order is deterministic (allEventTypes is a fixed slice, not a map).
	types := make([]EventType, 0, len(allEventTypes))
	weights := make([]int, 0, len(allEventTypes))
	totalWeight := 0
	for _, et := range allEventTypes {
		w := mix[et]
		if w > 0 {
			types = append(types, et)
			weights = append(weights, w)
			totalWeight += w
		}
	}
	if totalWeight == 0 {
		return nil, fmt.Errorf("gen.Generate: EventMix has no positive weights")
	}

	periodStart, periodEnd := periodBounds(period)
	daysInPeriod := int(periodEnd.Sub(periodStart).Hours() / 24)

	events := make([]Event, count)
	for i := range events {
		// Select event type via integer-weight proportional selection.
		pick := rng.IntN(totalWeight)
		cumulative := 0
		var chosen EventType
		for j, w := range weights {
			cumulative += w
			if pick < cumulative {
				chosen = types[j]
				break
			}
		}

		// Amount: a small positive decimal drawn from RNG.
		// Range: [1, 10000) cents (i.e. $0.01 – $99.99) expressed as dollars.
		// IntN(999900) gives [0, 999899]; add 1 for [1, 999900] cents = [$0.01, $9999.00].
		amountCents := int64(rng.IntN(999900)) + 1
		amount := decimal.NewFromInt(amountCents).Div(decimal.NewFromInt(100))

		// RefID: a small integer reference to a customer or vendor record.
		// Range [1, 100) so downstream sub-ledger tables have ≤100 distinct refs.
		refID := fmt.Sprintf("ref-%03d", rng.IntN(100)+1)

		offset := deterministicOffset(i, count, daysInPeriod)
		occurredAt := periodStart.Add(offset)

		events[i] = Event{
			ID:         fmt.Sprintf("evt-%06d", i+1),
			Type:       chosen,
			OccurredAt: occurredAt,
			Amount:     amount,
			RefID:      refID,
			Metadata:   map[string]string{},
		}
	}

	return events, nil
}
