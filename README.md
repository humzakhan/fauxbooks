# fauxbooks / fbx

**fauxbooks is an open-source Go CLI that generates reconcilable-by-construction synthetic accounting datasets.** It treats the generator as an event-sourced bookkeeping system: each business event emits a balanced set of journal entries, and every output file — GL, bank statement, invoices, bills, expense reports — is a projection over that event stream. Consistency is the only way the data can be created; there is no separate reconciliation pass. Same seed + bundle + period + version produces byte-identical output across runs, OSes, and architectures. The primary audiences are engineers building accounting and fintech software, AI/ML teams evaluating finance agents against balanced ground-truth data, auditors and instructors who need realistic datasets without touching client data, and researchers who need reproducible benchmarks.

## Status

Spec phase — v0.1 in progress, no Go code committed yet.

## Repo map

| Path | What lives here |
|---|---|
| `docs/` | Thesis, product spec, narrative docs |
| `architecture/` | Technical architecture; `OVERVIEW.md` is the canonical tech spec |
| `architecture/subsystems/` | Per-package architecture docs (empty until v0.1 implementation begins) |
| `architecture/adrs/` | Architecture Decision Records (empty) |
| `plans/` | Roadmap and implementation phasing |
| `.agents/` | Engineer and reviewer agent personas (used by Claude Code, Codex, Cursor, etc.) |
| `.agents/skills/` | Shared Claude Code / Codex skill packs (workflow + Go-specific). `.claude/skills` and `.codex/skills` are symlinks into this directory. |

## Quick links

- Product pitch: [docs/thesis.md](docs/thesis.md)
- Product spec and invariants: [docs/product.md](docs/product.md)
- Technical architecture: [architecture/OVERVIEW.md](architecture/OVERVIEW.md)
- Roadmap: [plans/roadmap.md](plans/roadmap.md)
- Engineer persona: [.agents/engineer.md](.agents/engineer.md)
- Reviewer persona: [.agents/reviewer.md](.agents/reviewer.md)

## Working with agents

The repo uses a three-tier orchestrator pattern: Opus orchestrates (decomposes tasks, owns sequencing and integration), Sonnet handles context-heavy drafting and analysis, and Haiku handles mechanical operations (reformatting, renaming, repetitive edits). The engineer and reviewer agent personas in `.agents/` encode the expectations for each role. See `.agents/skills/orchestrator-mode/SKILL.md` for the full orchestration protocol. Skills are shared between Claude Code and Codex via symlinks: `.claude/skills` and `.codex/skills` both point to `.agents/skills/`, so any skill pack added once is available to both harnesses without duplication.

## License

TBD — MIT vs Apache-2.0. See [architecture/OVERVIEW.md](architecture/OVERVIEW.md) for the open question.
