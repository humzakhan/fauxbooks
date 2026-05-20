package writer

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// newJSONLEncoder creates a json.Encoder configured for fauxbooks JSONL output:
// HTML escaping disabled (preserves < > & in company names / descriptions).
// json.Encoder.Encode appends \n automatically — one object per line.
func newJSONLEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

// coaJSONLRow is the JSONL representation of one CoA account.
// Monetary fields (none here) would be strings; all fields are plain strings.
type coaJSONLRow struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	NormalBalance string `json:"normal_balance"`
}

// journalEntryJSONLRow is the JSONL representation of a journal entry header.
type journalEntryJSONLRow struct {
	RowNum        string `json:"row_num"`
	EntryID       string `json:"entry_id"`
	OccurredAt    string `json:"occurred_at"`
	Description   string `json:"description"`
	SourceEventID string `json:"source_event_id"`
}

// glJSONLRow is the JSONL representation of a general ledger row.
// Debit and Credit are strings (decimal text with 9 fractional digits).
type glJSONLRow struct {
	RowNum      string `json:"row_num"`
	EntryID     string `json:"entry_id"`
	LineID      string `json:"line_id"`
	OccurredAt  string `json:"occurred_at"`
	AccountCode string `json:"account_code"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
}

// bankJSONLRow is the JSONL representation of a bank statement row.
// Debit and Credit are strings (decimal text with 9 fractional digits).
type bankJSONLRow struct {
	RowNum      string `json:"row_num"`
	EntryID     string `json:"entry_id"`
	LineID      string `json:"line_id"`
	OccurredAt  string `json:"occurred_at"`
	AccountCode string `json:"account_code"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
}

// subLedgerJSONLRow is the JSONL representation of a sub-ledger row.
// Amount is a string (decimal text with 9 fractional digits).
type subLedgerJSONLRow struct {
	RowNum        string `json:"row_num"`
	EntryID       string `json:"entry_id"`
	RefID         string `json:"ref_id"`
	OccurredAt    string `json:"occurred_at"`
	Amount        string `json:"amount"`
	SubLedgerType string `json:"sub_ledger_type"`
}

// WriteCoAJSONL writes the chart of accounts to w as JSONL, one object per line.
// Accounts are sorted by code for deterministic output.
func WriteCoAJSONL(w io.Writer, coa map[string]gen.Account) (n int, err error) {
	enc := newJSONLEncoder(w)

	codes := make([]string, 0, len(coa))
	for code := range coa {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		a := coa[code]
		row := coaJSONLRow{
			Code:          a.Code,
			Name:          a.Name,
			Type:          string(a.Type),
			NormalBalance: a.NormalBalance,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteCoAJSONL: %w", err)
		}
		n++
	}
	return n, nil
}

// WriteJournalEntriesJSONL writes journal entry headers to w as JSONL.
func WriteJournalEntriesJSONL(w io.Writer, entries []gen.JournalEntry) (n int, err error) {
	enc := newJSONLEncoder(w)

	for i, e := range entries {
		row := journalEntryJSONLRow{
			RowNum:        strconv.Itoa(i + 1),
			EntryID:       e.EntryID,
			OccurredAt:    e.OccurredAt.Format("2006-01-02T15:04:05.999999999Z"),
			Description:   e.Description,
			SourceEventID: e.SourceEventID,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteJournalEntriesJSONL: %w", err)
		}
		n++
	}
	return n, nil
}

// WriteGLJSONL writes general ledger rows to w as JSONL.
// Debit and Credit fields are decimal strings with 9 fractional digits.
func WriteGLJSONL(w io.Writer, gl []gen.GLRow) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range gl {
		row := glJSONLRow{
			RowNum:      strconv.Itoa(r.RowNum),
			EntryID:     r.EntryID,
			LineID:      r.LineID,
			OccurredAt:  r.OccurredAt,
			AccountCode: r.AccountCode,
			Debit:       r.Debit,
			Credit:      r.Credit,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteGLJSONL: %w", err)
		}
		n++
	}
	return n, nil
}

// WriteBankStatementJSONL writes bank statement rows to w as JSONL.
// Debit and Credit fields are decimal strings with 9 fractional digits.
func WriteBankStatementJSONL(w io.Writer, bank []gen.BankRow) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range bank {
		row := bankJSONLRow{
			RowNum:      strconv.Itoa(r.RowNum),
			EntryID:     r.EntryID,
			LineID:      r.LineID,
			OccurredAt:  r.OccurredAt,
			AccountCode: r.AccountCode,
			Debit:       r.Debit,
			Credit:      r.Credit,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteBankStatementJSONL: %w", err)
		}
		n++
	}
	return n, nil
}

// writeSubLedgerJSONL is the common implementation for all sub-ledger JSONL writers.
func writeSubLedgerJSONL(w io.Writer, rows []gen.SubLedgerRow, label string) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range rows {
		row := subLedgerJSONLRow{
			RowNum:        strconv.Itoa(r.RowNum),
			EntryID:       r.EntryID,
			RefID:         r.RefID,
			OccurredAt:    r.OccurredAt,
			Amount:        r.Amount,
			SubLedgerType: r.SubLedgerType,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("%s: %w", label, err)
		}
		n++
	}
	return n, nil
}

// WriteInvoicesJSONL writes sale invoice sub-ledger rows to w as JSONL.
func WriteInvoicesJSONL(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedgerJSONL(w, rows, "WriteInvoicesJSONL")
}

// WriteCustomerPaymentsJSONL writes customer payment sub-ledger rows to w as JSONL.
func WriteCustomerPaymentsJSONL(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedgerJSONL(w, rows, "WriteCustomerPaymentsJSONL")
}

// WriteBillsJSONL writes vendor bill sub-ledger rows to w as JSONL.
func WriteBillsJSONL(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedgerJSONL(w, rows, "WriteBillsJSONL")
}

// WriteBillPaymentsJSONL writes bill payment sub-ledger rows to w as JSONL.
func WriteBillPaymentsJSONL(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedgerJSONL(w, rows, "WriteBillPaymentsJSONL")
}

// WriteExpenseReportsJSONL writes expense report sub-ledger rows to w as JSONL.
func WriteExpenseReportsJSONL(w io.Writer, rows []gen.SubLedgerRow) (n int, err error) {
	return writeSubLedgerJSONL(w, rows, "WriteExpenseReportsJSONL")
}

// partyJSONLRow is the JSONL representation of a customer or vendor master record.
type partyJSONLRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
}

// WriteCustomersJSONL writes the customer master to w as JSONL, one object per line.
func WriteCustomersJSONL(w io.Writer, customers []gen.Customer) (n int, err error) {
	enc := newJSONLEncoder(w)
	for _, c := range customers {
		row := partyJSONLRow{
			ID:      c.ID,
			Name:    c.Name,
			Email:   c.Email,
			Phone:   c.Phone,
			Address: c.Address,
			City:    c.City,
			State:   c.State,
			Zip:     c.Zip,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteCustomersJSONL: %w", err)
		}
		n++
	}
	return n, nil
}

// WriteVendorsJSONL writes the vendor master to w as JSONL, one object per line.
func WriteVendorsJSONL(w io.Writer, vendors []gen.Vendor) (n int, err error) {
	enc := newJSONLEncoder(w)
	for _, v := range vendors {
		row := partyJSONLRow{
			ID:      v.ID,
			Name:    v.Name,
			Email:   v.Email,
			Phone:   v.Phone,
			Address: v.Address,
			City:    v.City,
			State:   v.State,
			Zip:     v.Zip,
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteVendorsJSONL: %w", err)
		}
		n++
	}
	return n, nil
}
