// Package gen contains all core domain logic for fauxbooks: CoA, RNG,
// event generation, template expansion, projection, and bundle dispatch.
// It has no file I/O; all output goes through internal/writer.
package gen

import (
	"errors"
	"fmt"
	"time"
)

// ErrBadPeriod is returned by ParseConfig when the --period flag is not a
// valid YYYY-MM string.
var ErrBadPeriod = errors.New("gen: invalid period; want YYYY-MM")

// ErrUnknownVolume is returned by ParseConfig when the --volume flag is not
// one of the recognised tier names (small, medium, large).
var ErrUnknownVolume = errors.New("gen: unknown volume tier; want small|medium|large")

// ErrUnknownBundle is returned by ParseConfig when the bundle name passed to
// fbx gen is not one of the bundles built into this version.
var ErrUnknownBundle = errors.New("gen: unknown bundle name")

// validVolumes is the exhaustive set of recognised volume tiers for v0.1.
var validVolumes = map[string]bool{
	"small":  true,
	"medium": true,
	"large":  true,
}

// validBundles is the exhaustive set of recognised bundle names for v0.1.
var validBundles = map[string]bool{
	"monthly-close":     true,
	"recon-stress-test": true,
}

// Config holds all parameters needed to reproduce a deterministic generation
// run. Every field is set from CLI flags parsed in cmd/fbx; no field derives
// from wall-clock time or any other non-deterministic source.
type Config struct {
	// Seed is the RNG seed. Same seed + other fields → byte-identical output.
	Seed int64

	// Period is the first instant of the target month in UTC (YYYY-MM → time.Time).
	Period time.Time

	// Volume is the size tier: small, medium, or large.
	Volume string

	// Bundle is the named event mix: monthly-close or recon-stress-test.
	Bundle string

	// OutDir is the filesystem path where output files are written.
	OutDir string

	// Format is the output format: csv or jsonl.
	Format string
}

// ParseConfig validates raw flag values and returns a populated Config.
// It returns ErrBadPeriod, ErrUnknownVolume, or ErrUnknownBundle for the
// corresponding invalid input; other errors are wrapped with context.
func ParseConfig(seed int64, period, volume, bundle, outDir, format string) (Config, error) {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s", ErrBadPeriod, period)
	}
	t = t.UTC()

	if !validVolumes[volume] {
		return Config{}, fmt.Errorf("%w: %s", ErrUnknownVolume, volume)
	}

	if !validBundles[bundle] {
		return Config{}, fmt.Errorf("%w: %s", ErrUnknownBundle, bundle)
	}

	return Config{
		Seed:   seed,
		Period: t,
		Volume: volume,
		Bundle: bundle,
		OutDir: outDir,
		Format: format,
	}, nil
}
