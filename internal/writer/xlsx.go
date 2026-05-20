package writer

// WriteXLSX writes all documents in ds and reports to a single XLSX workbook at
// outDir/fauxbooks.xlsx. One sheet is produced per document, using the same
// column order as the CSV writers. Monetary values are written as inline-string
// cells (type "inlineStr") to preserve decimal precision — never as numeric cells.
//
// Determinism: all ZIP-entry modification timestamps are pinned to cfg.Period
// (UTC midnight of the first day of the target month) so binary output is
// byte-identical across runs with the same inputs.
//
// Sheets that exceed 1,048,576 rows are split into SheetName_2, SheetName_3, etc.
//
// No time.Now(), no float64 for money, no RNG draws.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort" // used for CoA and ZIP entry ordering
	"strconv"
	"time"

	excelize "github.com/xuri/excelize/v2"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/report"
)

// xlsxMaxRows is the maximum number of data rows (excluding the header) per sheet.
// Excel's limit is 1,048,576 total rows; row 1 is the header, so data rows = limit - 1.
const xlsxMaxRows = 1_048_575

// xlsxRow is a single row of string cells for the XLSX writer.
type xlsxRow = []string

// xlsxSheet describes one logical document to be written into the workbook.
type xlsxSheet struct {
	// name is the sheet base name (e.g. "coa"). Overflow sheets get "_2", "_3", etc.
	name string
	// headers is the ordered list of column names for row 1.
	headers []string
	// rows is the sequence of data rows; each element maps 1:1 with headers.
	rows []xlsxRow
}

// WriteXLSX generates outDir/fauxbooks.xlsx containing one sheet per document.
// It is a complete substitute for writer.Write when cfg.Format == "xlsx"; the
// caller should not call writer.Write in that case.
func WriteXLSX(ds gen.Dataset, reports report.Reports, cfg gen.Config, outDir string) error {
	sheets := buildSheets(ds, reports)

	f := excelize.NewFile()
	defer f.Close()

	// Pin doc props to cfg.Period so creation/modification times are deterministic.
	periodStr := cfg.Period.UTC().Format(time.RFC3339)
	if err := f.SetDocProps(&excelize.DocProperties{
		Created:  periodStr,
		Modified: periodStr,
	}); err != nil {
		return fmt.Errorf("WriteXLSX: SetDocProps: %w", err)
	}

	// excelize creates a default "Sheet1"; we will rename or reuse it for the first sheet.
	firstSheet := true

	for _, sh := range sheets {
		batches := splitIntoBatches(sh.rows, xlsxMaxRows)
		for batchIdx, batch := range batches {
			sheetName := sh.name
			if batchIdx > 0 {
				sheetName = fmt.Sprintf("%s_%d", sh.name, batchIdx+1)
			}

			var idx int
			var err error
			if firstSheet {
				// Rename the default Sheet1 to avoid having an extra blank sheet.
				if err = f.SetSheetName("Sheet1", sheetName); err != nil {
					return fmt.Errorf("WriteXLSX: SetSheetName(%s): %w", sheetName, err)
				}
				idx = 0 // sheet index 0 is Sheet1 (now renamed)
				firstSheet = false
			} else {
				idx, err = f.NewSheet(sheetName)
				if err != nil {
					return fmt.Errorf("WriteXLSX: NewSheet(%s): %w", sheetName, err)
				}
				_ = idx
			}

			sw, err := f.NewStreamWriter(sheetName)
			if err != nil {
				return fmt.Errorf("WriteXLSX: NewStreamWriter(%s): %w", sheetName, err)
			}

			// Row 1: headers (plain strings).
			hCells := make([]interface{}, len(sh.headers))
			for i, h := range sh.headers {
				hCells[i] = excelize.Cell{Value: h}
			}
			if err := sw.SetRow("A1", hCells); err != nil {
				return fmt.Errorf("WriteXLSX: SetRow header %s: %w", sheetName, err)
			}

			// Data rows starting at row 2.
			for rowIdx, rowData := range batch {
				cells := make([]interface{}, len(rowData))
				for ci, v := range rowData {
					cells[ci] = excelize.Cell{Value: v}
				}
				cellRef := "A" + strconv.Itoa(rowIdx+2)
				if err := sw.SetRow(cellRef, cells); err != nil {
					return fmt.Errorf("WriteXLSX: SetRow data %s row %d: %w", sheetName, rowIdx+2, err)
				}
			}

			if err := sw.Flush(); err != nil {
				return fmt.Errorf("WriteXLSX: Flush %s: %w", sheetName, err)
			}
		}
	}

	// Write to a buffer so we can repack ZIP entries with pinned modtimes.
	buf, err := f.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("WriteXLSX: WriteToBuffer: %w", err)
	}

	pinned, err := repackZIPWithFixedModtime(buf.Bytes(), cfg.Period.UTC())
	if err != nil {
		return fmt.Errorf("WriteXLSX: repack: %w", err)
	}

	outPath := filepath.Join(outDir, "fauxbooks.xlsx")
	if err := os.WriteFile(outPath, pinned, 0644); err != nil {
		return fmt.Errorf("WriteXLSX: write file: %w", err)
	}
	return nil
}

// repackZIPWithFixedModtime reads rawZip, sorts all ZIP entries by name,
// overrides every entry's Modified timestamp to pinTime, and returns the
// rewritten ZIP bytes.
//
// Sorting entries is required because excelize uses goroutines internally when
// writing multiple sheets; the sheet order in the raw ZIP is non-deterministic
// across runs. Sorting by entry name normalises the order.
//
// We use OpenRaw + CreateRaw to copy already-compressed bytes verbatim, without
// re-compressing. Re-compression via CreateHeader produces non-deterministic
// output because Go's compress/flate may produce different compressed streams
// for the same input across calls (the compressed size differs while the CRC
// stays the same, causing the ZIP central directory to vary).
func repackZIPWithFixedModtime(rawZip []byte, pinTime time.Time) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(rawZip), int64(len(rawZip)))
	if err != nil {
		return nil, fmt.Errorf("zip.NewReader: %w", err)
	}

	// Sort entries by name for deterministic output ordering.
	entries := make([]*zip.File, len(r.File))
	copy(entries, r.File)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	var out bytes.Buffer
	w := zip.NewWriter(&out)

	for _, zf := range entries {
		fh := zf.FileHeader
		// SetModTime sets Modified, ModifiedDate, and ModifiedTime consistently.
		fh.SetModTime(pinTime)
		// Clear the Extra field so we don't carry the original extended timestamp
		// from excelize (which would contradict our pinned time). The MS-DOS fields
		// set by SetModTime provide sufficient timestamp information.
		fh.Extra = nil

		// CreateRaw writes the raw compressed bytes directly without re-compressing.
		fw, err := w.CreateRaw(&fh)
		if err != nil {
			return nil, fmt.Errorf("CreateRaw %s: %w", zf.Name, err)
		}
		// OpenRaw returns the already-compressed bytes as an io.Reader (no Close needed).
		rc, err := zf.OpenRaw()
		if err != nil {
			return nil, fmt.Errorf("OpenRaw %s: %w", zf.Name, err)
		}
		if _, err := io.Copy(fw, rc); err != nil {
			return nil, fmt.Errorf("Copy %s: %w", zf.Name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zip.Writer.Close: %w", err)
	}
	return out.Bytes(), nil
}

// splitIntoBatches splits rows into batches of at most maxPerBatch rows.
func splitIntoBatches(rows []xlsxRow, maxPerBatch int) [][]xlsxRow {
	if len(rows) == 0 {
		return [][]xlsxRow{nil}
	}
	var batches [][]xlsxRow
	for len(rows) > maxPerBatch {
		batches = append(batches, rows[:maxPerBatch])
		rows = rows[maxPerBatch:]
	}
	batches = append(batches, rows)
	return batches
}

// buildSheets constructs the ordered list of xlsxSheet values from ds and reports.
// The document order matches the CSV writer dispatch in manifest.go.
func buildSheets(ds gen.Dataset, reports report.Reports) []xlsxSheet {
	return []xlsxSheet{
		sheetCoA(ds.CoA),
		sheetCustomers(ds.Customers),
		sheetVendors(ds.Vendors),
		sheetJournalEntries(ds.Entries),
		sheetGL(ds.GL),
		sheetBankStatement(ds.Bank),
		sheetSubLedger("invoices", ds.SubLedger.Invoices),
		sheetSubLedger("customer_payments", ds.SubLedger.CustomerPayments),
		sheetSubLedger("bills", ds.SubLedger.Bills),
		sheetSubLedger("bill_payments", ds.SubLedger.BillPayments),
		sheetSubLedger("expense_reports", ds.SubLedger.ExpenseReports),
		sheetProfitLoss(reports.ProfitLoss),
		sheetBalanceSheet(reports.BalanceSheet),
		sheetCashFlow(reports.CashFlow),
		sheetTrialBalance(reports.TrialBalance),
		sheetARAging(reports.Aging),
		sheetAPAging(reports.Aging),
		sheetCustomerStatements(reports.CustomerStatements),
		sheetCheckRegister(reports.CheckRegister),
	}
}

// --- per-document sheet builders ---

func sheetCoA(coa map[string]gen.Account) xlsxSheet {
	codes := make([]string, 0, len(coa))
	for code := range coa {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	rows := make([]xlsxRow, 0, len(codes))
	for _, code := range codes {
		a := coa[code]
		rows = append(rows, xlsxRow{a.Code, a.Name, string(a.Type), a.NormalBalance})
	}
	return xlsxSheet{
		name:    "coa",
		headers: []string{"code", "name", "type", "normal_balance"},
		rows:    rows,
	}
}

func sheetCustomers(customers []gen.Customer) xlsxSheet {
	rows := make([]xlsxRow, 0, len(customers))
	for _, c := range customers {
		rows = append(rows, xlsxRow{c.ID, c.Name, c.Email, c.Phone, c.Address, c.City, c.State, c.Zip})
	}
	return xlsxSheet{
		name:    "customers",
		headers: []string{"id", "name", "email", "phone", "address", "city", "state", "zip"},
		rows:    rows,
	}
}

func sheetVendors(vendors []gen.Vendor) xlsxSheet {
	rows := make([]xlsxRow, 0, len(vendors))
	for _, v := range vendors {
		rows = append(rows, xlsxRow{v.ID, v.Name, v.Email, v.Phone, v.Address, v.City, v.State, v.Zip})
	}
	return xlsxSheet{
		name:    "vendors",
		headers: []string{"id", "name", "email", "phone", "address", "city", "state", "zip"},
		rows:    rows,
	}
}

func sheetJournalEntries(entries []gen.JournalEntry) xlsxSheet {
	rows := make([]xlsxRow, 0, len(entries))
	for i, e := range entries {
		rows = append(rows, xlsxRow{
			strconv.Itoa(i + 1),
			e.EntryID,
			e.OccurredAt.Format("2006-01-02T15:04:05.999999999Z"),
			e.Description,
			e.SourceEventID,
		})
	}
	return xlsxSheet{
		name:    "journal_entries",
		headers: []string{"row_num", "entry_id", "occurred_at", "description", "source_event_id"},
		rows:    rows,
	}
}

func sheetGL(gl []gen.GLRow) xlsxSheet {
	rows := make([]xlsxRow, 0, len(gl))
	for _, r := range gl {
		rows = append(rows, xlsxRow{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		})
	}
	return xlsxSheet{
		name:    "gl",
		headers: []string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"},
		rows:    rows,
	}
}

func sheetBankStatement(bank []gen.BankRow) xlsxSheet {
	rows := make([]xlsxRow, 0, len(bank))
	for _, r := range bank {
		rows = append(rows, xlsxRow{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.LineID,
			r.OccurredAt,
			r.AccountCode,
			r.Debit,
			r.Credit,
		})
	}
	return xlsxSheet{
		name:    "bank_statement",
		headers: []string{"row_num", "entry_id", "line_id", "occurred_at", "account_code", "debit", "credit"},
		rows:    rows,
	}
}

func sheetSubLedger(name string, sl []gen.SubLedgerRow) xlsxSheet {
	rows := make([]xlsxRow, 0, len(sl))
	for _, r := range sl {
		rows = append(rows, xlsxRow{
			strconv.Itoa(r.RowNum),
			r.EntryID,
			r.RefID,
			r.OccurredAt,
			r.Amount,
			r.SubLedgerType,
		})
	}
	return xlsxSheet{
		name:    name,
		headers: []string{"row_num", "entry_id", "ref_id", "occurred_at", "amount", "sub_ledger_type"},
		rows:    rows,
	}
}

func sheetProfitLoss(pl report.ProfitLoss) xlsxSheet {
	var rows []xlsxRow
	for _, r := range pl.Revenue {
		rows = append(rows, xlsxRow{"revenue", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_revenue", "", "", FormatDecimal(pl.TotalRevenue)})
	for _, r := range pl.Expense {
		rows = append(rows, xlsxRow{"expense", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_expense", "", "", FormatDecimal(pl.TotalExpense)})
	rows = append(rows, xlsxRow{"net_income", "", "", FormatDecimal(pl.NetIncome)})
	return xlsxSheet{
		name:    "profit-loss",
		headers: []string{"section", "account_code", "account_name", "amount"},
		rows:    rows,
	}
}

func sheetBalanceSheet(bs report.BalanceSheet) xlsxSheet {
	var rows []xlsxRow
	for _, r := range bs.Assets {
		rows = append(rows, xlsxRow{"asset", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_assets", "", "", FormatDecimal(bs.TotalAssets)})
	for _, r := range bs.Liabilities {
		rows = append(rows, xlsxRow{"liability", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_liabilities", "", "", FormatDecimal(bs.TotalLiabilities)})
	for _, r := range bs.Equity {
		rows = append(rows, xlsxRow{"equity", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"net_income", "", "", FormatDecimal(bs.NetIncome)})
	rows = append(rows, xlsxRow{"total_equity", "", "", FormatDecimal(bs.TotalEquity)})
	rows = append(rows, xlsxRow{"total_liabilities_and_equity", "", "", FormatDecimal(bs.TotalLiabilitiesAndEquity)})
	return xlsxSheet{
		name:    "balance-sheet",
		headers: []string{"section", "account_code", "account_name", "amount"},
		rows:    rows,
	}
}

func sheetCashFlow(cf report.CashFlow) xlsxSheet {
	var rows []xlsxRow
	rows = append(rows, xlsxRow{"opening_cash", "", "", FormatDecimal(cf.OpeningCash)})
	for _, r := range cf.Inflows {
		rows = append(rows, xlsxRow{"inflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_inflows", "", "", FormatDecimal(cf.TotalIn)})
	for _, r := range cf.Outflows {
		rows = append(rows, xlsxRow{"outflow", r.AccountCode, r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, xlsxRow{"total_outflows", "", "", FormatDecimal(cf.TotalOut)})
	rows = append(rows, xlsxRow{"net_change", "", "", FormatDecimal(cf.NetChange)})
	rows = append(rows, xlsxRow{"ending_cash", "", "", FormatDecimal(cf.EndingCash)})
	return xlsxSheet{
		name:    "cash-flow",
		headers: []string{"section", "account_code", "account_name", "amount"},
		rows:    rows,
	}
}

func sheetTrialBalance(tb report.TrialBalance) xlsxSheet {
	rows := make([]xlsxRow, 0, len(tb.Rows))
	for _, r := range tb.Rows {
		rows = append(rows, xlsxRow{
			r.AccountCode,
			r.AccountName,
			r.AccountType,
			FormatDecimal(r.Debit),
			FormatDecimal(r.Credit),
		})
	}
	return xlsxSheet{
		name:    "trial-balance",
		headers: []string{"account_code", "account_name", "account_type", "debit", "credit"},
		rows:    rows,
	}
}

func sheetARAging(aging report.AgingReport) xlsxSheet {
	return sheetAging("ar-aging", aging.AR)
}

func sheetAPAging(aging report.AgingReport) xlsxSheet {
	return sheetAging("ap-aging", aging.AP)
}

func sheetAging(name string, agingRows []report.AgingRow) xlsxSheet {
	rows := make([]xlsxRow, 0, len(agingRows))
	for _, r := range agingRows {
		rows = append(rows, xlsxRow{
			r.PartyID,
			r.PartyName,
			FormatDecimal(r.Bucket0_30),
			FormatDecimal(r.Bucket31_60),
			FormatDecimal(r.Bucket61_90),
			FormatDecimal(r.Bucket91Plus),
			FormatDecimal(r.Total),
		})
	}
	return xlsxSheet{
		name:    name,
		headers: []string{"party_id", "party_name", "bucket_0_30", "bucket_31_60", "bucket_61_90", "bucket_91_plus", "total"},
		rows:    rows,
	}
}

func sheetCustomerStatements(stmts []report.CustomerStatement) xlsxSheet {
	rows := make([]xlsxRow, 0, len(stmts))
	for _, s := range stmts {
		rows = append(rows, xlsxRow{
			s.CustomerID,
			s.CustomerName,
			FormatDecimal(s.Opening),
			FormatDecimal(s.Invoiced),
			FormatDecimal(s.Payments),
			FormatDecimal(s.Closing),
		})
	}
	return xlsxSheet{
		name:    "customer-statements",
		headers: []string{"customer_id", "customer_name", "opening_balance", "invoiced", "payments", "closing_balance"},
		rows:    rows,
	}
}

func sheetCheckRegister(ckRows []report.CheckRegisterRow) xlsxSheet {
	rows := make([]xlsxRow, 0, len(ckRows))
	for _, r := range ckRows {
		rows = append(rows, xlsxRow{
			r.Date.UTC().Format(checkRegisterDateFmt),
			r.EntryID,
			r.AccountCode,
			r.Payee,
			FormatDecimal(r.Amount),
		})
	}
	return xlsxSheet{
		name:    "check-register",
		headers: []string{"date", "entry_id", "account_code", "payee", "amount"},
		rows:    rows,
	}
}
