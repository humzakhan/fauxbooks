# Implementation Plan — fbx v0.1

Nine sequential PRs implement fauxbooks v0.1 in working, tested increments. Each PR builds directly on the prior; none can start until its predecessor is merged to `main`. The authoritative design is `architecture/architecture.md` — all package boundaries, type definitions, pipeline order, and error handling described there are binding. The per-PR process follows `.agents/skills/engineering-workflow/SKILL.md`.

## Sequencing rules

- **Strict order.** PR-N cannot start until PR-(N-1) is merged to `main`.
- **Branch from `main`.** Every PR opens from `main`; no stacked branches.
- **Reviewer red flags are blocking.** See `.agents/reviewer.md` §Diff red flags. Unresolved findings prevent merge.
- **Determinism is verified package-level in PRs 02–07, then end-to-end in PR-08+.** Earlier PRs run determinism unit tests within their package; PR-08 adds the full-pipeline byte-comparison golden test.
- **No new public packages beyond the four in `architecture/architecture.md` §Packages.** (`cmd/fbx`, `internal/gen`, `internal/invariant`, `internal/writer`.) New files go inside an existing package.
- **Roadmap-deferred items are out of scope.** Anything listed in `plans/roadmap.md` v0.2+ must not appear — not as code, not as flags, not as stubs, not as `// TODO` comments.

---

## PR-01 — Repository scaffold + CLI surface

**Objective.** Stand up the repository so `go build ./...` and `go test ./...` pass, with flag parsing wired and a smoke test that validates the parsed `Config`.

**Scope.**
- `go.mod` and `go.sum` initialized; module path matches the repo.
- `cmd/fbx/main.go`: `flag` parse of `--seed`, `--period`, `--volume`, `--out`, `--format`; `fbx gen <bundle>` subcommand dispatch; exit codes 0/1/2 per `architecture/OVERVIEW.md` §Exit codes.
- `internal/gen/config.go`: `Config` struct with fields `Seed int64`, `Period time.Time`, `Volume string`, `Bundle string`, `OutDir string`, `Format string`.
- `cmd/fbx/main_test.go`: smoke test runs `fbx gen monthly-close --seed 1 --period 2026-01 --volume small` as a subprocess, asserts exit 0, and prints the parsed `Config` to stdout.
- `.github/workflows/ci.yml`: stages `go build ./...`, `go vet ./...`, `gofmt -d .`, `go test ./... -race`.

**Out of scope.**
- No event generation, no writers, no output files.
- No GitHub Actions matrix across OS/arch (added in PR-09).
- No `fbx version` subcommand (can be added in PR-08 alongside the full wire-up).

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; commit hygiene; phase-scope guard.
- `go-idioms` — `flag` usage, error wrapping, `log.Fatal`/`os.Exit` in `cmd/` only, no package-level singletons.
- `go-testing` — smoke test as subprocess; `t.Helper`; no mocks needed here.

**Tests.**
- Smoke test: subprocess exits 0 for valid flags; stderr is empty.
- Unit test: `ParseConfig` returns `ErrBadPeriod` for a malformed `--period` value.
- Unit test: `ParseConfig` returns `ErrUnknownVolume` for an unrecognized `--volume` tier.
- Unit test: `ParseConfig` returns `ErrUnknownBundle` for an unrecognized bundle name.

**Done when.**
- `go build ./...` produces a binary with no CGo.
- `go vet ./...` is clean.
- `gofmt -d .` produces no output.
- `go test ./... -race` passes.
- CI workflow runs and is green on the PR branch.
- Smoke test passes: subprocess exits 0 and stdout contains the expected `Config` fields.

**Risk.** Period parsing (`YYYY-MM` → `time.Time`) may behave differently across Go versions or locales; test this explicitly with `time.UTC` and a fixed format string to catch it before it becomes a golden-fixture mystery.

---

## PR-02 — Deterministic foundation: types + CoA + RNG

**Objective.** Establish the core types, a hardcoded ~50-account Chart of Accounts, and a seeded RNG wrapper so every downstream package has a stable, reproducible foundation.

**Scope.**
- `internal/gen/types.go`: `Event`, `JournalEntry`, `JournalLine`, `Account`, `Dataset`, `Manifest` types (fields only, as defined in `architecture/architecture.md` §Key types); `InvariantError` stub with `Invariant int` and `Detail string` fields.
- `internal/gen/coa.go`: `func DefaultCoA() []Account` (hardcoded ~50 accounts, asset/liability/equity/revenue/expense, with `NormalBalance`); `func IndexCoA([]Account) map[string]Account`.
- `internal/gen/rng.go`: `*RNG` struct wrapping a `math/rand/v2` PCG seeded from `Config.Seed`; a `gofakeit.Faker` constructed from a deterministically derived `int64`; named accessors for each random need (`IntN`, `Name`, `CompanyName`, etc.).
- `internal/gen/types_test.go` + `internal/gen/rng_test.go`: determinism tests.

**Out of scope.**
- No event generation, no templates, no writers.
- No interface definitions (`architecture/architecture.md` §Interfaces: none in v0.1).
- CoA size is exactly what ships; do not expose a config knob for it.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; no v0.2 stubs.
- `go-idioms` — no package-level singletons; `New*` constructors return concrete types; doc comments on all exported symbols.
- `go-determinism` — PCG seeded exclusively from `Config.Seed`; gofakeit seeded from derived int64, never from its global source; no `math/rand` v1.
- `go-testing` — table-driven tests; `t.Helper` in shared assertion helpers.

**Tests.**
- Same seed → identical first 100 PCG draws (`IntN` calls produce the same sequence).
- Same seed → same first 10 `gofakeit` company names.
- `IndexCoA(DefaultCoA())` contains no duplicate codes; all `NormalBalance` values are `"debit"` or `"credit"`.
- `DefaultCoA()` returns the same slice in the same order on every call (determinism across calls).

**Done when.**
- `go test ./internal/gen/... -race` passes.
- `go vet ./...` clean; `gofmt -d .` clean.
- `DefaultCoA()` contains exactly the documented number of accounts (asserted in test).
- No import of `math/rand` v1 anywhere in `internal/`.
- CI green.

**Risk.** The derived `int64` seed for `gofakeit` must be a stable function of `Config.Seed` — not a subsequent PCG draw that would shift all future draws. Document and test this derivation explicitly before PR opens, or the determinism tests will be fragile.

---

## PR-03 — Event generation

**Objective.** Generate a deterministic, volume-scaled slice of `Event` values from the seeded RNG and the period flag.

**Scope.**
- `internal/gen/events.go`: `EventType` constants; `EventMix` struct (map of type → weight); `func Generate(rng *RNG, period time.Time, volume string, mix EventMix) ([]Event, error)`. All timestamps computed as `periodStart + deterministicOffset(i)`, no `time.Now()`. Volume tier maps to event-count target: small ≈150, medium ≈1500, large ≈15000.
- `internal/gen/events_test.go`: determinism and scaling tests.

**Out of scope.**
- No journal expansion, no templates, no projection.
- No goroutines (single-threaded per `architecture/architecture.md`).
- Volume counts are approximate targets; do not add a flag to override them.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; no v0.2 stubs; commit one logical unit.
- `go-idioms` — pure function signature; errors returned, not panicked; no package-level mutable state.
- `go-determinism` — all timestamps from period arithmetic; no `time.Now()`; RNG draws from `*RNG` only; integer weights for event-type selection (no `float64` probability).
- `go-testing` — determinism test; volume-scaling test; table-driven for volume tiers.

**Tests.**
- Same seed + period + volume → identical `[]Event` (deep equal across two calls).
- Different seeds → different event slices (probabilistic sanity check).
- Volume `small` produces event count in `[120, 180]`; `medium` in `[1200, 1800]`; `large` in `[12000, 18000]`.
- No event has a timestamp outside the period month.
- No `time.Now()` call reachable from `Generate` (enforced by static assertion or test that patches `time.Now` — or simply by code review).

**Done when.**
- `go test ./internal/gen/... -race` passes.
- `gofmt -d .` clean; `go vet ./...` clean.
- Determinism test: two calls with seed=1, period=2026-01, volume=small produce byte-identical (deep-equal) slices.
- Volume scaling within ±20% of targets for all three tiers.
- CI green.

**Risk.** The event-count distribution uses weighted selection — ensure the weights are integers and the selection uses `PCG.IntN(totalWeight)`, not `float64` probability, or the golden tests will diverge across platforms.

---

## PR-04 — Templates + journal expansion

**Objective.** Expand every `Event` into balanced `JournalEntry` + `[]JournalLine` pairs via hardcoded templates; every entry balanced by arithmetic construction, not post-hoc.

**Scope.**
- `internal/gen/templates.go`: one unexported `expand<EventType>` function per event type; each constructs debit/credit legs arithmetically so Σ debits = Σ credits by construction. Public `func Expand([]Event, map[string]Account) ([]JournalEntry, []JournalLine, error)` walks events and concatenates.
- `internal/gen/templates_test.go`: balance tests per template; determinism test.

**Out of scope.**
- No projection (GL rows, bank rows, sub-ledger rows) — that is PR-05.
- No new account codes outside `DefaultCoA()` — all codes used by templates must already exist in the CoA from PR-02.
- No `JournalEntry` construction outside `internal/gen/templates.go`.

**Skills to apply.**
- `engineering-workflow` — check all template account codes exist in CoA before writing template bodies; pre-code checklist.
- `go-idioms` — pure functions; error wrapping; no package-level state.
- `go-determinism` — no `time.Now()` in templates; `OccurredAt` copied from source `Event`; amounts use `decimal.Decimal` arithmetic with `RoundBank(9)` at mint.
- `go-testing` — balance test per template at amounts `0.01`, `1.00`, `1000.00`, `9999.99`; determinism test (same events in → same entries/lines out).

**Tests.**
- For each event type: `Σ debits = Σ credits` at amounts `$0.01`, `$1`, `$1000`, `$9999.99`.
- `Expand` called twice with identical input produces byte-equal output (no hidden state).
- `Expand` returns an error (not a panic) when an event references an account code not in the CoA index.
- All `AccountCode` values in output lines exist in `DefaultCoA()`.

**Done when.**
- `go test ./internal/gen/... -race` passes.
- `go vet ./...` clean; `gofmt -d .` clean.
- Every template has a balance test asserting `Σ debits = Σ credits` for all four representative amounts.
- No `JournalEntry` is constructed outside `templates.go` (confirmed by code review).
- CI green.

**Risk.** Rounding: constructing a balanced entry with three or more legs requires explicit remainder assignment to one leg — document the pattern (last leg = total minus sum-of-others) and verify it in tests, or the invariant check in PR-06 will fire in unexpected ways.

---

## PR-05 — Projection engine

**Objective.** Project `[]JournalEntry` + `[]JournalLine` into GL rows, bank rows, and sub-ledger row sets; at the end of this PR, `internal/gen` can produce a complete `Dataset`.

**Scope.**
- `internal/gen/project.go`: `func Project(entries []JournalEntry, lines []JournalLine, coa map[string]Account) (gl []GLRow, bank []BankRow, sub SubLedgerSet, err error)`. Sorts by deterministic key (entry timestamp + EntryID + line index) before assigning row numbers. `GLRow`, `BankRow`, `SubLedgerSet` types defined here or added to `types.go`.
- `internal/gen/run.go` (or extend an existing file): `func Run(cfg Config) (Dataset, error)` wiring the full `internal/gen` pipeline: `DefaultCoA → rng.New → Generate → Expand → Project`.
- `internal/gen/project_test.go`.

**Out of scope.**
- No file I/O — projection is a pure function.
- No invariant checks — that is PR-06.
- No `internal/rng/` sub-package — RNG stays in `internal/gen/rng.go` per PR-02.

**Skills to apply.**
- `engineering-workflow` — projection is pure: no file I/O, no RNG calls, no `time.Now()`.
- `go-idioms` — pure function signature; error return; no package-level mutable state.
- `go-determinism` — sort by composite key before row numbering; no map iteration without sorted keys; bank rows derived from lines on cash account codes only.
- `go-testing` — projection determinism (same input → same output); GL row count = `len(lines)`; bank row count = count of lines on cash account codes; sub-ledger referential consistency.

**Tests.**
- `Project` called twice with identical input produces identical output slices (deep equal).
- `len(gl) == len(lines)` for a valid input.
- Bank row count equals the number of journal lines whose `AccountCode` maps to a cash account.
- Every sub-ledger row's `EntryID` exists in the input `entries` slice.
- `Run(Config{Seed:1, Period:2026-01, Volume:"small"})` returns a non-nil `Dataset` with no error.

**Done when.**
- `go test ./internal/gen/... -race` passes.
- `go vet ./...` clean; `gofmt -d .` clean.
- `internal/gen/project.go` contains no calls to `os.*`, `io.*`, or the RNG.
- `Run` returns a populated `Dataset` with all four slices non-empty for `volume=small`.
- CI green.

**Risk.** The sort key for row ordering must be stable across all supported OS/arch pairs. Use a string comparison on `(entry.OccurredAt.Format(time.RFC3339Nano), entry.EntryID, strconv.Itoa(lineIndex))` and test that the order is identical across two independent projection calls.

---

## PR-06 — Invariant checker

**Objective.** Implement `internal/invariant.Check(Dataset) error` enforcing all four write-time invariants; confirm that `Check` returns the correct `InvariantError` for each violation, and `nil` for a valid `Dataset`.

**Scope.**
- `internal/invariant/check.go`: `func Check(ds gen.Dataset) error` running invariants 1–4 in order; returns the first `InvariantError` encountered.
- `internal/invariant/error.go`: `InvariantError{Invariant int, Detail string}` implementing `error`; `func (e InvariantError) Error() string`.
- `internal/invariant/check_test.go`: one negative test per invariant + one positive test.

**Out of scope.**
- No file I/O — the checker is a pure function of `Dataset`.
- No exit-code logic — that lives in `cmd/fbx/main.go` and is wired in PR-08.
- No fifth invariant check (determinism is verified via CI golden fixtures, not at runtime).

**Skills to apply.**
- `engineering-workflow` — the checker must not write any output; it only returns an error.
- `go-idioms` — `InvariantError` implements the `error` interface; `errors.As` is the consumer pattern; no `panic`.
- `go-testing` — one negative test per invariant (construct a `Dataset` that violates exactly one, assert `InvariantError.Invariant` field matches); one positive test (valid `Dataset` returns `nil`).

**Tests.**
- Inv-1 negative: a `Dataset` with one entry where `Σ debits ≠ Σ credits` → `InvariantError{Invariant: 1}`.
- Inv-2 negative: a `Dataset` with a journal line referencing a code absent from the `CoA` → `InvariantError{Invariant: 2}`.
- Inv-3 negative: a `Dataset` where bank totals differ from GL cash postings by one cent → `InvariantError{Invariant: 3}`.
- Inv-4 negative: a `Dataset` with a sub-ledger row whose `EntryID` does not exist in `Entries` → `InvariantError{Invariant: 4}`.
- Positive: `Run(Config{Seed:1, Period:2026-01, Volume:"small"})` produces a `Dataset` for which `Check` returns `nil`.

**Done when.**
- `go test ./internal/invariant/... -race` passes.
- `go vet ./...` clean; `gofmt -d .` clean.
- All four negative tests pass; positive test passes.
- `InvariantError` satisfies `errors.As` (test this explicitly).
- CI green.

**Risk.** Invariant 3 (bank-GL tie) requires agreement on which account codes are "cash accounts" — this set must be defined in one place (a constant in `internal/gen/coa.go`) and imported by both the projection engine (PR-05) and the invariant checker. If these sets diverge, the negative test will never trigger.

---

## PR-07 — Writers: CSV + JSONL + decimal format + manifest

**Objective.** Implement all streaming output writers and the manifest; confirm decimal formatting edge cases and that manifest SHA-256 hashes are stable.

**Scope.**
- `internal/writer/format.go`: `func FormatDecimal(d decimal.Decimal) string` — 9 fractional digits, no exponent, `−0 → "0.000000000"`.
- `internal/writer/csv.go`: one streaming `csv.Writer` per output file; `bufio.NewWriterSize` wrap; `cw.UseCRLF = false`; `Flush` + close discipline with named return `err`.
- `internal/writer/jsonl.go`: streaming `json.Encoder`, one object per line, `enc.SetEscapeHTML(false)`; monetary fields serialized as strings.
- `internal/writer/manifest.go`: `func Write(ds gen.Dataset, cfg gen.Config, outDir string) error` accumulates SHA-256 per file during write, finalizes `Manifest`, writes `manifest.json`; map keys sorted before writing.
- `internal/writer/*_test.go`.

**Out of scope.**
- No end-to-end wiring (that is PR-08); `Write` can be called standalone in tests.
- No XLSX, PDF, or other formats.
- `manifest.json` does not include a `generated_at` wall-clock field in v0.1 (that would break determinism tests); if it is needed, it must be excluded from the golden-fixture diff scope — flag this as a judgment call.

**Skills to apply.**
- `engineering-workflow` — all file I/O goes through `internal/writer/`; no monetary values as JSON numbers.
- `go-idioms` — writer constructors accept `io.Writer`, not `*os.File`; tests pass `bytes.Buffer`.
- `go-io-streaming` — `bufio.NewWriterSize`; deferred Flush before Close; `cw.UseCRLF = false`; no full-buffer accumulation.
- `go-determinism` — map keys sorted before iteration in manifest and any map-derived CSV; `FormatDecimal` guards `-0`; line endings `\n`.
- `go-testing` — decimal edge cases; manifest hash stability; `Flush` called (assert via custom `io.Writer` that tracks calls).

**Tests.**
- `FormatDecimal(0)` → `"0.000000000"`.
- `FormatDecimal(-0)` → `"0.000000000"` (neg-zero guard).
- `FormatDecimal(decimal.NewFromFloat(0.000000001))` → `"0.000000001"`.
- `FormatDecimal(decimal.NewFromFloat(9999.99))` → `"9999.990000000"`.
- Manifest hash stability: write the same `Dataset` twice to two `bytes.Buffer` sinks; SHA-256 of each buffer matches.
- CSV writer: `cw.UseCRLF` is false; line endings are `\n` only (assert on output bytes).
- JSONL writer: a monetary field serialized as a JSON string, not a JSON number (unmarshal and check type).

**Done when.**
- `go test ./internal/writer/... -race` passes.
- `go vet ./...` clean; `gofmt -d .` clean.
- All decimal edge cases pass.
- Manifest hash stability test passes (same input → same hash).
- `bufio.Flush` is confirmed called in every file-write path (test or code review).
- CI green.

**Risk.** `manifest.json` field ordering: if any field is serialized via `json.Marshal` on a `map[string]interface{}` rather than a struct, key order is non-deterministic. Use a typed struct with explicit JSON tags for all manifest fields.

---

## PR-08 — End-to-end wire-up + `monthly-close` bundle + golden fixture

**Objective.** Wire `cmd/fbx` → `gen.Run` → `invariant.Check` → `writer.Write`; add the `monthly-close` bundle definition; commit the first golden fixture; add the full-pipeline determinism test.

**Scope.**
- `cmd/fbx/main.go`: wire the full pipeline; exit 0 on success, exit 1 on config error, exit 2 on invariant failure or I/O error; progress bar to stderr when stderr is a TTY.
- `internal/gen/bundle.go`: `Bundle` struct; `func MonthlyClose() Bundle` returning the event mix, volume scale, and `stressTest: false`.
- `testdata/golden/monthly-close/2026-01/small/`: byte-exact committed fixture for `seed=1, period=2026-01, volume=small`.
- `cmd/fbx/main_test.go` or `internal/gen/run_test.go`: determinism test — `Run` twice with same config, diff every output file (byte-compare).
- CI golden test: runs `go test ./... -run TestGolden`; fails on any byte difference.

**Out of scope.**
- `recon-stress-test` bundle (PR-09).
- `fbx version` subcommand (add here or omit until explicitly needed).
- CI matrix across OS/arch (PR-09).

**Skills to apply.**
- `engineering-workflow` — golden fixtures updated atomically with code; exit codes match spec exactly.
- `go-idioms` — `os.Exit` only in `cmd/fbx/main.go`; error wrapping through the stack.
- `go-determinism` — full-pipeline byte-compare determinism test is the prime artifact of this PR.
- `go-testing` — golden-fixture pattern with `-update` flag; `t.TempDir()` for isolation; `requireEqualFiles` helper with `t.Helper()`.

**Tests.**
- Determinism test: `Run(Config{Seed:1, Period:2026-01, Volume:"small"})` twice → every output file byte-identical.
- Golden test: output of seed=1 run byte-matches `testdata/golden/monthly-close/2026-01/small/`.
- Invariant abort test: a deliberately broken `Dataset` causes `cmd/fbx` to exit 2 and write no files to the output directory.
- `manifest.json` present in output; contains correct seed, period, bundle, row counts, SHA-256 for all files.

**Done when.**
- `go build ./...` produces a working binary.
- `go test ./... -race` passes, including golden and determinism tests.
- `go vet ./...` clean; `gofmt -d .` clean.
- Golden fixture committed and CI golden test is green.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out /tmp/t` exits 0 and produces all v0.1 documents listed in `architecture/OVERVIEW.md`.
- Determinism test passes: two runs produce byte-identical files.

**Risk.** The golden fixture must be regenerated with exactly the CI Go toolchain version, not a local development version that may differ. Pin the Go version in the CI workflow and regenerate fixtures on that version to avoid spurious CI failures.

---

## PR-09 — `recon-stress-test` bundle + `truth.json` + cross-platform CI

**Objective.** Add the stress-injection bundle, `truth.json` output, the second golden fixture, and a cross-platform CI matrix; v0.1 is complete when this PR merges.

**Scope.**
- `internal/gen/stress.go`: `func InjectStress(ds Dataset, rng *RNG) (Dataset, TruthLog, error)` — injects timing differences, duplicates, and orphan bank rows after the clean projection; all injections are seeded and deterministic; does not re-run the invariant checker (injections deliberately violate invariants for the `recon-stress-test` purpose).
- `internal/gen/bundle.go` (extend): `func ReconStressTest() Bundle` with `stressTest: true`.
- `internal/writer/truth.go`: writes `truth.json` from `TruthLog`.
- `testdata/golden/recon-stress-test/2026-01/small/`: byte-exact committed fixture.
- `.github/workflows/ci.yml` (extend): matrix on `linux/amd64`, `linux/arm64`, `darwin/arm64`, `windows/amd64`; golden test must pass on every cell.
- `README.md`: `go install` instructions + example `fbx gen` calls.

**Out of scope.**
- No additional invariant violations for `monthly-close` — stress injection is `recon-stress-test` only.
- No `fbx verify` subcommand (roadmap v0.2).
- No XLSX, PDF, or official formats.

**Skills to apply.**
- `engineering-workflow` — stress injection is seeded and deterministic; golden fixtures committed atomically.
- `go-determinism` — `InjectStress` uses the seeded `*RNG`; `truth.json` written with sorted keys; no goroutines.
- `go-testing` — second golden fixture; golden test runs on all CI matrix cells; `t.TempDir()` isolation.

**Tests.**
- Determinism test: `Run(Config{Bundle:"recon-stress-test", Seed:1, Period:2026-01, Volume:"small"})` twice → byte-identical.
- Golden test: output matches `testdata/golden/recon-stress-test/2026-01/small/`.
- `truth.json` schema test: unmarshal and assert every injected record has `type`, `entry_id`, and `detail` fields.
- CI matrix: all 4 OS/arch cells pass the golden test.

**Done when.**
- `go test ./... -race` passes on all four CI matrix cells.
- `go vet ./...` clean; `gofmt -d .` clean.
- Both golden fixtures committed and CI green on every matrix cell.
- `fbx gen recon-stress-test --seed 1 --period 2026-01 --volume small --out /tmp/t` exits 0 and produces `truth.json` alongside all standard documents.
- `README.md` documents `go install` and both bundle examples.
- All 9 PRs merged; v0.1 complete.

**Risk.** Cross-platform CI for `linux/arm64` and `darwin/arm64` requires hosted runners or QEMU emulation — verify GitHub Actions runner availability before writing the matrix config, or the CI step will queue forever.

---

## Cross-cutting reminders

- **No `float64` for any monetary value.** Use `shopspring/decimal` throughout — storage, arithmetic, and serialization. Even intermediate accumulations must be `decimal.Decimal`. A `float64` money value is always a bug. (`.agents/engineer.md` §Common mistakes)
- **No `time.Now()` in the generation path.** Any call to `time.Now()` or `time.Since()` in any file under `internal/` makes output non-deterministic. All timestamps derive from `--period` arithmetic. `time.Now()` is permitted only in `cmd/fbx/` for the progress bar and manifest `generated_at` (if that field is excluded from golden diffs). (`.agents/engineer.md` §Common mistakes)
- **No `math/rand` (v1) outside `internal/gen/rng.go`.** The v1 package has a global source. Use only the `*RNG` wrapper, which holds the single PCG instance seeded from `--seed`. (`.agents/engineer.md` §Common mistakes)
- **No gofakeit global functions.** `gofakeit.Name()` and similar globals draw from an auto-seeded source. Always use the `gofakeit.Faker` instance constructed inside `*RNG`. (`.agents/engineer.md` §Common mistakes)
- **Sort map keys before any output derived from them.** `for k, v := range someMap` is non-deterministic in Go. Any map whose contents flow into CSV rows, JSONL objects, or `manifest.json` must have its keys sorted before iteration. (`.agents/engineer.md` §Common mistakes)
- **No journal entries constructed outside `internal/gen/templates.go`.** Templates are the single source of balanced debit/credit pairs. Constructing an entry in the event generator, projection engine, or a test helper bypasses the template invariant. (`.agents/engineer.md` §Common mistakes)
- **Write output only after all invariants pass.** The run exits 2 and writes nothing on invariant failure. Never write partial files and check invariants afterward. (`.agents/engineer.md` §Common mistakes)
- **No goroutines in v0.1.** Goroutine scheduling is non-deterministic. Single-threaded throughout. Parallel workers are deferred to v0.3. (`.agents/skills/go-determinism/SKILL.md` Rule 5)
