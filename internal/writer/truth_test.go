package writer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// sampleTruthLog returns a TruthLog with two distinct records for use in tests.
func sampleTruthLog() gen.TruthLog {
	return gen.TruthLog{
		{Type: "timing_difference", EntryID: "JE-00000001", Detail: "bank clears one day late"},
		{Type: "duplicate", EntryID: "JE-00000002", Detail: "duplicate invoice payment"},
	}
}

// decodeTruthOutput unmarshals writeTruthTo output and returns the inner records slice.
func decodeTruthOutput(t *testing.T, data []byte) []map[string]string {
	t.Helper()
	var outer struct {
		Records []map[string]string `json:"records"`
	}
	if err := json.Unmarshal(data, &outer); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return outer.Records
}

// TestWriteTruthTo_SchemaFields verifies that every decoded record has non-empty
// type, entry_id, and detail fields.
func TestWriteTruthTo_SchemaFields(t *testing.T) {
	log := sampleTruthLog()
	var buf bytes.Buffer
	n, err := writeTruthTo(&buf, log)
	if err != nil {
		t.Fatalf("writeTruthTo: %v", err)
	}
	if n != len(log) {
		t.Errorf("count = %d, want %d", n, len(log))
	}

	records := decodeTruthOutput(t, buf.Bytes())
	for i, rec := range records {
		if rec["type"] == "" {
			t.Errorf("record[%d]: type is empty", i)
		}
		if rec["entry_id"] == "" {
			t.Errorf("record[%d]: entry_id is empty", i)
		}
		if rec["detail"] == "" {
			t.Errorf("record[%d]: detail is empty", i)
		}
	}
}

// TestWriteTruthTo_WellFormedJSON verifies that the full output of writeTruthTo
// is valid JSON (json.Unmarshal returns no error).
func TestWriteTruthTo_WellFormedJSON(t *testing.T) {
	var buf bytes.Buffer
	if _, err := writeTruthTo(&buf, sampleTruthLog()); err != nil {
		t.Fatalf("writeTruthTo: %v", err)
	}
	var v interface{}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Errorf("output is not well-formed JSON: %v\ngot:\n%s", err, buf.Bytes())
	}
}

// TestWriteTruthTo_Empty verifies that a nil or empty TruthLog writes a valid
// JSON object with an empty records array and returns count 0.
func TestWriteTruthTo_Empty(t *testing.T) {
	for _, tc := range []struct {
		name string
		log  gen.TruthLog
	}{
		{"nil", nil},
		{"empty", gen.TruthLog{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := writeTruthTo(&buf, tc.log)
			if err != nil {
				t.Fatalf("writeTruthTo: %v", err)
			}
			if n != 0 {
				t.Errorf("count = %d, want 0", n)
			}
			records := decodeTruthOutput(t, buf.Bytes())
			if len(records) != 0 {
				t.Errorf("records length = %d, want 0", len(records))
			}
		})
	}
}

// TestWriteTruthTo_InputOrderPreserved verifies that writeTruthTo preserves
// the order of records as provided by the caller. Sorting is the caller's
// responsibility; writeTruthTo must not reorder input.
func TestWriteTruthTo_InputOrderPreserved(t *testing.T) {
	// Provide records in reverse lexicographic order by type to confirm
	// writeTruthTo does not sort them.
	log := gen.TruthLog{
		{Type: "timing_difference", EntryID: "JE-00000003", Detail: "late clearing"},
		{Type: "orphan_bank_row", EntryID: "JE-00000002", Detail: "unmatched bank row"},
		{Type: "duplicate", EntryID: "JE-00000001", Detail: "duplicate charge"},
	}
	var buf bytes.Buffer
	if _, err := writeTruthTo(&buf, log); err != nil {
		t.Fatalf("writeTruthTo: %v", err)
	}

	records := decodeTruthOutput(t, buf.Bytes())
	if len(records) != len(log) {
		t.Fatalf("record count = %d, want %d", len(records), len(log))
	}
	for i, rec := range records {
		if rec["type"] != log[i].Type {
			t.Errorf("record[%d] type = %q, want %q (input order not preserved)", i, rec["type"], log[i].Type)
		}
		if rec["entry_id"] != log[i].EntryID {
			t.Errorf("record[%d] entry_id = %q, want %q (input order not preserved)", i, rec["entry_id"], log[i].EntryID)
		}
	}
}
