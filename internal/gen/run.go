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
	// Resolve bundle → BundleDefinition (includes EventMix).
	b, err := bundleByName(cfg.Bundle)
	if err != nil {
		return Dataset{}, err
	}
	return runWithBundle(cfg, b)
}

// RunWithBundle executes the full pipeline using a pre-resolved BundleDefinition.
// This is the entry point used when --bundle-config is provided; the built-in
// bundle name validation in ParseConfig is bypassed because the caller has
// already loaded and validated the bundle from YAML.
//
// The cfg.Bundle field is used only to populate the manifest; set it to
// bd.Name before calling.
func RunWithBundle(cfg Config, bd BundleDefinition) (Dataset, error) {
	return runWithBundle(cfg, bd)
}

// runWithBundle is the shared implementation for Run and RunWithBundle.
// All RNG draws happen in this function; the order must never change.
func runWithBundle(cfg Config, b BundleDefinition) (Dataset, error) {
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

	// Resolve volume: YAML bundles may override the built-in event count via
	// VolumeScale. If VolumeScale is nil or does not contain the requested tier,
	// fall through to the built-in volumeEventCount.
	volume := cfg.Volume
	if len(b.VolumeScale) > 0 {
		if count, ok := b.VolumeScale[volume]; ok {
			// Generate events using the YAML-supplied count directly.
			events, err := generateWithCount(rng, cfg.Period, count, b.EventMix)
			if err != nil {
				return Dataset{}, fmt.Errorf("gen.Run: Generate: %w", err)
			}
			return finishPipeline(cfg, b, bundleName, coa, customers, vendors, events, rng)
		}
	}

	// Generate events using the standard volume tier.
	events, err := Generate(rng, cfg.Period, volume, b.EventMix)
	if err != nil {
		return Dataset{}, fmt.Errorf("gen.Run: Generate: %w", err)
	}

	return finishPipeline(cfg, b, bundleName, coa, customers, vendors, events, rng)
}

// finishPipeline completes Expand → Project → (optional) InjectStress → Manifest.
// Extracted so both volume paths share the same post-generate logic.
func finishPipeline(cfg Config, b BundleDefinition, bundleName string, coa map[string]Account, customers []Customer, vendors []Vendor, events []Event, rng *RNG) (Dataset, error) {
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

	ds := Dataset{
		CoA:       coa,
		Entries:   entries,
		Lines:     lines,
		GL:        gl,
		Bank:      bank,
		SubLedger: sub,
		Customers: customers,
		Vendors:   vendors,
	}

	// For stress bundles, inject anomalies after the standard pipeline.
	// InjectStress deliberately violates invariants 1–3; the caller (cmd/fbx)
	// must skip invariant.Check for stress bundles (gated on bundle.StressTest).
	if b.StressTest {
		var truthLog TruthLog
		var stressErr error
		ds, truthLog, stressErr = InjectStress(ds, rng)
		if stressErr != nil {
			return Dataset{}, fmt.Errorf("gen.Run: InjectStress: %w", stressErr)
		}
		ds.Truth = truthLog
	}

	manifest := Manifest{
		Seed:   cfg.Seed,
		Period: cfg.Period.Format("2006-01"),
		Bundle: bundleName,
		RowCounts: map[string]int{
			"gl.csv":              len(ds.GL),
			"bank_statement.csv":  len(ds.Bank),
			"journal_entries.csv": len(ds.Entries),
		},
		Hashes: map[string]string{},
	}
	ds.Manifest = manifest

	return ds, nil
}
