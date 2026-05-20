package writer

import (
	"fmt"
	"io"

	"github.com/humzakhan/fauxbooks/internal/report"
)

// agingRowJSONL is the JSONL representation of one aging row.
type agingRowJSONL struct {
	PartyID      string `json:"party_id"`
	PartyName    string `json:"party_name"`
	Bucket0_30   string `json:"bucket_0_30"`
	Bucket31_60  string `json:"bucket_31_60"`
	Bucket61_90  string `json:"bucket_61_90"`
	Bucket91Plus string `json:"bucket_91_plus"`
	Total        string `json:"total"`
}

// writeAging is the common implementation for AR and AP aging CSV writers.
func writeAging(w io.Writer, rows []report.AgingRow, label string) (n int, err error) {
	cw := newCSVWriter(w)

	if err = cw.Write([]string{"party_id", "party_name", "bucket_0_30", "bucket_31_60", "bucket_61_90", "bucket_91_plus", "total"}); err != nil {
		return 0, fmt.Errorf("%s: csv header: %w", label, err)
	}
	for _, r := range rows {
		row := []string{
			r.PartyID,
			r.PartyName,
			FormatDecimal(r.Bucket0_30),
			FormatDecimal(r.Bucket31_60),
			FormatDecimal(r.Bucket61_90),
			FormatDecimal(r.Bucket91Plus),
			FormatDecimal(r.Total),
		}
		if err = cw.Write(row); err != nil {
			return 0, fmt.Errorf("%s: csv row: %w", label, err)
		}
		n++
	}
	cw.Flush()
	if err = cw.Error(); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return n, nil
}

// writeAgingJSONL is the common implementation for AR and AP aging JSONL writers.
func writeAgingJSONL(w io.Writer, rows []report.AgingRow, label string) (n int, err error) {
	enc := newJSONLEncoder(w)

	for _, r := range rows {
		row := agingRowJSONL{
			PartyID:      r.PartyID,
			PartyName:    r.PartyName,
			Bucket0_30:   FormatDecimal(r.Bucket0_30),
			Bucket31_60:  FormatDecimal(r.Bucket31_60),
			Bucket61_90:  FormatDecimal(r.Bucket61_90),
			Bucket91Plus: FormatDecimal(r.Bucket91Plus),
			Total:        FormatDecimal(r.Total),
		}
		if err = enc.Encode(row); err != nil {
			return n, fmt.Errorf("%s: %w", label, err)
		}
		n++
	}
	return n, nil
}

// WriteARAging writes AR aging rows to w as CSV.
// Columns: party_id, party_name, bucket_0_30, bucket_31_60, bucket_61_90, bucket_91_plus, total.
func WriteARAging(w io.Writer, aging report.AgingReport) (n int, err error) {
	return writeAging(w, aging.AR, "WriteARAging")
}

// WriteARAgingJSONL writes AR aging rows to w as JSONL, one object per line.
func WriteARAgingJSONL(w io.Writer, aging report.AgingReport) (n int, err error) {
	return writeAgingJSONL(w, aging.AR, "WriteARAgingJSONL")
}

// WriteAPAging writes AP aging rows to w as CSV.
// Columns: party_id, party_name, bucket_0_30, bucket_31_60, bucket_61_90, bucket_91_plus, total.
func WriteAPAging(w io.Writer, aging report.AgingReport) (n int, err error) {
	return writeAging(w, aging.AP, "WriteAPAging")
}

// WriteAPAgingJSONL writes AP aging rows to w as JSONL, one object per line.
func WriteAPAgingJSONL(w io.Writer, aging report.AgingReport) (n int, err error) {
	return writeAgingJSONL(w, aging.AP, "WriteAPAgingJSONL")
}
