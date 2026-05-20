# future.md

Ideas deferred from v0.1. Nothing here is committed — each item ships only when a real user needs it. Listed roughly in the order I'd reach for them.

## v0.2 candidates

### Derived reports
P&L, balance sheet, cash flow, trial balance, AR aging, AP aging, customer statements, check register. All computable from the v0.1 GL; ship the rendering when someone wants it.

### XLSX output
One sheet per document, decimals as text to preserve precision, streaming write via `excelize.StreamWriter`. Auto-split sheets at 1,048,576 rows.

### PDF rendering
Invoices, bills, bank statements, expense reports, financial statements. Pure-Go `gofpdf` for plain reports; consider Typst (vendored binary) for invoice/statement layouts requiring richer composition. Per-tenant branding via `--company-config`.

### `fbx verify`
Standalone invariant checker against an existing directory. Runs the same checks the generator runs at write time. Useful for CI assertions over user-edited datasets.

### `fbx convert`
CSV ↔ XLSX ↔ JSONL conversion. Streaming, decimal-preserving. Skip if `csvkit` / `xsv` covers the need.

### MCP server (`fbx mcp`)
Stdio transport. Exposes typed tools: `fbx_gen`, `fbx_verify`, `fbx_docs_list`, `fbx_bundle_list`. Lets agentic harnesses invoke fauxbooks directly without shelling out. Defer until `AGENTS.md` proves insufficient.

### Custom bundles and templates (YAML)
Today's bundles and templates are hardcoded. Open them up to YAML config when someone wants a business shape we don't ship.

## v0.3 candidates

### Official document formats
Each is real work; each ships only on user demand.

- **Bank**: OFX, QFX, CAMT.053 (ISO 20022), MT940 (SWIFT), BAI2
- **Payments**: NACHA, ISO 20022 pain.001
- **E-invoicing**: UBL 2.1, PEPPOL BIS3, Factur-X (PDF/A-3 + embedded XML)
- **Accounting imports**: IIF (QuickBooks), QBO (Quicken), Xero CSV, NetSuite CSV

Estimate: one to two weeks per format, including conformance testing against a real-world parser.

### Multi-currency + FX
FX rate generator, currency-tagged tokens, no-silent-mixing invariant. Adds genuine complexity to the event model.

### Tax lines
Sales tax / VAT / GST as a third leg on invoice/bill entries, with separate authority accounts. Locale-dependent.

### Payroll cluster
Pay periods, pay stubs, payroll register, payroll tax liabilities. Generator emits per-employee per-period entries with withholdings.

### Parallel workers + sharding
`--workers N` parallel template execution; `--shard k/n` for splitting 10⁸-row runs across machines. Defer until single-threaded performance is actually a bottleneck.

## v0.4 candidates

### Tax/regulatory formats
IRS FIRE (1099 e-file), EFW2 (W-2 e-file), CRA-XML (Canadian T4), XBRL / iXBRL (SEC/ESEF financial statement filings).

### Inventory + fixed assets
SKU master, inventory movements, fixed asset register with depreciation schedules.

### Accrual accounting
Period-end adjusting entries: accruals, deferrals, prepaid amortization. Requires a real accounting model rather than the cash-basis-with-clearing-lag approximation of v0.1.

### International locales
EN-GB, FR, DE — names, addresses, IBANs, locale-aware date formats. E-invoice formats force the issue.

### Database loading
`fbx gen ... --into pg://...` direct Postgres COPY. Skip if the CSV → COPY two-step is fine.

### Embeddable Go library
`pkg/fauxbooks` for in-process use by other Go tools. Defer until someone asks.

### MCP HTTP transport + auth
For remote/shared `fbx mcp` deployments. Stdio covers all local cases.

## Explicitly not planned

- **GUI.** This is a CLI tool. Build a UI on top if you want one.
- **Anonymization.** fauxbooks generates from scratch; it does not de-identify real data.
- **Fraud / market simulation.** Data is plausible at the line level, not at the macroeconomic level.
- **Hosted service.** Run it yourself.
- **Natural-language wrapper tool inside MCP.** The host LLM in any agentic harness parses intent better than anything we'd ship.
