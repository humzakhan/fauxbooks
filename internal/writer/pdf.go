// Package writer — PDF rendering for fauxbooks financial reports.
//
// WritePDF produces exactly 8 PDF files, one per financial document, using
// gofpdf with standard PDF fonts (Helvetica / Times) — no OS font embedding,
// no CGo, no time.Now(). Both CreationDate and ModDate are pinned to
// cfg.Period so output is byte-identical across runs with the same inputs.
//
// Binary determinism was confirmed by a spike: two calls with identical inputs
// (same period, same company config, same data) produce byte-identical PDF
// files. The gofpdf trailer does not include a time-derived /ID array unless
// encryption is enabled (which we never use).
//
// LogoPath from CompanyConfig is accepted but not rendered; logo embedding is
// deferred to v0.3.
package writer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/report"
)

// PDFDocSet identifies one of the eight supported PDF financial documents.
// All eight are produced by a single WritePDF call (no per-doc selection).
type PDFDocSet int

const (
	PDFDocProfitLoss PDFDocSet = iota
	PDFDocBalanceSheet
	PDFDocCashFlow
	PDFDocTrialBalance
	PDFDocARAging
	PDFDocAPAging
	PDFDocCustomerStatements
	PDFDocCheckRegister
)

// pdfDoc describes one PDF output file.
type pdfDoc struct {
	filename string
	title    string
}

// orderedPDFDocs lists the eight documents in a fixed order for determinism.
var orderedPDFDocs = []pdfDoc{
	{filename: "profit-loss.pdf", title: "Profit & Loss"},
	{filename: "balance-sheet.pdf", title: "Balance Sheet"},
	{filename: "cash-flow.pdf", title: "Cash Flow Statement"},
	{filename: "trial-balance.pdf", title: "Trial Balance"},
	{filename: "ar-aging.pdf", title: "AR Aging"},
	{filename: "ap-aging.pdf", title: "AP Aging"},
	{filename: "customer-statements.pdf", title: "Customer Statements"},
	{filename: "check-register.pdf", title: "Check Register"},
}

// PDF layout constants.
const (
	pdfMargin     = 15.0 // mm — left/right/top margin
	pdfLineHeight = 6.0  // mm — standard row height
	pdfFontSize   = 9.0  // pt — body font size
	pdfHeaderSize = 14.0 // pt — company name
	pdfTitleSize  = 11.0 // pt — document title
	pdfColSize    = 8.0  // pt — column header font size
)

// WritePDF renders all eight financial reports as individual PDF files in outDir.
// Filenames: profit-loss.pdf, balance-sheet.pdf, cash-flow.pdf, trial-balance.pdf,
// ar-aging.pdf, ap-aging.pdf, customer-statements.pdf, check-register.pdf.
//
// Both CreationDate and ModDate are set to cfg.Period; no time.Now() is called.
// Standard PDF fonts (Helvetica, Times) are used — specification-defined, OS-independent,
// no embedding required.
func WritePDF(rpts report.Reports, cfg gen.Config, companyCfg CompanyConfig, outDir string) error {
	type renderer func(pdf *gofpdf.Fpdf)

	renderers := []renderer{
		func(pdf *gofpdf.Fpdf) { renderProfitLoss(pdf, rpts.ProfitLoss) },
		func(pdf *gofpdf.Fpdf) { renderBalanceSheet(pdf, rpts.BalanceSheet) },
		func(pdf *gofpdf.Fpdf) { renderCashFlow(pdf, rpts.CashFlow) },
		func(pdf *gofpdf.Fpdf) { renderTrialBalance(pdf, rpts.TrialBalance) },
		func(pdf *gofpdf.Fpdf) { renderAging(pdf, rpts.Aging.AR) },
		func(pdf *gofpdf.Fpdf) { renderAging(pdf, rpts.Aging.AP) },
		func(pdf *gofpdf.Fpdf) { renderCustomerStatements(pdf, rpts.CustomerStatements) },
		func(pdf *gofpdf.Fpdf) { renderCheckRegister(pdf, rpts.CheckRegister) },
	}

	for i, doc := range orderedPDFDocs {
		pdf := newPDF(cfg, companyCfg, doc.title)
		renderers[i](pdf)

		outPath := filepath.Join(outDir, doc.filename)
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("WritePDF: create %s: %w", outPath, err)
		}
		if err := pdf.Output(f); err != nil {
			f.Close()
			return fmt.Errorf("WritePDF: output %s: %w", outPath, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("WritePDF: close %s: %w", outPath, err)
		}
	}
	return nil
}

// newPDF creates a configured Fpdf instance with pinned timestamps and a
// standard company header already rendered on page 1.
func newPDF(cfg gen.Config, companyCfg CompanyConfig, title string) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Pin both timestamps to cfg.Period — no time.Now().
	pdf.SetCreationDate(cfg.Period)
	pdf.SetModificationDate(cfg.Period)

	// Sort the internal resource catalogs (fonts, images, etc.) by insertion order
	// rather than map iteration order. Without this, font objects may appear in
	// different orders across runs, producing non-identical bytes even when content
	// is identical. SetCatalogSort(true) is documented for exactly this purpose.
	pdf.SetCatalogSort(true)

	// Document metadata.
	pdf.SetTitle(title, true)
	pdf.SetAuthor("fauxbooks", true)
	pdf.SetCreator("fauxbooks", true)

	// Margins.
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(true, pdfMargin)

	pdf.AddPage()
	renderHeader(pdf, companyCfg, cfg, title)
	return pdf
}

// renderHeader writes the company name, address, phone/email, period, and
// document title at the top of the current page.
func renderHeader(pdf *gofpdf.Fpdf, companyCfg CompanyConfig, cfg gen.Config, title string) {
	// Company name — bold, large.
	pdf.SetFont("Helvetica", "B", pdfHeaderSize)
	pdf.CellFormat(0, 8, companyCfg.CompanyName, "", 1, "C", false, 0, "")

	// Address.
	pdf.SetFont("Helvetica", "", pdfFontSize)
	if companyCfg.Address != "" {
		pdf.CellFormat(0, 5, companyCfg.Address, "", 1, "C", false, 0, "")
	}
	// Phone / email on one line.
	contact := ""
	if companyCfg.Phone != "" && companyCfg.Email != "" {
		contact = companyCfg.Phone + "  |  " + companyCfg.Email
	} else if companyCfg.Phone != "" {
		contact = companyCfg.Phone
	} else if companyCfg.Email != "" {
		contact = companyCfg.Email
	}
	if contact != "" {
		pdf.CellFormat(0, 5, contact, "", 1, "C", false, 0, "")
	}

	// Period.
	pdf.CellFormat(0, 5, "Period: "+cfg.Period.Format("January 2006"), "", 1, "C", false, 0, "")

	// Document title.
	pdf.SetFont("Helvetica", "B", pdfTitleSize)
	pdf.CellFormat(0, 7, title, "B", 1, "C", false, 0, "")

	// Gap before table.
	pdf.Ln(3)
}

// --- Section renderers ---

// renderTwoColTable renders a table with columns (label, amount) for simple
// reports that have a section label and a decimal amount.
func renderTwoColTable(pdf *gofpdf.Fpdf, headers [2]string, rows [][2]string) {
	pageW, _, _ := pdf.PageSize(pdf.PageNo())
	l := pdfMargin
	r := pdfMargin
	colW := [2]float64{(pageW - l - r) * 0.70, (pageW - l - r) * 0.30}

	// Column headers.
	pdf.SetFont("Helvetica", "B", pdfColSize)
	pdf.CellFormat(colW[0], pdfLineHeight, headers[0], "B", 0, "L", false, 0, "")
	pdf.CellFormat(colW[1], pdfLineHeight, headers[1], "B", 1, "R", false, 0, "")

	pdf.SetFont("Helvetica", "", pdfFontSize)
	for _, row := range rows {
		pdf.CellFormat(colW[0], pdfLineHeight, row[0], "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], pdfLineHeight, row[1], "", 1, "R", false, 0, "")
	}
}

func renderProfitLoss(pdf *gofpdf.Fpdf, pl report.ProfitLoss) {
	var rows [][2]string
	for _, r := range pl.Revenue {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Revenue", FormatDecimal(pl.TotalRevenue)})
	for _, r := range pl.Expense {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Expense", FormatDecimal(pl.TotalExpense)})
	rows = append(rows, [2]string{"Net Income", FormatDecimal(pl.NetIncome)})
	renderTwoColTable(pdf, [2]string{"Account", "Amount"}, rows)
}

func renderBalanceSheet(pdf *gofpdf.Fpdf, bs report.BalanceSheet) {
	var rows [][2]string
	for _, r := range bs.Assets {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Assets", FormatDecimal(bs.TotalAssets)})
	for _, r := range bs.Liabilities {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Liabilities", FormatDecimal(bs.TotalLiabilities)})
	for _, r := range bs.Equity {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Net Income", FormatDecimal(bs.NetIncome)})
	rows = append(rows, [2]string{"Total Equity", FormatDecimal(bs.TotalEquity)})
	rows = append(rows, [2]string{"Total Liabilities & Equity", FormatDecimal(bs.TotalLiabilitiesAndEquity)})
	renderTwoColTable(pdf, [2]string{"Account", "Amount"}, rows)
}

func renderCashFlow(pdf *gofpdf.Fpdf, cf report.CashFlow) {
	var rows [][2]string
	rows = append(rows, [2]string{"Opening Cash", FormatDecimal(cf.OpeningCash)})
	for _, r := range cf.Inflows {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Inflows", FormatDecimal(cf.TotalIn)})
	for _, r := range cf.Outflows {
		rows = append(rows, [2]string{"  " + r.AccountCode + " " + r.AccountName, FormatDecimal(r.Amount)})
	}
	rows = append(rows, [2]string{"Total Outflows", FormatDecimal(cf.TotalOut)})
	rows = append(rows, [2]string{"Net Change", FormatDecimal(cf.NetChange)})
	rows = append(rows, [2]string{"Ending Cash", FormatDecimal(cf.EndingCash)})
	renderTwoColTable(pdf, [2]string{"Item", "Amount"}, rows)
}

func renderTrialBalance(pdf *gofpdf.Fpdf, tb report.TrialBalance) {
	pageW, _, _ := pdf.PageSize(pdf.PageNo())
	l := pdfMargin
	r := pdfMargin
	avail := pageW - l - r
	colW := [4]float64{avail * 0.18, avail * 0.42, avail * 0.20, avail * 0.20}

	// Headers.
	pdf.SetFont("Helvetica", "B", pdfColSize)
	for i, h := range []string{"Code", "Name", "Debit", "Credit"} {
		align := "L"
		if i >= 2 {
			align = "R"
		}
		pdf.CellFormat(colW[i], pdfLineHeight, h, "B", 0, align, false, 0, "")
	}
	pdf.Ln(pdfLineHeight)

	pdf.SetFont("Helvetica", "", pdfFontSize)
	for _, row := range tb.Rows {
		pdf.CellFormat(colW[0], pdfLineHeight, row.AccountCode, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], pdfLineHeight, row.AccountName, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], pdfLineHeight, FormatDecimal(row.Debit), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[3], pdfLineHeight, FormatDecimal(row.Credit), "", 1, "R", false, 0, "")
	}
}

func renderAging(pdf *gofpdf.Fpdf, rows []report.AgingRow) {
	pageW, _, _ := pdf.PageSize(pdf.PageNo())
	l := pdfMargin
	r := pdfMargin
	avail := pageW - l - r
	colW := [6]float64{avail * 0.22, avail * 0.22, avail * 0.14, avail * 0.14, avail * 0.14, avail * 0.14}

	pdf.SetFont("Helvetica", "B", pdfColSize)
	for i, h := range []string{"Party ID", "Party Name", "0-30", "31-60", "61-90", "91+"} {
		align := "L"
		if i >= 2 {
			align = "R"
		}
		pdf.CellFormat(colW[i], pdfLineHeight, h, "B", 0, align, false, 0, "")
	}
	pdf.Ln(pdfLineHeight)

	pdf.SetFont("Helvetica", "", pdfFontSize)
	for _, row := range rows {
		pdf.CellFormat(colW[0], pdfLineHeight, row.PartyID, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], pdfLineHeight, row.PartyName, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], pdfLineHeight, FormatDecimal(row.Bucket0_30), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[3], pdfLineHeight, FormatDecimal(row.Bucket31_60), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[4], pdfLineHeight, FormatDecimal(row.Bucket61_90), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[5], pdfLineHeight, FormatDecimal(row.Bucket91Plus), "", 1, "R", false, 0, "")
	}
}

func renderCustomerStatements(pdf *gofpdf.Fpdf, stmts []report.CustomerStatement) {
	pageW, _, _ := pdf.PageSize(pdf.PageNo())
	l := pdfMargin
	r := pdfMargin
	avail := pageW - l - r
	colW := [5]float64{avail * 0.22, avail * 0.30, avail * 0.16, avail * 0.16, avail * 0.16}

	pdf.SetFont("Helvetica", "B", pdfColSize)
	for i, h := range []string{"Customer ID", "Customer Name", "Invoiced", "Payments", "Closing"} {
		align := "L"
		if i >= 2 {
			align = "R"
		}
		pdf.CellFormat(colW[i], pdfLineHeight, h, "B", 0, align, false, 0, "")
	}
	pdf.Ln(pdfLineHeight)

	pdf.SetFont("Helvetica", "", pdfFontSize)
	for _, s := range stmts {
		pdf.CellFormat(colW[0], pdfLineHeight, s.CustomerID, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], pdfLineHeight, s.CustomerName, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], pdfLineHeight, FormatDecimal(s.Invoiced), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[3], pdfLineHeight, FormatDecimal(s.Payments), "", 0, "R", false, 0, "")
		pdf.CellFormat(colW[4], pdfLineHeight, FormatDecimal(s.Closing), "", 1, "R", false, 0, "")
	}
}

func renderCheckRegister(pdf *gofpdf.Fpdf, rows []report.CheckRegisterRow) {
	pageW, _, _ := pdf.PageSize(pdf.PageNo())
	l := pdfMargin
	r := pdfMargin
	avail := pageW - l - r
	colW := [5]float64{avail * 0.22, avail * 0.22, avail * 0.16, avail * 0.24, avail * 0.16}

	pdf.SetFont("Helvetica", "B", pdfColSize)
	for i, h := range []string{"Date", "Entry ID", "Account", "Payee", "Amount"} {
		align := "L"
		if i == 4 {
			align = "R"
		}
		pdf.CellFormat(colW[i], pdfLineHeight, h, "B", 0, align, false, 0, "")
	}
	pdf.Ln(pdfLineHeight)

	pdf.SetFont("Helvetica", "", pdfFontSize)
	for _, row := range rows {
		pdf.CellFormat(colW[0], pdfLineHeight, row.Date.UTC().Format("2006-01-02"), "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], pdfLineHeight, row.EntryID, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], pdfLineHeight, row.AccountCode, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[3], pdfLineHeight, row.Payee, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW[4], pdfLineHeight, FormatDecimal(row.Amount), "", 1, "R", false, 0, "")
	}
}
