# fauxbooks Architecture

This tree is the technical architecture for fauxbooks. `OVERVIEW.md` is the canonical entry.

## Navigation

| Path | What lives here |
|---|---|
| [`OVERVIEW.md`](OVERVIEW.md) | Canonical tech spec: stack, invariants, CLI, output format, determinism model |
| [`architecture.md`](architecture.md) | Internal design: pipeline, packages, key types, error and memory model |

## Invariants at a glance

Five properties enforced at write time. Any failure exits non-zero and writes nothing.

| Name | Description | Spec reference |
|---|---|---|
| Balanced journal entries (Inv-1) | Σ debits = Σ credits for every journal entry | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Account-code closure (Inv-2) | Every GL line references an account that exists in the chart of accounts | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Bank-to-GL tie (Inv-3) | Bank statement totals equal GL cash account postings for the period | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Sub-ledger referential integrity (Inv-4) | Every sub-ledger row references an existing journal entry | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Determinism | Same seed + bundle + period + version → byte-identical output across runs, OSes, and architectures | [`OVERVIEW.md`](OVERVIEW.md) §Determinism |

## Package summary

See [`architecture.md`](architecture.md) §Packages for the full package table and rationale.

| Package | Scope |
|---|---|
| `cmd/fbx` | CLI entry, subcommand dispatch, top-level orchestration, exit codes |
| `internal/gen` | CoA, seeded RNG, event generation, template expansion, projection, bundle dispatch |
| `internal/invariant` | Write-time invariant checker — runs the four invariants before any file is written |
| `internal/writer` | CSV, JSONL, XLSX, PDF writers; decimal formatting; manifest + SHA-256 hashing |
| `internal/reader` | Dataset reader for verify and convert subcommands |
| `internal/report` | Derived financial reports (P&L, balance sheet, aging) |
