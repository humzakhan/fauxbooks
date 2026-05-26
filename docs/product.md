# product.md

## What fauxbooks is

A free, open-source command-line tool that generates synthetic financial datasets. Single binary, no dependencies, runs anywhere Go runs. Install with `go install github.com/humzakhan/fauxbooks/cmd/fbx@latest` or grab a release binary.

Output is real-shaped accounting data: chart of accounts, general ledger, bank statements, invoices, bills, expense reports. All cross-referenced, all reconcilable, all reproducible from a seed.

## Who it's for

- Engineers building accounting/fintech software who need test datasets that behave like real books.
- AI/ML teams training or evaluating finance agents who need balanced, scale-able ground-truth data.
- Auditors and accountants teaching, demoing, or stress-testing tools without touching client data.
- Researchers who need reproducible benchmarks.
- Students learning double-entry bookkeeping and reconciliation.

## How it works (mental model)

You declare what kind of dataset you want and `fbx` runs a simulated bookkeeping system that emits every document a real one would produce.

```
You:  "Give me a month of bookkeeping for testing."
fbx:  generates → events → journal entries → all downstream documents
```

The generator is event-sourced. Each business event emits a balanced set of journal entries; every output file is a projection over those events. There is no separate reconciliation step — consistency is the only way the data can be created.

## How you use it

```bash
fbx gen monthly-close --seed 42 --period 2026-03 --out ./out
```

You get a complete month-end set: GL, journal entries, bank statement, customers/invoices/payments, vendors/bills/payments, expense reports. Every total ties.

For a tougher dataset (timing differences, duplicates, orphan bank transactions injected):

```bash
fbx gen recon-stress-test --seed 42 --period 2026-03 --out ./out
```

The injected mismatches are recorded in `truth.json` so reconciliation code can be scored against ground truth.

## Output

CSV by default, JSONL with `--format jsonl`. Every monetary column is decimal text with 9 fractional digits (never float). Every run writes a `manifest.json` with the seed, flags, row counts, and SHA-256 hashes of every file.

## Scale

| Volume | GL rows | Wall time | Memory |
|---|---|---|---|
| small | ~1,000 | <0.5 s | <50 MiB |
| medium | ~10,000 | <2 s | <100 MiB |
| large | ~100,000 | <20 s | <200 MiB |

For larger sets, run multiple periods. Hard cap 10⁷ rows per invocation.

## Determinism

Same seed + bundle + period + version → byte-identical output across runs, OSes, and architectures. If a bug in your reconciliation code shows up on row 7,318 of a dataset, you can reproduce that exact dataset on a colleague's machine with one command.

## Use it from your coding agent

The repo ships an `AGENTS.md` at the root. Any coding harness that runs shell commands (Claude Code, Codex, Cursor, Claude Desktop, Continue) reads it and translates natural-language requests into `fbx` calls.

```
You:   "I'm working on a bank reconciliation feature. Give me a tough
        test dataset for March 2026."

Agent: [runs: fbx gen recon-stress-test --seed 1234 --period 2026-03 --out ./fixtures]
       "Generated in ./fixtures/. Seed 1234. See truth.json for the
        injected mismatch labels."
```

No flag-memorization, no quoting, reproducible by seed.

## What we don't do

- **No GUI.** This is a CLI.
- **No anonymization.** fauxbooks generates from scratch; it does not de-identify real data.
- **No fraud simulation.** Data is plausible at the line level, not at the macro-pattern level.
- **No database loading.** Output is files; pipe them where you want.
- **No hosted service.** Run it yourself.

## Roadmap

**Shipped today:** the `gen`, `verify`, `convert`, and `mcp` subcommands; CSV, JSONL, XLSX, and PDF output; two built-in bundles plus YAML custom bundles and templates; derived financial reports.

**On the roadmap (see [`../plans/roadmap.md`](../plans/roadmap.md)):** official formats (OFX, UBL, NACHA, XBRL, …), payroll, multi-currency, and tax lines.

We add from the roadmap based on what users actually ask for.

## Getting involved

- Read [`thesis.md`](thesis.md) and the [contributor guide](../CONTRIBUTING.md).
- Try it: `fbx gen monthly-close --seed 1 --out /tmp/fbx`.
- File issues for missing document types or determinism failures.
- PRs welcome.
