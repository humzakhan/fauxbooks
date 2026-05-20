# fauxbooks Architecture

This tree is the technical architecture for fauxbooks. `OVERVIEW.md` is the canonical entry; subsystem docs and ADRs live below it.

## Navigation

| Path | What lives here |
|---|---|
| [`OVERVIEW.md`](OVERVIEW.md) | Canonical tech spec: stack, invariants, CLI, output format, determinism model |
| [`architecture.md`](architecture.md) | v0.1 internal design: pipeline, packages, key types, error and memory model |
| [`subsystems/`](subsystems/) | Per-package design docs (one per `internal/...` package). Populated as v0.1 packages land. |
| [`adrs/`](adrs/) | Architecture Decision Records. ADR-0001 onward as decisions are made. |

## Invariants at a glance

Five properties enforced at write time. Any failure exits non-zero and writes nothing.

| Name | Description | Spec reference |
|---|---|---|
| Balanced journal entries (Inv-1) | Σ debits = Σ credits for every journal entry | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Account-code closure (Inv-2) | Every GL line references an account that exists in the chart of accounts | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Bank-to-GL tie (Inv-3) | Bank statement totals equal GL cash account postings for the period | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Sub-ledger referential integrity (Inv-4) | Every sub-ledger row references an existing journal entry | [`OVERVIEW.md`](OVERVIEW.md) §Invariants |
| Determinism | Same seed + bundle + period + version → byte-identical output across runs, OSes, and architectures | [`OVERVIEW.md`](OVERVIEW.md) §Determinism |

## Planned subsystem docs

One doc per package in the v0.1 design (see [`architecture.md`](architecture.md) §Packages for rationale on the 4-package layout). Each doc will be added when the package implementation begins.

| Package | Scope | Status |
|---|---|---|
| `cmd/fbx` | CLI entry, flag parsing, top-level orchestration, exit codes | planned |
| `internal/gen` | CoA, seeded RNG, event generation, template expansion, projection, bundle dispatch | planned |
| `internal/invariant` | Write-time invariant checker — runs the four invariants before any file is written | planned |
| `internal/writer` | CSV and JSONL streaming writers, decimal formatting, manifest + SHA-256 hashing | planned |
