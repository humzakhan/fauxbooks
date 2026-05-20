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

// Dataset is the full in-memory dataset produced by a generation run.
// All four slices are populated before any write or invariant check occurs.
type Dataset struct {
	// Entries is the complete list of journal entry headers.
	Entries []JournalEntry

	// Lines is the complete list of journal lines across all entries.
	Lines []JournalLine

	// CoA is the chart of accounts index for this run, keyed by account code.
	CoA map[string]Account
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
