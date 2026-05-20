# Goal: harden fbx v0.1 (post-v0.1 quality round)

You are a fresh Claude Code session. The repo is at the path provided to you. Read the documents below before touching any code.

## Context to read first (priority order)

- `.planning/v0.1-hardening/implementation-plan.md` — the binding PR-by-PR plan for this hardening round; read this before any code
- `.planning/implementation-plan.md` — the original v0.1 plan; background context on sequencing conventions and section conventions
- `architecture/architecture.md` — authoritative v0.1 design: 4 packages, no interfaces, single-pass pipeline, hardcoded bundles/templates
- `architecture/OVERVIEW.md` — canonical spec: stack, invariants, decimal rules, bundles, output formats, volume tiers, exit codes
- `.agents/engineer.md` — pre-code checklist, code style, common mistakes, "when unsure" decision tree
- `.agents/reviewer.md` — red flags that block merge; read before opening a PR
- `.agents/skills/engineering-workflow/SKILL.md` — per-PR process (apply on every PR)
- `.agents/skills/go-io-streaming/SKILL.md` — streaming write rules; apply on PR-H1
- `.agents/skills/go-determinism/SKILL.md` — byte-identical output rules; apply on PR-H1
- `.agents/skills/go-testing/SKILL.md` — test patterns: table-driven, `t.Helper`, `bytes.Buffer` sink, subprocess pattern; apply on PR-H2 and PR-H3

## Execution loop

Repeat the following for PR-H1 through PR-H4 in strict sequence. Do not start PR-H(N) until PR-H(N-1) is merged.

1. Pull the latest `main`; create branch `pr/hN-<slug>` from `main` (e.g. `pr/h1-csv-streaming`).
2. Read the `## PR-HN` section in `.planning/v0.1-hardening/implementation-plan.md`. The Scope, Out of scope, Skills, Tests, and Done-when items are binding.
3. Implement exactly the Scope. Do not add anything listed under Out of scope. Do not implement items from `plans/roadmap.md`.
4. Run these checks in order — all must be clean before committing:
   - `gofmt -d .` → no output
   - `go vet ./...` → no output
   - `go test ./... -race` → all pass (for PR-H1: without `-update`; golden fixtures must be byte-identical)
5. Verify every item in the PR's "Done when" checklist. If any item is not met, fix it before continuing.
6. Commit with a conventional title and a body that lists the Scope items:
   ```
   refactor(PR-HN): <title>

   - <scope item 1>
   - <scope item 2>
   ...
   ```
   Use `test(PR-HN):` for PR-H2 and PR-H3. Use `docs(PR-H4):` for PR-H4.
7. Push the branch and open a PR:
   ```
   gh pr create \
     --title "PR-HN: <title>" \
     --body "Implements .planning/v0.1-hardening/implementation-plan.md §PR-HN."
   ```
8. Stop. Wait for human review and merge before starting the next PR.

## Hard rules

- **No PR may exceed its Scope.** New requirements get their own PR after the hardening round is complete.
- **No code from `plans/roadmap.md`.** XLSX, PDF, derived reports, `fbx verify`, `fbx convert`, MCP server, custom bundles, official formats — none of these appear, not even as flags, stubs, or `// TODO` comments.
- **Golden fixtures must be byte-identical after PR-H1.** This is a refactor; output must not change by a single byte. Run `go test ./... -race` without `-update` and confirm. If any fixture differs, the refactor introduced a behavioural change and must be corrected before the PR opens.
- **`go-determinism` applies to every PR that touches the generation path** (`internal/gen`, `internal/invariant`, `internal/writer`). No `time.Now()` in `internal/`. No map iteration without sorted keys where output is derived. (PR-H1 touches `internal/writer/`; rules apply.)
- **No goroutines in v0.1 production code.** Single-threaded throughout. Pipe-draining goroutines in existing subprocess tests are pre-existing and acceptable; do not add new goroutines in `internal/` packages.
- **No `float64` for money.** Any test helper that constructs a `gen.Dataset` must use `shopspring/decimal` for all monetary fields.
- **`writeTruthTo` is unexported.** The test file `truth_test.go` must declare `package writer` (not `package writer_test`) to access it.
- **When unsure, consult `architecture/architecture.md` first**, then the "When unsure" decision table in `.agents/engineer.md`.

## Orchestration note

This loop follows the orchestrator pattern described in `.agents/skills/orchestrator-mode/SKILL.md`. Recommended model assignments:

- **Sonnet sub-agents** for context-heavy work: reading and cross-referencing multiple writer files to plan the streaming refactor in PR-H1; drafting the test cases for PR-H2 and PR-H3 after reading existing test patterns; auditing comments in PR-H4 against the actual `io.MultiWriter` data-flow.
- **Haiku sub-agents** for mechanical work: running `gofmt`, running `go test`, running `gh pr create`, `grep` for `\[\]\[\]string` patterns, `ls` of directories, line counts.
- **Reserve Opus** for design calls (e.g., resolving a conflict between the plan and the architecture doc, or deciding whether a test-harness approach for the invariant-abort test belongs in PR-H3 or warrants a new PR).

## Definition of done

All of the following must be true before the v0.1-hardening round is considered complete:

- All 4 PRs (PR-H1 through PR-H4) merged to `main`; CI green on every commit on `main`.
- `go test ./... -race` passes; golden fixtures under `testdata/golden/` are byte-identical to the pre-hardening fixtures (PR-H1 introduced no output change).
- No `[][]string` pre-materialisation pattern remains in `internal/writer/csv.go`; every CSV writer emits one row at a time.
- New test assertions pass: `customers.csv` and `vendors.csv` CRLF guard and header check; `writeTruthTo` schema, well-formed JSON, and empty-log tests.
- Three negative-path tests pass in `cmd/fbx/main_test.go`: bad period exits 1, unknown subcommand exits 1, invariant abort exits 2 with no output files written.
- Misleading comments in `internal/writer/manifest.go` (`writeFile`, line ~83) and `internal/writer/truth.go` (`WriteTruth`, line ~54) are corrected to accurately describe the `io.MultiWriter` + `bufio` + SHA-256 data-flow.
- `go vet ./...` clean; `gofmt -d .` clean.
