package writer

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// TestCSVNoCarriageReturn verifies that CSV output uses \n line endings only,
// not \r\n, for all CSV write functions (cw.UseCRLF = false).
func TestCSVNoCarriageReturn(t *testing.T) {
	t.Run("coa", func(t *testing.T) {
		coa := map[string]gen.Account{
			"1000": {Code: "1000", Name: "Cash", Type: gen.AccountTypeAsset, NormalBalance: "debit"},
		}
		var buf bytes.Buffer
		if _, err := WriteCoA(&buf, coa); err != nil {
			t.Fatalf("WriteCoA: %v", err)
		}
		assertNoCarriageReturn(t, buf.Bytes())
	})

	t.Run("gl", func(t *testing.T) {
		rows := []gen.GLRow{{RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
			OccurredAt: "2026-01-01T00:00:00Z", AccountCode: "1000",
			Debit: "100.000000000", Credit: "0.000000000"}}
		var buf bytes.Buffer
		if _, err := WriteGL(&buf, rows); err != nil {
			t.Fatalf("WriteGL: %v", err)
		}
		assertNoCarriageReturn(t, buf.Bytes())
	})

	t.Run("bank_statement", func(t *testing.T) {
		rows := []gen.BankRow{{RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
			OccurredAt: "2026-01-01T00:00:00Z", AccountCode: "1010",
			Debit: "50.000000000", Credit: "0.000000000"}}
		var buf bytes.Buffer
		if _, err := WriteBankStatement(&buf, rows); err != nil {
			t.Fatalf("WriteBankStatement: %v", err)
		}
		assertNoCarriageReturn(t, buf.Bytes())
	})

	t.Run("journal_entries", func(t *testing.T) {
		entries := []gen.JournalEntry{{
			EntryID:       "JE-00000001",
			OccurredAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Description:   "Sale invoice",
			SourceEventID: "evt-001",
		}}
		var buf bytes.Buffer
		if _, err := WriteJournalEntries(&buf, entries); err != nil {
			t.Fatalf("WriteJournalEntries: %v", err)
		}
		assertNoCarriageReturn(t, buf.Bytes())
	})

	t.Run("sub_ledger", func(t *testing.T) {
		rows := []gen.SubLedgerRow{{RowNum: 1, EntryID: "JE-00000001",
			OccurredAt: "2026-01-01T00:00:00Z", Amount: "100.000000000",
			SubLedgerType: "invoice"}}
		var buf bytes.Buffer
		if _, err := WriteInvoices(&buf, rows); err != nil {
			t.Fatalf("WriteInvoices: %v", err)
		}
		assertNoCarriageReturn(t, buf.Bytes())
	})
}

// assertNoCarriageReturn fails the test if data contains any \r byte.
func assertNoCarriageReturn(t *testing.T, data []byte) {
	t.Helper()
	if bytes.Contains(data, []byte{'\r'}) {
		t.Errorf("output contains \\r (CRLF line endings); got:\n%s", data)
	}
}

// TestCSVHeader verifies that the first line of each CSV is the correct header.
func TestCSVHeader(t *testing.T) {
	t.Run("coa", func(t *testing.T) {
		coa := map[string]gen.Account{
			"1000": {Code: "1000", Name: "Cash", Type: gen.AccountTypeAsset, NormalBalance: "debit"},
		}
		var buf bytes.Buffer
		if _, err := WriteCoA(&buf, coa); err != nil {
			t.Fatalf("WriteCoA: %v", err)
		}
		lines := strings.Split(buf.String(), "\n")
		if want := "code,name,type,normal_balance"; lines[0] != want {
			t.Errorf("header = %q, want %q", lines[0], want)
		}
	})

	t.Run("gl", func(t *testing.T) {
		var buf bytes.Buffer
		if _, err := WriteGL(&buf, nil); err != nil {
			t.Fatalf("WriteGL: %v", err)
		}
		lines := strings.Split(buf.String(), "\n")
		if want := "row_num,entry_id,line_id,occurred_at,account_code,debit,credit"; lines[0] != want {
			t.Errorf("header = %q, want %q", lines[0], want)
		}
	})
}

// TestCSVFlushCalled verifies that bufio.Flush is called in every CSV write path.
// We pass a bytes.Buffer directly (WriteCoA accepts io.Writer) and confirm
// output is non-empty, which requires the internal csv.Writer flush to have run.
func TestCSVFlushCalled(t *testing.T) {
	coa := map[string]gen.Account{
		"1000": {Code: "1000", Name: "Cash", Type: gen.AccountTypeAsset, NormalBalance: "debit"},
	}
	var buf bytes.Buffer
	n, err := WriteCoA(&buf, coa)
	if err != nil {
		t.Fatalf("WriteCoA: %v", err)
	}
	if n != 1 {
		t.Errorf("row count = %d, want 1", n)
	}
	if buf.Len() == 0 {
		t.Error("buffer is empty — Flush was not called")
	}
}

// TestWriteCustomers_CRLF verifies that WriteCustomers output contains no \r bytes.
func TestWriteCustomers_CRLF(t *testing.T) {
	customers := []gen.Customer{
		{ID: "cust-001", Name: "Acme Corp", Email: "acme@example.com", Phone: "555-0100",
			Address: "1 Main St", City: "Springfield", State: "IL", Zip: "62701"},
		{ID: "cust-002", Name: "Globex", Email: "globex@example.com", Phone: "555-0101",
			Address: "2 Oak Ave", City: "Shelbyville", State: "IL", Zip: "62702"},
	}
	var buf bytes.Buffer
	if _, err := WriteCustomers(&buf, customers); err != nil {
		t.Fatalf("WriteCustomers: %v", err)
	}
	assertNoCarriageReturn(t, buf.Bytes())
}

// TestWriteCustomers_Header verifies that the first line of WriteCustomers output
// is exactly the expected header.
func TestWriteCustomers_Header(t *testing.T) {
	customers := []gen.Customer{
		{ID: "cust-001", Name: "Acme Corp", Email: "acme@example.com", Phone: "555-0100",
			Address: "1 Main St", City: "Springfield", State: "IL", Zip: "62701"},
	}
	var buf bytes.Buffer
	if _, err := WriteCustomers(&buf, customers); err != nil {
		t.Fatalf("WriteCustomers: %v", err)
	}
	lines := strings.Split(buf.String(), "\n")
	if want := "id,name,email,phone,address,city,state,zip"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
}

// TestWriteCustomers_RowCount verifies that the row count matches the number of customers.
func TestWriteCustomers_RowCount(t *testing.T) {
	customers := []gen.Customer{
		{ID: "cust-001", Name: "Acme Corp", Email: "a@example.com", Phone: "555-0100",
			Address: "1 Main St", City: "Springfield", State: "IL", Zip: "62701"},
		{ID: "cust-002", Name: "Globex", Email: "b@example.com", Phone: "555-0101",
			Address: "2 Oak Ave", City: "Shelbyville", State: "IL", Zip: "62702"},
		{ID: "cust-003", Name: "Initech", Email: "c@example.com", Phone: "555-0102",
			Address: "3 Elm Rd", City: "Capital City", State: "IL", Zip: "62703"},
	}
	var buf bytes.Buffer
	n, err := WriteCustomers(&buf, customers)
	if err != nil {
		t.Fatalf("WriteCustomers: %v", err)
	}
	if n != len(customers) {
		t.Errorf("row count = %d, want %d", n, len(customers))
	}
}

// TestWriteVendors_CRLF verifies that WriteVendors output contains no \r bytes.
func TestWriteVendors_CRLF(t *testing.T) {
	vendors := []gen.Vendor{
		{ID: "vend-001", Name: "Supplier A", Email: "sa@example.com", Phone: "555-0200",
			Address: "10 Pine St", City: "Springfield", State: "IL", Zip: "62701"},
		{ID: "vend-002", Name: "Supplier B", Email: "sb@example.com", Phone: "555-0201",
			Address: "11 Cedar Ave", City: "Shelbyville", State: "IL", Zip: "62702"},
	}
	var buf bytes.Buffer
	if _, err := WriteVendors(&buf, vendors); err != nil {
		t.Fatalf("WriteVendors: %v", err)
	}
	assertNoCarriageReturn(t, buf.Bytes())
}

// TestWriteVendors_Header verifies that the first line of WriteVendors output
// is exactly the expected header.
func TestWriteVendors_Header(t *testing.T) {
	vendors := []gen.Vendor{
		{ID: "vend-001", Name: "Supplier A", Email: "sa@example.com", Phone: "555-0200",
			Address: "10 Pine St", City: "Springfield", State: "IL", Zip: "62701"},
	}
	var buf bytes.Buffer
	if _, err := WriteVendors(&buf, vendors); err != nil {
		t.Fatalf("WriteVendors: %v", err)
	}
	lines := strings.Split(buf.String(), "\n")
	if want := "id,name,email,phone,address,city,state,zip"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
}

// TestWriteVendors_RowCount verifies that the row count matches the number of vendors.
func TestWriteVendors_RowCount(t *testing.T) {
	vendors := []gen.Vendor{
		{ID: "vend-001", Name: "Supplier A", Email: "sa@example.com", Phone: "555-0200",
			Address: "10 Pine St", City: "Springfield", State: "IL", Zip: "62701"},
		{ID: "vend-002", Name: "Supplier B", Email: "sb@example.com", Phone: "555-0201",
			Address: "11 Cedar Ave", City: "Shelbyville", State: "IL", Zip: "62702"},
	}
	var buf bytes.Buffer
	n, err := WriteVendors(&buf, vendors)
	if err != nil {
		t.Fatalf("WriteVendors: %v", err)
	}
	if n != len(vendors) {
		t.Errorf("row count = %d, want %d", n, len(vendors))
	}
}

// TestCSVRowCount verifies that row counts returned by each writer equal the
// number of data rows written (not including the header).
func TestCSVRowCount(t *testing.T) {
	t.Run("gl_empty", func(t *testing.T) {
		var buf bytes.Buffer
		n, err := WriteGL(&buf, nil)
		if err != nil {
			t.Fatalf("WriteGL: %v", err)
		}
		if n != 0 {
			t.Errorf("row count = %d, want 0", n)
		}
	})

	t.Run("gl_two_rows", func(t *testing.T) {
		rows := []gen.GLRow{
			{RowNum: 1, EntryID: "JE-00000001", LineID: "L-00000001",
				OccurredAt: "2026-01-01T00:00:00Z", AccountCode: "1100",
				Debit: "100.000000000", Credit: "0.000000000"},
			{RowNum: 2, EntryID: "JE-00000001", LineID: "L-00000002",
				OccurredAt: "2026-01-01T00:00:00Z", AccountCode: "4000",
				Debit: "0.000000000", Credit: "100.000000000"},
		}
		var buf bytes.Buffer
		n, err := WriteGL(&buf, rows)
		if err != nil {
			t.Fatalf("WriteGL: %v", err)
		}
		if n != 2 {
			t.Errorf("row count = %d, want 2", n)
		}
	})
}
