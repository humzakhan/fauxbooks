---
name: critic-defender-loop
description: Apply when hardening a spec, ADR, or architecture doc before implementation. Runs a critic/defender loop that finds blocking issues, terminates on STABLE or IMPASSE, and logs accepted trade-offs.
---

# Critic/Defender Loop — fauxbooks / fbx

## When to Use

- Before writing code against a substantial new spec or architecture doc.
- Before a phase or milestone begins (e.g., before starting v0.2 work on a feature).
- When an architecture decision feels under-justified or has a known tension that hasn't been resolved.
- When a doc has been edited significantly and you want to harden it before others build against it.

Do not run this loop for: routine bug fixes, small feature additions within settled spec, or pure implementation work with no doc impact.

---

## The Two Roles

### Critic

Finds **blocking issues only**. Not preferences. Not style. Not "have you considered."

A blocking issue is one that, if unaddressed, will:
- Cause the implementation to fail or produce incorrect output.
- Require significant rework after code is written.
- Contradict another part of the spec in a way that makes compliance impossible.

Each issue must be:
- **Specific** — names the document section and the exact claim it targets.
- **Falsifiable** — states a condition that would prove the issue resolved.
- **Consequential** — states what breaks and when.

No alternatives. No praise. No softening. Finding holes and filling holes are separate jobs.

### Defender

Resolves each issue from the current cycle in the same cycle. For each Critic issue:
- Either produce a doc change (diff or rewrite of the targeted section) that closes the issue.
- Or accept the issue as an irresolvable trade-off, document the rationale, and add an entry to `docs/KNOWN_TRADE_OFFS.md`.

The Defender must address every VALID issue before the next Critic cycle begins.

---

## Issue Classification

The Critic classifies each issue before presenting it:

| Class | Meaning |
|---|---|
| **VALID** | A real blocking problem that requires resolution or an accepted trade-off. |
| **INVALID** | Not a blocking issue — is a preference, style note, or mitigable at implementation time without spec change. State why. |

---

## Issue Format (Critic)

```
ISSUE #N  [VALID | INVALID]
Document: <file path from repo root>
Section: <exact section heading>
Claim: <one to two sentences — what the doc asserts>
Problem: <one to two sentences — why that assertion is false, contradictory, or unverifiable>
Consequence: <what breaks, when, and how a developer or user experiences it>
```

INVALID issues are listed but not counted toward the cycle's blocking total.

---

## Resolution Format (Defender)

For each VALID issue from the current cycle:

```
RESOLUTION #N
Action: [DOC_CHANGE | TRADE_OFF]
Summary: <one sentence>
```

For `DOC_CHANGE`: include the specific text or structural change made (inline diff or replacement paragraph).

For `TRADE_OFF`: include a one-paragraph rationale and the entry added to `docs/KNOWN_TRADE_OFFS.md`.

---

## Loop Mechanics

1. Critic reads the target doc(s) cold and produces its issue list for cycle N.
2. Defender resolves all VALID issues from cycle N.
3. Docs are updated to reflect resolutions.
4. Cycle N+1 begins: Critic re-reads the updated docs and produces a new issue list.
5. Repeat until termination.

**Hard cap:** 10 cycles. If STABLE has not been reached by cycle 10, declare IMPASSE at cycle 10 and document open items.

---

## Termination Conditions

| Condition | Definition |
|---|---|
| **STABLE** | A full Critic pass produces zero new VALID issues. |
| **IMPASSE** | Critic and Defender have irreconcilable disagreement on a VALID issue for two consecutive cycles, or the 10-cycle cap is reached. |

At STABLE or IMPASSE, append a final entry to `docs/SESSION.md` recording the outcome.

---

## Output Artifacts

| Artifact | Location | Purpose |
|---|---|---|
| Session log | `docs/SESSION.md` | Append-only. One entry per cycle. Never edit retroactively. |
| Accepted trade-offs | `docs/KNOWN_TRADE_OFFS.md` | Each trade-off: ID, description, rationale, "would revisit if" condition. |
| Project-specific critic prompt | `docs/CRITIC.md` | Calibrated for fauxbooks. Reuse across sessions. Update if the doc set changes. |

---

## `docs/SESSION.md` Entry Format

```
## Critique Cycle N — YYYY-MM-DD

**Issues raised:** N
**VALID:** N — [comma-separated short labels]
**INVALID:** N — [comma-separated short labels, if any]
**TRADE-OFF:** N — [comma-separated short labels, if any]
**Documents changed:** [comma-separated list of paths]
**Outstanding:** [open tensions or flagged items for the next cycle, or "none"]
```

---

## `docs/KNOWN_TRADE_OFFS.md` Entry Format

```
## Trade-off N — <short title>

**Issue:** <one sentence>
**Rationale:** <one to three sentences>
**Consequence:** <what a developer or user gives up>
**Would revisit if:** <condition that would reopen the question>
```

---

## Critic Scope for fauxbooks

When writing `docs/CRITIC.md` for fauxbooks, focus the Critic on these areas:

1. **Invariant completeness** — do the 5 invariants cover every consistency property the output claims to have? Are there edge cases (e.g., zero-value entries, rounding in multi-line splits) where an invariant could pass but the output still be wrong?
2. **Determinism** — does the spec fully enumerate every source of non-determinism that must be eliminated? Does the golden-fixture strategy catch regressions at all scale levels?
3. **Sequencing** — are there dependencies between packages (`internal/events` → `internal/templates` → `internal/project` → `internal/invariant` → `internal/writer`) that the spec assumes but doesn't state?
4. **Scope boundary** — does the spec clearly distinguish v0.1 deliverables from v0.2+ candidates? Are there ambiguous items that could be implemented differently by two engineers?
5. **Format completeness** — are CSV and JSONL output formats fully specified (column order, null representation, decimal format, line endings)?

---

## How to Invoke

1. Write or update `docs/CRITIC.md` with the calibrated critic prompt for the target doc set.
2. Start a fresh session (no prior conversation context for the Critic role).
3. Critic produces cycle 1 issues.
4. Defender resolves in the same session or a follow-up session.
5. Append cycle entry to `docs/SESSION.md`.
6. Repeat until STABLE or IMPASSE.
7. Final `docs/SESSION.md` entry records outcome and any outstanding items for implementation-time awareness.
