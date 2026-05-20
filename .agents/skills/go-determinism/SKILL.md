---
name: go-determinism
description: Apply when writing or reviewing any code in the generation path (internal/events, internal/templates, internal/project, internal/writer, internal/rng). Enforces byte-identical output: no wall-clock, no unseeded RNG, no map-iteration leaks, no float for money, no goroutines in v0.1.
---

# Go Determinism — fauxbooks / fbx

The central promise: same `--seed` + bundle + period + version → byte-identical output
across runs, OSes, and architectures. See `architecture/OVERVIEW.md` §Determinism and
`docs/thesis.md` §The claim.

---

## Rule 1 — No Wall-Clock in the Generation Path

```go
// VIOLATION — anywhere under internal/
t := time.Now()

// ALLOWED — CLI only, for manifest.json `generated_at` (excluded from CI diff)
manifest.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
```

All timestamps in output files derive arithmetically from `--period`:

```go
postingDate := cfg.Period.AddDate(0, 0, offset) // offset from seeded RNG
```

Reviewer red flag: `time.Now()`, `time.Since()`, or `os.Getenv("TZ")` inside `internal/`.

---

## Rule 2 — All Randomness Through internal/rng/

```go
// VIOLATION — direct use anywhere outside internal/rng/
import "math/rand/v2"
n := rand.IntN(100)       // global source, nondeterministic

// CORRECT
src := rng.New(seed)      // wraps math/rand/v2 PCG with explicit seed
n := src.IntN(100)
```

`gofakeit` must use the seeded source, not its package-level default:

```go
// VIOLATION
name := gofakeit.Name()   // gofakeit's global source

// CORRECT
faker := gofakeit.New(0)
faker.Seed(src.Uint64())  // re-seed from our PCG
name := faker.Name()
```

`crypto/rand` is also a violation. The canonical pattern lives in `internal/rng/` —
follow it exactly. A second random source anywhere in `internal/` is always a violation.

---

## Rule 3 — Map Iteration Order Is Nondeterministic

`for k, v := range m` returns keys in randomized order. If iteration values reach a writer,
sort first.

```go
// VIOLATION
for code, acct := range chart.Accounts {
    w.Write([]string{code, acct.Name})
}

// CORRECT
codes := make([]string, 0, len(chart.Accounts))
for code := range chart.Accounts {
    codes = append(codes, code)
}
sort.Strings(codes) // or slices.Sort with Go 1.21+
for _, code := range codes {
    w.Write([]string{code, chart.Accounts[code].Name})
}
```

Reviewer red flag: any `range someMap` inside a function reachable from a writer.

---

## Rule 4 — Decimal Arithmetic, No float64

`shopspring/decimal` for all monetary values. `float64` for money is a hard violation.

```go
// VIOLATION — FPU result varies: 33.333333333333336
amount := 100.0 / 3.0

// CORRECT — deterministic
amount := decimal.NewFromInt(100).Div(decimal.NewFromInt(3))
```

Round at the point of mint (journal line creation), not at the writer:

```go
amount = amount.RoundBank(9)

// Serialize with exactly 9 fractional digits; guard against -0.
if amount.IsZero() {
    s = "0.000000000"
} else {
    s = amount.StringFixed(9)
}
```

Output: `100.000000000` — no separator, no exponent. See `architecture/OVERVIEW.md` §Decimal.

Probabilities and weights use integer ratios via PCG — no `float64` conversion needed:

```go
threshold := src.IntN(totalWeight) // weight[i]/sum(weights) without float
```

---

## Rule 5 — No Goroutines in v0.1

Goroutine scheduling is nondeterministic. v0.1 is single-threaded.

```go
// VIOLATION in v0.1
go func() { generateEvents(ch) }()

// CORRECT — sequential
events, err := generateEvents(cfg)
```

Do not introduce goroutines, `sync.WaitGroup`, channels, or `go` statements in `internal/`
code in v0.1. Parallel workers are deferred to v0.3.

---

## Rule 6 — Sorted Output for Unordered Collections

Any output file with no natural row ordering needs a documented sort key in `internal/writer/`:

```go
// Rows emitted in ascending account-code order for reproducibility.
sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
```

If the sort key is composite, document the tiebreaker in the same comment.

---

## Rule 7 — OS and Architecture Neutrality

| Concern | Rule |
|---|---|
| Line endings | `\n` only. Set `csv.Writer.UseCRLF = false`. |
| Path separators | Forward slashes in output/embedded paths. Never `os.PathSeparator`. |
| String encoding | UTF-8 throughout. No locale-dependent encoding. |
| `runtime.GOOS` | Never read in the generation path or output content. |
| Integer size | Use `int64` for row counts / offsets that could exceed 2³¹. |

---

## Quick Reference — Reviewer Checklist

```
[ ] No time.Now() / time.Since() inside internal/
[ ] No math/rand, math/rand/v2, crypto/rand import outside internal/rng/
[ ] No gofakeit default source — always seeded from internal/rng/
[ ] No `range map` whose values flow into a writer
[ ] No float64 for monetary arithmetic
[ ] No goroutines or channels in v0.1 internal/ code
[ ] Line endings \n, UTF-8, no runtime.GOOS in output paths
[ ] Golden fixtures updated atomically with any output-affecting change
```
