---
name: orchestrator-mode
description: Apply when handling multi-step tasks. Defines orchestrator role, model assignments (Opus = decisions, Sonnet = research, Haiku = mechanical), delegation heuristics, and parallelism rules.
---

# Orchestrator Mode — fauxbooks / fbx

## Role

The orchestrator makes decisions, synthesizes sub-agent output, and talks to the user. It does not do grunt work. Every minute spent reading files or running shell commands is a minute not spent deciding what to build.

---

## Model Assignments

| Model | Role | What it does |
|---|---|---|
| **Opus 4.7** | Orchestrator | Decision-making, synthesis, delegation, user communication |
| **Sonnet 4.6** | Sub-agent | Context building, multi-file reads, codebase exploration, drafting non-trivial content |
| **Haiku 4.5** | Sub-agent | Single-file reads, simple lookups, mechanical filesystem ops (`mkdir`, `mv`, `ln -s`, `ls`) |

These assignments are load-bearing. Do not use Opus for file reading. Do not use Haiku for synthesizing across multiple files or writing substantive content.

---

## Delegation Heuristics

### Delegate to Sonnet when:

- The task requires reading more than 2 files to answer a question.
- The task involves exploring an unfamiliar package or directory tree.
- The task requires drafting a doc section, SKILL.md, or non-trivial code block.
- The task requires understanding how two or more packages interact.

### Delegate to Haiku when:

- The task is a single-file read at a known path.
- The task is a sequence of filesystem operations with no reasoning required (`mkdir`, `cp`, `ln -s`, `chmod`).
- The task is a simple grep or exact-string lookup.
- The task is running a known, fixed shell command and returning its output verbatim.

### Do NOT delegate:

- Deciding what to build or which approach to take.
- Synthesizing across multiple sub-agent reports into a single recommendation.
- Writing the final response to the user.
- Choosing which sub-agents to spawn or in what order.
- Resolving a conflict between two sub-agent findings.

---

## Parallelism Rule

Send independent sub-agent calls in a single message. Sequence only when there is a data dependency.

**Parallel (correct):**
```
[Message to orchestrator]
  → Sonnet: read internal/templates/ and summarize the template interface
  → Sonnet: read internal/invariant/ and summarize the check surface
  → Haiku: ls internal/rng/
```

**Sequential (required when dependent):**
```
Step 1: Haiku reads manifest.json → returns field list
Step 2: Sonnet reads the files named in manifest.json → returns content
```

Never use a placeholder or guess a value that a sub-agent call would return. Wait for the result.

---

## Sub-Agent Prompt Structure

Every sub-agent prompt must be cold-startable. Include:

1. **Goal** — one sentence stating what the sub-agent must find or produce.
2. **Paths** — absolute paths to files or directories relevant to the task.
3. **What's already ruled out** — any approaches already tried or rejected, so the sub-agent doesn't repeat them.
4. **Output format** — ask for a concise report (bullet list or short prose), not raw file dumps.

Example prompt to Sonnet:
> Goal: Determine what data each template in `internal/templates/` emits and whether it produces balanced journal lines by construction or by assertion.
> Paths: `/Users/humza/projects/fauxbooks/internal/templates/`
> Already ruled out: nothing yet.
> Output: Bullet list, one entry per template file, stating: file name, what business event it models, whether balance is structural or asserted.

---

## Verification

Trust but verify. A sub-agent's summary describes what it intended to find. When stakes are non-trivial:

- Spot-check 1–2 specific claims against the actual file content before acting on the summary.
- If the sub-agent's report contradicts a known spec fact (e.g., claims a package imports `net/http`), re-query before trusting.
- If two sub-agent reports conflict, resolve the conflict yourself — do not send a third sub-agent to adjudicate.

---

## Reporting to the User

Keep it short. The user needs:

1. What changed or was found (1–3 bullets).
2. What is next (1 bullet, or "nothing — task is complete").
3. Any decision that requires user input before proceeding.

Do not relay raw sub-agent output. Synthesize it.

---

## fauxbooks-Specific Context for Sub-Agents

Include this in sub-agent prompts when the task involves the codebase:

- Binary: `fbx`. Language: Go. No CGo.
- Build layout: `cmd/fbx/`, `internal/{coa,events,templates,project,invariant,writer,rng}/`
- Spec doc: `architecture/OVERVIEW.md` (4 hard invariants + determinism invariant + doc list)
- All monetary values: `shopspring/decimal`, serialized as decimal text with 9 fractional digits.
- No wall-clock reads in the generation path. All timestamps derive from `--period`.
- v0.1 only. Anything in `plans/roadmap.md` v0.2+ is out of scope.
