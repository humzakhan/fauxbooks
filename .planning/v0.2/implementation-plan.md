# Implementation Plan — fbx v0.2

Eight sequential PRs extend fauxbooks with derived financial reports, new output formats (XLSX, PDF), standalone verification and conversion subcommands, YAML-configurable bundles and templates, and an MCP stdio server. Each PR builds directly on the prior; none can start until its predecessor is merged to `main`. The authoritative design is `architecture/architecture.md` — all package boundaries, type definitions, pipeline order, and error handling described there are binding. The per-PR process follows `.agents/skills/engineering-workflow/SKILL.md`.

## Sequencing rules

- **Strict order.** PR-V0N cannot start until PR-V0(N-1) is merged to `main`.
- **Branch from `main`.** Every PR opens from `main`; no stacked branches.
- **Branch naming.** `pr/v0N-<slug>` (e.g. `pr/v01-financial-reports`).
- **Commit prefixes.** Use `feat` for new capabilities, `refactor` for internal restructuring, `test` for test-only changes, `docs` for documentation-only changes.
- **Reviewer red flags are blocking.** See `.agents/reviewer.md` §Diff red flags. Unresolved findings prevent merge.
- **Golden fixtures updated atomically.** Any PR that changes output schema or adds output files must update `testdata/golden/` in the same commit. CI is byte-comparison; no staged breakage.
- **No new public packages beyond the six listed below.** The v0.1 four (`cmd/fbx`, `internal/gen`, `internal/invariant`, `internal/writer`) plus the two new v0.2 packages (`internal/report`, `internal/reader`). New files go inside an existing package wherever possible. The `internal/reader` package is introduced in PR-V05; `internal/report` is introduced in PR-V01.
- **Roadmap-deferred items are out of scope.** Anything listed in `plans/roadmap.md` v0.3+ must not appear — not as code, not as flags, not as stubs, not as `// TODO` comments.
- **v0.1 invariants are preserved.** No PR may weaken invariants 1–4 or the determinism invariant. Every PR that touches the generation path runs `go test ./... -race` and confirms golden fixtures for both existing bundles remain byte-identical before introducing new fixtures.

---

## PR-V01 — `internal/report` + financial statements (CSV + JSONL)

**Objective.** Introduce a new `internal/report` package that computes P&L, Balance Sheet, Cash Flow Statement, and Trial Balance as typed structs derived purely from the in-memory `gen.Dataset`. Add CSV + JSONL writers for each report and update the manifest to include the new files.

**Scope.**
- `internal/report/compute.go`: `func Compute(ds gen.Dataset) Reports` returning a `Reports` struct with four fields: `ProfitLoss ProfitLoss`, `BalanceSheet BalanceSheet`, `CashFlow CashFlow`, `TrialBalance TrialBalance`. All computation derives from `ds.Lines`, `ds.GLRows`, and `ds.CoA`; no RNG calls; no `time.Now()`.
- `internal/report/types.go`: typed structs for each report. Monetary fields are `decimal.Decimal`. Period-level aggregations (revenue, expense, asset, liability, equity buckets) derived by iterating `ds.CoA` account types with sorted key access.
- `internal/writer/reports.go`: streaming CSV + JSONL writers for each of the four report documents. Follows `go-io-streaming` patterns: `bufio.NewWriterSize`, deferred `Flush`, one row at a time. Output files: `profit-loss.csv/.jsonl`, `balance-sheet.csv/.jsonl`, `cash-flow.csv/.jsonl`, `trial-balance.csv/.jsonl`.
- `internal/writer/manifest.go` (extend): add hash entries for the 8 new files.
- `testdata/golden/monthly-close/2026-01/small/`: update with the 8 new report files.
- `testdata/golden/recon-stress-test/2026-01/small/`: update with the 8 new report files.
- `cmd/fbx/main.go` (extend): wire `report.Compute` into the pipeline after `invariant.Check`, pass `Reports` to `writer.Write`.

**Out of scope.**
- Aging reports, customer statements, check register (PR-V02).
- XLSX or PDF rendering of any report (PR-V03, PR-V04).
- Chart-of-accounts changes: all account types already exist in the hardcoded CoA from v0.1.
- New CLI flags: the existing `--format csv|jsonl` flag selects format for all output including the new reports.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; update golden fixtures atomically; confirm v0.1 fixtures unchanged.
- `go-idioms` — pure function `Compute` with no side effects; exported types with doc comments; no package-level mutable state.
- `go-determinism` — CoA iteration sorted by account code key; all decimal accumulation via `decimal.Decimal`; no `float64` at any aggregation step; no `time.Now()` in `internal/report/`.
- `go-io-streaming` — each report writer emits one row at a time; `bufio.NewWriterSize` wrap; deferred `Flush`; no full-report in-memory accumulation before writing.
- `go-testing` — golden-fixture pattern with `-update` flag; determinism test for `Compute` (same input → same output, two calls).

**Tests.**
- `Compute` called twice with the same `Dataset` returns structurally identical `Reports` (deep equal).
- Trial balance: sum of all debit-normal account balances equals sum of all credit-normal account balances across the trial balance rows (double-entry conservation).
- P&L net income equals revenue total minus expense total (arithmetic identity test).
- Balance sheet: assets = liabilities + equity (fundamental accounting equation, asserted in test).
- Cash flow: ending cash derived from GL cash-account postings matches the balance sheet cash balance.
- Golden fixture update: `testdata/golden/monthly-close/2026-01/small/` and `recon-stress-test` include all 8 new files; byte-identical on re-run.
- `manifest.json` in each golden fixture now lists 8 additional files with SHA-256 and row counts.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including updated golden fixtures for both bundles.
- `internal/report/` contains no calls to `os.*`, the RNG, or `time.Now()`.
- `Compute` is a pure function with no exported mutable state.
- Both existing golden fixture sets updated and CI green.

**Risk.** CoA account-type bucketing: the P&L, balance sheet, and cash flow derivations depend on consistent classification of account codes as asset/liability/equity/revenue/expense. If any account in `internal/gen/coa.go` is miscategorized (e.g., a contra-revenue account coded as an expense), the accounting-equation test will fire unexpectedly. Audit every account code against its `Type` field before implementing the aggregation logic, and assert the equation identities in tests — not just that numbers are non-zero.

---

## PR-V02 — Aging reports + customer statements + check register

**Objective.** Add AR/AP aging by bucket, per-customer statements, and a check register as new output documents derived from the existing `gen.Dataset`. Four new CSV+JSONL output pairs; manifest and golden fixtures updated.

**Scope.**
- `internal/report/aging.go`: `func ComputeAging(ds gen.Dataset, periodEnd time.Time) AgingReport` returning `ARAgingReport` (rows: customer × bucket 0-30, 31-60, 61-90, 91+ days, open balance) and `APAgingReport` (rows: vendor × same buckets). `periodEnd` is derived from `Config.Period`, not from `time.Now()`.
- `internal/report/statements.go`: `func ComputeStatements(ds gen.Dataset) []CustomerStatement` where each `CustomerStatement` has customer ID, opening balance, period activity (invoiced, payments received), and closing balance. Derived from AR sub-ledger rows and customer payment rows in the Dataset.
- `internal/report/checkregister.go`: `func ComputeCheckRegister(ds gen.Dataset) []CheckRegisterRow` — a filtered GL view: journal lines that credit a cash account and are classified as payments to vendors (AP disbursements, bill payments). Sorted by date, then entry ID.
- `internal/writer/aging.go`, `internal/writer/statements.go`, `internal/writer/checkregister.go`: streaming CSV + JSONL writers for each new document. Output files: `ar-aging.csv/.jsonl`, `ap-aging.csv/.jsonl`, `customer-statements.csv/.jsonl`, `check-register.csv/.jsonl`.
- `internal/writer/manifest.go` (extend): 8 additional hash entries.
- `testdata/golden/` (both bundles): update with 8 new files.
- `cmd/fbx/main.go` (extend): wire the three new compute calls into the post-`invariant.Check` pipeline.

**Out of scope.**
- XLSX or PDF rendering (PR-V03, PR-V04).
- Multi-period aging (each run is a single period; inter-period carry-forward is v0.3+).
- Invoice-level PDF statements (PR-V04 covers statement PDF layout).

**Skills to apply.**
- `engineering-workflow` — pure compute functions; golden fixtures updated atomically; v0.1 fixture byte-identity confirmed.
- `go-idioms` — pure functions; `periodEnd` derived from `Config.Period + one month - 1 day`; no package-level state.
- `go-determinism` — aging bucket assignment is deterministic arithmetic on `OccurredAt`; customer statement ordering sorted by customer ID; check register sorted by `(OccurredAt, EntryID)` composite key.
- `go-io-streaming` — streaming writers; one row per write call; `bufio.NewWriterSize`.
- `go-testing` — aging boundary test (invoice exactly 30 days old falls in 0-30 bucket; 31 days falls in 31-60 bucket); customer statement identity (opening + invoiced - payments = closing); check register count equals AP disbursement event count.

**Tests.**
- AR aging bucket boundary: an invoice dated exactly 30 days before `periodEnd` appears in the 0-30 bucket; one dated 31 days before appears in the 31-60 bucket.
- AP aging: same boundary logic for vendor bills.
- Customer statement arithmetic: `opening_balance + invoiced_amount - payments_received = closing_balance` for every customer in the fixture (asserted across the full golden dataset, not just a synthetic one).
- Check register: row count equals the number of `BillPayment` + `ExpenseReport` events in the Dataset (AP cash credits only); no AR payment rows included.
- Golden fixture update: both bundles updated and byte-identical on re-run.
- `manifest.json` includes all 8 new files.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including updated golden fixtures for both bundles.
- `ComputeAging`, `ComputeStatements`, `ComputeCheckRegister` contain no calls to `time.Now()`, the RNG, or `os.*`.
- Aging bucket assignment is tested at the exact boundary (30 days, 31 days).
- CI green.

**Risk.** Aging dates require `periodEnd` to be derived from `Config.Period` without calling `time.Now()`. The idiomatic Go expression is `periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)` to get the last nanosecond of the period month. Test this explicitly across period values that span month-length differences (e.g., February), or the bucket assignment will be off by one day for short months and the golden fixture will differ across period values.

---

## PR-V03 — XLSX output format

**Objective.** Add `--format xlsx` as a third output format alongside the existing `csv` and `jsonl`. Every document currently written as CSV is instead written to a single XLSX workbook, one sheet per document. Decimals are written as text to preserve precision. Sheets auto-split at 1,048,576 rows.

**Scope.**
- `go.mod` / `go.sum`: add `github.com/xuri/excelize/v2` dependency.
- `internal/writer/xlsx.go`: `func WriteXLSX(ds gen.Dataset, reports report.Reports, cfg gen.Config, outDir string) error`. Uses `excelize.StreamWriter` for each sheet; one call to `SetRow` per data row; decimals written as string cells (not numeric); column headers in row 1; sheet split logic at 1,048,576 rows (creates `SheetName_2`, etc.).
- Determinism: ZIP entries inside `.xlsx` are ZIP archives — `excelize` allows setting file modification timestamps. Set all modification timestamps to the period start (`Config.Period`) so the binary is byte-identical across runs. Verify this explicitly.
- `cmd/fbx/main.go` (extend): add `xlsx` to the valid values for `--format`; dispatch to `writer.WriteXLSX` when selected.
- `testdata/golden/monthly-close/2026-01/small/xlsx/`: committed binary golden fixture for seed=1; CI diff is `cmp.Diff` on the raw bytes.
- `testdata/golden/recon-stress-test/2026-01/small/xlsx/`: same.

**Out of scope.**
- PDF rendering (PR-V04).
- `fbx convert` (PR-V06 handles XLSX as a conversion target).
- Formula cells: decimals are text strings, not Excel formulas.
- Conditional formatting, pivot tables, charts.

**Skills to apply.**
- `engineering-workflow` — golden fixtures committed atomically; confirm v0.1 CSV/JSONL golden fixtures byte-identical (XLSX is additive, not replacing).
- `go-idioms` — no CGo; `excelize` is a pure-Go library; writer accepts `io.Writer` or output path with `os.Create` inside `WriteXLSX`.
- `go-determinism` — ZIP modification timestamps pinned to `Config.Period`; sheet and column ordering follows the same deterministic order as CSV headers; map key sort for any map-derived columns.
- `go-io-streaming` — `excelize.StreamWriter` is the streaming API; do not accumulate a full sheet in memory; flush the stream after each sheet.
- `go-testing` — binary golden fixture with `-update` flag; byte-identical determinism test (two calls, same seed → same bytes); round-trip test: parse XLSX output with `excelize.OpenFile`, verify decimal text cells preserved as written.

**Tests.**
- Determinism: `WriteXLSX` called twice with identical inputs produces byte-identical output files.
- Decimal preservation: parse written XLSX cells for a monetary column; assert the string value equals what `FormatDecimal` would produce (not a float approximation).
- Sheet split: a synthetic dataset with exactly 1,048,576 + 1 rows causes a second sheet to be created.
- Golden fixture: byte-identical to committed fixture for both bundles at seed=1.
- Existing CSV/JSONL golden fixtures unchanged by this PR (regression check).

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including XLSX golden fixtures.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format xlsx --out /tmp/t` exits 0 and produces a single `.xlsx` file.
- Decimal cells in the XLSX are string type, not numeric type (verifiable by unzipping the XLSX and inspecting the `xl/worksheets/sheet1.xml` cell type attribute: `t="s"` for shared string, not empty/numeric).
- ZIP entry modification timestamps are pinned (verify by inspecting the ZIP directory with `unzip -v`).
- CI green on all matrix cells.

**Risk.** `excelize`'s `StreamWriter` does not expose a direct API for setting ZIP entry modification times as of v2. The workaround requires writing to a `bytes.Buffer`, then rewriting the ZIP central directory with `archive/zip` to override modtimes before flushing to disk. This is non-trivial; spike it first (build a 5-line test that writes a minimal XLSX and checks modtime) before implementing the full writer. If the workaround proves fragile across `excelize` versions, document the known modtime variance and exclude the XLSX file's binary hash from the CI byte-comparison (comparing sheet content via parse instead).

NOTE: No existing skill covers XLSX determinism (ZIP modtime pinning). Treat the modtime-pinning approach as a plan-level constraint; if the spike shows it cannot be done cleanly, fall back to content-level comparison (parse sheets, compare cell values) rather than binary comparison for the XLSX golden fixture.

---

## PR-V04 — PDF rendering + `--company-config`

**Objective.** Add `--format pdf` for a defined document subset using `gofpdf`, and introduce `--company-config <path.yaml>` for per-tenant branding (name, logo, address, phone, email). PDF output is byte-deterministic: creation timestamp pinned to `Config.Period`, fonts embedded and OS-independent.

**Scope.**
- `go.mod` / `go.sum`: add `github.com/jung-kurt/gofpdf` dependency (pure Go, no CGo).
- `internal/writer/pdf.go`: `func WritePDF(subset PDFDocSet, cfg gen.Config, companyCfg CompanyConfig, outDir string) error`. Renders: `profit-loss.pdf`, `balance-sheet.pdf`, `cash-flow.pdf`, `trial-balance.pdf`, `ar-aging.pdf`, `ap-aging.pdf`, `customer-statements.pdf`, `check-register.pdf`. Each document is one PDF file (not a merged omnibus).
- `internal/writer/companyconfig.go`: `type CompanyConfig struct { CompanyName, Address, Phone, Email string; LogoPath string }`. `func LoadCompanyConfig(path string) (CompanyConfig, error)` parses a YAML file. A hardcoded `DefaultCompanyConfig()` is used when no flag is provided (ensures golden-fixture reproducibility without requiring a real config file in CI).
- `cmd/fbx/main.go` (extend): add `--company-config <path>` flag; add `pdf` to valid `--format` values. `--format pdf` is valid only for the 8 document types listed above; if requested for `journal_entries`, `gl`, `bank_statement`, `customers`, `vendors`, `invoices`, `bills`, `bill_payments`, `customer_payments`, `expense_reports` — return a clear error (exit 1) explaining which documents support PDF.
- PDF determinism: set `gofpdf`'s `CreationDate` and `ModDate` fields to `Config.Period` (first day of the period). Embed the font metrics directly (use `gofpdf`'s built-in standard fonts — Helvetica, Times — which are specification-defined and produce identical output across OS). Do not embed OS-resident fonts (e.g., system Arial or platform-default sans-serif).
- `testdata/golden/monthly-close/2026-01/small/pdf/`: committed PDF golden fixture for seed=1, using `DefaultCompanyConfig()`.
- `testdata/golden/recon-stress-test/2026-01/small/pdf/`: same.

**Out of scope.**
- Typst (vendored binary): deferred to v0.3 consideration per roadmap.
- Invoice/bill PDFs: these require richer per-line-item layout; deferred.
- PDF merging or bundling into a single file.
- PDF digital signatures or encryption.
- `--format pdf` for the sub-ledger documents (invoices, bills, customers, vendors, GL, journal entries, bank statement).

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; `--company-config` flag validated at parse time, not at write time.
- `go-idioms` — `LoadCompanyConfig` returns `(CompanyConfig, error)`; `DefaultCompanyConfig()` is a constructor returning a concrete value; no package-level `var` holding the config.
- `go-determinism` — `gofpdf` creation date pinned to period start; built-in standard fonts only; `CompanyConfig` fields come from YAML or the hardcoded default, never from `time.Now()` or OS environment.
- `go-testing` — PDF golden fixture with `-update` flag; byte-identical determinism test.

**Tests.**
- `WritePDF` called twice with identical inputs (same `Dataset`, same `CompanyConfig`, same `Config`) produces byte-identical output files.
- `--format pdf` with a document type outside the supported set (e.g., `--doc gl`) exits 1 with a clear error message naming the unsupported document.
- `LoadCompanyConfig` returns a populated `CompanyConfig` when given a valid YAML file.
- `LoadCompanyConfig` returns an error (not a panic) on a missing or malformed YAML file.
- PDF golden fixture: both bundles at seed=1 using `DefaultCompanyConfig()` produce byte-identical output across two runs.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including PDF golden fixtures.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format pdf --out /tmp/t` exits 0 and produces 8 PDF files.
- PDF golden fixture is byte-identical across two runs on the same machine (determinism test passes).
- Existing CSV, JSONL, and XLSX golden fixtures unchanged.
- CI green.

**Risk.** `gofpdf` standard-font metrics are defined by the PDF specification and are OS-independent, but the library may embed a creation timestamp in sub-second resolution via the system clock if not explicitly overridden. Inspect the generated PDF's `/CreationDate` field in the cross-reference table before committing the golden fixture. If `gofpdf` writes the current time anywhere other than the explicitly set `CreationDate`, the golden test will flap across runs. Pin or strip all timestamps and verify by binary-diffing two runs before finalizing the fixture.

NOTE: No existing skill covers PDF determinism specifically. Apply `go-determinism` broadly (no `time.Now()` in the PDF write path) and manually verify the `gofpdf` timestamp fields before committing the golden fixture.

---

## PR-V05 — `fbx verify` subcommand

**Objective.** Add `fbx verify <dir>` which reads CSV files from an existing fbx output directory, reconstructs an in-memory `gen.Dataset`, runs `invariant.Check`, and exits 0 on pass or 2 on failure. Introduces a new `internal/reader` package.

**Scope.**
- `internal/reader/csv.go`: `func Load(dir string) (gen.Dataset, error)` — reads the standard CSV output files from `dir`, parses decimal text columns via `decimal.NewFromString`, reconstructs `gen.Dataset` with populated `Entries`, `Lines`, `GLRows`, `BankRows`, `SubLedgers`, and `CoA`. Must be robust to missing optional files (e.g., `truth.json` is not a CSV and is ignored). Does not call the RNG; does not generate new data.
- `cmd/fbx/main.go` (extend): add `fbx verify <dir>` subcommand dispatch. Exit 0 = invariants pass; exit 2 = at least one invariant fails (print `InvariantError.Invariant` and `.Detail` to stderr); exit 1 = config or I/O error (directory not found, required CSV missing, parse error).
- `cmd/fbx/verify_test.go` (or extend `main_test.go`): round-trip test — generate a dataset via `fbx gen monthly-close --seed 1`, then run `fbx verify` on the output directory, assert exit 0. Negative test: corrupt a monetary column in `gl.csv` to produce a debit/credit imbalance, assert exit 2.
- `internal/reader/csv_test.go`: unit test for `Load` — parse a minimal hand-crafted CSV set and assert all `Dataset` fields populated correctly.

**Out of scope.**
- JSONL input (CSV only in v0.2; JSONL reader deferred).
- XLSX or PDF input.
- Verifying `recon-stress-test` output (deliberately injected mismatches will cause exit 2 — document this behavior; do not special-case it).
- Network or database sources.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; `internal/reader` is a new package; its public surface is `Load(dir string) (gen.Dataset, error)` only.
- `go-idioms` — `Load` returns `(gen.Dataset, error)`; error wrapping with `fmt.Errorf("reader: %w", err)` at every I/O boundary; no package-level state.
- `go-determinism` — `Load` does not call the RNG or `time.Now()`; it is a pure I/O function with deterministic output for a given input directory.
- `go-testing` — round-trip test (generate → verify → exit 0); negative test (corrupt → verify → exit 2 with InvariantError detail on stderr).

**Tests.**
- Round-trip: `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out <dir>` followed by `fbx verify <dir>` exits 0.
- Negative: a `gl.csv` with one row where the debit column is incremented by `0.000000001` causes `fbx verify` to exit 2 and print the invariant number and entry ID to stderr.
- I/O error: `fbx verify /nonexistent` exits 1.
- `Load` unit test: a minimal 3-entry, 6-line CSV set reconstructs a `Dataset` where `invariant.Check` returns `nil`.
- `Load` returns an error (not a panic) when a decimal column contains a non-numeric string.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including round-trip and negative tests.
- `fbx verify` exit codes match the spec (0/1/2) for all three tested paths.
- `internal/reader/` contains no calls to `internal/gen/rng.go` functions.
- `fbx verify` on a fresh `monthly-close` output exits 0 (confirmed by running the subprocess test).
- CI green.

**Risk.** The `gen.Dataset` type holds a `CoA` field (`map[string]Account`). Reconstructing the CoA from `coa.csv` must produce a map that is byte-for-byte equivalent to `gen.DefaultCoA()` output — same codes, same types, same `NormalBalance` values — or invariant 2 will fire on every verified dataset. Validate by asserting `reflect.DeepEqual(loaded.CoA, gen.IndexCoA(gen.DefaultCoA()))` in the round-trip test.

---

## PR-V06 — `fbx convert` subcommand

**Objective.** Add `fbx convert --in <path> --out <path>` with automatic source/destination format detection from file extension. Supported conversions: CSV↔XLSX, CSV↔JSONL, XLSX↔JSONL. Streaming, decimal-preserving.

**Scope.**
- `cmd/fbx/convert.go` (or extend `main.go`): parse `--in` and `--out` flags; detect source and destination formats from extensions (`.csv`, `.xlsx`, `.jsonl`). Call the appropriate reader+writer pair. Exit 1 on config error (unsupported extension pair, missing flag). Exit 2 on I/O error.
- Format routing:
  - CSV → XLSX: stream rows from `encoding/csv` reader through `excelize.StreamWriter` (reuse the XLSX sheet writer logic from PR-V03).
  - CSV → JSONL: stream rows from `encoding/csv`, emit one JSON object per row with column names as keys; decimal text fields preserved as JSON strings, not parsed to float.
  - XLSX → CSV: use `excelize.Rows` iterator; write via `encoding/csv`.
  - XLSX → JSONL: `excelize.Rows` iterator → JSON object per row.
  - JSONL → CSV: `bufio.Scanner` + `json.Unmarshal` per line → `csv.Writer`.
  - JSONL → XLSX: same but stream into `excelize.StreamWriter`.
- `cmd/fbx/convert_test.go`: round-trip tests for each supported pair.
- Decimal preservation invariant: a decimal string `"100.000000001"` must survive any conversion path as the string `"100.000000001"` — never routed through `float64` or `json.Number` parsed to float.

**Out of scope.**
- PDF input or output.
- Multi-file batch conversion (single file at a time per invocation).
- Schema-aware conversion (column type inference beyond decimal-text detection).
- Merging or splitting sheets in XLSX.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; `convert` is a new subcommand, not a new package; all conversion logic lives in `cmd/fbx/convert.go` delegating to `internal/writer/` and a thin streaming reader.
- `go-idioms` — error wrapping at each I/O boundary; no package-level state; format detection via `filepath.Ext`, not MIME sniffing.
- `go-io-streaming` — streaming at every step; do not load the full source file into memory; `bufio.Scanner` for JSONL; `encoding/csv` reader for CSV; `excelize.Rows` for XLSX.
- `go-testing` — round-trip tests per conversion pair; decimal preservation test.

**Tests.**
- CSV → JSONL: a CSV with a decimal column `"123.456000000"` converts to JSONL where the field value is the JSON string `"123.456000000"`, not the JSON number `123.456`.
- JSONL → CSV: a JSONL with a decimal string field converts to CSV where the column value is `"123.456000000"`.
- CSV → XLSX → CSV round-trip: original and final CSV files are byte-identical (decimal text preserved through XLSX).
- XLSX → JSONL: decimal cells (stored as strings per PR-V03) appear as JSON strings in the output.
- Unsupported extension pair (`--in foo.pdf --out bar.csv`) exits 1.
- Missing `--in` or `--out` exits 1 with a usage message.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including round-trip conversion tests.
- `fbx convert --in monthly-close.csv --out monthly-close.xlsx` exits 0 and produces a valid XLSX file.
- Decimal preservation test passes: no conversion path introduces a `float64` parse of a monetary value.
- CI green.

**Risk.** JSONL → XLSX conversion requires that each JSON object's keys be stable in order when written to XLSX columns. Go's `json.Unmarshal` into `map[string]interface{}` does not preserve key order. Use `json.Decoder` with `Token` or unmarshal into a known struct, or collect keys from the first row and sort them before writing the header — then apply that fixed key order to all subsequent rows. If key order is left to map iteration, the XLSX column order will vary across runs.

---

## PR-V07 — YAML custom bundles + templates

**Objective.** Refactor `internal/gen/bundle.go` and `internal/gen/templates.go` to accept YAML-loaded bundle and template definitions in addition to the hardcoded built-ins. Add `--bundle-config <path.yaml>` and `--templates-config <path.yaml>` flags. The built-in `monthly-close` and `recon-stress-test` bundles continue to work without flags.

**Scope.**
- `internal/gen/bundle.go` (refactor): introduce a `BundleDefinition` struct with fields `Name string`, `EventMix map[string]int` (event type → weight, integer weights only — no float), `VolumeScale map[string]int` (tier → count), `StressTest bool`. Existing `MonthlyClose()` and `ReconStressTest()` return `BundleDefinition` values using the same struct, so the built-in path is not duplicated.
- `internal/gen/bundle_yaml.go` (new): `func LoadBundleConfig(path string) (BundleDefinition, error)` — reads a YAML file and validates: all referenced event types exist in the registered type list; all weights are positive integers; all volume tiers present. Returns a clear error (not panic) on any validation failure.
- `internal/gen/templates_yaml.go` (new): `func LoadTemplatesConfig(path string) ([]TemplateDefinition, error)` — reads a YAML file. `TemplateDefinition` has fields `EventType string`, `DebitAccount string`, `CreditAccount string`, `AmountField string`, `Description string`. Validation: all account codes exist in `DefaultCoA()`; debit and credit sides are both named; `AmountField` must be `"amount"` or a known metadata key. Returns a clear error on any validation failure.
- `cmd/fbx/main.go` (extend): add `--bundle-config <path>` and `--templates-config <path>` flags. If provided, load and validate before any generation. If absent, use built-in defaults. Both flags are optional and independent.
- `internal/gen/run.go` (extend): accept `BundleDefinition` and `[]TemplateDefinition` (or nil for built-ins) in `Run`; dispatch accordingly.
- YAML schema documentation: inline Go doc comments on `BundleDefinition` and `TemplateDefinition` fields are the schema reference; no separate schema file needed in v0.2.

**Out of scope.**
- GUI or builder for YAML; documentation site.
- Templates that allocate new account codes: CoA remains hardcoded; YAML templates may only reference existing codes from `DefaultCoA()`.
- Multi-file template configs (one YAML file per invocation).
- Inheritance or composition of built-in bundles via YAML (YAML bundles are standalone, not modifiers of built-ins).

**Skills to apply.**
- `engineering-workflow` — refactor does not change behavior for built-in bundles; confirm golden fixtures byte-identical after refactor; YAML loading is I/O, so it lives in the flag-parse / `cmd/fbx/` path, not deep in `internal/gen/`.
- `go-idioms` — `BundleDefinition` and `TemplateDefinition` are value types, not pointers; validation returns an `error`; no `init()` side effects.
- `go-determinism` — YAML-loaded event mix uses integer weights (same as built-ins); no float weights; no `time.Now()` in loader path.
- `go-testing` — table-driven validation tests for `LoadBundleConfig` (missing field, invalid weight type, unknown event type); golden fixture test confirms built-in path is byte-identical after refactor.

**Tests.**
- Built-in bundles (`monthly-close`, `recon-stress-test`) produce byte-identical golden fixtures after the refactor (regression test: run with `-update=false` and assert no diff).
- `LoadBundleConfig` with a valid YAML file returns a `BundleDefinition` where `EventMix` sums match the YAML values.
- `LoadBundleConfig` returns an error for: a YAML with a negative weight, an unknown event type name, a missing `name` field.
- `LoadTemplatesConfig` returns an error for: a template referencing an account code not in `DefaultCoA()`, a template with no credit account, a template with no debit account.
- `fbx gen --bundle-config <valid.yaml> --seed 1 --period 2026-01 --volume small --out /tmp/t` exits 0 and produces output passing `fbx verify`.

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes; golden fixtures for built-in bundles are byte-identical (refactor introduced no output change).
- `LoadBundleConfig` and `LoadTemplatesConfig` error paths are tested and return useful error messages.
- `fbx gen --bundle-config <custom.yaml>` + `fbx verify` exits 0 on the output.
- CI green.

**Risk.** The `BundleDefinition` refactor touches `internal/gen/bundle.go` and `internal/gen/run.go`, which are in the hot path for both existing bundles. Any structural change to how the event mix is passed to `Generate` may alter the sequence of RNG draws, producing different event counts and breaking both golden fixtures. Implement the refactor as a type-alias or struct-wrap of the existing parameters first, confirm golden fixtures are identical, then add YAML loading. Do not change RNG call order in the same commit as the type change.

---

## PR-V08 — MCP server `fbx mcp`

**Objective.** Add `fbx mcp` subcommand implementing a JSON-RPC 2.0 server over stdio, per the Model Context Protocol spec. Exposes four typed tools: `fbx_gen`, `fbx_verify`, `fbx_docs_list`, `fbx_bundle_list`. Each tool maps to existing internal functions; no new generation logic.

**Scope.**
- `cmd/fbx/mcp.go` (new): the MCP server entry point. Reads JSON-RPC requests from `os.Stdin`, dispatches to tool handlers, writes JSON-RPC responses to `os.Stdout`. Uses `bufio.Scanner` for line-delimited JSON-RPC (one JSON object per line). Errors written to `os.Stderr` (not `os.Stdout`).
- Tool definitions:
  - `fbx_gen`: parameters `bundle` (string), `seed` (int64), `period` (string, YYYY-MM), `volume` (string), `out_dir` (string), `format` (string, optional, default csv). Calls `gen.Run` + `invariant.Check` + `writer.Write`. Returns `{"ok": true, "out_dir": "<path>"}` or `{"ok": false, "error": "<message>"}`.
  - `fbx_verify`: parameters `dir` (string). Calls `reader.Load` + `invariant.Check`. Returns `{"ok": true}` or `{"ok": false, "invariant": N, "detail": "<message>"}`.
  - `fbx_docs_list`: no parameters. Returns the list of documents produced by a `monthly-close` run (hardcoded from `architecture/OVERVIEW.md` §Documents). Does not generate data.
  - `fbx_bundle_list`: no parameters. Returns the list of available built-in bundles with a one-line description each.
- Tool schema registration: tools registered in a fixed alphabetical order (`fbx_bundle_list`, `fbx_docs_list`, `fbx_gen`, `fbx_verify`) so the `tools/list` response is deterministic.
- Wire format determinism: JSON-RPC response fields marshaled via a typed struct with explicit JSON tags (no `map[string]interface{}` marshaling for responses).
- `cmd/fbx/mcp_test.go`: subprocess test — starts `fbx mcp`, sends a `tools/list` JSON-RPC request on stdin, asserts the response lists all 4 tools in alphabetical order.

**Out of scope.**
- HTTP transport (v0.4 per roadmap).
- Authentication or authorization.
- Natural-language wrapper tool inside MCP (explicitly excluded in `plans/roadmap.md` §Explicitly not planned).
- Streaming (SSE or chunked) responses.
- Tool result pagination.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist; MCP server is a new subcommand, not a new package; all protocol logic lives in `cmd/fbx/mcp.go`; existing internal packages are called directly.
- `go-idioms` — `bufio.Scanner` for line reading; typed response structs with JSON tags; error returned to caller as JSON-RPC error object, not written to `os.Stderr` mid-stream; `os.Stderr` for server-level panics only.
- `go-determinism` — tool schema list in fixed order; JSON-RPC response fields marshaled via struct, not map; no goroutines (single-threaded request loop: read request → dispatch → write response → repeat).
- `go-testing` — subprocess test for `tools/list`; `fbx_gen` tool test (write to `t.TempDir()`, assert exit and output directory populated); `fbx_verify` tool test (verify the `fbx_gen` output).

**Tests.**
- `tools/list` response lists exactly 4 tools in alphabetical order; each has a `name` and `description` field.
- `fbx_gen` tool call with valid parameters writes output to the specified `out_dir` and returns `{"ok": true}`.
- `fbx_gen` tool call with an invalid `bundle` returns `{"ok": false, "error": "..."}` (not a server crash).
- `fbx_verify` tool call on the output of `fbx_gen` returns `{"ok": true}`.
- `fbx_verify` tool call on a nonexistent directory returns `{"ok": false, "error": "..."}`.
- JSON-RPC error response for an unknown method name returns a well-formed JSON-RPC error object (not a raw Go error string).

**Done when.**
- `gofmt -d .` produces no output.
- `go vet ./...` is clean.
- `go test ./... -race` passes, including MCP subprocess tests.
- `fbx mcp` starts, responds to `tools/list`, and exits cleanly when stdin is closed.
- `tools/list` response is deterministic across two runs (byte-identical JSON).
- `fbx_gen` + `fbx_verify` round-trip via MCP produces exit 0 on verify.
- No goroutines introduced in the MCP server (single-threaded request loop; subprocess tests may use goroutines for pipe draining, which is acceptable).
- CI green.

**Risk.** JSON-RPC over stdio requires the server to flush `os.Stdout` after every response, or the client will block waiting for output. `bufio.Writer` wrapping `os.Stdout` must call `Flush()` after each response write. Additionally, `os.Stdout` and `os.Stdin` must not be used by any other part of the binary while `fbx mcp` is running (the progress bar currently writes to `os.Stderr`, which is fine; verify no code path writes to `os.Stdout` in the MCP path). Add an assertion in the subprocess test that the response arrives within a timeout (2 seconds) to catch flush bugs.

---

## Cross-cutting reminders

- **No `float64` for any monetary value.** Use `shopspring/decimal` throughout — storage, arithmetic, and serialization. Even intermediate accumulations in `internal/report/` must be `decimal.Decimal`. A `float64` money value is always a bug. (`.agents/engineer.md` §Common mistakes)
- **No `time.Now()` in the generation path.** Any call to `time.Now()` or `time.Since()` in any file under `internal/` makes output non-deterministic. All timestamps derive from `--period` arithmetic. `time.Now()` is permitted only in `cmd/fbx/` for the progress bar and manifest `generated_at` (if that field is excluded from golden diffs). This applies equally to `internal/report/`, `internal/reader/`, and any new v0.2 package. (`.agents/engineer.md` §Common mistakes)
- **No `math/rand` (v1) outside the seeded RNG wrapper.** The v1 package has a global source. Use only the seeded PCG instance in `internal/gen/rng.go`. (`.agents/engineer.md` §Common mistakes)
- **No gofakeit global functions.** `gofakeit.Name()` and similar globals draw from an auto-seeded source. Always use the `gofakeit.Faker` instance constructed inside the RNG wrapper. (`.agents/engineer.md` §Common mistakes)
- **Sort map keys before any output derived from them.** `for k, v := range someMap` is non-deterministic in Go. Any map whose contents flow into CSV rows, JSONL objects, XLSX cells, PDF tables, or `manifest.json` must have its keys sorted before iteration. This is the single most common source of golden-fixture flakiness. (`.agents/engineer.md` §Common mistakes)
- **No journal entries constructed outside `internal/gen/templates.go`.** Templates are the single source of balanced debit/credit pairs. `internal/report/` reads existing entries — it does not create new ones. (`.agents/engineer.md` §Common mistakes)
- **Write output only after all invariants pass.** The run exits 2 and writes nothing on invariant failure. Never write partial files and check invariants afterward. This applies to `fbx gen` in all formats and `fbx mcp`'s `fbx_gen` tool. (`.agents/engineer.md` §Common mistakes)
- **No goroutines in v0.2 production code.** Goroutine scheduling is non-deterministic. Single-threaded throughout. Parallel workers are deferred to v0.3. Subprocess tests may use pipe-draining goroutines (pre-existing acceptable pattern); do not add new goroutines inside any `internal/` package. (`.agents/skills/go-determinism/SKILL.md` Rule 5)
- **We are at v0.2; do not implement v0.3+.** Multi-currency, tax lines, payroll, official wire formats (OFX, NACHA, UBL, ISO 20022), parallel workers, sharding, accrual accounting, database loading, and the embeddable library are all deferred. Do not stub, comment-plan, or scaffold any of them. See `plans/roadmap.md` §v0.3 candidates. (`.agents/engineer.md` §Phase awareness)
- **Golden fixtures updated atomically with code.** Any PR that changes output schema, adds output files, or changes any serialization detail must update `testdata/golden/` in the same commit. CI is byte-exact comparison; no deferred fixture updates. (`.agents/skills/engineering-workflow/SKILL.md` §Commit hygiene)
- **`internal/reader/` is a new package introduced in PR-V05.** It has one public function: `Load(dir string) (gen.Dataset, error)`. Do not expand its surface in v0.2. It does not call the RNG, does not call `time.Now()`, and does not write any files — it is a pure I/O reader.
- **`internal/report/` is a new package introduced in PR-V01.** Its public surface is `Compute`, `ComputeAging`, `ComputeStatements`, `ComputeCheckRegister` returning typed structs. It does not write files (that is `internal/writer/`) and does not call the RNG.
