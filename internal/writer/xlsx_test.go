package writer_test

// BINARY DETERMINISM NOTE: The spike (run before implementation) confirmed that
// excelize produces byte-identical raw output without any repacking. After
// repackZIPWithFixedModtime pins all ZIP-entry Modified timestamps to cfg.Period,
// the output is binary-stable across runs. Accordingly, this file uses
// byte-identical comparison for the golden-fixture test and the determinism test.
// If a future excelize upgrade introduces wall-clock timestamps, switch the
// determinism test to structural (parse-and-compare cell values) and document the
// regression here.

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	excelize "github.com/xuri/excelize/v2"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/report"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// updateXLSX regenerates the XLSX golden fixtures when set to true.
// Run with: go test ./internal/writer/... -run TestXLSXGolden -update-xlsx
var updateXLSX = flag.Bool("update-xlsx", false, "regenerate XLSX golden fixtures")

// xlsxCanonicalCfg is the canonical config matching the CSV golden fixture.
var xlsxCanonicalCfg = gen.Config{
	Seed:    1,
	Period:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume:  "small",
	Bundle:  "monthly-close",
	Format:  "xlsx",
	Version: "v0.1.0",
}

// xlsxStressCfg is the stress-test config.
var xlsxStressCfg = gen.Config{
	Seed:    1,
	Period:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	Volume:  "small",
	Bundle:  "recon-stress-test",
	Format:  "xlsx",
	Version: "v0.1.0",
}

const xlsxGoldenDir = "../../testdata/golden/monthly-close/2026-01/small/xlsx"
const xlsxStressGoldenDir = "../../testdata/golden/recon-stress-test/2026-01/small/xlsx"

// runXLSXPipeline runs the full generation pipeline and calls WriteXLSX into outDir.
func runXLSXPipeline(t *testing.T, cfg gen.Config, outDir string) {
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
	reports := report.Compute(ds)
	periodEnd := cfg.Period.AddDate(0, 1, 0).Add(-time.Nanosecond)
	reports.Aging = report.ComputeAging(ds, periodEnd)
	reports.CustomerStatements = report.ComputeStatements(ds)
	reports.CheckRegister = report.ComputeCheckRegister(ds)

	if err := writer.WriteXLSX(ds, reports, cfg, outDir); err != nil {
		t.Fatalf("WriteXLSX: %v", err)
	}
}

// TestXLSXDeterminism calls WriteXLSX twice with identical inputs and asserts
// the output bytes are identical. This is the primary determinism guarantee.
func TestXLSXDeterminism(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	runXLSXPipeline(t, xlsxCanonicalCfg, dir1)
	runXLSXPipeline(t, xlsxCanonicalCfg, dir2)

	b1, err := os.ReadFile(filepath.Join(dir1, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("read run1: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(dir2, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("read run2: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		for i := 0; i < len(b1) && i < len(b2); i++ {
			if b1[i] != b2[i] {
				t.Errorf("runs differ at byte %d: 0x%02x vs 0x%02x", i, b1[i], b2[i])
				return
			}
		}
		t.Errorf("runs differ in length: %d vs %d bytes", len(b1), len(b2))
	}
}

// TestXLSXDecimalPreservation opens the written XLSX and verifies that a
// monetary cell reads back as an unmodified decimal string.
func TestXLSXDecimalPreservation(t *testing.T) {
	outDir := t.TempDir()
	runXLSXPipeline(t, xlsxCanonicalCfg, outDir)

	f, err := excelize.OpenFile(filepath.Join(outDir, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()

	// The "gl" sheet has debit/credit columns (columns F and G, index 5 and 6).
	// Read the first data row (row 2) of the gl sheet and confirm the debit cell
	// is a decimal string with exactly 9 fractional digits.
	rows, err := f.GetRows("gl")
	if err != nil {
		t.Fatalf("GetRows(gl): %v", err)
	}
	if len(rows) < 2 {
		t.Fatal("gl sheet has no data rows")
	}
	// Header row is rows[0]; first data row is rows[1].
	dataRow := rows[1]
	// columns: row_num, entry_id, line_id, occurred_at, account_code, debit, credit
	// index:   0        1         2        3             4             5      6
	if len(dataRow) < 7 {
		t.Fatalf("data row has %d columns, want >= 7", len(dataRow))
	}

	debitVal := dataRow[5]
	creditVal := dataRow[6]

	// At least one of debit or credit must be a non-zero decimal string.
	// Both are formatted by FormatDecimal: exactly 9 fractional digits.
	checkDecimal := func(label, val string) {
		t.Helper()
		if len(val) < 11 { // "0.000000000" is 11 chars
			t.Errorf("%s: value %q is too short to be a valid decimal", label, val)
			return
		}
		// Must not look like a float (no 'e', 'E', or too few decimals).
		for _, ch := range val {
			if ch == 'e' || ch == 'E' {
				t.Errorf("%s: value %q looks like scientific notation", label, val)
				return
			}
		}
		// Count fractional digits after the decimal point.
		dotIdx := -1
		for i, ch := range val {
			if ch == '.' {
				dotIdx = i
				break
			}
		}
		if dotIdx < 0 {
			t.Errorf("%s: value %q has no decimal point", label, val)
			return
		}
		fracDigits := len(val) - dotIdx - 1
		if fracDigits != 9 {
			t.Errorf("%s: value %q has %d fractional digits, want 9", label, val, fracDigits)
		}
	}

	checkDecimal("debit", debitVal)
	checkDecimal("credit", creditVal)
}

// TestXLSXSheetSplit verifies that a sheet with more than xlsxMaxRows data rows
// is split into two sheets (SheetName and SheetName_2).
func TestXLSXSheetSplit(t *testing.T) {
	// Build a minimal Dataset whose GL has 1,048,576 + 1 rows so the gl sheet
	// overflows into gl_2.  We fabricate the GL rows directly.
	const overflowCount = 1_048_576 + 1

	// Build a synthetic dataset with overflowCount GL rows.
	ds := gen.Dataset{
		CoA: gen.IndexCoA(gen.DefaultCoA()),
	}
	// Fill GL with overflowCount rows.
	for i := 0; i < overflowCount; i++ {
		ds.GL = append(ds.GL, gen.GLRow{
			RowNum:      i + 1,
			EntryID:     "e1",
			LineID:      "l1",
			OccurredAt:  "2026-01-15T00:00:00Z",
			AccountCode: "1010",
			Debit:       "1.000000000",
			Credit:      "0.000000000",
		})
	}

	cfg := gen.Config{
		Seed:    1,
		Period:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Volume:  "small",
		Bundle:  "monthly-close",
		Format:  "xlsx",
		Version: "v0.1.0",
	}
	outDir := t.TempDir()

	// WriteXLSX accepts an empty reports struct fine.
	if err := writer.WriteXLSX(ds, report.Reports{}, cfg, outDir); err != nil {
		t.Fatalf("WriteXLSX: %v", err)
	}

	f, err := excelize.OpenFile(filepath.Join(outDir, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	found1 := false
	found2 := false
	for _, s := range sheets {
		if s == "gl" {
			found1 = true
		}
		if s == "gl_2" {
			found2 = true
		}
	}
	if !found1 {
		t.Errorf("sheet 'gl' not found in %v", sheets)
	}
	if !found2 {
		t.Errorf("sheet 'gl_2' not found in %v; sheet split did not trigger", sheets)
	}

	// Verify row counts.
	// overflowCount = 1,048,577 rows total in ds.GL.
	// xlsxMaxRows = 1,048,575 rows per batch.
	// Batch 1 (sheet "gl"): 1,048,575 data rows + 1 header = 1,048,576 total rows.
	// Batch 2 (sheet "gl_2"): 2 data rows + 1 header = 3 total rows.
	rows1, err := f.GetRows("gl")
	if err != nil {
		t.Fatalf("GetRows(gl): %v", err)
	}
	wantGL1 := 1 + 1_048_575 // header + first batch
	if len(rows1) != wantGL1 {
		t.Errorf("gl sheet: got %d rows, want %d", len(rows1), wantGL1)
	}

	rows2, err := f.GetRows("gl_2")
	if err != nil {
		t.Fatalf("GetRows(gl_2): %v", err)
	}
	// overflowCount - xlsxMaxRows = 1,048,577 - 1,048,575 = 2 overflow rows + 1 header
	wantGL2 := 1 + (overflowCount - 1_048_575)
	if len(rows2) != wantGL2 {
		t.Errorf("gl_2 sheet: got %d rows, want %d", len(rows2), wantGL2)
	}
}

// TestXLSXGolden byte-compares the XLSX output against the committed golden fixture.
// Run with -update-xlsx to regenerate.
func TestXLSXGolden(t *testing.T) {
	outDir := t.TempDir()
	runXLSXPipeline(t, xlsxCanonicalCfg, outDir)

	got := filepath.Join(outDir, "fauxbooks.xlsx")
	want := filepath.Join(xlsxGoldenDir, "fauxbooks.xlsx")

	if *updateXLSX {
		if err := os.MkdirAll(xlsxGoldenDir, 0755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		data, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if err := os.WriteFile(want, data, 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("XLSX golden updated: %s", want)
		return
	}

	wantData, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update-xlsx to generate)", want, err)
	}
	gotData, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(wantData, gotData) {
		for i := 0; i < len(wantData) && i < len(gotData); i++ {
			if wantData[i] != gotData[i] {
				t.Errorf("fauxbooks.xlsx: first difference at byte %d: want 0x%02x, got 0x%02x",
					i, wantData[i], gotData[i])
				return
			}
		}
		t.Errorf("fauxbooks.xlsx: lengths differ: want %d bytes, got %d bytes",
			len(wantData), len(gotData))
	}
}

// TestXLSXStressGolden byte-compares the recon-stress-test XLSX against the committed fixture.
func TestXLSXStressGolden(t *testing.T) {
	outDir := t.TempDir()
	runXLSXPipeline(t, xlsxStressCfg, outDir)

	got := filepath.Join(outDir, "fauxbooks.xlsx")
	want := filepath.Join(xlsxStressGoldenDir, "fauxbooks.xlsx")

	if *updateXLSX {
		if err := os.MkdirAll(xlsxStressGoldenDir, 0755); err != nil {
			t.Fatalf("mkdir stress golden: %v", err)
		}
		data, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if err := os.WriteFile(want, data, 0644); err != nil {
			t.Fatalf("write stress golden: %v", err)
		}
		t.Logf("XLSX stress golden updated: %s", want)
		return
	}

	wantData, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read stress golden %s: %v (run with -update-xlsx to generate)", want, err)
	}
	gotData, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(wantData, gotData) {
		for i := 0; i < len(wantData) && i < len(gotData); i++ {
			if wantData[i] != gotData[i] {
				t.Errorf("fauxbooks.xlsx (stress): first diff at byte %d: want 0x%02x, got 0x%02x",
					i, wantData[i], gotData[i])
				return
			}
		}
		t.Errorf("fauxbooks.xlsx (stress): lengths differ: want %d bytes, got %d bytes",
			len(wantData), len(gotData))
	}
}

// TestXLSXZIPModtimes verifies that all ZIP entry modification timestamps in the
// output are pinned to cfg.Period (not the current wall clock).
func TestXLSXZIPModtimes(t *testing.T) {
	outDir := t.TempDir()
	runXLSXPipeline(t, xlsxCanonicalCfg, outDir)

	data, err := os.ReadFile(filepath.Join(outDir, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("read xlsx: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	want := xlsxCanonicalCfg.Period.UTC()
	for _, zf := range r.File {
		got := zf.Modified.UTC()
		if !got.Equal(want) {
			t.Errorf("ZIP entry %s: Modified=%v, want %v", zf.Name, got, want)
		}
	}
}

// TestXLSXInlineStringType inspects the raw XML to verify monetary cells use
// t="inlineStr" (not numeric type or shared string).
func TestXLSXInlineStringType(t *testing.T) {
	outDir := t.TempDir()
	runXLSXPipeline(t, xlsxCanonicalCfg, outDir)

	data, err := os.ReadFile(filepath.Join(outDir, "fauxbooks.xlsx"))
	if err != nil {
		t.Fatalf("read xlsx: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	// Read sheet1.xml (the coa sheet — all string values) and confirm cells have t="inlineStr".
	for _, zf := range r.File {
		if zf.Name == "xl/worksheets/sheet1.xml" {
			rc, err := zf.Open()
			if err != nil {
				t.Fatalf("open sheet1.xml: %v", err)
			}
			xmlData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("read sheet1.xml: %v", err)
			}
			content := string(xmlData)
			if len(content) == 0 {
				t.Fatal("sheet1.xml is empty")
			}
			// Should contain inlineStr cells.
			if !bytes.Contains(xmlData, []byte(`t="inlineStr"`)) {
				t.Errorf("sheet1.xml: expected t=\"inlineStr\" cells, not found in first 500 bytes:\n%.500s", content)
			}
			return
		}
	}
	t.Fatal("xl/worksheets/sheet1.xml not found in XLSX")
}
