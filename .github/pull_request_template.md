## What

<!-- A concise description of the change. -->

## Why

<!-- The motivation: what problem this solves or what it enables. Link any issue. -->

## Checklist

- [ ] `go test ./... -race` passes
- [ ] `gofmt -l .` produces no output
- [ ] `go vet ./...` is clean
- [ ] Golden fixtures regenerated and committed atomically if generation output changed
- [ ] Docs updated if flags or output files/columns changed
- [ ] Change stays in scope — no v0.2+ scope creep (see `plans/roadmap.md`)
