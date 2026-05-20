package writer_test

// BINARY DETERMINISM NOTE: The pre-implementation spike confirmed that gofpdf
// produces byte-identical output when SetCreationDate and SetModificationDate
// are both called with the same time.Time. The PDF trailer does not include a
// time-derived /ID array (that only appears when encryption is enabled, which
// we never use). Standard PDF fonts (Helvetica) are specification-defined and
// require no OS-level font data. Accordingly, this file uses byte-identical
// comparison for both the determinism test and the golden-fixture test.
// If a future gofpdf upgrade introduces wall-clock timestamps, switch the
// determinism test to structural (extract text / page count) and document the
// regression here.

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/report"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// updatePDF regenerates the PDF golden fixtures when set to true.
// Run with: go test ./internal/writer/... -run TestPDFGolden -update-pdf
var updatePDF = flag.Bool("update-pdf", false, "regenerate PDF golden fixtures")

// pdfCanonicalCfg is the config that matches the existing CSV/XLSX golden fixtures.
var pdfCanonicalCfg = gen.Config{
	Seed:    1,
	Period:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume:  "small",
	Bundle:  "monthly-close",
	Format:  "pdf",
	Version: "v0.1.0",
}

// pdfStressCfg is the recon-stress-test variant.
var pdfStressCfg = gen.Config{
	Seed:    1,
	Period:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume:  "small",
	Bundle:  "recon-stress-test",
	Format:  "pdf",
	Version: "v0.1.0",
}

const pdfGoldenDir = "../../testdata/golden/monthly-close/2026-01/small/pdf"
const pdfStressGoldenDir = "../../testdata/golden/recon-stress-test/2026-01/small/pdf"

// pdfOutputFiles are the eight files WritePDF must produce.
var pdfOutputFiles = []string{
	"profit-loss.pdf",
	"balance-sheet.pdf",
	"cash-flow.pdf",
	"trial-balance.pdf",
	"ar-aging.pdf",
	"ap-aging.pdf",
	"customer-statements.pdf",
	"check-register.pdf",
}

// runPDFPipeline runs the full generation pipeline and calls WritePDF into outDir.
func runPDFPipeline(t *testing.T, cfg gen.Config, outDir string) {
	t.Helper()
	ds, err := gen.Run(cfg)
	if err != nil {
		t.Fatalf("gen.Run: %v", err)
	}
	if ds.Truth == nil {
		if err := invariant.Check(ds); err != nil {
			t.Fatalf("invariant.Check: %v", err)
		}
	}
	rpts := report.Compute(ds)
	periodEnd := cfg.Period.AddDate(0, 1, 0).Add(-time.Nanosecond)
	rpts.Aging = report.ComputeAging(ds, periodEnd)
	rpts.CustomerStatements = report.ComputeStatements(ds)
	rpts.CheckRegister = report.ComputeCheckRegister(ds)

	if err := writer.WritePDF(rpts, cfg, writer.DefaultCompanyConfig(), outDir); err != nil {
		t.Fatalf("WritePDF: %v", err)
	}
}

// TestWritePDFDeterminism calls WritePDF twice with identical inputs and asserts
// that all eight output files are byte-identical across the two calls.
func TestWritePDFDeterminism(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	runPDFPipeline(t, pdfCanonicalCfg, dir1)
	runPDFPipeline(t, pdfCanonicalCfg, dir2)

	for _, name := range pdfOutputFiles {
		b1, err := os.ReadFile(filepath.Join(dir1, name))
		if err != nil {
			t.Fatalf("read run1/%s: %v", name, err)
		}
		b2, err := os.ReadFile(filepath.Join(dir2, name))
		if err != nil {
			t.Fatalf("read run2/%s: %v", name, err)
		}
		if !bytes.Equal(b1, b2) {
			t.Errorf("determinism violation: %s differs between two runs (len %d vs %d)", name, len(b1), len(b2))
		}
	}
}

// TestPDFGolden compares WritePDF output against committed golden fixtures for
// both the monthly-close and recon-stress-test bundles. Run with -update-pdf to
// regenerate.
func TestPDFGolden(t *testing.T) {
	cases := []struct {
		name      string
		cfg       gen.Config
		goldenDir string
	}{
		{"monthly-close", pdfCanonicalCfg, pdfGoldenDir},
		{"recon-stress-test", pdfStressCfg, pdfStressGoldenDir},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := t.TempDir()
			runPDFPipeline(t, tc.cfg, outDir)

			if *updatePDF {
				if err := os.MkdirAll(tc.goldenDir, 0755); err != nil {
					t.Fatalf("mkdir golden: %v", err)
				}
				for _, name := range pdfOutputFiles {
					src := filepath.Join(outDir, name)
					dst := filepath.Join(tc.goldenDir, name)
					data, err := os.ReadFile(src)
					if err != nil {
						t.Fatalf("read %s: %v", src, err)
					}
					if err := os.WriteFile(dst, data, 0644); err != nil {
						t.Fatalf("write golden %s: %v", dst, err)
					}
				}
				t.Logf("golden fixtures updated in %s", tc.goldenDir)
				return
			}

			for _, name := range pdfOutputFiles {
				got, err := os.ReadFile(filepath.Join(outDir, name))
				if err != nil {
					t.Fatalf("read output %s: %v", name, err)
				}
				want, err := os.ReadFile(filepath.Join(tc.goldenDir, name))
				if err != nil {
					t.Fatalf("read golden %s (run with -update-pdf to regenerate): %v", name, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("golden mismatch for %s (got %d bytes, want %d bytes)", name, len(got), len(want))
				}
			}
		})
	}
}

// TestWritePDFProducesAllFiles asserts that WritePDF creates exactly the 8 expected files.
func TestWritePDFProducesAllFiles(t *testing.T) {
	outDir := t.TempDir()
	runPDFPipeline(t, pdfCanonicalCfg, outDir)

	for _, name := range pdfOutputFiles {
		path := filepath.Join(outDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file %s is empty", name)
		}
	}
}
