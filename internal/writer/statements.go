package writer

import (
	"fmt"
	"io"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// customerStatementJSONL is the JSONL representation of one customer statement.
type customerStatementJSONL struct {
	CustomerID   string `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Opening      string `json:"opening_balance"`
	Invoiced     string `json:"invoiced"`
	Payments     string `json:"payments"`
	Closing      string `json:"closing_balance"`
}

// WriteCustomerStatements writes customer statements to w as CSV.
// Columns: customer_id, customer_name, opening_balance, invoiced, payments, closing_balance.
func WriteCustomerStatements(w io.Writer, stmts []report.CustomerStatement) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"customer_id", "customer_name", "opening_balance", "invoiced", "payments", "closing_balance"}); err != nil {
		return 0, fmt.Errorf("WriteCustomerStatements: csv header: %w", err)
	}
	for _, s := range stmts {
		row := []string{
			s.CustomerID,
			s.CustomerName,
			FormatDecimal(s.Opening),
			FormatDecimal(s.Invoiced),
			FormatDecimal(s.Payments),
			FormatDecimal(s.Closing),
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("WriteCustomerStatements: csv row: %w", err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("WriteCustomerStatements: %w", err)
	}
	return n, nil
}

// WriteCustomerStatementsJSONL writes customer statements to w as JSONL, one object per line.
func WriteCustomerStatementsJSONL(w io.Writer, stmts []report.CustomerStatement) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, s := range stmts {
		row := customerStatementJSONL{
			CustomerID:   s.CustomerID,
			CustomerName: s.CustomerName,
			Opening:      FormatDecimal(s.Opening),
			Invoiced:     FormatDecimal(s.Invoiced),
			Payments:     FormatDecimal(s.Payments),
			Closing:      FormatDecimal(s.Closing),
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("WriteCustomerStatementsJSONL: %w", err)
		}
		n++
	}
	return n, nil
}
