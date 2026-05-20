# Goal: implement fbx v0.2

You are a fresh Claude Code session. The repo is at the path provided to you. Read the documents below before touching any code. This loop drives eight sequential PRs that ship the v0.2 roadmap candidates: derived financial reports, aging and customer documents, XLSX output, PDF output, `fbx verify`, `fbx convert`, YAML custom bundles and templates, and the `fbx mcp` stdio server.

## Context to read first (priority order)

- `.planning/v0.2/implementation-plan.md` — the binding PR-by-PR plan for this round; read this first and in full before any code
- `.planning/implementation-plan.md` — the original v0.1 plan; background on sequencing conventions and section conventions
- `architecture/architecture.md` — authoritative design: 4 packages in v0.1 (cmd/fbx, internal/gen, internal/invariant, internal/writer); v0.2 adds internal/report (PR-V01) and internal/reader (PR-V05); no other new public packages
- `architecture/OVERVIEW.md` — canonical spec: stack, invariants, decimal rules, bundles, v0.1 document list, volume tiers, exit codes
- `.agents/engineer.md` — pre-code checklist, code style, common mistakes, "when unsure" decision tree
- `.agents/reviewer.md` — red flags that block merge; read before opening any PR
- `.agents/skills/engineering-workflow/SKILL.md` — per-PR process (apply on every PR)
- `.agents/skills/go-io-streaming/SKILL.md` — streaming write rules; apply on PR-V01, PR-V02, PR-V03, PR-V06
- `.agents/skills/go-determinism/SKILL.md` — byte-identical output rules; apply on every PR that touches the generation path
- `.agents/skills/go-testing/SKILL.md` — test patterns: table-driven, `t.Helper`, golden-fixture pattern with `-update` flag, subprocess pattern; apply on every PR
- `.agents/skills/go-idioms/SKILL.md` — error wrapping, interface placement, package boundaries, constructor patterns; apply on every PR

## Execution loop

Repeat the following for PR-V01 through PR-V08 in strict sequence. Do not start PR-V0N until PR-V0(N-1) is merged.

1. Pull the latest `main`; create branch `pr/v0N-<slug>` from `main` (e.g. `pr/v01-financial-reports`, `pr/v02-aging-statements`, `pr/v03-xlsx`, `pr/v04-pdf`, `pr/v05-verify`, `pr/v06-convert`, `pr/v07-yaml-bundles`, `pr/v08-mcp`).
2. Read the `## PR-V0N` section in `.planning/v0.2/implementation-plan.md`. The Scope, Out of scope, Skills, Tests, and Done-when items are binding.
3. Implement exactly the Scope. Do not add anything listed under Out of scope. Do not implement items from `plans/roadmap.md` v0.3+.
4. Run these checks in order — all must be clean before committing:
   - `gofmt -d .` → no output
   - `go vet ./...` → no output
   - `go test ./... -race` → all pass (run without `-update` first; if existing golden fixtures differ, the implementation introduced a regression — fix it before updating fixtures)
5. Verify every item in the PR's "Done when" checklist. If any item is not met, fix it before continuing.
6. Commit with a conventional prefix and a body that lists the Scope items implemented:
   - PR-V01, V02: `feat(pr-v0N): <title>` — new compute and writer capabilities
   - PR-V03: `feat(pr-v03): xlsx output format`
   - PR-V04: `feat(pr-v04): pdf rendering and company-config`
   - PR-V05: `feat(pr-v05): fbx verify subcommand`
   - PR-V06: `feat(pr-v06): fbx convert subcommand`
   - PR-V07: `refactor(pr-v07): yaml custom bundles and templates` — refactor prefix because built-in behavior must not change
   - PR-V08: `feat(pr-v08): fbx mcp stdio server`
   - Golden fixture updates use the same commit as the code change (never a separate commit)
   - Test-only additions use `test(pr-v0N):` prefix
7. Push the branch and open a PR:
   ```
   gh pr create \
     --title "PR-V0N: <title>" \
     --body "Implements .planning/v0.2/implementation-plan.md §PR-V0N."
   ```
8. Stop. Wait for human review and merge before starting the next PR.

## Hard rules

- **No PR may exceed its Scope.** New requirements get their own PR after the v0.2 round is complete.
- **No code from `plans/roadmap.md` v0.3+.** Multi-currency, tax lines, payroll, parallel workers, sharding, official wire formats (OFX, UBL, NACHA, ISO 20022), accrual accounting, database loading, and the embeddable library are all deferred. None of these appear — not as flags, stubs, or `// TODO` comments.
- **v0.1 golden fixtures must remain byte-identical unless the PR explicitly adds to them.** PR-V01 and PR-V02 add new output files to the fixture directories (manifest.json expands); the existing files in those directories must not change by a single byte. Run `go test ./... -race` without `-update` and confirm before committing.
- **`go-determinism` applies to every PR that touches the generation path** (`internal/gen`, `internal/invariant`, `internal/writer`, `internal/report`, `internal/reader`, `cmd/fbx`). No `time.Now()` in `internal/`. No map iteration without sorted keys where output is derived. No `float64` for monetary values at any step, including intermediate accumulation.
- **No goroutines in v0.2 production code.** Single-threaded throughout. Pipe-draining goroutines in subprocess tests are pre-existing and acceptable; do not add new goroutines inside any `internal/` package. The MCP server in PR-V08 uses a synchronous request loop — one request in, one response out, repeat.
- **No `float64` for money.** Any compute function in `internal/report/` that accumulates to `float64` is always a bug. All monetary fields are `decimal.Decimal` through storage, arithmetic, and serialization.
- **Output only after invariants pass.** `fbx gen` (in all formats) and the `fbx_gen` MCP tool must run `invariant.Check` before writing any output. Exit 2 with no files written on invariant failure.
- **When unsure, consult `architecture/architecture.md` first**, then the "When unsure" decision table in `.agents/engineer.md`.

## Orchestration note

This loop follows the orchestrator pattern described in `.agents/skills/orchestrator-mode/SKILL.md`. Recommended model assignments:

- **Opus** for design decisions only: resolving a conflict between the plan and the architecture doc; deciding whether a workaround (e.g., ZIP modtime pinning for XLSX in PR-V03) belongs in this PR or warrants deferral; deciding whether a new validation path in PR-V07 needs its own error type.
- **Sonnet sub-agents** for context-heavy work: reading and cross-referencing multiple files to plan `internal/report/compute.go` account-type bucketing (PR-V01); drafting the `internal/reader/csv.go` parse logic after reading the writer output schema (PR-V05); writing the MCP tool dispatch and typed response structs (PR-V08); auditing determinism across a new writer against the existing patterns.
- **Haiku sub-agents** for mechanical work: running `gofmt`, `go vet`, `go test`; running `gh pr create`; listing directory contents with `ls`; grepping for `float64` in new files; checking `go.mod` for a new dependency.

Do not use Opus for file reading or for work a Sonnet sub-agent can do. Do not use Haiku for synthesizing across multiple files or writing substantive code. Reserve Opus for calls where a wrong decision would require rewinding the PR.

## Definition of done for the whole goal

All of the following must be true before v0.2 is considered complete:

- All 8 PRs (PR-V01 through PR-V08) merged to `main`; CI green on every commit on `main`.
- `go install ./cmd/fbx` still produces a working single static binary with no CGo.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out /tmp/mc` exits 0 and produces all v0.1 documents plus the 8 new financial-statement files (P&L, BS, CF, TB in CSV + JSONL) and the 8 new ancillary documents (AR aging, AP aging, customer statements, check register in CSV + JSONL).
- `fbx gen recon-stress-test --seed 1 --period 2026-01 --volume small --out /tmp/rst` exits 0 and produces the same document set; `truth.json` still present.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format xlsx --out /tmp/mc` exits 0 and produces a valid XLSX file with byte-identical output across two runs.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --format pdf --out /tmp/mc` exits 0 and produces 8 PDF files with byte-identical output across two runs.
- `fbx verify /tmp/mc` (on the CSV output of the monthly-close run) exits 0.
- `fbx convert --in /tmp/mc/gl.csv --out /tmp/gl.xlsx` exits 0 and produces a valid XLSX with decimal columns preserved as text.
- `fbx gen --bundle-config <valid-custom.yaml> --seed 1 --period 2026-01 --volume small --out /tmp/custom` exits 0; `fbx verify /tmp/custom` exits 0.
- `fbx mcp` responds to a `tools/list` JSON-RPC request with all 4 tools listed in alphabetical order; `fbx_gen` and `fbx_verify` tools work correctly via stdio.
- `go test ./... -race` passes for both existing golden bundles at small volume (byte-identical fixtures).
- `go vet ./...` clean; `gofmt -d .` clean on every merged commit.
