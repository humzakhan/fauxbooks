# spec.md

**fauxbooks v0.1** — synthetic financial data generator.
Binary: `fbx`. Language: Go. License: MIT (TBD).

## What it does

Generates reconcilable synthetic financial datasets at any scale, deterministically. Output is double-entry-correct by construction.

## Model

The generator is event-sourced. Each business event (sale, bill payment, expense report) emits a balanced set of journal lines via a built-in template. Every document is a projection over the event stream. There is no separate reconciliation step — consistency is the only way the data can exist.

```
events → journal entries → {GL, bank statement, sub-ledgers}
```

## Invariants

Four invariants are checked at runtime before any output is written; if any fails the run exits 2 and writes nothing:

1. Every journal entry: Σ debits = Σ credits exactly.
2. Every account code referenced exists in the chart of accounts.
3. Bank statement totals tie to GL cash account postings.
4. Every sub-ledger row references an existing journal entry.

A fifth invariant — byte-identical output across runs, OSes, and architectures — is enforced by the golden-fixture CI test rather than at runtime.

## CLI

```
fbx gen <bundle>   --seed N --period YYYY-MM --out DIR [--format csv|jsonl]
fbx version
```

Two bundles in v0.1:

- **`monthly-close`** — coa, journal entries, GL, bank statement, customers, invoices, customer payments, vendors, bills, bill payments, expense reports.
- **`recon-stress-test`** — the same, but with timing differences, duplicates, and orphan bank transactions injected; a `truth.json` records what was injected.

Sizing via `--volume small|medium|large`:

| Tier | Events | GL rows (approx) |
|---|---|---|
| `small` | 150 | ≈333 |
| `medium` | 1 500 | ≈3 300 |
| `large` | 15 000 | ≈33 000 |

Hard cap 10⁷ GL rows; beyond that, run multiple periods.

## Documents in v0.1

| File | Description |
|---|---|
| `coa.csv` | Chart of accounts |
| `journal_entries.csv` | Entry headers (one row per entry) |
| `gl.csv` | General ledger lines |
| `bank_statement.csv` | Cash account postings with clearing lag |
| `customers.csv` | Customer master |
| `invoices.csv` | Sales invoice line items |
| `customer_payments.csv` | AR payments applied to invoices |
| `vendors.csv` | Vendor master |
| `bills.csv` | Vendor bill line items |
| `bill_payments.csv` | AP disbursements |
| `expense_reports.csv` | Expense reports with line items |
| `manifest.json` | Seed, bundle, period, volume, format, version, row counts, SHA-256 hashes per file |
| `truth.json` | (stress-test only) Ground truth of injected mismatches |

Every monetary column is decimal text with 9 fractional digits (e.g. `100.000000000`) — never float, no thousands separators, no scientific notation.

## Formats

- **CSV** (default): RFC 4180, UTF-8, `\n` endings, streaming writer.
- **JSONL**: one JSON object per line, decimals as strings.

That's it for v0.1. XLSX, PDF, and the official formats (OFX, UBL, NACHA, etc.) are in [`future.md`](./future.md).

## Determinism

Same `--seed` + bundle + period + version → byte-identical output across runs, OSes, and architectures.

- Single seeded PCG RNG (`math/rand/v2`).
- `shopspring/decimal` arithmetic.
- No wall-clock reads in the generation path; all timestamps derive from `--period`.
- Golden-fixture test in CI exercises every bundle at small volume.

## Exit codes

- `0` — success
- `1` — configuration or input error
- `2` — invariant violation or I/O failure (nothing written)

## Agent integration

Ships an `AGENTS.md` at the repo root. Any coding harness that runs shell commands (Claude Code, Codex, Cursor, Claude Desktop, Continue, …) can read it and translate natural-language requests into `fbx gen` calls. No MCP server in v0.1 — `AGENTS.md` alone covers the use case at one file's cost.

MCP server is in [`future.md`](./future.md).

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.22+ |
| Decimal | `github.com/shopspring/decimal` |
| RNG | `math/rand/v2` (PCG) |
| Faker | `github.com/brianvoe/gofakeit/v7` |
| CSV | stdlib `encoding/csv` |
| CLI | stdlib `flag` (Cobra not needed for three flags) |
| Testing | stdlib + golden fixtures |

No CGo. Single static binary. Cross-compiles to linux/macOS/windows × amd64/arm64.

## Build layout

```
fauxbooks/
├── cmd/fbx/main.go
├── internal/
│   ├── coa/         # Chart of accounts (built-in)
│   ├── events/      # Event generation
│   ├── templates/   # Hardcoded balanced-entry templates
│   ├── project/     # GL + bank + sub-ledger projection
│   ├── invariant/   # Write-time checks
│   ├── writer/      # CSV + JSONL
│   └── rng/         # Seeded RNG
├── testdata/golden/ # Determinism fixtures
├── AGENTS.md
├── README.md
└── spec.md, thesis.md, product.md, future.md
```

## Out of scope for v0.1 (see `future.md`)

XLSX, PDF, official formats (OFX/UBL/NACHA/etc.), MCP server, custom templates, custom bundles, derived statements (P&L, balance sheet, aging reports), multi-currency, tax lines, payroll, parallel workers, sharding, `convert` command, separate `verify` command, database loading.

## Open questions

1. License: MIT vs Apache-2.0. Apache-2.0 is safer for the formats work later; MIT is friendlier now.
2. Default chart of accounts: 50 accounts or 200? Ship one, accept the trade-off.
3. Repo namespace: `github.com/fauxbooks/fbx` proposed.
