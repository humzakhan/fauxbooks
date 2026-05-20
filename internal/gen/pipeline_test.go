package gen_test

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// update, when true, regenerates golden fixtures from the current Run output.
// Run with: go test ./internal/gen/... -run TestGolden -update
var update = flag.Bool("update", false, "regenerate golden fixtures")

// canonicalCfg is the fixed configuration used for both golden and determinism tests.
var canonicalCfg = gen.Config{
	Seed:   1,
	Period: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume: "small",
	Bundle: "monthly-close",
	Format: "csv",
}

// goldenDir is the committed fixture directory for the canonical config.
const goldenDir = "../../testdata/golden/monthly-close/2026-01/small"

// runPipeline runs gen.Run + invariant.Check + writer.Write into outDir.
// Fails the test immediately on any error.
func runPipeline(t *testing.T, cfg gen.Config, outDir string) {
	t.Helper()

	ds, err := gen.Run(cfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}

	if err := invariant.Check(ds); err != nil {
		t.Fatalf("invariant.Check: %v", err)
	}

	if err := writer.Write(ds, cfg, outDir); err != nil {
		t.Fatalf("writer.Write: %v", err)
	}
}

// requireEqualFiles reads both files and byte-compares them.
// On mismatch it reports the first differing byte position.
func requireEqualFiles(t *testing.T, wantPath, gotPath string) {
	t.Helper()

	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", wantPath, err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read output %s: %v", gotPath, err)
	}

	if !bytes.Equal(want, got) {
		// Find the first differing byte for a useful error message.
		for i := 0; i < len(want) && i < len(got); i++ {
			if want[i] != got[i] {
				t.Errorf("file %s: first difference at byte %d: want 0x%02x, got 0x%02x",
					filepath.Base(wantPath), i, want[i], got[i])
				return
			}
		}
		t.Errorf("file %s: lengths differ: want %d bytes, got %d bytes",
			filepath.Base(wantPath), len(want), len(got))
	}
}

// TestGolden byte-compares the full pipeline output against the committed
// golden fixture in testdata/golden/monthly-close/2026-01/small/.
// Run with -update to regenerate the fixture.
func TestGolden(t *testing.T) {
	outDir := t.TempDir()
	runPipeline(t, canonicalCfg, outDir)

	if *update {
		// Regenerate: copy every output file into the golden directory.
		if err := os.MkdirAll(goldenDir, 0755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		entries, err := os.ReadDir(outDir)
		if err != nil {
			t.Fatalf("readdir outDir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			src := filepath.Join(outDir, e.Name())
			dst := filepath.Join(goldenDir, e.Name())
			data, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("read %s: %v", src, err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				t.Fatalf("write golden %s: %v", dst, err)
			}
		}
		t.Logf("golden fixtures updated in %s", goldenDir)
		return
	}

	// Compare against committed fixtures.
	fixtures, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read golden dir %s: %v (run with -update to generate)", goldenDir, err)
	}

	for _, f := range fixtures {
		if f.IsDir() {
			continue
		}
		wantPath := filepath.Join(goldenDir, f.Name())
		gotPath := filepath.Join(outDir, f.Name())
		if _, err := os.Stat(gotPath); os.IsNotExist(err) {
			t.Errorf("output missing file %s", f.Name())
			continue
		}
		requireEqualFiles(t, wantPath, gotPath)
	}

	// Also verify every output file has a corresponding fixture (no extras).
	outputs, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("readdir outDir: %v", err)
	}
	fixtureNames := make(map[string]bool, len(fixtures))
	for _, f := range fixtures {
		fixtureNames[f.Name()] = true
	}
	for _, o := range outputs {
		if !o.IsDir() && !fixtureNames[o.Name()] {
			t.Errorf("output produced unexpected file %s (not in golden fixture)", o.Name())
		}
	}
}

// TestDeterminism runs gen.Run + writer.Write twice with the same config
// and byte-compares every output file. Two runs must produce identical bytes.
func TestDeterminism(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	runPipeline(t, canonicalCfg, dir1)
	runPipeline(t, canonicalCfg, dir2)

	entries, err := os.ReadDir(dir1)
	if err != nil {
		t.Fatalf("readdir dir1: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p1 := filepath.Join(dir1, e.Name())
		p2 := filepath.Join(dir2, e.Name())
		requireEqualFiles(t, p1, p2)
	}
}

// TestInvariantAbort verifies that a deliberately broken Dataset causes
// invariant.Check to return a gen.InvariantError, and that no output files
// are written when the pipeline aborts before writer.Write.
func TestInvariantAbort(t *testing.T) {
	// Build a valid dataset, then corrupt one journal line to break invariant 1.
	ds, err := gen.Run(canonicalCfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}

	// Corrupt: append a debit-only line to the first entry so Σ debits ≠ Σ credits.
	broken := ds
	lines := make([]gen.JournalLine, len(ds.Lines))
	copy(lines, ds.Lines)

	firstEntryID := ds.Entries[0].EntryID
	firstAccountCode := ds.Lines[0].AccountCode
	lines = append(lines, gen.JournalLine{
		LineID:      "corrupt-line",
		EntryID:     firstEntryID,
		AccountCode: firstAccountCode,
		Debit:       decimal.NewFromInt(1),
		// Credit is zero — this unbalances the entry.
	})
	broken.Lines = lines

	checkErr := invariant.Check(broken)
	if checkErr == nil {
		t.Fatal("invariant.Check on broken Dataset: expected error, got nil")
	}

	var ie gen.InvariantError
	if !errors.As(checkErr, &ie) {
		t.Fatalf("expected gen.InvariantError, got %T: %v", checkErr, checkErr)
	}
	if ie.Invariant != 1 {
		t.Errorf("InvariantError.Invariant = %d; want 1", ie.Invariant)
	}

	// Confirm no files were written to the output dir (since we never called writer.Write).
	outDir := t.TempDir()
	dirEntries, err2 := os.ReadDir(outDir)
	if err2 != nil {
		t.Fatalf("readdir: %v", err2)
	}
	if len(dirEntries) != 0 {
		t.Errorf("expected empty output dir after invariant abort, got %d files", len(dirEntries))
	}
}
