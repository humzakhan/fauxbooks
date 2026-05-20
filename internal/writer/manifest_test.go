package writer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// TestManifestHashStability verifies that writing the same Dataset twice
// produces byte-identical files and thus identical SHA-256 hashes.
// This is the prime determinism assertion for the writer package.
func TestManifestHashStability(t *testing.T) {
	ds, cfg := buildTestDataset(t)

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	if err := Write(ds, cfg, dir1); err != nil {
		t.Fatalf("Write dir1: %v", err)
	}
	if err := Write(ds, cfg, dir2); err != nil {
		t.Fatalf("Write dir2: %v", err)
	}

	// Read manifest from each directory and compare file hashes.
	m1 := readManifestJSON(t, dir1)
	m2 := readManifestJSON(t, dir2)

	if len(m1.FileSHA256) != len(m2.FileSHA256) {
		t.Fatalf("file count differs: dir1=%d dir2=%d", len(m1.FileSHA256), len(m2.FileSHA256))
	}

	// Verify hash-by-hash match and also that file bytes are identical.
	for i := range m1.FileSHA256 {
		fh1 := m1.FileSHA256[i]
		fh2 := m2.FileSHA256[i]
		if fh1.Name != fh2.Name {
			t.Errorf("file[%d] name mismatch: %q vs %q", i, fh1.Name, fh2.Name)
			continue
		}
		if fh1.SHA256 != fh2.SHA256 {
			t.Errorf("file %q: SHA-256 mismatch\n  run1: %s\n  run2: %s", fh1.Name, fh1.SHA256, fh2.SHA256)
		}

		// Also verify the hash matches the actual file content.
		data1, err := os.ReadFile(filepath.Join(dir1, fh1.Name))
		if err != nil {
			t.Fatalf("read %s from dir1: %v", fh1.Name, err)
		}
		h := sha256.Sum256(data1)
		wantHash := hex.EncodeToString(h[:])
		if fh1.SHA256 != wantHash {
			t.Errorf("file %q: manifest hash %s != computed hash %s", fh1.Name, fh1.SHA256, wantHash)
		}
	}
}

// TestManifestSchema verifies the manifest.json has the expected top-level fields.
func TestManifestSchema(t *testing.T) {
	ds, cfg := buildTestDataset(t)
	dir := t.TempDir()

	if err := Write(ds, cfg, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	m := readManifestJSON(t, dir)

	if m.Seed != cfg.Seed {
		t.Errorf("seed = %d, want %d", m.Seed, cfg.Seed)
	}
	if m.Period != cfg.Period.Format("2006-01") {
		t.Errorf("period = %q, want %q", m.Period, cfg.Period.Format("2006-01"))
	}
	if m.Bundle != cfg.Bundle {
		t.Errorf("bundle = %q, want %q", m.Bundle, cfg.Bundle)
	}
	if m.Volume != cfg.Volume {
		t.Errorf("volume = %q, want %q", m.Volume, cfg.Volume)
	}
	if len(m.RowCounts) == 0 {
		t.Error("row_counts is empty")
	}
	if len(m.FileSHA256) == 0 {
		t.Error("file_sha256 is empty")
	}

	// Verify no generated_at field (determinism requirement).
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}
	if _, ok := generic["generated_at"]; ok {
		t.Error("manifest.json contains generated_at — this breaks determinism")
	}
}

// TestManifestFileSHA256SortedByName verifies that file_sha256 entries in
// manifest.json are sorted by file name for deterministic output.
func TestManifestFileSHA256SortedByName(t *testing.T) {
	ds, cfg := buildTestDataset(t)
	dir := t.TempDir()

	if err := Write(ds, cfg, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	m := readManifestJSON(t, dir)
	for i := 1; i < len(m.FileSHA256); i++ {
		if m.FileSHA256[i].Name < m.FileSHA256[i-1].Name {
			t.Errorf("file_sha256 not sorted: %q < %q at index %d", m.FileSHA256[i].Name, m.FileSHA256[i-1].Name, i)
		}
	}
}

// manifestJSONParsed is used to parse manifest.json in tests.
type manifestJSONParsed struct {
	Seed       int64          `json:"seed"`
	Period     string         `json:"period"`
	Bundle     string         `json:"bundle"`
	Volume     string         `json:"volume"`
	Format     string         `json:"format"`
	RowCounts  []fileRowCount `json:"row_counts"`
	FileSHA256 []fileHash     `json:"file_sha256"`
}

// readManifestJSON reads and parses outDir/manifest.json.
func readManifestJSON(t *testing.T, outDir string) manifestJSONParsed {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var m manifestJSONParsed
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest.json: %v", err)
	}
	return m
}

// buildTestDataset constructs a Dataset and Config for writer tests using
// gen.Run with seed=1, period=2026-01, volume=small.
func buildTestDataset(t *testing.T) (gen.Dataset, gen.Config) {
	t.Helper()
	period, err := time.Parse("2006-01", "2026-01")
	if err != nil {
		t.Fatalf("parse period: %v", err)
	}
	cfg := gen.Config{
		Seed:   1,
		Period: period.UTC(),
		Volume: "small",
		Bundle: "monthly-close",
		Format: "csv",
	}
	ds, err := gen.Run(cfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}
	return ds, cfg
}
