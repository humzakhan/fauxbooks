package writer

import (
	"fmt"
	"io"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// checkRegisterRowJSONL is the JSONL representation of one check register row.
type checkRegisterRowJSONL struct {
	Date        string `json:"date"`
	EntryID     string `json:"entry_id"`
	AccountCode string `json:"account_code"`
	Payee       string `json:"payee"`
	Amount      string `json:"amount"`
}

const checkRegisterDateFmt = "2006-01-02T15:04:05.999999999Z"

// WriteCheckRegister writes check register rows to w as CSV.
// Columns: date, entry_id, account_code, payee, amount.
// date format: 2006-01-02T15:04:05.999999999Z (matching project.go).
func WriteCheckRegister(w io.Writer, rows []report.CheckRegisterRow) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"date", "entry_id", "account_code", "payee", "amount"}); err != nil {
		return 0, fmt.Errorf("WriteCheckRegister: csv header: %w", err)
	}
	for _, r := range rows {
		row := []string{
			r.Date.UTC().Format(checkRegisterDateFmt),
			r.EntryID,
			r.AccountCode,
			r.Payee,
			FormatDecimal(r.Amount),
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("WriteCheckRegister: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteCheckRegister: %w", err)
	}
	return n, nil
}

// WriteCheckRegisterJSONL writes check register rows to w as JSONL, one object per line.
func WriteCheckRegisterJSONL(w io.Writer, rows []report.CheckRegisterRow) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range rows {
		row := checkRegisterRowJSONL{
			Date:        r.Date.UTC().Format(checkRegisterDateFmt),
			EntryID:     r.EntryID,
			AccountCode: r.AccountCode,
			Payee:       r.Payee,
			Amount:      FormatDecimal(r.Amount),
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteCheckRegisterJSONL: %w", err)
		}
		n++
	}
	return n, nil
}
