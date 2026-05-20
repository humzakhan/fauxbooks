## fbx — synthetic financial data generator

`fbx` generates deterministic, reconcilable synthetic accounting datasets. Each run
produces a complete set of double-entry-correct financial files from a seeded event
stream. Same seed, bundle, period, and version → byte-identical output across runs,
operating systems, and CPU architectures.

**Install:**

```bash
go install github.com/humzakhan/fauxbooks/cmd/fbx@latest
```

Requires Go 1.24 or later. No CGo. Single static binary.

**Quickstart — generate a monthly-close bundle and verify it:**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out ./output
fbx verify ./output
echo "Exit code: $?"
```

---

## Subcommands

### fbx gen

Generates a synthetic financial dataset for a given bundle, seed, and period.

**Synopsis:**

```
fbx gen [<bundle>] --seed N --period YYYY-MM [--volume small|medium|large]
        [--out DIR] [--format csv|jsonl|xlsx|pdf]
        [--company-config PATH] [--bundle-config PATH] [--templates-config PATH]
```

The positional `<bundle>` argument is required when `--bundle-config` is not provided.
When `--bundle-config` is provided, the bundle name is taken from the YAML file and
`<bundle>` may be omitted.

**Flags:**

| Flag | Type | Default | Description |
|---|---|---|---|
| `--seed` | int64 | `0` | RNG seed. Any value gives deterministic output; `0` is valid. |
| `--period` | YYYY-MM | required | Target month. All timestamps in the output derive from this value. |
| `--volume` | string | `small` | Size tier: `small`, `medium`, or `large`. |
| `--out` | string | `.` | Output directory. Created if it does not exist. |
| `--format` | string | `csv` | Output format: `csv`, `jsonl`, `xlsx`, or `pdf`. XLSX writes one `fauxbooks.xlsx` workbook; PDF writes 8 individual `.pdf` files. |
| `--company-config` | string | `""` | Path to YAML file with company branding for PDF headers. Uses a built-in default when omitted. |
| `--bundle-config` | string | `""` | Path to YAML file defining a custom event mix and volume scale. Overrides the built-in bundle. |
| `--templates-config` | string | `""` | Path to YAML file defining custom journal-entry templates. Validated at startup; must reference existing account codes from the built-in CoA. |

**Exit codes:**

| Code | Condition |
|---|---|
| `0` | Success — all files written. |
| `1` | Configuration or input error (bad flag, unknown bundle, malformed period). Nothing written. |
| `2` | Invariant violation or I/O failure. Nothing written. |

**Examples:**

```bash
# Standard monthly-close, small volume, CSV output
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out ./output

# JSONL output, medium volume
fbx gen monthly-close --seed 42 --period 2026-03 --volume medium --format jsonl --out ./output-jsonl

# XLSX output (single workbook, byte-deterministic)
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format xlsx --out ./output-xlsx

# PDF with custom company branding
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format pdf \
    --company-config ./company.yaml --out ./output-pdf

# Custom bundle from YAML config
fbx gen --bundle-config ./my-bundle.yaml --seed 1 --period 2026-01 --volume small --out ./output
```

---

### fbx verify

Reads CSV output in a directory, reconstructs an in-memory dataset, and re-runs all
four invariants. Useful for confirming a dataset is intact after copying or transfer.

**Synopsis:**

```
fbx verify <dir>
```

**Exit codes:**

| Code | Condition |
|---|---|
| `0` | All invariants pass. |
| `1` | I/O or parse error (directory not found, required CSV missing, malformed decimal). |
| `2` | At least one invariant violated. Detail printed to stderr with invariant number and entry ID. |

Note: verifying `recon-stress-test` output exits 2 by design — the injected anomalies
deliberately violate invariant 3.

**Examples:**

```bash
fbx verify ./output

# Expected: exit 2 (stress bundle injects invariant violations by design)
fbx verify ./stress-output
echo $?
```

---

### fbx convert

Converts a single file between CSV, XLSX, and JSONL formats. Format is detected from
the file extension. All six conversion directions are supported.

**Synopsis:**

```
fbx convert --in <source> --out <destination>
```

**Flags:**

| Flag | Required | Description |
|---|---|---|
| `--in` | yes | Source file path. Extension must be `.csv`, `.xlsx`, or `.jsonl`. |
| `--out` | yes | Destination file path. Extension must be `.csv`, `.xlsx`, or `.jsonl`. |

Supported pairs: CSV↔XLSX, CSV↔JSONL, XLSX↔JSONL (all six directions).

**Exit codes:**

| Code | Condition |
|---|---|
| `0` | Conversion succeeded. |
| `1` | Configuration error: missing flag, unsupported extension, or same-format (no-op) conversion. |
| `2` | I/O error during conversion. |

Decimal strings are never parsed through `float64`. XLSX output from `convert` is NOT
byte-deterministic (timestamps not pinned); use `fbx gen --format xlsx` when deterministic
XLSX is required.

**Examples:**

```bash
# Convert a single CSV file to XLSX
fbx convert --in ./output/gl.csv --out ./output/gl.xlsx

# Convert JSONL to CSV
fbx convert --in ./output/journal_entries.jsonl --out ./journal_entries.csv
```

---

### fbx mcp

Starts a JSON-RPC 2.0 MCP server on stdin/stdout. One JSON-RPC request per line;
one response per line with an explicit flush after each. Exits 0 on stdin EOF.

**Synopsis:**

```
fbx mcp
```

No flags. Runs until stdin is closed. All tool errors are returned as
`{"ok": false, "error": "..."}` in the result field; server panics go to stderr.

**Exit codes:**

| Code | Condition |
|---|---|
| `0` | Normal exit on stdin EOF. |
| `1` | Scanner error reading stdin. |

See [## MCP server](#mcp-server) for the full tool reference.

---

### fbx version

Prints the binary version string and exits 0.

**Synopsis:**

```
fbx version
```

**Example:**

```bash
fbx version
# v0.1.0
```

---

## Output files

Files are always written into the directory specified by `--out`. The table below lists
every file by category.

### Core ledger

| File | Formats | Description | When produced |
|---|---|---|---|
| `coa.csv` / `coa.jsonl` | csv, jsonl | Chart of accounts — all account codes and types. | Both bundles |
| `journal_entries.csv` / `journal_entries.jsonl` | csv, jsonl | Entry headers — one row per journal entry. | Both bundles |
| `gl.csv` / `gl.jsonl` | csv, jsonl | General ledger lines — one row per debit or credit. | Both bundles |
| `bank_statement.csv` / `bank_statement.jsonl` | csv, jsonl | Cash account postings with clearing lag. | Both bundles |

### Masters

| File | Formats | Description | When produced |
|---|---|---|---|
| `customers.csv` / `customers.jsonl` | csv, jsonl | Customer master records. | Both bundles |
| `vendors.csv` / `vendors.jsonl` | csv, jsonl | Vendor master records. | Both bundles |

### Sub-ledgers

| File | Formats | Description | When produced |
|---|---|---|---|
| `invoices.csv` / `invoices.jsonl` | csv, jsonl | Sales invoice line items. | Both bundles |
| `customer_payments.csv` / `customer_payments.jsonl` | csv, jsonl | AR payments applied to invoices. | Both bundles |
| `bills.csv` / `bills.jsonl` | csv, jsonl | Vendor bill line items. | Both bundles |
| `bill_payments.csv` / `bill_payments.jsonl` | csv, jsonl | AP disbursements. | Both bundles |
| `expense_reports.csv` / `expense_reports.jsonl` | csv, jsonl | Expense reports with line items. | Both bundles |

### Financial statements

| File | Formats | Description | When produced |
|---|---|---|---|
| `profit-loss.csv` / `profit-loss.jsonl` | csv, jsonl | Profit & Loss derived from GL revenue and expense accounts. | Both bundles |
| `balance-sheet.csv` / `balance-sheet.jsonl` | csv, jsonl | Balance Sheet (assets, liabilities, equity) as of period end. | Both bundles |
| `cash-flow.csv` / `cash-flow.jsonl` | csv, jsonl | Cash Flow statement (operating, investing, financing). | Both bundles |
| `trial-balance.csv` / `trial-balance.jsonl` | csv, jsonl | Trial Balance — debit and credit totals per account. | Both bundles |

### Ancillary reports

| File | Formats | Description | When produced |
|---|---|---|---|
| `ar-aging.csv` / `ar-aging.jsonl` | csv, jsonl | AR aging by 0-30, 31-60, 61-90, 91+ day buckets per customer. | Both bundles |
| `ap-aging.csv` / `ap-aging.jsonl` | csv, jsonl | AP aging by 0-30, 31-60, 61-90, 91+ day buckets per vendor. | Both bundles |
| `customer-statements.csv` / `customer-statements.jsonl` | csv, jsonl | Opening balance, invoiced, payments, and closing balance per customer. | Both bundles |
| `check-register.csv` / `check-register.jsonl` | csv, jsonl | Bill payments and expense reports sorted by date and entry ID. | Both bundles |

### Metadata

| File | Format | Description | When produced |
|---|---|---|---|
| `manifest.json` | json | Seed, bundle, period, volume, format, version, row counts, and SHA-256 hashes per file. | Both bundles |
| `truth.json` | json | Ground truth of every injected anomaly (type, entry ID, detail). | `recon-stress-test` only |

**XLSX format:** all of the above in a single `fauxbooks.xlsx` workbook — one sheet per document.

**PDF format:** the 8 derived report documents only (P&L, Balance Sheet, Cash Flow, Trial
Balance, AR Aging, AP Aging, Customer Statements, Check Register) as individual `.pdf` files.
Core ledger and master files are not produced in PDF format.

---

## Output formats

### CSV

- RFC 4180 compliant.
- UTF-8 encoding, LF (`\n`) line endings.
- Decimal monetary values as plain text strings with exactly 9 fractional digits (e.g. `100.000000000`).
- No float representation, no scientific notation, no thousands separator.
- Streaming writer — O(1) memory per row regardless of dataset size.

### JSONL

- One JSON object per line.
- Decimal monetary fields as JSON strings (never JSON numbers) to preserve precision.
- All other fields also as strings.

### XLSX

- Single workbook: `fauxbooks.xlsx`.
- One sheet per document, sheet name matches the base file name (e.g. `gl`, `journal_entries`).
- Decimal values as inline-string cells to preserve precision.
- All ZIP-entry modification timestamps pinned to the period start for byte-deterministic binary output.

### PDF

- Eight individual `.pdf` files, one per derived report document.
- Helvetica font only (no OS-level font embedding, no external font files).
- Creation and modification timestamps pinned to the period start.
- Byte-deterministic across runs and platforms.
- `--company-config` controls header branding; defaults to built-in placeholder values.

---

## Bundles

### monthly-close

A standard month-end close dataset with a realistic event mix: sales invoices, customer
payments, vendor bills, bill payments, expense reports, payroll, depreciation, inventory
purchases, inventory COGS, and interest income. All four write-time invariants hold —
entries balance, every account code exists in the CoA, bank totals tie to GL cash postings,
and every sub-ledger row references an existing entry. Suitable for testing accounting
pipelines, training ML models on balanced ground-truth data, and generating reproducible
demo datasets.

### recon-stress-test

Uses the same event mix as `monthly-close`, then runs a second injection pass that
deliberately introduces three classes of anomaly:

- **Timing differences** — bank row dates shifted relative to GL.
- **Duplicate bank rows** — verbatim copies with new row numbers.
- **Orphan bank rows** — bank entries with no GL counterpart.

Invariant checks are skipped by design. Every injection is recorded in `truth.json`.
Suitable for testing and benchmarking reconciliation tools and agents. Running
`fbx verify` against this output exits 2 by design.

### YAML custom bundles via --bundle-config

Provide a YAML file to define a completely custom event mix and volume scale, bypassing
the built-in bundle name requirement. See [## YAML configuration](#yaml-configuration).

---

## YAML configuration

### --bundle-config \<path\>

Defines a custom bundle. Overrides the built-in bundle selection. The bundle name may
still be passed as a positional argument, or omitted (in which case it is taken from
the YAML `name` field).

**Schema** (source of truth: `internal/gen/bundle_yaml.go`):

```yaml
name: my-bundle             # required; non-empty string
stress_test: false          # optional; enables injection pass (like recon-stress-test)
event_mix:
  sale_invoice: 5           # positive integer weight; key must be a registered event type
  vendor_bill: 5
  customer_payment: 3
  bill_payment: 3
volume_scale:               # required; must include small, medium, and large
  small: 10
  medium: 50
  large: 100
```

Valid `event_mix` keys: `sale_invoice`, `customer_payment`, `vendor_bill`, `bill_payment`,
`expense_report`, `payroll`, `depreciation`, `inventory_purchase`, `inventory_cogs`,
`interest_income`.

`volume_scale` must include all three tiers (`small`, `medium`, `large`) with positive integer values.

---

### --templates-config \<path\>

Defines custom journal-entry templates that override the built-in hardcoded templates for
specific event types. Validated at startup; errors exit 1.

**Schema** (source of truth: `internal/gen/templates_yaml.go`):

```yaml
templates:
  - event_type: sale_invoice      # registered event type (see valid keys above)
    debit_account: "1100"         # account code from the built-in CoA
    credit_account: "4000"        # account code from the built-in CoA
    amount_field: amount          # must be "amount" (only supported value in v0.2)
    description: "Custom invoice"
```

All `debit_account` and `credit_account` values must exist in the built-in chart of
accounts. `amount_field` must be `"amount"` — the only supported metadata key in v0.2.

---

### --company-config \<path\>

Provides per-tenant PDF branding. Fields map directly to PDF header content.

**Schema** (source of truth: `internal/writer/companyconfig.go`):

```yaml
company_name: "Acme Corp"
address: "123 Main St, Springfield, IL 62701"
phone: "+1-555-0100"
email: "finance@acme.example"
logo_path: ""              # reserved for v0.3; parsed but not rendered in v0.2
```

All fields are optional; missing fields render as empty strings in the PDF header.

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Configuration or input error — bad flag, parse failure, missing required file, unknown bundle or event type. Nothing written. |
| `2` | Invariant violation OR I/O failure during generation or write. No partial output is ever written on invariant abort. |

---

## Invariants

Five properties hold at runtime. If any of the first four fails, `fbx gen` exits 2 and
writes nothing. The fifth is verified by CI golden fixtures.

**Inv-1 — Balanced journal entries.** Every journal entry satisfies Σ debits = Σ credits
exactly, to the last decimal digit. Templates are the only code path that constructs
journal entries.

**Inv-2 — Account-code closure.** Every account code referenced on any GL or journal line
exists in the chart of accounts for that run. The CoA is built before any events are
generated.

**Inv-3 — Bank-to-GL tie.** Bank statement cash totals equal GL cash account postings for
the period. Both derive from the same event stream; divergence is impossible unless the
projection engine introduces a discrepancy.

**Inv-4 — Sub-ledger referential integrity.** Every sub-ledger row (invoice, bill, payment,
expense) references an existing journal entry ID. No dangling foreign keys.

**Inv-5 — Determinism.** Same seed + bundle + period + version → byte-identical output
across runs, operating systems, and CPU architectures. Verified by CI golden fixture for
both bundles at small volume.

`fbx verify` checks Inv-1 through Inv-4 against an existing output directory.

---

## MCP server

`fbx mcp` starts a JSON-RPC 2.0 server over stdio. Line-delimited (one JSON object per
line). Single-threaded. Flushes stdout after every response. Exits 0 on stdin EOF.

stdout is reserved exclusively for JSON-RPC output. Errors within tool calls are returned
as `{"ok": false, "error": "..."}` in the result field. Server-level errors go to stderr.

### Tools (alphabetical)

Agents that speak MCP should use `tools/list` as the canonical machine-readable interface;
this section is the human/markdown mirror.

---

#### fbx_bundle_list

Lists the available built-in bundles.

**Parameters:** none.

**Returns:**

```json
{
  "bundles": [
    {"name": "monthly-close", "description": "..."},
    {"name": "recon-stress-test", "description": "..."}
  ]
}
```

**Error contract:** this tool does not fail; it always returns a non-empty bundles list.

---

#### fbx_docs_list

Lists the document files produced by a monthly-close generation run (CSV names).

**Parameters:** none.

**Returns:**

```json
{
  "documents": ["coa.csv", "customers.csv", "vendors.csv", "journal_entries.csv",
                "gl.csv", "bank_statement.csv", "invoices.csv", "customer_payments.csv",
                "bills.csv", "bill_payments.csv", "expense_reports.csv",
                "profit-loss.csv", "balance-sheet.csv", "cash-flow.csv",
                "trial-balance.csv", "ar-aging.csv", "ap-aging.csv",
                "customer-statements.csv", "check-register.csv", "manifest.json"]
}
```

**Error contract:** this tool does not fail.

---

#### fbx_gen

Generates a synthetic financial dataset.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `bundle` | string | yes | Bundle name: `monthly-close` or `recon-stress-test`. |
| `seed` | integer | yes | RNG seed for deterministic output. |
| `period` | string | yes | Target month in YYYY-MM format. |
| `volume` | string | yes | Size tier: `small`, `medium`, or `large`. |
| `out_dir` | string | yes | Output directory path (created if missing). |
| `format` | string | no | Output format: `csv` (default), `jsonl`, `xlsx`, or `pdf`. |

**Returns (success):**

```json
{"ok": true, "out_dir": "/path/to/output"}
```

**Returns (failure):**

```json
{"ok": false, "error": "config error: unknown bundle \"bad-bundle\""}
```

**Error contract:** errors are always returned in the result, never as a JSON-RPC error
response. The caller should check `ok` before using `out_dir`.

---

#### fbx_verify

Verifies that a directory of fbx CSV output passes all four invariants.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `dir` | string | yes | Directory containing fbx CSV output files. |

**Returns (success):**

```json
{"ok": true}
```

**Returns (invariant violation):**

```json
{"ok": false, "invariant": 3, "detail": "bank total 1234.000000000 != GL cash 1233.000000000"}
```

**Returns (I/O or parse error):**

```json
{"ok": false, "error": "open coa.csv: no such file or directory"}
```

**Error contract:** `invariant` and `detail` are set only on invariant violations; `error`
is set only on I/O or parse failures. These fields are mutually exclusive.

---

### Worked JSON-RPC examples

**tools/list request and response:**

Request:
```json
{"jsonrpc":"2.0","id":1,"method":"tools/list"}
```

Response (abbreviated):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "fbx_bundle_list",
        "description": "List the available built-in generation bundles with a one-line description each.",
        "inputSchema": {"type": "object", "properties": {}}
      },
      {
        "name": "fbx_gen",
        "description": "Generate a synthetic financial dataset for a given bundle, seed, period, and volume.",
        "inputSchema": {
          "type": "object",
          "properties": {
            "bundle": {"type": "string", "description": "Bundle name (e.g. monthly-close, recon-stress-test)."},
            "seed": {"type": "integer", "description": "RNG seed for deterministic output."},
            "period": {"type": "string", "description": "Target month in YYYY-MM format."},
            "volume": {"type": "string", "description": "Size tier: small, medium, or large."},
            "out_dir": {"type": "string", "description": "Output directory path."},
            "format": {"type": "string", "description": "Output format: csv (default), jsonl, xlsx, or pdf."}
          },
          "required": ["bundle", "seed", "period", "volume", "out_dir"]
        }
      }
    ]
  }
}
```

**fbx_gen call:**

Request:
```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fbx_gen","arguments":{"bundle":"monthly-close","seed":1,"period":"2026-01","volume":"small","out_dir":"/tmp/fbx-output"}}}
```

Response:
```json
{"jsonrpc":"2.0","id":2,"result":{"ok":true,"out_dir":"/tmp/fbx-output"}}
```

---

## Determinism contract

Same `--seed` + bundle + `--period` + fbx version → byte-identical output across:
- repeated runs on the same machine
- different operating systems (Linux, macOS, Windows)
- different CPU architectures (amd64, arm64)

No wall-clock dependence in the generation path. All timestamps in the output derive
from the `--period` argument. The seeded PCG RNG (`math/rand/v2`) is the only source
of randomness; it is initialised once from `--seed` and never re-seeded.

This contract is verified by CI golden fixtures for both built-in bundles at small volume.

---

## Examples

**Generate and verify a round-trip (CSV):**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out ./output
fbx verify ./output
echo "Verified: exit $?"
```

**Generate XLSX; check file count:**

```bash
fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format xlsx --out ./output-xlsx
ls ./output-xlsx/
# fauxbooks.xlsx  manifest.json
```

**Generate PDF with company config:**

```bash
cat > company.yaml <<'EOF'
company_name: "Acme Corp"
address: "123 Main St, Springfield, IL 62701"
phone: "+1-555-0100"
email: "finance@acme.example"
EOF

fbx gen monthly-close --seed 1 --period 2026-01 --volume small \
    --format pdf --company-config ./company.yaml --out ./output-pdf
ls ./output-pdf/
# ar-aging.pdf  ap-aging.pdf  balance-sheet.pdf  cash-flow.pdf
# check-register.pdf  customer-statements.pdf  profit-loss.pdf  trial-balance.pdf
# manifest.json
```

**Convert CSV to XLSX:**

```bash
fbx convert --in ./output/gl.csv --out ./gl.xlsx
```

**Run MCP server and call fbx_gen from a stdin pipe:**

```bash
tmpdir=$(mktemp -d)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"fbx_gen","arguments":{"bundle":"monthly-close","seed":1,"period":"2026-01","volume":"small","out_dir":"'"$tmpdir"'"}}}' \
  | fbx mcp
# {"jsonrpc":"2.0","id":1,"result":{"ok":true,"out_dir":"/tmp/fbx-outputXXXXX"}}
```
