package gen

import "fmt"

// Run executes the full internal/gen pipeline for the given Config and returns
// a populated Dataset. It is a pure function of cfg: no file I/O, no
// wall-clock reads, no goroutines. The returned Dataset has all four slices
// non-empty for any valid volume tier with at least one event in the mix.
//
// Pipeline: DefaultCoA → IndexCoA → NewRNG → Generate → Expand → Project.
//
// Manifest in the returned Dataset carries seed, period, bundle, and version
// identity fields; file hashes are populated by internal/writer during Write.
func Run(cfg Config) (Dataset, error) {
	// Resolve bundle → Bundle (includes EventMix).
	b, err := bundleByName(cfg.Bundle)
	if err != nil {
		return Dataset{}, err
	}
	bundleName := b.Name

	// Build CoA.
	accounts := DefaultCoA()
	coa := IndexCoA(accounts)

	// Seed RNG.
	rng := NewRNG(cfg.Seed)

	// Generate customer and vendor master records before events so that RefIDs
	// in events reference a populated master. The master is deterministic because
	// all draws come from the seeded RNG.
	customers := GenerateCustomers(rng, customerCount)
	vendors := GenerateVendors(rng, vendorCount)

	// Generate events.
	events, err := Generate(rng, cfg.Period, cfg.Volume, b.Mix)
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
		Bundle: bundleName,
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
		Customers: customers,
		Vendors:   vendors,
		Manifest:  manifest,
	}, nil
}
