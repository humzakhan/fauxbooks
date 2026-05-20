package writer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// TestJSONLMonetaryFieldIsString verifies that monetary fields (debit, credit,
// amount) in JSONL output are JSON strings, not JSON numbers.
// This guards against the decimal being marshalled as a JSON number, which
// could produce scientific notation or drop trailing zeros.
func TestJSONLMonetaryFieldIsString(t *testing.T) {
	t.Run("gl_debit_credit_are_strings", func(t *testing.T) {
		rows := []gen.GLRow{{
			RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
			OccurredAt:  "2026-01-01T00:00:00Z",
			AccountCode: "1100",
			Debit:       "100.000000000",
			Credit:      "0.000000000",
		}}
		var buf bytes.Buffer
		if _, err := WriteGLJSONL(&buf, rows); err != nil {
			t.Fatalf("WriteGLJSONL: %v", err)
		}
		assertMonetaryStrings(t, buf.Bytes(), "debit", "credit")
	})

	t.Run("bank_debit_credit_are_strings", func(t *testing.T) {
		rows := []gen.BankRow{{
			RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
			OccurredAt:  "2026-01-01T00:00:00Z",
			AccountCode: "1010",
			Debit:       "50.000000000",
			Credit:      "0.000000000",
		}}
		var buf bytes.Buffer
		if _, err := WriteBankStatementJSONL(&buf, rows); err != nil {
			t.Fatalf("WriteBankStatementJSONL: %v", err)
		}
		assertMonetaryStrings(t, buf.Bytes(), "debit", "credit")
	})

	t.Run("sub_ledger_amount_is_string", func(t *testing.T) {
		rows := []gen.SubLedgerRow{{
			RowNum: 1, EntryID: "JE-00000001",
			OccurredAt:    "2026-01-01T00:00:00Z",
			Amount:        "100.000000000",
			SubLedgerType: "invoice",
		}}
		var buf bytes.Buffer
		if _, err := WriteInvoicesJSONL(&buf, rows); err != nil {
			t.Fatalf("WriteInvoicesJSONL: %v", err)
		}
		assertMonetaryStrings(t, buf.Bytes(), "amount")
	})
}

// assertMonetaryStrings unmarshals each line of jsonlData into
// map[string]any and asserts that each field in fields is a string, not a
// json.Number or float64.
func assertMonetaryStrings(t *testing.T, jsonlData []byte, fields ...string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(jsonlData)), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d: unmarshal: %v", i+1, err)
		}
		for _, field := range fields {
			v, ok := obj[field]
			if !ok {
				t.Errorf("line %d: field %q missing", i+1, field)
				continue
			}
			if _, isStr := v.(string); !isStr {
				t.Errorf("line %d: field %q is %T (%v), want string", i+1, field, v, v)
			}
		}
	}
}

// TestJSONLOneObjectPerLine verifies that each JSONL line is a valid JSON object.
func TestJSONLOneObjectPerLine(t *testing.T) {
	entries := []gen.JournalEntry{{
		EntryID:       "JE-00000001",
		OccurredAt:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Description:   "Sale invoice",
		SourceEventID: "evt-001",
	}, {
		EntryID:       "JE-00000002",
		OccurredAt:    time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC),
		Description:   "Customer payment",
		SourceEventID: "evt-002",
	}}

	var buf bytes.Buffer
	n, err := WriteJournalEntriesJSONL(&buf, entries)
	if err != nil {
		t.Fatalf("WriteJournalEntriesJSONL: %v", err)
	}
	if n != 2 {
		t.Errorf("row count = %d, want 2", n)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
	}
}

// TestJSONLNoCarriageReturn verifies that JSONL output uses \n line endings.
func TestJSONLNoCarriageReturn(t *testing.T) {
	rows := []gen.GLRow{{
		RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
		OccurredAt: "2026-01-01T00:00:00Z", AccountCode: "1100",
		Debit: "100.000000000", Credit: "0.000000000",
	}}
	var buf bytes.Buffer
	if _, err := WriteGLJSONL(&buf, rows); err != nil {
		t.Fatalf("WriteGLJSONL: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte{'\r'}) {
		t.Error("JSONL output contains \\r (CRLF line endings)")
	}
}
