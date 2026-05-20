package gen

import "fmt"

// defaultMonthlyCloseMix returns the EventMix used for the monthly-close bundle.
// PR-08 owns the Bundle struct; this function is the hook PR-08 will replace
// with a lookup from bundle.go. Weights reflect a typical month-end close:
// invoices and payments dominate, with smaller shares for bills, expenses,
// payroll, and depreciation.
func defaultMonthlyCloseMix() EventMix {
	return EventMix{
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
}

// mixForBundle returns the EventMix for the given bundle name.
// Returns ErrUnknownBundle for any name not built into this version.
func mixForBundle(bundle string) (EventMix, error) {
	switch bundle {
	case "monthly-close", "":
		return defaultMonthlyCloseMix(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownBundle, bundle)
	}
}

// Run executes the full internal/gen pipeline for the given Config and returns
// a populated Dataset. It is a pure function of cfg: no file I/O, no
// wall-clock reads, no goroutines. The returned Dataset has all four slices
// non-empty for any valid volume tier with at least one event in the mix.
//
// Pipeline: DefaultCoA → IndexCoA → NewRNG → Generate → Expand → Project.
//
// Manifest in the returned Dataset carries seed, period, bundle, and version
// identity fields; file hashes are populated in PR-07.
func Run(cfg Config) (Dataset, error) {
	// Resolve bundle → EventMix.
	bundle := cfg.Bundle
	if bundle == "" {
		bundle = "monthly-close"
	}
	mix, err := mixForBundle(bundle)
	if err != nil {
		return Dataset{}, err
	}

	// Build CoA.
	accounts := DefaultCoA()
	coa := IndexCoA(accounts)

	// Seed RNG.
	rng := NewRNG(cfg.Seed)

	// Generate events.
	events, err := Generate(rng, cfg.Period, cfg.Volume, mix)
	if err != nil {
		return Dataset{}, fmt.Errorf("gen.Run: Generate: %w", err)
	}

	// Expand events into journal entries and lines.
	entries, lines, err := Expand(events, coa)
	if err != nil {
		return Dataset{}, fmt.Errorf("gen.Run: Expand: %w", err)
	}

	// Project into GL, bank, and sub-ledger rows.
	gl, bank, sub, err := Project(entries, lines, coa)
	if err != nil {
		return Dataset{}, fmt.Errorf("gen.Run: Project: %w", err)
	}

	manifest := Manifest{
		Seed:   cfg.Seed,
		Period: cfg.Period.Format("2006-01"),
		Bundle: bundle,
		RowCounts: map[string]int{
			"gl.csv":              len(gl),
			"bank_statement.csv":  len(bank),
			"journal_entries.csv": len(entries),
		},
		Hashes: map[string]string{},
	}

	return Dataset{
		CoA:       coa,
		Entries:   entries,
		Lines:     lines,
		GL:        gl,
		Bank:      bank,
		SubLedger: sub,
		Manifest:  manifest,
	}, nil
}
