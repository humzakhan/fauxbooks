package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	excelize "github.com/xuri/excelize/v2"
)

// TestConvertCSVToJSONL verifies that a decimal string in a CSV column survives
// CSV → JSONL conversion as a JSON string (not a JSON number).
func TestConvertCSVToJSONL(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	csvPath := filepath.Join(dir, "input.csv")
	jsonlPath := filepath.Join(dir, "output.jsonl")

	// Write a CSV with a decimal column.
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"name", "amount"})
	_ = w.Write([]string{"Alice", "123.456000000"})
	w.Flush()
	f.Close()

	code := runConvert([]string{"--in", csvPath, "--out", jsonlPath})
	if code != 0 {
		t.Fatalf("runConvert exited %d", code)
	}

	// Read the JSONL output.
	jf, err := os.Open(jsonlPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer jf.Close()

	sc := bufio.NewScanner(jf)
	if !sc.Scan() {
		t.Fatal("expected at least one JSONL line")
	}

	// Parse using RawMessage to detect whether the value is a string or number.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(sc.Bytes(), &obj); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	raw, ok := obj["amount"]
	if !ok {
		t.Fatal("missing 'amount' key in JSONL")
	}
	// The raw value must be a JSON string (starts with '"'), not a number.
	if len(raw) == 0 || raw[0] != '"' {
		t.Errorf("amount raw JSON = %s; want a JSON string (starts with '\"')", raw)
	}
	var amtStr string
	if err := json.Unmarshal(raw, &amtStr); err != nil {
		t.Fatalf("json.Unmarshal amount: %v", err)
	}
	if amtStr != "123.456000000" {
		t.Errorf("amount = %q; want %q", amtStr, "123.456000000")
	}
}

// TestConvertJSONLToCSV verifies that a decimal string in JSONL survives
// JSONL → CSV conversion as the original string.
func TestConvertJSONLToCSV(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	jsonlPath := filepath.Join(dir, "input.jsonl")
	csvPath := filepath.Join(dir, "output.csv")

	if err := os.WriteFile(jsonlPath, []byte(`{"x":"123.456000000"}`+"\n"), 0644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	code := runConvert([]string{"--in", jsonlPath, "--out", csvPath})
	if code != 0 {
		t.Fatalf("runConvert exited %d", code)
	}

	// Read the CSV output.
	cf, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer cf.Close()

	r := csv.NewReader(cf)
	headers, err := r.Read()
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	row, err := r.Read()
	if err != nil {
		t.Fatalf("read row: %v", err)
	}

	xIdx := -1
	for i, h := range headers {
		if h == "x" {
			xIdx = i
			break
		}
	}
	if xIdx < 0 {
		t.Fatal("column 'x' not found in output CSV")
	}
	if row[xIdx] != "123.456000000" {
		t.Errorf("x = %q; want %q", row[xIdx], "123.456000000")
	}
}

// TestConvertRoundTripCSVXLSX verifies that a CSV round-trips through XLSX
// without any data loss or decimal corruption.
func TestConvertRoundTripCSVXLSX(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	csvIn := filepath.Join(dir, "original.csv")
	xlsxPath := filepath.Join(dir, "middle.xlsx")
	csvOut := filepath.Join(dir, "final.csv")

	// Write original CSV.
	f, err := os.Create(csvIn)
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	cw := csv.NewWriter(f)
	_ = cw.Write([]string{"id", "amount"})
	_ = cw.Write([]string{"1", "100.000000001"})
	_ = cw.Write([]string{"2", "999.999999999"})
	cw.Flush()
	f.Close()

	// CSV → XLSX.
	if code := runConvert([]string{"--in", csvIn, "--out", xlsxPath}); code != 0 {
		t.Fatalf("csv→xlsx: runConvert exited %d", code)
	}

	// XLSX → CSV.
	if code := runConvert([]string{"--in", xlsxPath, "--out", csvOut}); code != 0 {
		t.Fatalf("xlsx→csv: runConvert exited %d", code)
	}

	// Compare original and final CSV byte-for-byte.
	orig, err := os.ReadFile(csvIn)
	if err != nil {
		t.Fatalf("read original csv: %v", err)
	}
	final, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("read final csv: %v", err)
	}
	if string(orig) != string(final) {
		t.Errorf("round-trip CSV mismatch:\noriginal:\n%s\nfinal:\n%s", orig, final)
	}
}

// TestConvertXLSXToJSONL writes an XLSX with a string cell containing a decimal,
// converts to JSONL, and asserts the value appears as a JSON string.
func TestConvertXLSXToJSONL(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	xlsxPath := filepath.Join(dir, "input.xlsx")
	jsonlPath := filepath.Join(dir, "output.jsonl")

	// Build a minimal XLSX with one string cell.
	f := excelize.NewFile()
	defer f.Close()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatalf("NewStreamWriter: %v", err)
	}
	_ = sw.SetRow("A1", []interface{}{excelize.Cell{Value: "amount"}})
	_ = sw.SetRow("A2", []interface{}{excelize.Cell{Value: "100.000000000"}})
	if err := sw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	code := runConvert([]string{"--in", xlsxPath, "--out", jsonlPath})
	if code != 0 {
		t.Fatalf("runConvert exited %d", code)
	}

	jf, err := os.Open(jsonlPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer jf.Close()

	sc := bufio.NewScanner(jf)
	if !sc.Scan() {
		t.Fatal("expected at least one JSONL line")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(sc.Bytes(), &obj); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	raw, ok := obj["amount"]
	if !ok {
		t.Fatal("missing 'amount' key in JSONL output")
	}
	if len(raw) == 0 || raw[0] != '"' {
		t.Errorf("amount raw JSON = %s; want a JSON string", raw)
	}
	var amtStr string
	if err := json.Unmarshal(raw, &amtStr); err != nil {
		t.Fatalf("json.Unmarshal amount: %v", err)
	}
	if amtStr != "100.000000000" {
		t.Errorf("amount = %q; want %q", amtStr, "100.000000000")
	}
}

// TestConvertJSONLToXLSXKeyOrder verifies that JSONL → XLSX produces columns in
// sorted alphabetical order, regardless of key insertion order in the JSON objects.
func TestConvertJSONLToXLSXKeyOrder(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	jsonlPath := filepath.Join(dir, "input.jsonl")
	xlsxPath := filepath.Join(dir, "output.xlsx")

	// Two lines with keys in non-alphabetical order: b before a.
	lines := `{"b":"1","a":"2"}` + "\n" + `{"a":"3","b":"4"}` + "\n"
	if err := os.WriteFile(jsonlPath, []byte(lines), 0644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	code := runConvert([]string{"--in", jsonlPath, "--out", xlsxPath})
	if code != 0 {
		t.Fatalf("runConvert exited %d", code)
	}

	// Open the XLSX and check column order.
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(rows))
	}

	// Row 0 is the header; columns must be sorted: a, b.
	header := rows[0]
	if len(header) < 2 || header[0] != "a" || header[1] != "b" {
		t.Errorf("header = %v; want [a b]", header)
	}

	// Row 1 should be: a=2, b=1 (first JSONL line, reordered to match sorted header).
	row1 := rows[1]
	if len(row1) < 2 || row1[0] != "2" || row1[1] != "1" {
		t.Errorf("row1 = %v; want [2 1]", row1)
	}

	// Row 2 should be: a=3, b=4.
	row2 := rows[2]
	if len(row2) < 2 || row2[0] != "3" || row2[1] != "4" {
		t.Errorf("row2 = %v; want [3 4]", row2)
	}
}

// TestConvertUnsupportedExtension verifies that an unsupported source extension
// causes runConvert to exit 1.
func TestConvertUnsupportedExtension(t *testing.T) {
	code := runConvert([]string{"--in", "foo.pdf", "--out", "bar.csv"})
	if code != 1 {
		t.Errorf("expected exit 1 for unsupported extension, got %d", code)
	}
}

// TestConvertMissingFlags verifies that missing --in or --out causes runConvert
// to exit 1 with a usage message.
func TestConvertMissingFlags(t *testing.T) {
	// Missing both.
	if code := runConvert([]string{}); code != 1 {
		t.Errorf("no flags: expected exit 1, got %d", code)
	}
	// Missing --out.
	if code := runConvert([]string{"--in", "file.csv"}); code != 1 {
		t.Errorf("missing --out: expected exit 1, got %d", code)
	}
	// Missing --in.
	if code := runConvert([]string{"--out", "file.jsonl"}); code != 1 {
		t.Errorf("missing --in: expected exit 1, got %d", code)
	}
}

// TestConvertNoOp verifies that same-format conversion (e.g. csv → csv) exits 1.
func TestConvertNoOp(t *testing.T) {
	code := runConvert([]string{"--in", "a.csv", "--out", "b.csv"})
	if code != 1 {
		t.Errorf("expected exit 1 for no-op conversion, got %d", code)
	}
}
