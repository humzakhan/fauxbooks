# Contributing to fauxbooks

Thanks for your interest in contributing. This guide covers everything you need
to get a change from local dev to a merged PR.

For deep implementation detail, read [AGENTS.md](AGENTS.md). For the code of
conduct, read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

---

## Getting started

```bash
git clone https://github.com/humzakhan/fauxbooks.git
cd fauxbooks
go build ./...          # confirm the repo builds cleanly
go test ./... -race     # run the full suite
```

Requires Go 1.24 or later. No CGo. No external runtime dependencies.

---

## Before opening a PR

All three checks must pass with no output or failures:

```bash
go test ./... -race   # full suite with race detector
gofmt -l .            # must produce no output (no unformatted files)
go vet ./...          # must be clean
```

CI runs exactly these same steps on linux/amd64, linux/arm64, darwin/arm64,
and windows/amd64. GitHub Actions CI may be billing-gated on forks; the local
checks are the gate that matters.

---

## The five generation invariants

Every change must preserve all five. A PR that breaks any of these does not ship.

| # | Name | Rule |
|---|---|---|
| 1 | Balanced entries | Every journal entry: Σ debits = Σ credits exactly. |
| 2 | Account-code closure | Every account code on any GL line exists in the CoA for that run. |
| 3 | Bank-to-GL tie | Bank statement cash totals equal GL cash account postings. |
| 4 | Sub-ledger referential integrity | Every sub-ledger row references an existing journal entry ID. |
| 5 | Determinism | Same seed + bundle + period + version → byte-identical output across runs, OSes, and architectures. |

Invariants 1–4 abort the run at write time (exit 2, nothing written). Invariant 5
is verified by CI golden fixtures. If your change touches the generation path,
regenerate fixtures and confirm the diff is intentional:

```bash
go test ./... -run TestGolden -update
git diff testdata/golden/   # must be empty unless generation changed on purpose
```

When generation does change, commit the updated fixtures atomically with the
code change — never in a separate follow-up commit.

---

## Commit hygiene

- Small, focused commits. One logical change per commit.
- Commit message format: `type(scope): short description` (e.g. `feat(gen): add payroll event type`).
- Every commit must leave the repo in a state where all three local checks pass.
- `gofmt -l .` must produce no output — format before committing, not as a cleanup pass.
- Golden fixture updates are committed atomically with the code that caused them.

---

## Opening a PR

1. Branch from `main`: `git checkout -b my-feature main`
2. Make your changes, keeping commits small and passing the local checks.
3. Open a PR against `main`. Fill in the PR template — the checklist mirrors the local-checks gate.
4. A reviewer will check that all five invariants are preserved and that CI (or equivalent local verification) is clean.

---

## AI-agent skills

The `.agents/` directory holds shared skills and personas used by AI coding
agents (Claude Code, Codex, Cursor). These are the same files symlinked under
`.claude/skills/` and `.codex/skills/`. If you are contributing via an AI agent,
point it at `.agents/engineer.md` and the relevant skill files before writing code.

---

## Questions

Open an issue or reach out at hello@humza.io.
