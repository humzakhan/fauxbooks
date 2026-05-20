---
name: engineering-workflow
description: Apply when writing or modifying Go code in the fauxbooks repo. Enforces the 5 generation invariants, spec-driven workflow, and v0.1 phase scope.
---

# Engineering Workflow — fauxbooks / fbx

## Persona

You write code for `fbx`, a deterministic synthetic financial data generator. Your prime directive: every change must preserve all 5 generation invariants and must not expand v0.1 scope. You write against the spec; you do not invent features.

Canonical persona file (read if present): `.agents/engineer.md`

---

## The 5 Invariants

These gate every change. A run that violates any of them exits non-zero and writes nothing.

| # | Invariant | Where enforced |
|---|---|---|
| 1 | Every journal entry: Σ debits = Σ credits exactly | `internal/invariant/` + write time |
| 2 | Every account code referenced exists in the chart of accounts | `internal/invariant/` + write time |
| 3 | Bank statement totals tie to GL cash account postings | `internal/invariant/` + write time |
| 4 | Every sub-ledger row references an existing journal entry | `internal/invariant/` + write time |
| 5 | Same `--seed` + bundle + period + version → byte-identical output across runs, OSes, architectures | `internal/rng/` + golden fixtures in CI |

If a change could cause any invariant to fail silently — even in a corner case — stop and resolve it before committing.

---

## Pre-Code Checklist

Answer these before opening a file:

1. **Which doc covers this area?** See the map below.
2. **Does my change touch the RNG or any timestamp derivation?** If yes, invariant 5 requires a golden-fixture update committed atomically with the change.
3. **Does my change touch a template or projection?** Verify invariants 1–4 still hold by tracing the journal-line math end-to-end.
4. **Is this feature in v0.1 scope?** Check `architecture/OVERVIEW.md` §"Out of scope for v0.1". If not listed there or in `plans/roadmap.md`, do not implement it.
5. **Does this change add a new output column or file?** Update `manifest.json` hash accounting and add a row to the documents table in `architecture/OVERVIEW.md`.

---

## Doc Map — Authoritative Answers

| Question | Go to |
|---|---|
| What documents does v0.1 produce? | `architecture/OVERVIEW.md` §Documents |
| What are the two bundles and their contents? | `architecture/OVERVIEW.md` §Bundles |
| Decimal precision and serialization format | `architecture/OVERVIEW.md` §Decimal |
| RNG seeding and determinism rules | `architecture/OVERVIEW.md` §Determinism |
| Build layout, package responsibilities | `architecture/OVERVIEW.md` §Build layout |
| CLI flags and exit codes | `architecture/OVERVIEW.md` §CLI |
| v0.2/v0.3/v0.4 candidates (do not implement) | `plans/roadmap.md` |
| Why this project exists | `docs/thesis.md` |
| User-facing description of behavior | `docs/product.md` |

---

## Commit Hygiene

- **Small commits.** One commit per spec item, bug fix, or named task. Do not bundle unrelated changes.
- **`gofmt`-clean before commit.** Run `gofmt -l .` and fix all output before staging.
- **Golden fixtures updated atomically.** If a change causes `testdata/golden/` drift, the fixture update and the code change go in the same commit. Never commit code that breaks golden tests and plan to fix fixtures "later."
- **No float64 for monetary values.** `shopspring/decimal` everywhere. A `float64` monetary value is always a bug.
- **No wall-clock reads in the generation path.** All timestamps derive from `--period`. `time.Now()` in any file under `internal/` (except test helpers that don't affect output) is a determinism violation.
- **No CGo.** The binary must cross-compile without a C toolchain.

---

## Spec-Driven Development

Write the doc update before the code when the change affects:

- The set of output files or columns
- An invariant definition or check
- CLI flags or exit codes
- The decimal serialization format
- Bundle contents

For everything else (a new template, a bug fix within a settled spec), code first, then update inline comments if the WHY is non-obvious.

---

## Phase Awareness

We are at **v0.1**. Do not implement, stub, scaffold, or comment-plan anything from `plans/roadmap.md` v0.2+.

| Version | Status |
|---|---|
| v0.1 | Current — two bundles, CSV + JSONL, 5 invariants, golden CI |
| v0.2+ | Deferred — XLSX, PDF, derived statements, MCP server, custom templates |
| v0.3+ | Deferred — official formats (OFX, UBL, NACHA, ...) |
| v0.4+ | Deferred — regulatory formats, multi-currency, payroll |

If a user asks for something in v0.2+, acknowledge it, note it is in `plans/roadmap.md`, and stop. Do not implement it.

---

## Common Mistakes to Avoid

- **Generating a non-deterministic ID or timestamp.** Every ID in output files must derive from the seeded RNG, not from `uuid.New()`, `rand.Int()`, or `time.Now()`.
- **Writing output before all invariants pass.** The writer runs after the invariant checker. Never skip or defer the check to "speed things up."
- **Adding a new monetary column as float64.** Decimal text with 9 fractional digits, always. See `architecture/OVERVIEW.md` §Decimal.
- **Touching v0.2+ code paths.** Even a `// TODO: v0.2` comment that implies unwritten hooks is scope creep.
- **Breaking the golden fixture without updating it.** CI will catch this, but don't leave it broken in a commit.

---

## "When Unsure" Decision Table

| Question | Go to |
|---|---|
| "Is this column format correct?" | `architecture/OVERVIEW.md` §Decimal |
| "Which package owns this?" | `architecture/OVERVIEW.md` §Build layout |
| "Does this feature belong in v0.1?" | `architecture/OVERVIEW.md` §Out of scope + `plans/roadmap.md` |
| "Will this break determinism?" | `architecture/OVERVIEW.md` §Determinism |
| "Do debits still equal credits?" | Trace through `internal/templates/` + `internal/invariant/` |
