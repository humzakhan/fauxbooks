package writer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// fileHash records the SHA-256 digest for one output file.
// Using a slice of structs (sorted by Name) rather than a map guarantees
// deterministic JSON key order regardless of map iteration behaviour.
type fileHash struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// fileRowCount records the row count for one output file.
type fileRowCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// manifestJSON is the typed struct written to manifest.json.
// Field order determines JSON object key order (encoding/json preserves struct
// field order). No generated_at field — wall-clock reads in the generation
// path break determinism (architecture/OVERVIEW.md §Determinism).
type manifestJSON struct {
	Seed       int64          `json:"seed"`
	Period     string         `json:"period"`
	Bundle     string         `json:"bundle"`
	Volume     string         `json:"volume"`
	Format     string         `json:"format"`
	RowCounts  []fileRowCount `json:"row_counts"`
	FileSHA256 []fileHash     `json:"file_sha256"`
}

// newHashingWriter returns an io.Writer that tees all writes to w and into a
// SHA-256 hasher. Call the returned sum func after all writes to obtain the
// hex digest. The caller must flush any bufio.Writer wrapping w before calling
// sum to ensure all buffered bytes are hashed.
func newHashingWriter(w io.Writer) (io.Writer, func() string) {
	h := sha256.New()
	tee := io.MultiWriter(w, h)
	sum := func() string {
		return hex.EncodeToString(h.Sum(nil))
	}
	return tee, sum
}

// writeFile creates outDir/name, wraps it in a bufio.Writer + SHA-256 tee,
// calls fn, flushes, and returns the row count and hex hash.
// We flush explicitly before computing the hash so the digest covers all
// buffered bytes. Named return err ensures Close errors surface.
func writeFile(outDir, name string, fn func(io.Writer) (int, error)) (rowCount int, hexHash string, err error) {
	path := filepath.Join(outDir, name)
	f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if ferr != nil {
		return 0, "", fmt.Errorf("open %s: %w", name, ferr)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", name, cerr)
		}
	}()

	bw := bufio.NewWriterSize(f, csvBufSize)
	tee, sum := newHashingWriter(bw)

	rowCount, err = fn(tee)
	if err != nil {
		return 0, "", fmt.Errorf("write %s: %w", name, err)
	}

	// Flush bw so all buffered bytes are hashed before sum() is called.
	if ferr := bw.Flush(); ferr != nil {
		return 0, "", fmt.Errorf("flush %s: %w", name, ferr)
	}
	hexHash = sum()
	return rowCount, hexHash, nil
}

// Write orchestrates writing all output files for ds into outDir, accumulates
// SHA-256 per file during the write pass, then writes manifest.json last.
// Returns an error if any file write fails.
func Write(ds gen.Dataset, cfg gen.Config, outDir string) error {
	type fileSpec struct {
		name string
		fn   func(io.Writer) (int, error)
	}

	type result struct {
		name  string
		count int
		hash  string
	}

	var files []fileSpec

	if cfg.Format == "jsonl" {
		files = []fileSpec{
			{"coa.jsonl", func(w io.Writer) (int, error) {
				return WriteCoAJSONL(w, ds.CoA)
			}},
			{"customers.jsonl", func(w io.Writer) (int, error) {
				return WriteCustomersJSONL(w, ds.Customers)
			}},
			{"vendors.jsonl", func(w io.Writer) (int, error) {
				return WriteVendorsJSONL(w, ds.Vendors)
			}},
			{"journal_entries.jsonl", func(w io.Writer) (int, error) {
				return WriteJournalEntriesJSONL(w, ds.Entries)
			}},
			{"gl.jsonl", func(w io.Writer) (int, error) {
				return WriteGLJSONL(w, ds.GL)
			}},
			{"bank_statement.jsonl", func(w io.Writer) (int, error) {
				return WriteBankStatementJSONL(w, ds.Bank)
			}},
			{"invoices.jsonl", func(w io.Writer) (int, error) {
				return WriteInvoicesJSONL(w, ds.SubLedger.Invoices)
			}},
			{"customer_payments.jsonl", func(w io.Writer) (int, error) {
				return WriteCustomerPaymentsJSONL(w, ds.SubLedger.CustomerPayments)
			}},
			{"bills.jsonl", func(w io.Writer) (int, error) {
				return WriteBillsJSONL(w, ds.SubLedger.Bills)
			}},
			{"bill_payments.jsonl", func(w io.Writer) (int, error) {
				return WriteBillPaymentsJSONL(w, ds.SubLedger.BillPayments)
			}},
			{"expense_reports.jsonl", func(w io.Writer) (int, error) {
				return WriteExpenseReportsJSONL(w, ds.SubLedger.ExpenseReports)
			}},
		}
	} else {
		// Default: CSV.
		files = []fileSpec{
			{"coa.csv", func(w io.Writer) (int, error) {
				return WriteCoA(w, ds.CoA)
			}},
			{"customers.csv", func(w io.Writer) (int, error) {
				return WriteCustomers(w, ds.Customers)
			}},
			{"vendors.csv", func(w io.Writer) (int, error) {
				return WriteVendors(w, ds.Vendors)
			}},
			{"journal_entries.csv", func(w io.Writer) (int, error) {
				return WriteJournalEntries(w, ds.Entries)
			}},
			{"gl.csv", func(w io.Writer) (int, error) {
				return WriteGL(w, ds.GL)
			}},
			{"bank_statement.csv", func(w io.Writer) (int, error) {
				return WriteBankStatement(w, ds.Bank)
			}},
			{"invoices.csv", func(w io.Writer) (int, error) {
				return WriteInvoices(w, ds.SubLedger.Invoices)
			}},
			{"customer_payments.csv", func(w io.Writer) (int, error) {
				return WriteCustomerPayments(w, ds.SubLedger.CustomerPayments)
			}},
			{"bills.csv", func(w io.Writer) (int, error) {
				return WriteBills(w, ds.SubLedger.Bills)
			}},
			{"bill_payments.csv", func(w io.Writer) (int, error) {
				return WriteBillPayments(w, ds.SubLedger.BillPayments)
			}},
			{"expense_reports.csv", func(w io.Writer) (int, error) {
				return WriteExpenseReports(w, ds.SubLedger.ExpenseReports)
			}},
		}
	}

	results := make([]result, 0, len(files))
	for _, fs := range files {
		count, hash, werr := writeFile(outDir, fs.name, fs.fn)
		if werr != nil {
			return fmt.Errorf("writer.Write: %w", werr)
		}
		results = append(results, result{name: fs.name, count: count, hash: hash})
	}

	// Build row counts and hashes as sorted slices for deterministic JSON.
	rowCounts := make([]fileRowCount, len(results))
	hashes := make([]fileHash, len(results))
	for i, r := range results {
		rowCounts[i] = fileRowCount{Name: r.name, Count: r.count}
		hashes[i] = fileHash{Name: r.name, SHA256: r.hash}
	}
	sort.Slice(rowCounts, func(i, j int) bool { return rowCounts[i].Name < rowCounts[j].Name })
	sort.Slice(hashes, func(i, j int) bool { return hashes[i].Name < hashes[j].Name })

	mj := manifestJSON{
		Seed:       cfg.Seed,
		Period:     cfg.Period.Format("2006-01"),
		Bundle:     cfg.Bundle,
		Volume:     cfg.Volume,
		Format:     cfg.Format,
		RowCounts:  rowCounts,
		FileSHA256: hashes,
	}

	if err := writeManifest(outDir, mj); err != nil {
		return fmt.Errorf("writer.Write: manifest: %w", err)
	}
	return nil
}

// writeManifest writes mj to outDir/manifest.json.
// Named return err ensures deferred Flush and Close errors propagate.
func writeManifest(outDir string, mj manifestJSON) (err error) {
	path := filepath.Join(outDir, "manifest.json")
	f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if ferr != nil {
		return fmt.Errorf("open manifest.json: %w", ferr)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close manifest.json: %w", cerr)
		}
	}()

	bw := bufio.NewWriterSize(f, csvBufSize)
	defer func() {
		if ferr := bw.Flush(); ferr != nil && err == nil {
			err = fmt.Errorf("flush manifest.json: %w", ferr)
		}
	}()

	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err = enc.Encode(mj); err != nil {
		return fmt.Errorf("encode manifest.json: %w", err)
	}
	return nil
}
