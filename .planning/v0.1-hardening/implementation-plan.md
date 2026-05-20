# Implementation Plan — fbx v0.1-hardening

Four sequential PRs harden the v0.1 codebase by addressing three known-deferred quality items: the `[][]string` pre-materialisation pattern in `internal/writer/csv.go`, coverage gaps in writer and CLI tests, and misleading comments in `internal/writer/manifest.go` and `internal/writer/truth.go`. Each PR builds directly on the prior; none can start until its predecessor is merged to `main`. The per-PR process follows `.agents/skills/engineering-workflow/SKILL.md`. No new behaviour, no new output files, no new CLI flags.

## Sequencing rules

- **Strict order.** PR-H(N) cannot start until PR-H(N-1) is merged to `main`.
- **Branch from `main`.** Every PR opens from `main`; no stacked branches.
- **Reviewer red flags are blocking.** See `.agents/reviewer.md` §Diff red flags. Unresolved findings prevent merge.
- **Golden fixtures must remain byte-identical throughout.** PR-H1 is a pure internal-representation change; run golden tests without `-update` to confirm. If any fixture byte changes, the refactor has introduced a behavioural change and must be corrected before opening the PR.
- **No new public packages.** All changes go inside existing packages (`internal/writer/`, `cmd/fbx/`).
- **Roadmap-deferred items are out of scope.** Anything in `plans/roadmap.md` v0.2+ must not appear — not as code, not as flags, not as stubs, not as `// TODO` comments.

---

## PR-H1 — CSV writer streaming refactor

**Objective.** Remove the `[][]string` pre-materialisation pattern from every CSV writer in `internal/writer/csv.go`. Each writer now calls `csv.Writer.Write` once per row inside the producer loop, keeping per-row memory O(1). Golden fixtures must remain byte-identical.

**Scope.**
- `internal/writer/csv.go`: eliminate all intermediate `rows [][]string` slices and the shared `writeCSV(cw, header, rows)` helper that takes a pre-built slice. Replace with an inline loop that calls `cw.Write(row)` per item. Each exported writer function (`WriteCoA`, `WriteJournalEntries`, `WriteGL`, `WriteBankStatement`, `writeSubLedger`, `writePartyMaster`) is refactored independently.
- The helper `newCSVWriter` (sets `UseCRLF = false`) is kept; add a `writeCSVHeader` helper if the pattern warrants it, or inline the header write — either is acceptable as long as no slice of all rows is built.
- Flush + close discipline, `UseCRLF = false`, `bufio` wrapping (applied by the caller via `writeFile`), and named return `err` patterns are preserved exactly.
- No changes to function signatures, exported names, or CSV column order.

**Out of scope.**
- Changes to `internal/writer/jsonl.go`, `manifest.go`, `truth.go`, or any other file.
- Benchmark additions (deferred; not a v0.1 requirement).
- Any change that alters output bytes — this is a refactor, not a schema change.

**Skills to apply.**
- `go-io-streaming` — streaming write pattern; no full-buffer accumulation; `cw.Write` inside the producer loop.
- `go-determinism` — line endings `\n`, `UseCRLF = false`, flush discipline unchanged.
- `engineering-workflow` — pre-code checklist; commit one logical unit; run golden tests without `-update` before committing.
- `go-idioms` — no change to public surface; no premature abstraction.

**Tests.**
- Run `go test ./internal/writer/... -race` without any `-update` flag; all existing tests must pass without fixture regeneration.
- Run `go test ./... -race` without `-update`; the full golden suite must be byte-identical to the committed fixtures.
- Verify `gofmt -l .` and `go vet ./...` are clean.
- Confirm `cw.UseCRLF` is false in every writer (code review; no test needed beyond the existing CRLF assertion in `csv_test.go`).

**Done when.**
- `go test ./... -race` passes with no `-update` flag and golden fixtures are unchanged.
- `go vet ./...` clean; `gofmt -d .` clean.
- No `[][]string` accumulation pattern remains in `internal/writer/csv.go` (confirmed by code review or `grep -n 'rows := make.*\[\]\[\]string'`).
- `writeCSV(cw, header, rows [][]string)` helper removed or replaced with a streaming equivalent that does not accept a pre-built slice.
- CI green.

**Risk.** The `writeCSV` helper is the single flush point for all CSV writers; removing it requires ensuring every refactored writer calls `cw.Flush()` and checks `cw.Error()` before returning — missing a flush in one writer will silently truncate output at small volume (where the bufio threshold is never exceeded) and only fail under load. Verify with a custom `io.Writer` that counts bytes and asserts the flushed total equals the written total.

---

## PR-H2 — Writer test coverage gaps

**Objective.** Close two coverage gaps in `internal/writer/`: (1) missing CRLF + header assertions for `customers.csv` and `vendors.csv` in `csv_test.go`, and (2) no unit tests for `writeTruthTo` in a new `truth_test.go`.

**Scope.**
- `internal/writer/csv_test.go`: add subtests for `WriteCustomers` and `WriteVendors` asserting:
  - No `\r\n` byte sequences anywhere in the output (CRLF guard).
  - First line is exactly the expected header (`id,name,email,phone,address,city,state,zip`).
  - Row count matches input length.
  - Use `bytes.Buffer` as the `io.Writer` sink; no file I/O in these tests.
- `internal/writer/truth_test.go` (new file): unit tests for `writeTruthTo` covering:
  - Schema fields: every record in the decoded output has `type`, `entry_id`, and `detail` fields (unmarshal and assert).
  - Sorted record order: write a `TruthLog` with records in reverse order; assert the output reflects the order passed in (the caller is responsible for sorting; `writeTruthTo` preserves input order — test this contract).
  - Well-formed JSON: `json.Unmarshal` of the full output returns no error.
  - Empty log: `writeTruthTo` with a nil or empty `TruthLog` writes a valid JSON object with an empty `records` array; returns count 0.
  - Use `bytes.Buffer` throughout; no filesystem interaction.

**Out of scope.**
- Changes to `csv.go`, `manifest.go`, `truth.go`, or any non-test file.
- Tests for `WriteTruth` (the filesystem-level function) — only `writeTruthTo` is tested here.
- Benchmark additions.
- Golden fixture updates (this PR adds no new production code).

**Skills to apply.**
- `go-testing` — table-driven tests; `t.Helper` in assertion helpers; `bytes.Buffer` sink to avoid file I/O.
- `go-io-streaming` — tests confirm the writer operates on an `io.Writer`, not a file path.
- `engineering-workflow` — pre-code checklist; no production code changes.

**Tests.**
- `TestWriteCustomers_CRLF`: `WriteCustomers` output contains no `\r` bytes.
- `TestWriteCustomers_Header`: first line of `WriteCustomers` output is `id,name,email,phone,address,city,state,zip`.
- `TestWriteVendors_CRLF`: `WriteVendors` output contains no `\r` bytes.
- `TestWriteVendors_Header`: first line of `WriteVendors` output is `id,name,email,phone,address,city,state,zip`.
- `TestWriteTruthTo_SchemaFields`: each decoded record has non-empty `type`, `entry_id`, `detail`.
- `TestWriteTruthTo_WellFormedJSON`: `json.Unmarshal` returns no error on the full output.
- `TestWriteTruthTo_Empty`: nil/empty log → valid JSON, count = 0.

**Done when.**
- `go test ./internal/writer/... -race` passes, including all new tests.
- `go vet ./...` clean; `gofmt -d .` clean.
- No production files changed.
- CI green.

**Risk.** The `writeTruthTo` function is unexported; the test file must be in `package writer` (not `package writer_test`) to access it. Confirm this does not create an import cycle before writing the test file.

---

## PR-H3 — cmd/fbx negative-path subprocess tests

**Objective.** Add three negative-path tests to `cmd/fbx/main_test.go`: bad `--period` value exits 1, unknown subcommand exits 1, and invariant abort exits 2 with no files written.

**Scope.**
- `cmd/fbx/main_test.go`: add the following tests using the already-established subprocess pattern (build binary once, run via `exec.Command`):
  - `TestBadPeriod`: run `fbx gen monthly-close --period bad --seed 1 --volume small`; assert exit code 1, stderr non-empty.
  - `TestUnknownSubcommand`: run `fbx unknown`; assert exit code 1, stderr non-empty.
  - `TestInvariantAbort`: invoke `run()` directly (in-process, not subprocess) with a `Config` that points `--out` at a non-writable directory on a system where the directory cannot be created (or use a pre-created non-writable dir via `os.Chmod`); assert return value is 2 and the output directory contains no files written by this run. Alternatively, if a non-writable-directory approach is too fragile across CI environments, construct a deliberately unbalanced `Dataset` in a `_test.go` helper and assert that `invariant.Check` returns an `InvariantError` and that the run exits 2. The in-process `run()` path is preferred over subprocess for this case to avoid binary build overhead in a slow CI cell.
- The binary build in `TestGenMonthlyClose` must not be duplicated; extract a `buildFBX(t *testing.T) string` helper with `t.Helper()` that is shared across all subprocess tests.

**Out of scope.**
- Changes to `main.go`, `internal/`, or any non-test file.
- Adding a `run()` export or changing its signature — it is already accessible within `package main` tests.
- New subcommands or flags.
- Performance or large-volume tests.

**Skills to apply.**
- `go-testing` — `t.Helper` in shared build helper; `t.TempDir()` for output isolation; table-driven where multiple negative cases share structure.
- `engineering-workflow` — no production code changes; pre-code checklist confirms scope is tests only.

**Tests.**
- `TestBadPeriod`: exit 1, stderr contains an error message.
- `TestUnknownSubcommand`: exit 1, stderr contains an error message.
- `TestInvariantAbort`: exit 2, no files present in `--out` directory after the run.

**Done when.**
- `go test ./cmd/fbx/... -race` passes, including all three new tests.
- `go vet ./...` clean; `gofmt -d .` clean.
- No production files changed.
- `buildFBX` helper is shared and annotated with `t.Helper()`.
- CI green.

**Risk.** The non-writable-directory approach for the invariant-abort test is unreliable under CI root users (Linux containers may run as root, for whom `os.Chmod(dir, 0)` does not prevent writes). The fallback — calling `run()` in-process with a Config whose `--out` points at a tmp dir, after injecting an invalid period that causes `gen.ParseConfig` to fail with exit 1 — does not test exit 2. Use the `invariant.Check` in-process path: construct a `gen.Dataset` with a known invariant violation in a `TestMain`-visible helper, call `run()` with a shim, or test `invariant.Check` return value separately. Document the chosen approach in a comment so the next engineer does not silently downgrade the coverage.

---

## PR-H4 — Manifest and truth comment cleanup

**Objective.** Correct the misleading comment in `internal/writer/manifest.go` line 83 and audit `internal/writer/truth.go` for any similarly inaccurate comments.

**Scope.**
- `internal/writer/manifest.go`: change the comment on the explicit `bw.Flush()` call inside `writeFile` from:
  > `// Flush bw so all buffered bytes are hashed before sum() is called.`
  to:
  > `// Flush so all buffered bytes reach the underlying file before Close.`
  The bytes reach the hasher via `io.MultiWriter` at write time, not at flush. The flush serves only to drain the bufio buffer to the file; hashing has already occurred.
- `internal/writer/truth.go`: the comment at line 54 reads:
  > `// Flush must happen before sum() to ensure all buffered bytes are hashed.`
  Correct it to:
  > `// Flush drains the bufio buffer to the underlying file; hashing occurred at write time via io.MultiWriter.`
- No other files changed.

**Out of scope.**
- Any logic changes — this PR is comments only.
- Refactoring the flush/hash ordering (the current ordering is correct; only the comment is wrong).
- Adding tests (comment correctness is verified by code review, not by tests).
- Any changes to `csv.go`, `jsonl.go`, or other writer files.

**Skills to apply.**
- `engineering-workflow` — pre-code checklist (even for comment-only PRs); gofmt clean; commit one logical unit.
- `go-idioms` — comments state WHY; corrected comments accurately describe the mechanism.

**Tests.**
- No new tests. Run `go test ./... -race` to confirm the comment-only diff does not inadvertently affect compilation or test outcomes.
- `go vet ./...` clean; `gofmt -d .` clean.

**Done when.**
- Both misleading comments are corrected in `manifest.go` and `truth.go`.
- `go test ./... -race` passes (no regressions).
- `go vet ./...` clean; `gofmt -d .` clean.
- CI green.
- No logic changes anywhere.

**Risk.** Minimal. A comment-only change cannot break compilation or tests. The only risk is introducing a typographical error that is misleading in a different way — proofread both corrected comments against the actual `io.MultiWriter` + `bufio` + SHA-256 data-flow before committing.

---

## Cross-cutting reminders

- **Golden fixtures are the prime correctness signal for PR-H1.** Run `go test ./... -race` (without `-update`) immediately after the refactor. Any byte difference in a fixture means the refactor changed output; stop and investigate before pushing.
- **No `float64` for any monetary value.** These PRs do not touch monetary logic, but any incidental test helper that constructs a `gen.Dataset` must use `shopspring/decimal` for all monetary fields. (`.agents/engineer.md` §Common mistakes)
- **No `time.Now()` in test helpers that construct `gen.Dataset`.** Timestamps in test datasets must derive from a fixed `time.Time` value, not `time.Now()`. (`.agents/engineer.md` §Common mistakes)
- **Sort map keys before any output derived from them.** Not a concern for PR-H1–H4 directly, but any test helper that builds a `map[string]Account` for a `gen.Dataset` must sort keys before iterating if the iteration result flows into an assertion. (`.agents/engineer.md` §Common mistakes)
- **`package writer` (not `package writer_test`) for tests of unexported functions.** `writeTruthTo` is unexported; `truth_test.go` must declare `package writer`. Confirm before writing. (PR-H2 risk)
- **No goroutines in v0.1.** These PRs introduce no concurrency. The existing subprocess tests in `main_test.go` use goroutines only to drain pipes, which is correct and pre-existing. Do not add new goroutines in internal packages. (`.agents/skills/go-determinism/SKILL.md` Rule 5)
- **Commit one logical unit per PR.** PR-H4 is a single commit touching two comment lines. PR-H1 is a single commit touching one file. PR-H2 and PR-H3 may each be a single commit or split by file — split only if the reviewer would benefit from independent review.
- **No roadmap-deferred items.** None of these PRs introduce flags, stubs, or `// TODO` comments for v0.2+ features. (`plans/roadmap.md`)
