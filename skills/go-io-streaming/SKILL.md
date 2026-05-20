---
name: go-io-streaming
description: Apply when writing file output, CSV/JSONL writers, or any code under internal/writer/. Enforces streaming patterns, bufio wrapping, Flush/Close discipline, and O(1)-per-row memory.
---

# Go I/O Streaming — fauxbooks / fbx

Volume tiers reach ~10M rows per invocation. Every writer must be O(1) per row in memory.
No full-buffer materialization is ever acceptable outside `testdata/` fixtures.
See `architecture/OVERVIEW.md` §Scale for wall-time and memory budgets.

---

## The Core Rule

Never accumulate a full result set. Write one row at a time.

```go
// Good — streaming.
for _, event := range events {
    if err := w.Write(projectToGLRow(event)); err != nil {
        return fmt.Errorf("writer.GL: %w", err)
    }
}

// Bad — materializes everything before writing.
rows := collectAll(events)
writeAll(rows) // never do this for production output
```

---

## bufio Wrapping + File Creation

Wrap every `*os.File` before writing. Syscall frequency drives wall-time at volume.

```go
const writeBufSize = 1 << 20 // 1 MiB; use 64 KiB (1<<16) for small volume

f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
if err != nil {
    return fmt.Errorf("open %s: %w", path, err)
}
defer func() {
    if ferr := f.Close(); ferr != nil && err == nil {
        err = fmt.Errorf("close %s: %w", path, ferr)
    }
}()

bw := bufio.NewWriterSize(f, writeBufSize)
defer func() {
    if ferr := bw.Flush(); ferr != nil && err == nil {
        err = fmt.Errorf("flush %s: %w", path, ferr)
    }
}()
```

Use a named return `err` so deferred Flush/Close errors surface correctly.
Always check the `Close` error on write paths — unflushed OS page cache data can fail there.

Volume-tier guidance:

| Volume | GL rows | Recommended buf |
|---|---|---|
| small / medium | ~1k–10k | 64 KiB |
| large | ~100k+ | 1 MiB |

---

## io.Writer Composition

Writer constructors accept `io.Writer`, not `*os.File`. Tests pass `bytes.Buffer`;
the CLI passes a `bufio.Writer` over `os.File`.

```go
// internal/writer/gl.go
func NewGLWriter(w io.Writer) *GLWriter {
    return &GLWriter{csv: csv.NewWriter(w)}
}
```

`internal/writer/` types know nothing about file paths. The CLI is the only place
that opens files and wraps them in `bufio.Writer`.

---

## CSV Streaming

```go
cw := csv.NewWriter(w)
cw.UseCRLF = false // fauxbooks uses \n, not the RFC 4180 default \r\n

if err := cw.Write(header); err != nil {
    return fmt.Errorf("csv header: %w", err)
}
for _, row := range rows {
    if err := cw.Write(row); err != nil {
        return fmt.Errorf("csv row: %w", err)
    }
}
cw.Flush()
if err := cw.Error(); err != nil {
    return fmt.Errorf("csv flush: %w", err)
}
```

For outputs >1M rows consider `cw.Flush()` every 100k rows to bound peak buffer memory,
then flush again at the end. UTF-8 throughout; no BOM.

---

## JSONL Streaming

One JSON object per line. Never marshal a slice of all rows.

```go
enc := json.NewEncoder(w)
enc.SetEscapeHTML(false) // preserve < > & in company names

for _, obj := range objects {
    if err := enc.Encode(obj); err != nil { // Encode appends \n automatically
        return fmt.Errorf("jsonl encode: %w", err)
    }
}
```

Decimal values serialize as strings to preserve precision. `shopspring/decimal` does
this by default when used as a struct field with `json:"..."`.

---

## io.Copy vs Hand-Rolled Loops

- Prefer `io.Copy(dst, src)` when piping between a `Reader` and a `Writer`.
- Never use `ioutil.ReadAll` / `io.ReadAll` on large inputs; stream with `bufio.Scanner`
  or `csv.Reader` instead.
- Avoid `fmt.Fprintln` in hot loops — allocates an intermediate string. Use
  `csv.Writer.Write` or `json.Encoder.Encode`.

---

## Common Bugs

| Bug | Fix |
|---|---|
| Forgot `bufio.Writer.Flush()` | Defer Flush before Close; name the return err |
| Ignored `Close()` error | Named return + deferred capture (see bufio section) |
| CRLF line endings | Set `cw.UseCRLF = false` right after `csv.NewWriter` |
| `fmt.Fprintln` over map iteration | Nondeterministic order — see `go-determinism` skill |
| `io.ReadAll` on large file | Stream with `bufio.Scanner` or `csv.Reader` |
| Accumulating rows before write | Write inside the generation loop, not after |

---

## Memory Target

O(1) per-row allocation (proportional to row width, not row count). Peak heap <200 MiB
at large volume (see `docs/product.md` §Scale). Run `go test -benchmem ./internal/writer/...`
to confirm alloc counts do not grow with row count.
