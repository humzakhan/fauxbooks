# fauxbooks Architecture

This tree is the technical architecture for fauxbooks. `OVERVIEW.md` is the canonical entry; subsystem docs and ADRs live below it.

## Navigation

| Path | What lives here |
|---|---|
| [`OVERVIEW.md`](OVERVIEW.md) | Canonical tech spec: stack, build layout, invariants, CLI, output format, determinism model |
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

One doc per `internal/` package. Each will be added when the package implementation begins.

| Package | Scope | Status |
|---|---|---|
| `internal/coa` | Chart of accounts registry — account codes, types, hierarchy | planned |
| `internal/events` | Event-stream generator — business event types and sequencing | planned |
| `internal/templates` | Balanced-entry templates per event type | planned |
| `internal/project` | Projection engine — GL, bank statement, and sub-ledger views over the event stream | planned |
| `internal/invariant` | Write-time invariant checker — runs all five invariants before any file is written | planned |
| `internal/writer` | CSV and JSONL serializers — streaming, decimal-preserving | planned |
| `internal/rng` | Seeded RNG wrapper — PCG via `math/rand/v2`, deterministic seeding contract | planned |
