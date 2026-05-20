package gen

import (
	"time"

	"github.com/shopspring/decimal"
)

// EventType discriminates the template dispatch for each business event.
type EventType string

// Event is a single business event in the generation stream.
// Every field is deterministic: OccurredAt derives from --period arithmetic,
// Amount uses decimal.Decimal, and ID is assigned from a counter.
type Event struct {
	// ID uniquely identifies this event within a run.
	ID string

	// Type selects the template used to expand this event into journal lines.
	Type EventType

	// OccurredAt is the event timestamp, derived from period + deterministic offset.
	OccurredAt time.Time

	// Amount is the primary monetary value of the event. Never float64.
	Amount decimal.Decimal

	// RefID links to a customer or vendor master record.
	RefID string

	// Metadata carries event-specific ancillary data (e.g. invoice number).
	Metadata map[string]string
}

// JournalEntry is the header record for a double-entry accounting entry.
// Entries are constructed exclusively in internal/gen/templates.go.
type JournalEntry struct {
	// EntryID uniquely identifies this entry within a run; referenced by sub-ledger rows.
	EntryID string

	// OccurredAt is copied from the source Event.
	OccurredAt time.Time

	// Description is a human-readable label for the entry.
	Description string

	// SourceEventID traces this entry back to the originating Event.
	SourceEventID string
}

// JournalLine is one leg (debit or credit) of a JournalEntry.
// Exactly one of Debit and Credit is non-zero for any given line.
type JournalLine struct {
	// LineID uniquely identifies this line within a run.
	LineID string

	// EntryID is a foreign key to the parent JournalEntry.
	EntryID string

	// AccountCode must exist in the CoA for this run (invariant 2).
	AccountCode string

	// Debit is the debit amount; zero when this leg is a credit.
	Debit decimal.Decimal

	// Credit is the credit amount; zero when this leg is a debit.
	Credit decimal.Decimal
}

// AccountType classifies an account in the chart of accounts.
type AccountType string

const (
	// AccountTypeAsset represents asset accounts (normal balance: debit).
	AccountTypeAsset AccountType = "asset"

	// AccountTypeLiability represents liability accounts (normal balance: credit).
	AccountTypeLiability AccountType = "liability"

	// AccountTypeEquity represents equity accounts (normal balance: credit).
	AccountTypeEquity AccountType = "equity"

	// AccountTypeRevenue represents revenue accounts (normal balance: credit).
	AccountTypeRevenue AccountType = "revenue"

	// AccountTypeExpense represents expense accounts (normal balance: debit).
	AccountTypeExpense AccountType = "expense"
)

// Account is one entry in the chart of accounts.
type Account struct {
	// Code is the unique CoA key (e.g. "1000").
	Code string

	// Name is the human-readable display name.
	Name string

	// Type classifies the account: asset, liability, equity, revenue, or expense.
	Type AccountType

	// NormalBalance is "debit" for asset/expense accounts, "credit" for others.
	NormalBalance string
}

// GLRow is one row in the general ledger projection.
// Row count equals len(Lines); every journal line produces exactly one GL row.
type GLRow struct {
	// RowNum is the 1-based sequential position assigned after sorting by composite key.
	RowNum int

	// EntryID is the parent journal entry identifier.
	EntryID string

	// LineID is the source journal line identifier.
	LineID string

	// OccurredAt is the timestamp of the parent journal entry.
	OccurredAt string // RFC3339Nano

	// AccountCode is the account referenced by the journal line.
	AccountCode string

	// Debit is the debit amount for this line; zero string when the leg is a credit.
	Debit string // decimal text, 9 fractional digits

	// Credit is the credit amount for this line; zero string when the leg is a debit.
	Credit string // decimal text, 9 fractional digits
}

// BankRow is one row in the bank statement projection.
// Only journal lines posted to cash account codes produce bank rows.
type BankRow struct {
	// RowNum is the 1-based sequential position within the bank statement.
	RowNum int

	// EntryID is the parent journal entry identifier.
	EntryID string

	// LineID is the source journal line identifier.
	LineID string

	// OccurredAt is the timestamp of the parent journal entry.
	OccurredAt string // RFC3339Nano

	// AccountCode is the cash account code.
	AccountCode string

	// Debit is the cash debit (inflow) amount; zero string when this is an outflow.
	Debit string // decimal text, 9 fractional digits

	// Credit is the cash credit (outflow) amount; zero string when this is an inflow.
	Credit string // decimal text, 9 fractional digits
}

// SubLedgerRow is one row in a sub-ledger document.
// Every sub-ledger row references an existing journal entry (invariant 4).
type SubLedgerRow struct {
	// RowNum is the 1-based sequential position within this sub-ledger.
	RowNum int

	// EntryID is the journal entry identifier for this sub-ledger record.
	EntryID string

	// RefID is the customer or vendor master reference (e.g. "ref-001").
	RefID string

	// OccurredAt is the timestamp of the source event.
	OccurredAt string // RFC3339Nano

	// Amount is the monetary amount in decimal text with 9 fractional digits.
	Amount string

	// SubLedgerType classifies the sub-ledger document (e.g. "invoice", "bill").
	SubLedgerType string
}

// SubLedgerSet holds the complete set of sub-ledger projections for a run.
type SubLedgerSet struct {
	// Invoices holds sub-ledger rows for sale invoice events.
	Invoices []SubLedgerRow

	// CustomerPayments holds sub-ledger rows for customer payment events.
	CustomerPayments []SubLedgerRow

	// Bills holds sub-ledger rows for vendor bill events.
	Bills []SubLedgerRow

	// BillPayments holds sub-ledger rows for bill payment events.
	BillPayments []SubLedgerRow

	// ExpenseReports holds sub-ledger rows for expense report events.
	ExpenseReports []SubLedgerRow
}

// Customer is a customer master record.
// All fields are deterministic: names and contact details come from the seeded RNG.
type Customer struct {
	// ID is the stable customer identifier within a run (e.g. "cust-001").
	ID string

	// Name is the fake company name for this customer.
	Name string

	// Email is the fake contact email for this customer.
	Email string

	// Phone is the fake contact phone number.
	Phone string

	// Address is the fake street address.
	Address string

	// City is the fake city.
	City string

	// State is the fake US state abbreviation.
	State string

	// Zip is the fake US ZIP code.
	Zip string
}

// Vendor is a vendor master record.
// All fields are deterministic: names and contact details come from the seeded RNG.
type Vendor struct {
	// ID is the stable vendor identifier within a run (e.g. "vend-001").
	ID string

	// Name is the fake company name for this vendor.
	Name string

	// Email is the fake contact email for this vendor.
	Email string

	// Phone is the fake contact phone number.
	Phone string

	// Address is the fake street address.
	Address string

	// City is the fake city.
	City string

	// State is the fake US state abbreviation.
	State string

	// Zip is the fake US ZIP code.
	Zip string
}

// TruthRecord describes one anomaly injected by InjectStress.
// Every record has all three fields populated; no field is optional.
type TruthRecord struct {
	// Type classifies the anomaly: "timing_difference", "duplicate", or "orphan_bank_row".
	Type string `json:"type"`

	// EntryID is the journal entry or bank row identifier involved in the injection.
	EntryID string `json:"entry_id"`

	// Detail is a human-readable description of the injected anomaly.
	Detail string `json:"detail"`
}

// TruthLog is the ordered list of injected anomalies for a recon-stress-test run.
// It is sorted by (Type, EntryID) before write so truth.json is deterministic.
type TruthLog []TruthRecord

// Dataset is the full in-memory dataset produced by a generation run.
// All slices are populated before any write or invariant check occurs.
type Dataset struct {
	// Entries is the complete list of journal entry headers.
	Entries []JournalEntry

	// Lines is the complete list of journal lines across all entries.
	Lines []JournalLine

	// CoA is the chart of accounts index for this run, keyed by account code.
	CoA map[string]Account

	// GL is the general ledger projection (one row per journal line).
	GL []GLRow

	// Bank is the bank statement projection (one row per cash-account journal line).
	Bank []BankRow

	// SubLedger is the sub-ledger projection set.
	SubLedger SubLedgerSet

	// Customers is the customer master list for this run.
	Customers []Customer

	// Vendors is the vendor master list for this run.
	Vendors []Vendor

	// Manifest records run identity, row counts, and file hashes (hashes populated by writer.Write).
	Manifest Manifest

	// Truth holds the injected anomaly log for recon-stress-test runs.
	// Nil for standard bundles (StressTest: false).
	Truth TruthLog
}

// Manifest records run identity, row counts, and per-file SHA-256 hashes.
// It is written as manifest.json after all invariants pass.
type Manifest struct {
	// Seed is the --seed value used for this run.
	Seed int64

	// Period is the --period value, formatted as YYYY-MM.
	Period string

	// Bundle is the --bundle name used for this run.
	Bundle string

	// Version is the fbx binary version that produced this dataset.
	Version string

	// RowCounts maps each output filename to its row count.
	RowCounts map[string]int

	// Hashes maps each output filename to its SHA-256 hex digest.
	Hashes map[string]string
}

// InvariantError is returned by internal/invariant.Check when a write-time
// invariant is violated. The Invariant field matches the numbering in
// architecture/OVERVIEW.md §Invariants (1–4).
type InvariantError struct {
	// Invariant is the invariant number that was violated (1–4).
	Invariant int

	// Detail identifies the offending entry or row.
	Detail string
}

// Error implements the error interface.
func (e InvariantError) Error() string {
	return "invariant " + itoa(e.Invariant) + " violated: " + e.Detail
}

// itoa converts a small non-negative integer to its decimal string representation
// without importing strconv (avoids a circular import risk in future refactors).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
