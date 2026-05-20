# Goal: implement fbx v0.1

You are a fresh Claude Code session. The repo is at the path provided to you. Read the documents below before touching any code.

## Context to read first (priority order)

- `.planning/implementation-plan.md` — the binding PR-by-PR plan; read this before any code
- `architecture/architecture.md` — authoritative v0.1 design: 4 packages, no interfaces, single-pass pipeline, hardcoded bundles/templates
- `architecture/OVERVIEW.md` — canonical spec: stack, invariants, decimal rules, bundles, output formats, volume tiers, exit codes
- `.agents/engineer.md` — pre-code checklist, code style, common mistakes, "when unsure" decision tree
- `.agents/reviewer.md` — red flags that block merge; read before opening a PR
- `.agents/skills/engineering-workflow/SKILL.md` — per-PR process (apply on every PR)
- `.agents/skills/go-determinism/SKILL.md` — rules for byte-identical output (apply on every PR that touches the generation path)
- `.agents/skills/go-idioms/SKILL.md` — Go code conventions (apply on most PRs)
- `.agents/skills/go-testing/SKILL.md` — test patterns: golden fixtures, determinism tests, invariant negative tests

## Execution loop

Repeat the following for PR-01 through PR-09 in strict sequence. Do not start PR-N until PR-(N-1) is merged.

1. Pull the latest `main`; create branch `pr/NN-<slug>` from `main` (e.g. `pr/01-scaffold`).
2. Read the `## PR-NN` section in `.planning/implementation-plan.md`. The Scope, Out of scope, Skills, Tests, and Done-when items are binding.
3. Implement exactly the Scope. Do not add anything listed under Out of scope. Do not implement items from `plans/roadmap.md`.
4. Run these checks in order — all must be clean before committing:
   - `gofmt -d .` → no output
   - `go vet ./...` → no output
   - `go test ./... -race` → all pass
5. Verify every item in the PR's "Done when" checklist. If any item is not met, fix it before continuing.
6. Commit with a conventional title and a body that lists the Scope items:
   ```
   feat(PR-NN): <title>

   - <scope item 1>
   - <scope item 2>
   ...
   ```
7. Push the branch and open a PR:
   ```
   gh pr create \
     --title "PR-NN: <title>" \
     --body "Implements .planning/implementation-plan.md §PR-NN. See architecture/architecture.md §<relevant section>."
   ```
8. Stop. Wait for human review and merge before starting the next PR.

## Hard rules

- **No PR may exceed its Scope.** New requirements get their own PR after v0.1 is complete.
- **No code from `plans/roadmap.md`.** XLSX, PDF, derived reports, `fbx verify`, `fbx convert`, MCP server, custom bundles, official formats — none of these appear in v0.1, not even as flags, stubs, or `// TODO` comments.
- **`go-determinism` applies to every PR that touches the generation path** (`internal/gen`, `internal/invariant`, `internal/writer`). No `time.Now()` in `internal/`. No `math/rand` outside `internal/gen/rng.go`. No map iteration without sorted keys where output is derived.
- **No `float64` for money.** `shopspring/decimal` everywhere — storage, arithmetic, serialization. Decimal text serialized as exactly 9 fractional digits, no scientific notation, no thousands separator.
- **No goroutines in v0.1.** Single-threaded throughout. Parallel workers are deferred to v0.3.
- **The 4 invariants are the prime directive.** PR-06 adds the checker; PR-08 onward enforces exit 2 with no output on any failure. Never write output before all invariant checks pass.
- **When unsure, consult `architecture/architecture.md` first**, then the "When unsure" decision table in `.agents/engineer.md`.

## Orchestration note

This loop follows the orchestrator pattern described in `.agents/skills/orchestrator-mode/SKILL.md`. Recommended model assignments:

- **Sonnet sub-agents** for context-heavy drafting: generating the 50-account CoA, writing balanced templates for each event type, generating golden fixtures for the first time, reading and synthesizing multiple spec files.
- **Haiku sub-agents** for mechanical work: running `gofmt`, running `go test`, running `gh pr create`, `ls`, `mkdir`.
- **Reserve Opus** for design calls (e.g., resolving a conflict between the plan and the architecture doc) and synthesis between PRs (e.g., deciding whether a scope creep question requires a new PR or belongs in the current one).

## Definition of done for the whole goal

All of the following must be true before v0.1 is considered complete:

- All 9 PRs merged to `main`; CI green on every commit on `main`.
- `go test ./... -race` passes on linux/amd64, linux/arm64, darwin/arm64, windows/amd64 (GitHub Actions matrix, added in PR-09).
- Both golden fixtures committed: `testdata/golden/monthly-close/2026-01/small/` and `testdata/golden/recon-stress-test/2026-01/small/`.
- `fbx gen monthly-close --seed 1 --period 2026-01 --volume small --out /tmp/t` exits 0 and produces all documents listed in `architecture/OVERVIEW.md`.
- `fbx gen recon-stress-test --seed 1 --period 2026-01 --volume small --out /tmp/t` exits 0 and produces `truth.json` alongside all standard documents.
- `go install github.com/fauxbooks/fbx/cmd/fbx@latest` (or the resolved module path) produces a working binary.
