# thesis.md

## The problem

Anyone building, testing, or auditing financial software needs realistic data. Production data can't be used — it's regulated, sensitive, and tied up in contracts. The alternatives are bad:

- **Hand-rolled fixtures** that cover three cases and break on the fourth.
- **Random data** that produces numbers but not *reconcilable* numbers — debits don't equal credits, invoice totals don't match the GL, bank balances drift from cash accounts.
- **Anonymized production data**, which is expensive, legally fraught, and often still leaks.

Result: tests that pass on toy data and fail in production. Accounting agents that hallucinate plausible numbers because their eval data never had to balance. Reconciliation tools shipped without ever being run against a dataset large enough to expose their O(n²) loops.

## The claim

**Synthetic financial data can be reconcilable by construction, at any scale, deterministically.** Not "balanced after the fact" — *unable to be unbalanced*, because every document projects from the same double-entry event stream.

The trick is treating the generator as an event-sourced bookkeeping system. Templates emit balanced journal entries; every downstream document (GL, bank statement, invoice, expense report) is a view over the event log. If the math works once at the event, it works everywhere downstream — automatically, no glue code, no reconciliation pass.

## Why this matters

- **Eval datasets for finance agents** that scale from 100 to 10⁷ rows without losing internal consistency.
- **Load testing** of accounting systems against realistic month-end volumes.
- **Format conformance testing** against real-world standards using data that's structurally correct.
- **Teaching** — students get datasets that behave like real books.
- **Reproducible research** — same seed, same dataset, anywhere, forever.

## Why free and open source

Financial data tooling has historically been gated behind enterprise contracts, leaving academics, indie developers, students, and small teams to either fake it badly or pay too much. The reconciliation problem isn't a competitive moat — it's a public good every finance-adjacent project needs. Putting fauxbooks in the commons raises the bar for "did you actually reconcile this?" across the whole ecosystem.

Open source also means the grammar is auditable. When someone uses fauxbooks data to benchmark a reconciliation agent, they can read exactly how the data was constructed. No black-box "trust us, it balances."

## What success looks like

- A `go install` away. Single binary. No setup.
- Default settings produce a sensible month-end dataset in seconds.
- A seed reproduces byte-identical output anywhere.
- Used as test-data infrastructure by other open-source finance projects.
- "Test it on fauxbooks first" becomes routine in finance-software code review.
