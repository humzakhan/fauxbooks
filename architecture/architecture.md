# Architecture

`fbx` is a single-pass, single-goroutine pipeline: seed → generate → check → write. Three flags drive a seeded RNG, which produces a deterministic event stream; each event expands through a hardcoded template into balanced journal lines; those lines project into GL, bank, and sub-ledger views held entirely in memory; four invariant checks run against the full in-memory dataset; only then does the write phase flush all files plus a manifest. The generation path remains single-goroutine (no goroutines under `internal/gen`). An MCP stdio entry point ships as `fbx mcp` alongside the core pipeline.

---

## Pipeline

```
fbx gen <bundle> --seed N --period YYYY-MM --volume tier
         │
         ▼
 [cmd/fbx] flag parse → build Config{seed, period, volume, bundle, outDir, format}
         │
         ▼
 [gen/rng] seed PCG RNG; bind gofakeit to seeded instance
         │
         ▼
 [gen/coa] build CoA (hardcoded accounts → map[code]Account)
         │
         ▼
 [gen/events] generate []Event  ← RNG + period + volume determine count and shape
         │
         ▼
 [gen/templates] per-event: expand → []JournalEntry + []JournalLine
         │
         ▼
 [gen/project] project accumulated entries → GL rows, bank rows, sub-ledger rows
         │
         ▼
 [gen/invariant] check all 4 invariants; abort (exit 2) on any failure
         │
         ▼
 [cmd/fbx / gen/writer] stream each []Row to CSV or JSONL; write manifest.json
```

The full dataset lives in memory until all invariants pass. This is bounded by the volume tier (max ~100k GL rows at `large`). The 10⁷ hard cap in `architecture/OVERVIEW.md` §Sizing is a safety limit, not a routine target — a dataset at that scale will exceed a typical process memory budget. The resolution is v0.3 streaming (see `plans/roadmap.md`); v0.1 does not attempt it.

---

## Packages

OVERVIEW.md §Build layout names 7 packages. The shipped design uses **6**.

| Path | Responsibility | Public surface | Why separate |
|---|---|---|---|
| `cmd/fbx` | Flag parse, subcommand dispatch, top-level orchestration, exit codes | `main` only | Entry point; the only place `time.Now()` is permitted (progress bar to stderr) |
| `internal/gen` | CoA, RNG, event generation, template expansion, projection, bundle dispatch | `Run(Config) error`, `Config`, `Bundle` | All core domain logic; no file I/O |
| `internal/invariant` | Four write-time checks; returns `InvariantError` | `Check(Dataset) error`, `InvariantError` | Isolated test seam for negative tests (deliberately broken `Dataset` values) |
| `internal/writer` | CSV, JSONL, XLSX, PDF streaming write, decimal formatting, manifest + SHA-256 | `Write(Dataset, Config) error`, `FormatDecimal(decimal.Decimal) string` | Isolates all file I/O and enforces format rules |
| `internal/reader` | Dataset reader for verify and convert subcommands | `Read(dir string) (Dataset, error)` | Isolates read-path I/O; no dependency on gen |
| `internal/report` | Derived financial reports (P&L, balance sheet, aging) | `Generate(Dataset) Reports` | Pure projection; no file I/O |

Four original packages instead of seven because: `rng` → merged into `gen` (one caller); `coa` → merged into `gen` (no external consumers); `templates` → merged into `gen` (pure functions, no state, no second implementation); `project` → merged into `gen` (pure function over `[]JournalEntry`, no independent test seam). The rationale for each merge is documented in `architecture/OVERVIEW.md` §Build layout alongside the original 7-package sketch.

---

## Key types

**`Config`**
- Seed (int64, deterministic input)
- Period (time.Time, derived from --period flag)
- Volume (string, tier selector)
- Bundle (string, dispatch key)
- OutDir (string, output destination)
- Format (string, csv|jsonl|xlsx|pdf)

**`Event`**
- ID (string, stable within a run)
- Type (EventType, discriminates template dispatch)
- OccurredAt (time.Time, derived from period + deterministic offset)
- Amount (decimal.Decimal, monetary, never float64)
- RefID (string, links to customer / vendor master)
- Metadata (map[string]string, event-specific ancillary data)

**`JournalEntry`**
- EntryID (string, referenced by sub-ledger rows)
- OccurredAt (time.Time, from source event)
- Description (string, human-readable label)
- SourceEventID (string, traceability back to Event)

**`JournalLine`**
- LineID (string, unique within run)
- EntryID (string, foreign key to JournalEntry)
- AccountCode (string, must exist in CoA — invariant 2)
- Debit (decimal.Decimal, exactly one of Debit/Credit is non-zero)
- Credit (decimal.Decimal)

**`Account`**
- Code (string, unique CoA key)
- Name (string, display)
- Type (AccountType, asset / liability / equity / revenue / expense)
- NormalBalance (string, debit or credit)

**`Dataset`**
- Entries ([]JournalEntry)
- Lines ([]JournalLine)
- GLRows, BankRows, SubLedgers (projected views, passed to invariant.Check and writer.Write)
- CoA (map[code]Account, built before any generation)

**`Manifest`**
- Seed, Period, Bundle, Version (run identity)
- RowCounts (map[file]int)
- Hashes (map[file]string, SHA-256 per output file)

**`InvariantError`**
- Invariant (int, 1–4, matches OVERVIEW.md numbering)
- Detail (string, identifies the offending entry or row)

---

## Interfaces

None in v0.1. There is one writer, one invariant checker, and one generator. Test seams are direct function calls — `invariant.Check` accepts a `Dataset` and can be called with a deliberately malformed value in unit tests without any interface indirection. Adding an interface for a single implementation adds no value and obscures the call graph. The reviewer's red flag is relevant here: any export without a consumer is noise (see `.agents/reviewer.md` §Public surface cleanliness).

---

## Invariant enforcement

For the full statement of each invariant, see `architecture/OVERVIEW.md` §Invariants.

| Invariant | Enforced in | On failure |
|---|---|---|
| 1. Σ debits = Σ credits per entry | `internal/invariant.Check` | Exit 2, nothing written |
| 2. All account codes exist in CoA | `internal/invariant.Check` | Exit 2, nothing written |
| 3. Bank totals tie to GL cash | `internal/invariant.Check` | Exit 2, nothing written |
| 4. Sub-ledger rows reference existing entries | `internal/invariant.Check` | Exit 2, nothing written |

Invariant 5 (determinism) is not checked at runtime — it is verified by CI golden fixtures. A run that violates it produces wrong output silently; the CI byte-comparison catches it on the next run.

---

## Memory and streaming

Single goroutine throughout. Generation accumulates `[]Event`, then expands to `[]JournalEntry` + `[]JournalLine`, then projects to GL, bank, and sub-ledger slices — all in memory before `invariant.Check` is called. The write phase streams row-by-row via `encoding/csv` or `encoding/json`: O(1) working memory per row, no encoded-output buffer held in memory. The split is explicit: all rows in memory before invariant check; output streamed row-by-row after.

---

## Error handling

1. Invariant failure → `InvariantError` → `cmd/fbx` prints the invariant number and detail, exits 2. Nothing is written to the output directory.
2. Config error (bad flag, unrecognized bundle, unrecognized volume tier) → plain `error` → exit 1.
3. I/O error during the write phase → propagates as a plain `error` → exit 2.

`InvariantError` is the only sentinel error type. No other custom error types in v0.1. Exit code semantics match `architecture/OVERVIEW.md` §Exit codes exactly.

---

## Bundles and templates

Bundle dispatch and template functions are hardcoded Go in `internal/gen`. This is intentional: a hardcoded bundle is auditable, version-controlled, and cannot fail due to a missing or malformed config file. YAML custom bundles and templates also ship, allowing user-defined event mixes without recompilation (see `docs/usage.md` for details).

- **Bundle** — a struct in `internal/gen` with fields: mix (event type distribution), volumeScale (row count multiplier per tier), stressTest (bool, enables the second injection pass and `truth.json` output for `recon-stress-test`).
- **Template** — one function per EventType: takes an `Event` and a `CoAIndex`, returns `(JournalEntry, []JournalLine)`. Each function constructs balanced debit/credit legs by arithmetic — the entry is balanced by construction, not by post-hoc sum. Templates are the only place in the codebase where `JournalEntry` and `JournalLine` values are assembled.

---

## Determinism

All randomness in the generation path flows from a single `math/rand/v2` PCG seeded with `--seed`. A `gofakeit.Faker` instance is constructed with a deterministic int64 derived from that seed — never from gofakeit's global auto-seeded source. All timestamps are computed as `periodStart + deterministicOffset(eventIndex)`; no `time.Now()` call is permitted under `internal/`. Maps are sorted by key before any output is derived from them. The manifest hashes each output file with SHA-256 during the write pass, producing a content fingerprint for the full run.

The four determinism-critical seams:
- PCG seed (sourced exclusively from `--seed`)
- gofakeit seed derivation (from run RNG, not global)
- sorted map iteration before output (keys sorted explicitly before ranging)
- manifest SHA-256 hash (computed during write, not post-hoc)

---

## Testing

Golden fixtures live under `testdata/golden/{bundle}/{seed}/` and are committed to the repo. The CI golden test runs `Run(Config)` and byte-compares every output file against the fixture — one byte difference fails the build.

| Category | What it tests | Pass condition |
|---|---|---|
| Golden fixtures | Full pipeline output for both bundles at small volume | Byte-identical to committed fixture |
| Determinism test | `Run(Config{seed:1, period:"2026-01", volume:"small"})` twice | Byte-identical between the two runs |
| Invariant unit tests | Deliberately broken `Dataset` values passed to `invariant.Check` | Correct `InvariantError` returned for each of invariants 1–4 |
| Template tests | Debit and credit sides summed for representative amounts per EventType | Σ debits = Σ credits for every template |

Invariant unit tests must also verify the abort behavior: when `Check` returns an error, the caller exits 2 and the output directory is empty.

---

## Rejected alternatives

- `internal/rng` as a separate package — one caller (`internal/gen`), no benefit from isolation.
- `internal/coa` as a separate package — no external consumers; merging removes a package boundary with zero callers outside `gen`.
- `Writer` interface — one format implementation; the format switch lives in `writer.Write`, not behind an interface.
- Streaming event generation via channel or iterator — `[]Event` accumulation is simpler at v0.1 volume and avoids goroutine coordination overhead.
- Separate `internal/project` package — a pure function with no independent test seam; merging into `gen` does not weaken testability.
- `fbx verify` as a standalone verifier of arbitrary datasets — the invariant checker validates at generation time; `fbx verify` ships for post-hoc verification of previously-generated output.

---

## Out of scope (roadmap)

See `plans/roadmap.md` for the full phase schedule. Items not yet shipped:

- Official wire formats: OFX, NACHA, ISO 20022, UBL, PEPPOL, Factur-X
- Multi-currency and FX
- Tax lines, payroll
- Parallel workers and streaming at 10⁷+ scale
- Embeddable library API
