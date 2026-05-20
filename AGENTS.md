# AGENTS.md — developer entry point for fauxbooks

> This file is for AI agents and humans who want to **develop** (extend, fix, refactor)
> the fauxbooks codebase. If you want to **use** `fbx` as a tool, read
> [`docs/usage.md`](docs/usage.md) instead.

---

## Two audiences

Developer-agents (Claude Code, Codex, Cursor, and similar) opening this repo to write
or review code should start here. This file points at the authoritative specs, explains
the invariants, and maps the package layout. Tool-users — humans or agents calling
`fbx gen` to generate datasets — should read `docs/usage.md`, which covers flags, output
files, formats, bundles, and the MCP interface without any implementation detail.

---

## Read first

Priority order. Read these before writing or reviewing any code.

1. [`architecture/OVERVIEW.md`](architecture/OVERVIEW.md) — canonical spec: stack, 5 invariants, CLI, output format, determinism model.
2. [`architecture/architecture.md`](architecture/architecture.md) — internal design: pipeline, packages, key types, error and memory model.
3. [`.agents/engineer.md`](.agents/engineer.md) — engineer persona: the 5 invariants, stack conventions, pre-code checklist, how to add new event types / output files / CoA accounts.
4. [`.agents/reviewer.md`](.agents/reviewer.md) — review red flags and blocking criteria.
5. [`.agents/skills/engineering-workflow/SKILL.md`](.agents/skills/engineering-workflow/SKILL.md) — per-PR workflow: branch, implement, test, review, merge.
6. [`.agents/skills/go-determinism/SKILL.md`](.agents/skills/go-determinism/SKILL.md) — determinism rules: no wall-clock, no unseeded RNG, no map-iteration leaks.
7. [`.agents/skills/go-testing/SKILL.md`](.agents/skills/go-testing/SKILL.md) — table-driven tests, golden fixture pattern with `-update` flag.
8. [`.agents/skills/go-io-streaming/SKILL.md`](.agents/skills/go-io-streaming/SKILL.md) — bufio wrapping, Flush/Close discipline, O(1)-per-row memory.
9. [`.agents/skills/go-idioms/SKILL.md`](.agents/skills/go-idioms/SKILL.md) — error wrapping, interface placement, package boundaries, gofmt cleanliness.

---

## The five invariants

Non-negotiable. Any change that breaks one of these does not ship.

| # | Name | Rule |
|---|---|---|
| 1 | Balanced entries | Every journal entry: Σ debits = Σ credits exactly. |
| 2 | Account-code closure | Every account code on any GL line exists in the CoA for that run. |
| 3 | Bank-to-GL tie | Bank statement cash totals equal GL cash account postings. |
| 4 | Sub-ledger referential integrity | Every sub-ledger row references an existing journal entry ID. |
| 5 | Determinism | Same seed + bundle + period + version → byte-identical output across runs, OSes, and architectures. |

Inv-1 through Inv-4 abort the run (exit 2, nothing written) if violated. Inv-5 is
verified by CI golden fixtures. Full detail: `architecture/OVERVIEW.md` §Invariants.

---

## Build layout

| Package | Responsibility |
|---|---|
| `cmd/fbx` | CLI entry point, flag parsing, subcommand dispatch (`gen`, `verify`, `convert`, `mcp`, `version`). |
| `internal/gen` | CoA, seeded RNG, event generation, template expansion, GL/bank/sub-ledger projection, bundle dispatch, YAML bundle and template config loading. |
| `internal/invariant` | Write-time invariant checker — runs Inv-1 through Inv-4 before any file is written. |
| `internal/writer` | CSV, JSONL, XLSX, and PDF streaming writers; manifest and SHA-256 hashing; company config for PDF. |
| `internal/reader` | CSV reader that reconstructs an in-memory `gen.Dataset` from an existing output directory (used by `fbx verify`). |
| `internal/report` | Pure projection functions: P&L, Balance Sheet, Cash Flow, Trial Balance, AR/AP Aging, Customer Statements, Check Register. |

---

## How to add X

These are pointers, not full procedures. Full procedures live in `.agents/engineer.md`.

- **Add an event type** — see `.agents/engineer.md` §"How to add a new event type". Requires: new constant in `internal/gen/events.go`, a balanced template in `internal/gen/templates.go`, projection handling in `internal/gen/project.go`, and golden fixture update.
- **Add an output document** — see `.agents/engineer.md` §"How to add a new output file". Requires: writer in `internal/writer/`, manifest entry, and golden fixture update.
- **Add a CoA account** — see `.agents/engineer.md` §"How to add a new CoA account". Requires: new entry in `internal/gen/coa.go` and invariant compatibility check.

---

## Test workflow

```bash
go test ./... -race        # full suite with race detector; must pass before any PR
gofmt -l .                 # must produce no output
go vet ./...               # must be clean
```

Golden fixture pattern: tests accept a `-update` flag that regenerates fixtures in place.

```bash
go test ./... -run TestGolden -update
git diff testdata/golden/   # must be empty for a determinism-preserving change
```

When a test that uses golden fixtures fails after a code change, the first question is:
was the change intentional and determinism-preserving? If yes, run with `-update` and
commit the new fixtures. If no, investigate the source of non-determinism before proceeding.

---

## Out of scope

v0.3+ deferrals: multi-currency, tax lines, payroll expansion, logo embedding in PDF,
official export formats (OFX, UBL, NACHA), database loading, parallel workers, sharding,
and per-doc PDF selection. Reference [`plans/roadmap.md`](plans/roadmap.md) for the full
list and rationale.
