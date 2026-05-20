---
name: go-idioms
description: Apply when writing or reviewing Go code. Enforces error wrapping, interface placement, package boundaries (cmd/internal), constructor patterns, no-init-side-effects, and gofmt cleanliness.
---

# Go Idioms — fauxbooks / fbx

Authoritative build layout and package responsibilities: `architecture/OVERVIEW.md` §Build layout.

---

## Error Handling

Return errors; do not panic in library code (`internal/...`). The only sanctioned halt is an
`InvariantError` in `internal/invariant/` which the CLI converts to exit code 2 and writes nothing.

```go
// Wrap at every call boundary so the chain is navigable.
if err := doSomething(); err != nil {
    return fmt.Errorf("events.Generate: %w", err)
}

// Sentinel errors — compare with errors.Is, not ==.
var ErrUnbalanced = errors.New("invariant: debits != credits")

if errors.Is(err, ErrUnbalanced) { ... }
```

Rules:
- Wrap with `fmt.Errorf("context: %w", err)` at every package boundary.
- Sentinel errors are package-level `var`, not `const`.
- Do not wrap the same error twice in a single call stack segment.
- `log.Fatal` and `os.Exit` are allowed only in `cmd/fbx/main.go`.
- `panic` is never acceptable in `internal/`; recover is never needed.

---

## Interfaces

Accept interfaces, return concrete types.

```go
// Good — interface defined at the consumer (writer package).
type RowEmitter interface {
    Emit(row []string) error
    Flush() error
}

// Bad — interface defined at the producer and exported there.
// internal/events/emitter.go: type Emitter interface { ... }  // don't do this
```

Rules:
- Interfaces belong in the package that *uses* them, not the package that *provides* them.
- Keep interfaces small: 1–3 methods. A 10-method interface is a concrete type wearing a costume.
- Embed small interfaces to compose larger ones if needed; avoid monolithic interface definitions.

---

## Package Boundaries

```
cmd/fbx/           CLI assembly: flag parsing, wiring, os.Exit
internal/coa/      Chart of accounts — built-in data
internal/events/   Event generation
internal/templates/ Balanced-entry templates
internal/project/  GL + bank + sub-ledger projection
internal/invariant/ Write-time invariant checks
internal/writer/   CSV + JSONL streaming writers
internal/rng/      Seeded RNG (single source of truth for randomness)
```

Rules:
- `cmd/fbx/` imports `internal/` packages; `internal/` packages never import `cmd/fbx/`.
- No cyclic imports. Run `go build ./...` — the toolchain enforces it.
- Do not add a new `internal/` package without a clear single responsibility. Shared helpers
  that do not fit an existing package belong in `internal/` with a descriptive name, not in
  a catch-all `internal/util/`.
- Cross-package dependencies flow: `cmd` → `invariant` → `project` → `templates` → `events` → `rng`.
  `coa` and `writer` are leaves. Adding an edge that reverses this order requires explicit review.

---

## Naming

| Scope | Rule |
|---|---|
| Narrow (inside a short function) | Short: `r`, `w`, `n`, `err` |
| Wide (package-level, exported) | Descriptive: `GenerateEvents`, `ChartOfAccounts` |
| Exported types and funcs | Doc-commented (one-line minimum) |
| No Hungarian | `strName` → `name`; `intCount` → `count` |
| No stuttering | `coa.CoaChart` → `coa.Chart`; `rng.RNGSource` → `rng.Source` |
| Acronyms | All-caps: `ID`, `CSV`, `GL`, `AP`, `AR` |

---

## Constructors

```go
// Return a concrete type + error. No init() side effects.
func NewWriter(w io.Writer, sep rune) (*CSVWriter, error) {
    if w == nil {
        return nil, errors.New("writer: nil io.Writer")
    }
    return &CSVWriter{enc: csv.NewWriter(w), sep: sep}, nil
}
```

Rules:
- `New*` functions return a concrete pointer (or value) and `error`.
- No package-level `init()` that mutates global state. If setup is required, make it explicit
  construction via a `New*` call.
- No package-level singletons (`var globalRNG = ...`). Anything stateful is owned by a struct
  and passed explicitly. This is what makes the codebase testable and determinism-safe.

---

## Concurrency

v0.1 is single-threaded. This is intentional — see `architecture/OVERVIEW.md` §Determinism.

- Do not introduce goroutines in v0.1. Parallel workers are deferred to v0.3.
- If a future phase introduces goroutines: no shared writable state. Use channels or
  per-goroutine ownership (`sync.Mutex` as a last resort, not a first instinct).
- Package-level mutable variables (`var x = 0` at package scope that gets written) are
  forbidden — they prevent `go test -race ./...` from passing cleanly.

---

## Context

Pass `context.Context` as the first argument to any function that could be long-running
or cancelled. In v0.1 the generation path is synchronous and short, so context is not
threaded through generation internals — but CLI-entry functions should accept one for
forward compatibility.

```go
// v0.1 stub pattern — accept now, wire later.
func Generate(ctx context.Context, cfg Config) (*Result, error) {
    _ = ctx // TODO v0.2: check cancellation in the event loop
    ...
}
```

---

## Formatting and Hygiene

- `gofmt -l .` must return empty output before any commit. Run `goimports` too if available.
- Magic numbers: extract as `const`. Package-level if shared across functions; function-level
  if used only in one place.

```go
const (
    decimalPlaces    = 9
    bufferSizeBytes  = 64 * 1024
    maxRowsPerVolume = 10_000_000
)
```

- No `//nolint` or `//go:build ignore` except with an explanatory comment and a tracking issue.
- All exported symbols need a doc comment. `// Chart is the chart of accounts for a bundle run.`
  One sentence is sufficient; do not write a novel.
