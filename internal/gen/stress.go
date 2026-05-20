package gen

import (
	"fmt"
	"sort"
	"time"
)

// stressTimingCount is the number of bank rows that receive a timing difference
// injection for volume=small. Derived from the RNG so it is deterministic.
// The actual count is rng.IntN(stressTimingMax) + stressTimingMin.
const (
	stressTimingMin = 2
	stressTimingMax = 4
	stressDupMin    = 1
	stressDupMax    = 3
	stressOrphanMin = 1
	stressOrphanMax = 3
)

// InjectStress injects three classes of anomaly into a clean Dataset and
// returns the modified Dataset and a TruthLog describing every injection.
// It is called only for recon-stress-test bundles (StressTest: true).
//
// Anomaly classes:
//  1. Timing differences — a subset of bank rows have their OccurredAt shifted
//     by one or more days relative to the corresponding GL row. The GL is
//     unchanged; only the bank row date moves.
//  2. Duplicates — a small number of bank rows are duplicated verbatim. The
//     duplicate receives a new RowNum but identical content.
//  3. Orphan bank rows — new bank rows with no matching GL entry are appended.
//     These simulate transactions that cleared the bank but were never booked.
//
// InjectStress does NOT run the invariant checker. The injections deliberately
// violate invariants 1–3. The caller (gen.Run) must skip invariant.Check for
// stress bundles.
//
// All randomness flows exclusively from rng. The returned Dataset and TruthLog
// are deterministic for a given seed.
func InjectStress(ds Dataset, rng *RNG) (Dataset, TruthLog, error) {
	if len(ds.Bank) == 0 {
		return ds, nil, fmt.Errorf("gen.InjectStress: bank statement is empty; cannot inject anomalies")
	}

	var log TruthLog

	// Work on a copy of the bank rows so the original GL is untouched.
	bank := make([]BankRow, len(ds.Bank))
	copy(bank, ds.Bank)

	// ── 1. Timing differences ────────────────────────────────────────────────
	// Pick a random count, then pick distinct row indices to shift.
	timingCount := stressTimingMin + rng.IntN(stressTimingMax-stressTimingMin+1)
	if timingCount > len(bank) {
		timingCount = len(bank)
	}
	timingIndices := distinctIndices(rng, len(bank), timingCount)
	for _, idx := range timingIndices {
		// Shift by 1–3 days (deterministic via RNG).
		shiftDays := 1 + rng.IntN(3)
		original := bank[idx].OccurredAt
		shifted, err := shiftDate(original, shiftDays)
		if err != nil {
			return Dataset{}, nil, fmt.Errorf("gen.InjectStress: timing shift: %w", err)
		}
		bank[idx].OccurredAt = shifted
		log = append(log, TruthRecord{
			Type:    "timing_difference",
			EntryID: bank[idx].EntryID,
			Detail:  fmt.Sprintf("bank row date shifted +%d day(s): %s -> %s", shiftDays, original, shifted),
		})
	}

	// ── 2. Duplicates ────────────────────────────────────────────────────────
	dupCount := stressDupMin + rng.IntN(stressDupMax-stressDupMin+1)
	if dupCount > len(bank) {
		dupCount = len(bank)
	}
	dupIndices := distinctIndices(rng, len(bank), dupCount)
	dupRows := make([]BankRow, 0, dupCount)
	for _, idx := range dupIndices {
		dup := bank[idx]
		// RowNum will be re-assigned after appending; use a placeholder.
		dup.RowNum = 0
		log = append(log, TruthRecord{
			Type:    "duplicate",
			EntryID: dup.EntryID,
			Detail:  fmt.Sprintf("bank row duplicated (source RowNum %d)", bank[idx].RowNum),
		})
		dupRows = append(dupRows, dup)
	}
	bank = append(bank, dupRows...)

	// ── 3. Orphan bank rows ──────────────────────────────────────────────────
	orphanCount := stressOrphanMin + rng.IntN(stressOrphanMax-stressOrphanMin+1)
	// Determine a reference date from the first bank row for period-correct timestamps.
	refDate := ds.Bank[0].OccurredAt

	cashCodes := sortedCashCodes()
	for i := range orphanCount {
		// Pick a cash account code deterministically.
		code := cashCodes[rng.IntN(len(cashCodes))]

		// Derive a period-correct date by shifting the reference date.
		dayOffset := rng.IntN(28) // 0–27 days within the month
		orphanDate, err := shiftDate(refDate, dayOffset)
		if err != nil {
			return Dataset{}, nil, fmt.Errorf("gen.InjectStress: orphan date: %w", err)
		}

		orphanID := fmt.Sprintf("orphan-%03d", i+1)
		// Amount: a small deterministic integer cent value (1–999 dollars).
		centValue := int64(1 + rng.IntN(999))
		// Format as decimal text with 9 fractional digits.
		amount := fmt.Sprintf("%d.000000000", centValue)

		orphan := BankRow{
			RowNum:      0, // re-assigned below
			EntryID:     orphanID,
			LineID:      orphanID + "-line",
			OccurredAt:  orphanDate,
			AccountCode: code,
			Debit:       amount,
			Credit:      "0.000000000",
		}
		log = append(log, TruthRecord{
			Type:    "orphan_bank_row",
			EntryID: orphanID,
			Detail:  fmt.Sprintf("orphan bank row with no GL match: account %s amount %s on %s", code, amount, orphanDate),
		})
		bank = append(bank, orphan)
	}

	// Re-assign RowNums in the order rows appear (no sort — preserve injection
	// order to keep the output deterministic given the same RNG draws).
	for i := range bank {
		bank[i].RowNum = i + 1
	}

	// Sort TruthLog by (Type, EntryID) so truth.json is deterministic regardless
	// of the order anomalies were appended above.
	sort.Slice(log, func(i, j int) bool {
		if log[i].Type != log[j].Type {
			return log[i].Type < log[j].Type
		}
		return log[i].EntryID < log[j].EntryID
	})

	out := ds
	out.Bank = bank
	return out, log, nil
}

// distinctIndices returns count distinct random indices in [0, n).
// Indices are chosen without replacement using reservoir-style selection
// to keep the number of RNG draws predictable and deterministic.
func distinctIndices(rng *RNG, n, count int) []int {
	// Fisher-Yates partial shuffle on [0, n-1].
	pool := make([]int, n)
	for i := range pool {
		pool[i] = i
	}
	for i := range count {
		j := i + rng.IntN(n-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	result := make([]int, count)
	copy(result, pool[:count])
	return result
}

// shiftDate parses an RFC3339Nano string, adds days, and returns the result
// formatted the same way. It is used only by InjectStress and is pure.
func shiftDate(rfc3339 string, days int) (string, error) {
	t, err := time.Parse("2006-01-02T15:04:05.999999999Z", rfc3339)
	if err != nil {
		return "", fmt.Errorf("shiftDate: parse %q: %w", rfc3339, err)
	}
	shifted := t.AddDate(0, 0, days)
	return shifted.UTC().Format("2006-01-02T15:04:05.999999999Z"), nil
}

// sortedCashCodes returns the cash account codes in sorted order for
// deterministic selection in orphan row injection.
func sortedCashCodes() []string {
	codes := make([]string, 0, len(cashAccountCodes))
	for code := range cashAccountCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
