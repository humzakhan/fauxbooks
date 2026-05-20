# fauxbooks / fbx

**fauxbooks** is an open-source Go CLI that generates reconcilable-by-construction synthetic accounting datasets. It treats the generator as an event-sourced bookkeeping system: each business event emits a balanced set of journal entries, and every output file — GL, bank statement, invoices, bills, expense reports — is a projection over that event stream. Consistency is the only way the data can be created; there is no separate reconciliation pass. Same seed + bundle + period + version produces byte-identical output across runs, OSes, and architectures.

The primary audiences are engineers building accounting and fintech software, AI/ML teams evaluating finance agents against balanced ground-truth data, auditors and instructors who need realistic datasets without touching client data, and researchers who need reproducible benchmarks.

## Install

```
go install github.com/humzakhan/fauxbooks/cmd/fbx@latest
```

Requires Go 1.22 or later. No CGo. Produces a single static binary.

## Quick start

**Generate a standard monthly-close dataset (small volume, CSV):**

```
fbx gen monthly-close --seed 42 --period 2026-01 --volume small --out ./output
```

**Generate a recon-stress-test dataset with injected anomalies:**

```
fbx gen recon-stress-test --seed 42 --period 2026-01 --volume small --out ./output
```

The stress-test bundle produces the same files as monthly-close plus `truth.json`, which records every injected anomaly (timing differences, duplicates, orphan bank rows).

**JSONL output:**

```
fbx gen monthly-close --seed 42 --period 2026-01 --volume small --format jsonl --out ./output
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--seed N` | `0` | RNG seed; any int64 value gives deterministic output |
| `--period YYYY-MM` | required | Target month |
| `--volume small\|medium\|large` | `small` | Dataset size (≈1k / 10k / 100k GL rows) |
| `--out DIR` | `.` | Output directory (created if missing) |
| `--format csv\|jsonl` | `csv` | Output format |

## Output files

| File | Description |
|---|---|
| `coa.csv` | Chart of accounts |
| `journal_entries.csv` | Entry headers (one row per entry) |
| `gl.csv` | General ledger lines |
| `bank_statement.csv` | Cash account postings |
| `customers.csv` | Customer master |
| `invoices.csv` | Sales invoice line items |
| `customer_payments.csv` | AR payments applied to invoices |
| `vendors.csv` | Vendor master |
| `bills.csv` | Vendor bill line items |
| `bill_payments.csv` | AP disbursements |
| `expense_reports.csv` | Expense reports |
| `manifest.json` | Seed, flags, version, row counts, SHA-256 per file |
| `truth.json` | (recon-stress-test only) Ground truth of injected anomalies |

Every monetary column is decimal text with 9 fractional digits (e.g. `100.000000000`) — never float, no thousands separators, no scientific notation.

## Bundles

- **`monthly-close`** — standard month-end close dataset. All four write-time invariants hold: entries balance, all account codes exist in the CoA, bank totals tie to GL cash postings, and all sub-ledger rows reference existing entries.
- **`recon-stress-test`** — same event mix, but with three classes of anomaly injected after projection: timing differences (bank date shifted vs. GL date), duplicates (verbatim duplicate bank rows), and orphan bank rows (bank entries with no GL counterpart). `truth.json` records exactly what was injected.

## Determinism

Same `--seed` + bundle + period + version → byte-identical output across runs, OSes, and architectures. The CI golden-fixture test enforces this for both bundles at small volume on linux/amd64, linux/arm64, darwin/arm64, and windows/amd64.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Configuration or input error |
| `2` | Invariant violation or I/O error (nothing written) |

## Repo map

| Path | What lives here |
|---|---|
| `cmd/fbx/` | CLI entry point |
| `internal/gen/` | CoA, RNG, event generation, templates, projection, bundle dispatch |
| `internal/invariant/` | Four write-time invariant checks |
| `internal/writer/` | CSV + JSONL streaming writers, manifest, truth.json |
| `testdata/golden/` | Determinism golden fixtures (committed) |
| `architecture/` | Technical architecture; `OVERVIEW.md` is the canonical tech spec |
| `docs/` | Thesis, product spec, narrative docs |
| `plans/` | Roadmap and implementation phasing |
| `.agents/` | Engineer and reviewer agent personas |

## License

TBD — MIT vs Apache-2.0. See [architecture/OVERVIEW.md](architecture/OVERVIEW.md) for the open question.
