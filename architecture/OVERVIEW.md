# fauxbooks — architecture overview

**fauxbooks v0.1** — synthetic financial data generator.
Binary: `fbx`. Language: Go. License: MIT (see `LICENSE`).

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
fbx gen <bundle>   --seed N --period YYYY-MM --out DIR [--format csv|jsonl|xlsx|pdf]
fbx verify <dir>   verify a previously-generated dataset
fbx convert <dir>  convert output to a different format
fbx mcp            start the MCP stdio server
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
- **XLSX**: Excel workbook, one sheet per document type.
- **PDF**: formatted report output.

Official wire formats (OFX, UBL, NACHA, XBRL, etc.) are roadmap items; see [`../plans/roadmap.md`](../plans/roadmap.md).

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

Ships an `AGENTS.md` at the repo root. Any coding harness that runs shell commands (Claude Code, Codex, Cursor, Claude Desktop, Continue, …) can read it and translate natural-language requests into `fbx gen` calls. An MCP stdio server ships as `fbx mcp`, providing a structured interface for agent runtimes that support the Model Context Protocol.

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.24+ |
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
│   ├── gen/         # CoA, RNG, events, templates, projection, bundle dispatch
│   ├── invariant/   # Write-time checks
│   ├── writer/      # CSV, JSONL, XLSX, PDF writers + manifest
│   ├── reader/      # Dataset reader (used by verify/convert)
│   └── report/      # Derived financial reports (P&L, balance sheet, aging)
├── testdata/golden/ # Determinism fixtures
├── AGENTS.md
└── README.md
```

## Now shipped (was out of scope in early drafts)

XLSX, PDF, MCP server (`fbx mcp`), custom templates, custom bundles via YAML, derived financial statements (P&L, balance sheet, aging reports), `convert` subcommand, `verify` subcommand.

## Out of scope (roadmap — see `../plans/roadmap.md`)

Official wire formats (OFX/UBL/NACHA/XBRL/etc.), multi-currency, tax lines, payroll, parallel workers, sharding, database loading.

## Open questions

1. Default chart of accounts: 50 accounts or 200? Ship one, accept the trade-off.
