# CLAUDE.md — Claude Code notes for fauxbooks

Read [`AGENTS.md`](AGENTS.md) first. This file adds Claude-specific notes only.

---

## Developer vs. user

- **Developing this repo** — you are in the right place. Start with `AGENTS.md`.
- **Using `fbx` as a tool** — read [`docs/usage.md`](docs/usage.md) for CLI flags, output
  files, MCP interface, and examples.

---

## Claude-specific notes

**Orchestrator pattern.** For non-trivial work (new feature, refactor spanning multiple
packages, anything touching the generation pipeline), apply the orchestrator-mode skill
before writing code:

```
.claude/skills/orchestrator-mode/SKILL.md
```

This ensures Opus handles direction and synthesis; Sonnet and Haiku handle research and
mechanical work. Do not use Opus for routine coding tasks in this repo.

**Go skills.** All Go code in this repo is subject to the domain skills under
`.claude/skills/`. Apply the relevant skill(s) before writing or reviewing code:

- `go-determinism` — generation path, any code under `internal/gen`, `internal/writer`, `internal/rng`
- `go-testing` — all test files
- `go-io-streaming` — `internal/writer/` and any file-output code
- `go-idioms` — all Go code

**README and docs.** `README.md` is the user-facing landing page. `AGENTS.md` is the
developer-agent entry point. `docs/usage.md` is the full CLI/MCP reference. Do not
add implementation detail to README; do not add marketing prose to `docs/usage.md`.

**CI policy.** Local checks must pass before merging: `go test ./... -race`, `gofmt -l .`,
`go vet ./...`. GitHub Actions CI may be billing-blocked; do not wait for it.
