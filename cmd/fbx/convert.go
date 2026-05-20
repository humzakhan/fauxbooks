// convert.go implements the "fbx convert --in <path> --out <path>" subcommand.
//
// Supported conversion pairs:
//
//	CSV  → XLSX
//	CSV  → JSONL
//	XLSX → CSV
//	XLSX → JSONL
//	JSONL → CSV
//	JSONL → XLSX
//
// Format is detected from file extension (.csv, .xlsx, .jsonl).
// All conversions are streaming: O(1) memory per row.
// Decimal strings are never parsed through float64.
//
// XLSX output timestamps are NOT pinned (no golden fixture for convert output);
// use fbx gen --format xlsx if byte-deterministic XLSX is required.
//
// Exit codes:
//
//	0  success
//	1  configuration error (missing flags, unsupported extension, no-op conversion)
//	2  I/O error during conversion
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	excelize "github.com/xuri/excelize/v2"
)

// runConvert is the entry point for the "fbx convert" subcommand.
// args should be the arguments after "convert".
func runConvert(args []string) int {
	fs := flag.NewFlagSet("fbx convert", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		inPath  = fs.String("in", "", "source file path (required)")
		outPath = fs.String("out", "", "destination file path (required)")
	)

	if err := fs.Parse(args); err != nil {
		// fs already wrote to stderr.
		return 1
	}

	if *inPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: fbx convert --in <path> --out <path>")
		return 1
	}

	srcExt := strings.ToLower(filepath.Ext(*inPath))
	dstExt := strings.ToLower(filepath.Ext(*outPath))

	validExts := map[string]bool{".csv": true, ".xlsx": true, ".jsonl": true}
	if !validExts[srcExt] {
		fmt.Fprintf(os.Stderr, "fbx convert: unsupported source extension %q (must be .csv, .xlsx, or .jsonl)\n", srcExt)
		return 1
	}
	if !validExts[dstExt] {
		fmt.Fprintf(os.Stderr, "fbx convert: unsupported destination extension %q (must be .csv, .xlsx, or .jsonl)\n", dstExt)
		return 1
	}
	if srcExt == dstExt {
		fmt.Fprintf(os.Stderr, "fbx convert: no-op conversion (%s → %s); source and destination formats are identical\n", srcExt, dstExt)
		return 1
	}

	var err error
	switch srcExt + "→" + dstExt {
	case ".csv→.jsonl":
		err = convertCSVToJSONL(*inPath, *outPath)
	case ".csv→.xlsx":
		err = convertCSVToXLSX(*inPath, *outPath)
	case ".jsonl→.csv":
		err = convertJSONLToCSV(*inPath, *outPath)
	case ".jsonl→.xlsx":
		err = convertJSONLToXLSX(*inPath, *outPath)
	case ".xlsx→.csv":
		err = convertXLSXToCSV(*inPath, *outPath)
	case ".xlsx→.jsonl":
		err = convertXLSXToJSONL(*inPath, *outPath)
	default:
		fmt.Fprintf(os.Stderr, "fbx convert: unsupported pair %s → %s\n", srcExt, dstExt)
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "fbx convert: %v\n", err)
		return 2
	}
	return 0
}

// convertCSVToJSONL streams rows from a CSV file and writes one JSON object per
// line. Column names come from the header row. All values are preserved as JSON
// strings (no float64 coercion).
func convertCSVToJSONL(in, out string) error {
	inf, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("convert csv→jsonl: open input: %w", err)
	}
	defer inf.Close()

	outf, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("convert csv→jsonl: create output: %w", err)
	}
	defer outf.Close()

	bw := bufio.NewWriterSize(outf, 1<<16)
	defer bw.Flush()

	r := csv.NewReader(inf)
	headers, err := r.Read()
	if err != nil {
		return fmt.Errorf("convert csv→jsonl: read header: %w", err)
	}

	enc := json.NewEncoder(bw)
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("convert csv→jsonl: read row: %w", err)
		}
		// Build map[string]string so all values remain JSON strings (no float coercion).
		obj := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				obj[h] = row[i]
			} else {
				obj[h] = ""
			}
		}
		// json.Marshal on map[string]string produces sorted keys (Go sorts map keys
		// for JSON encoding), which is deterministic.
		if err := enc.Encode(obj); err != nil {
			return fmt.Errorf("convert csv→jsonl: encode row: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("convert csv→jsonl: flush: %w", err)
	}
	return nil
}

// convertCSVToXLSX streams rows from a CSV file into an XLSX workbook with a
// single sheet named "Sheet1". All cells are written as string values to
// preserve decimal precision. Timestamps are NOT pinned (convert output is not
// subject to the byte-determinism requirement).
func convertCSVToXLSX(in, out string) error {
	inf, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("convert csv→xlsx: open input: %w", err)
	}
	defer inf.Close()

	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return fmt.Errorf("convert csv→xlsx: NewStreamWriter: %w", err)
	}

	r := csv.NewReader(inf)
	rowIdx := 1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("convert csv→xlsx: read row %d: %w", rowIdx, err)
		}
		cells := make([]interface{}, len(record))
		for i, v := range record {
			cells[i] = excelize.Cell{Value: v}
		}
		cellRef := "A" + strconv.Itoa(rowIdx)
		if err := sw.SetRow(cellRef, cells); err != nil {
			return fmt.Errorf("convert csv→xlsx: SetRow %d: %w", rowIdx, err)
		}
		rowIdx++
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("convert csv→xlsx: Flush: %w", err)
	}
	if err := f.SaveAs(out); err != nil {
		return fmt.Errorf("convert csv→xlsx: SaveAs: %w", err)
	}
	return nil
}

// convertJSONLToCSV streams JSONL lines, extracts column names from the first
// line (sorted for stability), and writes a CSV with one row per JSON object.
// Values are extracted as raw JSON strings and unquoted for string fields.
// Non-string JSON values (numbers, booleans, nulls) are written as their JSON
// text representation. Decimal strings survive unchanged (no float64 parse).
func convertJSONLToCSV(in, out string) error {
	inf, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("convert jsonl→csv: open input: %w", err)
	}
	defer inf.Close()

	outf, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("convert jsonl→csv: create output: %w", err)
	}
	defer outf.Close()

	bw := bufio.NewWriterSize(outf, 1<<16)
	cw := csv.NewWriter(bw)
	defer func() {
		cw.Flush()
		bw.Flush()
	}()

	sc := bufio.NewScanner(inf)
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	var headers []string
	lineNum := 0

	for sc.Scan() {
		lineNum++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse as map[string]json.RawMessage to avoid float64 coercion.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			return fmt.Errorf("convert jsonl→csv: parse line %d: %w", lineNum, err)
		}

		if lineNum == 1 {
			// Collect and sort keys from the first line for stable column order.
			headers = make([]string, 0, len(raw))
			for k := range raw {
				headers = append(headers, k)
			}
			sort.Strings(headers)
			if err := cw.Write(headers); err != nil {
				return fmt.Errorf("convert jsonl→csv: write header: %w", err)
			}
		}

		row := make([]string, len(headers))
		for i, h := range headers {
			v, ok := raw[h]
			if !ok {
				row[i] = ""
				continue
			}
			row[i] = rawJSONToString(v)
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("convert jsonl→csv: write row %d: %w", lineNum, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("convert jsonl→csv: scan: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("convert jsonl→csv: csv flush: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("convert jsonl→csv: flush: %w", err)
	}
	return nil
}

// convertJSONLToXLSX streams JSONL lines into an XLSX workbook with a single
// sheet. Column order is determined by sorting the keys of the first JSONL line.
// That fixed order is applied to all subsequent rows (missing keys → empty cell).
// No float64 parsing at any step.
func convertJSONLToXLSX(in, out string) error {
	inf, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("convert jsonl→xlsx: open input: %w", err)
	}
	defer inf.Close()

	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return fmt.Errorf("convert jsonl→xlsx: NewStreamWriter: %w", err)
	}

	sc := bufio.NewScanner(inf)
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	var headers []string
	xlsxRow := 1
	lineNum := 0

	for sc.Scan() {
		lineNum++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			return fmt.Errorf("convert jsonl→xlsx: parse line %d: %w", lineNum, err)
		}

		if lineNum == 1 {
			// Collect and sort keys from first line for stable column order.
			headers = make([]string, 0, len(raw))
			for k := range raw {
				headers = append(headers, k)
			}
			sort.Strings(headers)

			// Write header row.
			hCells := make([]interface{}, len(headers))
			for i, h := range headers {
				hCells[i] = excelize.Cell{Value: h}
			}
			if err := sw.SetRow("A1", hCells); err != nil {
				return fmt.Errorf("convert jsonl→xlsx: SetRow header: %w", err)
			}
			xlsxRow = 2
		}

		cells := make([]interface{}, len(headers))
		for i, h := range headers {
			v, ok := raw[h]
			if !ok {
				cells[i] = excelize.Cell{Value: ""}
				continue
			}
			cells[i] = excelize.Cell{Value: rawJSONToString(v)}
		}
		cellRef := "A" + strconv.Itoa(xlsxRow)
		if err := sw.SetRow(cellRef, cells); err != nil {
			return fmt.Errorf("convert jsonl→xlsx: SetRow %d: %w", xlsxRow, err)
		}
		xlsxRow++
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("convert jsonl→xlsx: scan: %w", err)
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("convert jsonl→xlsx: Flush: %w", err)
	}
	if err := f.SaveAs(out); err != nil {
		return fmt.Errorf("convert jsonl→xlsx: SaveAs: %w", err)
	}
	return nil
}

// convertXLSXToCSV reads the first sheet of an XLSX file and writes a CSV.
// Cells are returned as strings by excelize, preserving decimal text.
func convertXLSXToCSV(in, out string) error {
	xlsx, err := excelize.OpenFile(in)
	if err != nil {
		return fmt.Errorf("convert xlsx→csv: open input: %w", err)
	}
	defer xlsx.Close()

	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("convert xlsx→csv: workbook has no sheets")
	}
	// Only the first sheet is converted; multi-sheet workbooks are documented
	// as single-sheet only for convert (per plan scope).
	sheetName := sheets[0]

	rows, err := xlsx.Rows(sheetName)
	if err != nil {
		return fmt.Errorf("convert xlsx→csv: Rows(%s): %w", sheetName, err)
	}
	defer rows.Close()

	outf, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("convert xlsx→csv: create output: %w", err)
	}
	defer outf.Close()

	bw := bufio.NewWriterSize(outf, 1<<16)
	cw := csv.NewWriter(bw)

	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("convert xlsx→csv: read row: %w", err)
		}
		if err := cw.Write(cols); err != nil {
			return fmt.Errorf("convert xlsx→csv: write row: %w", err)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("convert xlsx→csv: rows close: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("convert xlsx→csv: csv flush: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("convert xlsx→csv: flush: %w", err)
	}
	return nil
}

// convertXLSXToJSONL reads the first sheet of an XLSX file and writes one JSON
// object per row. The first row is treated as column headers. All cell values
// are preserved as JSON strings.
func convertXLSXToJSONL(in, out string) error {
	xlsx, err := excelize.OpenFile(in)
	if err != nil {
		return fmt.Errorf("convert xlsx→jsonl: open input: %w", err)
	}
	defer xlsx.Close()

	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("convert xlsx→jsonl: workbook has no sheets")
	}
	sheetName := sheets[0]

	rows, err := xlsx.Rows(sheetName)
	if err != nil {
		return fmt.Errorf("convert xlsx→jsonl: Rows(%s): %w", sheetName, err)
	}
	defer rows.Close()

	outf, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("convert xlsx→jsonl: create output: %w", err)
	}
	defer outf.Close()

	bw := bufio.NewWriterSize(outf, 1<<16)
	enc := json.NewEncoder(bw)

	var headers []string
	rowNum := 0

	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("convert xlsx→jsonl: read row: %w", err)
		}
		rowNum++
		if rowNum == 1 {
			headers = cols
			continue
		}
		// Build map[string]string — all values remain JSON strings.
		obj := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(cols) {
				obj[h] = cols[i]
			} else {
				obj[h] = ""
			}
		}
		if err := enc.Encode(obj); err != nil {
			return fmt.Errorf("convert xlsx→jsonl: encode row %d: %w", rowNum, err)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("convert xlsx→jsonl: rows close: %w", err)
	}

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("convert xlsx→jsonl: flush: %w", err)
	}
	return nil
}

// rawJSONToString converts a json.RawMessage to a plain string value for CSV/XLSX output.
//   - JSON string "foo" → foo (quotes stripped, escape sequences decoded via json.Unmarshal)
//   - JSON number, boolean, null → their raw text representation (e.g. "123.456000000")
//
// Crucially, numeric raw messages are returned as their raw text (no float64 parse),
// preserving decimal strings like "123.456000000" exactly.
func rawJSONToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// JSON strings start and end with '"'.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// For non-string values (numbers, booleans, null), return the raw JSON text.
	// This preserves decimal text like 123.456000000 without float64 coercion.
	return strings.TrimSpace(string(raw))
}
