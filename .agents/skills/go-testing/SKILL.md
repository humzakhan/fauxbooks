---
name: go-testing
description: Apply when writing tests for fauxbooks code. Covers table-driven tests, t.Helper, testdata/ conventions, golden-fixture pattern with -update flag, determinism tests, and invariant-test scaffolding.
---

# Go Testing — fauxbooks / fbx

See also: `go-determinism` (what to test for), `go-idioms` (constructor patterns),
`architecture/OVERVIEW.md` §Determinism and §Build layout.

---

## Table-Driven Tests

Every test with more than one case uses the standard Go table-driven pattern.
Subtest names must be specific enough that `-run TestDecimalFormat/neg-zero` targets one case.
Use `t.Errorf` when further assertions still make sense; `t.Fatalf` when they do not.

```go
cases := []struct{ name string; in decimal.Decimal; want string }{
    {"positive", decimal.NewFromFloat(100.5), "100.500000000"},
    {"zero",     decimal.Zero,                "0.000000000"},
    {"neg-zero", decimal.NewFromFloat(-0.0),  "0.000000000"},
}
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        if got := formatDecimal(tc.in); got != tc.want {
            t.Errorf("got %q, want %q", got, tc.want)
        }
    })
}
```

---

## t.Helper()

Every helper that calls `t.Fatal` / `t.Error` must start with `t.Helper()` so failures
surface at the call site, not inside the helper.

```go
func requireEqualFiles(t *testing.T, want, got string) {
    t.Helper()
    wb, err := os.ReadFile(want)
    if err != nil { t.Fatalf("read want: %v", err) }
    gb, err := os.ReadFile(got)
    if err != nil { t.Fatalf("read got: %v", err) }
    if !bytes.Equal(wb, gb) {
        t.Errorf("files differ.\n  want: %s\n   got: %s\nRun with -update to regenerate.", want, got)
    }
}
```

---

## testdata/ Conventions

`testdata/` is ignored by `go build`. Golden fixtures live there.

```
testdata/
  golden/
    monthly-close/small/{coa.csv, journal_entries.csv, gl.csv, ..., manifest.json}
    recon-stress-test/small/{..., truth.json}
```

- Commit golden fixtures at `small` volume only — fast CI, small VCS footprint.
- Never commit `medium` or `large` goldens.
- Fixture filenames must match output filenames exactly (case-sensitive).

---

## Golden-Fixture Pattern

```go
var update = flag.Bool("update", false, "rewrite golden fixtures instead of comparing")

func TestGoldenMonthlyClose(t *testing.T) {
    outDir   := t.TempDir()
    goldenDir := filepath.Join("testdata", "golden", "monthly-close", "small")

    if err := Run(Config{Seed: 1, Period: mustParsePeriod("2026-01"), Volume: "small"}, outDir); err != nil {
        t.Fatalf("Run: %v", err)
    }
    entries, _ := os.ReadDir(outDir)
    for _, e := range entries {
        if e.IsDir() { continue }
        got    := filepath.Join(outDir, e.Name())
        golden := filepath.Join(goldenDir, e.Name())
        if *update {
            data, _ := os.ReadFile(got)
            _ = os.WriteFile(golden, data, 0644)
            continue
        }
        requireEqualFiles(t, golden, got)
    }
}
```

Regenerate: `go test ./... -run TestGolden -update`. Commit fixture updates atomically
with the code change that caused the drift — see `.agents/skills/engineering-workflow/SKILL.md`
§Commit Hygiene.

---

## Determinism Test

Run the same generation twice on the same seed; assert byte-identical output.
CI runs this on linux/amd64, linux/arm64, darwin/arm64, windows/amd64 — any drift fails.

```go
func TestDeterminism(t *testing.T) {
    cfg := Config{Seed: 42, Period: mustParsePeriod("2026-03"), Volume: "small"}
    dir1, dir2 := t.TempDir(), t.TempDir()
    if err := Run(cfg, dir1); err != nil { t.Fatalf("run1: %v", err) }
    if err := Run(cfg, dir2); err != nil { t.Fatalf("run2: %v", err) }
    for _, e := range mustReadDir(t, dir1) {
        requireEqualFiles(t, filepath.Join(dir1, e.Name()), filepath.Join(dir2, e.Name()))
    }
}
```

---

## Invariant Tests

`internal/invariant/` needs unit tests for each of the 4 hard invariants using
deliberately broken inputs. Each invariant needs three cases: passing (valid input),
failing (broken input returns the correct sentinel error), and an edge case
(zero-value entry, single-line entry, etc.).

```go
func TestInvariantUnbalanced(t *testing.T) {
    entry := JournalEntry{Lines: []Line{
        {Side: Debit,  Amount: decimal.NewFromInt(100)},
        {Side: Credit, Amount: decimal.NewFromInt(99)}, // wrong
    }}
    if err := CheckBalance(entry); !errors.Is(err, ErrUnbalanced) {
        t.Errorf("got %v, want ErrUnbalanced", err)
    }
}
```

---

## Avoid Mocking

The seeded RNG and `--period` provide full input control. Prefer real code with
controlled inputs over mocks. Pass `bytes.Buffer` via the `io.Writer` constructor
when avoiding file I/O:

```go
var buf bytes.Buffer
w := writer.NewGLWriter(&buf) // real writer, in-memory sink
_ = w.WriteAll(generateEntries(rng.New(1), cfg))
```

---

## Race Detector and Subtest Isolation

`go test -race ./...` must pass cleanly. No package-level mutable state.
Each subtest that touches the filesystem uses its own `t.TempDir()` — never share
writable state (files, global vars) across subtests.

```go
t.Run(tc.name, func(t *testing.T) {
    tc := tc           // capture range variable
    dir := t.TempDir() // cleaned up when subtest ends
})
```

---

## Benchmarks

Optional for v0.1. Target the projection engine:

```sh
go test -bench BenchmarkProjectGL -benchmem ./internal/project/
```

Pre-generate events outside the `b.N` loop. Stable `allocs/op` confirms O(1)-per-row
memory; growth with `b.N` signals a buffering bug.
