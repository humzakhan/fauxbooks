# fauxbooks (fbx)

**fauxbooks** is an open-source Go CLI that generates deterministic, reconcilable synthetic accounting datasets. The generator is event-sourced: each business event (sale, bill payment, expense report) emits a balanced set of journal lines via a built-in template, and every output file — GL, bank statement, invoices, bills, expense reports — is a projection over that event stream. Consistency is the only way the data can be created; there is no separate reconciliation pass. The same seed, bundle, period, and version produce byte-identical output across runs, operating systems, and CPU architectures. The primary audiences are engineers building accounting and fintech software, AI/ML teams evaluating finance agents against balanced ground-truth data, and researchers who need reproducible benchmarks.

## Install

```bash
go install github.com/humzakhan/fauxbooks/cmd/fbx@latest
```

Requires Go 1.24 or later. No CGo. Produces a single static binary.

**From source:**

```bash
git clone https://github.com/humzakhan/fauxbooks.git
cd fauxbooks
go build -o fbx ./cmd/fbx
```

## Quick start

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out ./output
```

This generates a clean month-end close dataset for January 2026 into `./output/` — twelve files including GL, bank statement, sub-ledgers, and a `manifest.json` with SHA-256 hashes. All four write-time invariants hold; entries are balanced and cross-references are valid.

## Commands

### `fbx gen <bundle>`

```
fbx gen <bundle> --seed N --period YYYY-MM [--volume small|medium|large] [--out DIR] [--format csv|jsonl]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--seed` | int64 | `0` | RNG seed. Any value gives deterministic output; `0` is a valid seed. |
| `--period` | YYYY-MM | required | Target month. All timestamps derive from this value. |
| `--volume` | string | `small` | Dataset size tier: `small`, `medium`, or `large`. |
| `--out` | string | `.` | Output directory. Created if it does not exist. |
| `--format` | string | `csv` | Output format: `csv` or `jsonl`. |

**Exit codes:**

| Code | Condition |
|---|---|
| `0` | Success — all files written. |
| `1` | Configuration or input error (bad flag, unknown bundle, malformed period). Nothing written. |
| `2` | Invariant violation or I/O error. Nothing written. |

`fbx gen` is the only subcommand in v0.1.

## v0.2

PR-V01 adds four derived financial report files per run (Profit & Loss, Balance Sheet, Cash Flow, Trial Balance), each written in both CSV and JSONL formats. These are computed from the event-sourced dataset by the new `internal/report` package and written by streaming writers in `internal/writer`.

PR-V02 adds three more derived report types: AR/AP aging by bucket (`ar-aging.csv/.jsonl`, `ap-aging.csv/.jsonl`), per-customer statements (`customer-statements.csv/.jsonl`), and a check register of cash disbursements (`check-register.csv/.jsonl`). Party attribution uses deterministic FNV-1a hashing — no new RNG draws.

## Bundles

### `monthly-close`

Generates a standard month-end close dataset with a realistic event mix: sales invoices, customer payments, vendor bills, bill payments, expense reports, payroll, depreciation, inventory purchases, inventory COGS, and interest income. All four write-time invariants hold — entries balance, every account code exists in the chart of accounts, bank totals tie to GL cash postings, and every sub-ledger row references an existing entry. Suitable for testing accounting pipelines, training ML models on balanced data, and generating reproducible demo datasets.

### `recon-stress-test`

Uses the same event mix as `monthly-close`, then runs a second injection pass that deliberately introduces three classes of anomaly: timing differences (bank row dates shifted relative to GL), duplicate bank rows (verbatim copies with new row numbers), and orphan bank rows (bank entries with no GL counterpart). Invariant checks are skipped by design — the injected anomalies would fail them. Every injection is recorded in `truth.json`. Suitable for testing and benchmarking reconciliation tools and agents.

## Volume tiers

| Tier | Events | Approximate GL rows | Approximate output size |
|---|---|---|---|
| `small` | 150 | ~333 | ~90 KB |
| `medium` | 1,500 | ~3,300 | ~900 KB |
| `large` | 15,000 | ~33,000 | ~9 MB |

Each event emits multiple journal lines; GL row count varies with the event mix. Hard cap is 10^7 GL rows; for larger datasets, generate multiple periods.

## Examples

**1. Minimal monthly-close run (seed 1, January 2026, small, CSV):**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out ./output
```

**2. Stress-test run with a different seed:**

```bash
fbx gen recon-stress-test --seed 7 --period 2026-01 --volume small --out ./stress-output
```

Produces all standard files plus `truth.json` listing every injected anomaly.

**3. JSONL output:**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format jsonl --out ./output-jsonl
```

Each output file is `.jsonl`; every row is a JSON object with decimal amounts as strings.

**4. Medium volume:**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume medium --out ./output-medium
```

**5. Specifying an explicit output directory:**

```bash
fbx gen monthly-close --seed 42 --period 2026-03 --volume small --out /tmp/march-close
```

## Output

All files are written under `--out`. CSV files are RFC 4180, UTF-8, `\n` line endings.

**Both bundles:**

| File | Purpose |
|---|---|
| `coa.csv` | Chart of accounts — all account codes and types. |
| `journal_entries.csv` | Entry headers — one row per journal entry. |
| `gl.csv` | General ledger lines — one row per debit or credit. |
| `bank_statement.csv` | Cash account postings with clearing lag. |
| `customers.csv` | Customer master records. |
| `invoices.csv` | Sales invoice line items. |
| `customer_payments.csv` | AR payments applied to invoices. |
| `vendors.csv` | Vendor master records. |
| `bills.csv` | Vendor bill line items. |
| `bill_payments.csv` | AP disbursements. |
| `expense_reports.csv` | Expense reports with line items. |
| `manifest.json` | Run metadata and file hashes (see below). |
| `profit-loss.csv` / `.jsonl` | Profit & Loss statement derived from GL revenue and expense accounts. |
| `balance-sheet.csv` / `.jsonl` | Balance Sheet (assets, liabilities, equity) as of period end. |
| `cash-flow.csv` / `.jsonl` | Cash Flow statement (operating, investing, financing). |
| `trial-balance.csv` / `.jsonl` | Trial Balance — debit and credit totals per account. |
| `ar-aging.csv` / `.jsonl` | AR aging — gross invoiced amounts per customer by 0-30, 31-60, 61-90, 91+ day buckets. |
| `ap-aging.csv` / `.jsonl` | AP aging — gross billed amounts per vendor by 0-30, 31-60, 61-90, 91+ day buckets. |
| `customer-statements.csv` / `.jsonl` | Customer statements — opening balance, invoiced, payments, and closing balance per customer. |
| `check-register.csv` / `.jsonl` | Check register — one row per bill payment and expense report, sorted by date and entry ID. |

**Stress-test only:**

| File | Purpose |
|---|---|
| `truth.json` | Ground truth of every injected anomaly — type, entry ID, and detail. |

**`manifest.json`** records the seed, bundle, period, volume, format, fbx version, row counts per file, and a SHA-256 hash of each output file. The hashes let callers verify that a dataset has not been modified after generation and confirm byte-identical reproduction across machines.

**Decimal format:** Every monetary column is decimal text with exactly 9 fractional digits (e.g. `100.000000000`). No float representation, no scientific notation, no thousands separator.

## Determinism

The same `--seed`, bundle, `--period`, `--volume`, and fbx version produce byte-identical output across runs, operating systems, and CPU architectures. This is enforced by five invariants in the generation path:

1. Single seeded PCG RNG (`math/rand/v2`) — no global state.
2. `shopspring/decimal` for all monetary arithmetic — no floating-point.
3. No wall-clock reads in the generation path — all timestamps derive from `--period`.
4. No map-iteration ordering in output paths — sorted before use.
5. No goroutines in v0.1 — single-threaded, sequential pipeline.

The CI golden-fixture test verifies byte-identical output for both bundles at `small` volume across linux/amd64, linux/arm64, darwin/arm64, and windows/amd64.

## Test data

The committed golden fixtures are the canonical reference outputs for `--seed 1 --period 2026-01 --volume small`:

- `testdata/golden/monthly-close/2026-01/small/`
- `testdata/golden/recon-stress-test/2026-01/small/`

To regenerate them locally and diff:

```bash
go test ./... -run TestGolden -update
git diff testdata/golden/
```

The `-update` flag overwrites the fixtures in place. A clean diff confirms your build is deterministic against the committed reference.

## Development

```bash
go test ./... -race   # full test suite with race detector
go vet ./...          # static analysis
gofmt -d .            # formatting check (must produce no output)
```

Regenerate golden fixtures:

```bash
go test ./... -run TestGolden -update
```

## Documentation

| Path | Contents |
|---|---|
| `architecture/OVERVIEW.md` | Canonical CLI spec: flags, exit codes, output format, invariants. |
| `architecture/architecture.md` | Internal design: packages, pipeline, type definitions. |
| `docs/thesis.md` | Problem statement and design rationale. |
| `docs/product.md` | Product spec and audience. |
| `plans/roadmap.md` | v0.2+ planned features and out-of-scope items. |
