// Command fbx is the fauxbooks CLI for generating synthetic financial datasets.
//
// Usage:
//
//	fbx gen <bundle> --seed N --period YYYY-MM --volume small|medium|large [--out DIR] [--format csv|jsonl]
//
// Exit codes:
//
//	0  success
//	1  configuration or input error
//	2  invariant violation or I/O failure (nothing written)
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/report"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// Version is the fbx binary version embedded at compile time.
// Use a package-level constant so tests and the version subcommand
// reference the same value without ldflags.
const Version = "v0.1.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and dispatches subcommands. It returns an exit code so
// that main can call os.Exit once, keeping defer statements effective in tests.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fbx gen <bundle> [flags]")
		return 1
	}

	switch args[0] {
	case "gen":
		return runGen(args[1:])
	case "version":
		fmt.Println(Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "fbx: unknown subcommand %q\n", args[0])
		return 1
	}
}

// runGen handles the "fbx gen <bundle>" subcommand.
func runGen(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fbx gen <bundle> [flags]")
		return 1
	}
	bundle := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("fbx gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		seed   = fs.Int64("seed", 0, "RNG seed (required for deterministic output)")
		period = fs.String("period", "", "target month in YYYY-MM format (required)")
		volume = fs.String("volume", "small", "size tier: small|medium|large")
		out    = fs.String("out", ".", "output directory")
		format = fs.String("format", "csv", "output format: csv|jsonl")
	)

	if err := fs.Parse(rest); err != nil {
		// fs already wrote the error to stderr.
		return 1
	}

	cfg, err := gen.ParseConfig(*seed, *period, *volume, bundle, *out, *format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fbx: config error: %v\n", err)
		return 1
	}
	cfg.Version = Version

	// Ensure output directory exists before generating anything.
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fbx: cannot create output directory: %v\n", err)
		return 2
	}

	// Step 1: run the full generation pipeline.
	ds, err := gen.Run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fbx: generation failed: %v\n", err)
		return 2
	}

	// Step 2: check all invariants before writing any files.
	// Stress bundles (recon-stress-test) deliberately inject invariant violations;
	// skip the check when Truth is non-nil (set only by InjectStress in gen.Run).
	// On invariant failure: print the violation and exit 2; nothing is written.
	if ds.Truth == nil {
		if err := invariant.Check(ds); err != nil {
			var ie gen.InvariantError
			if errors.As(err, &ie) {
				fmt.Fprintf(os.Stderr, "fbx: invariant %d violated: %s\n", ie.Invariant, ie.Detail)
			} else {
				fmt.Fprintf(os.Stderr, "fbx: invariant check failed: %v\n", err)
			}
			return 2
		}
	}

	// Step 3: derive period reports from the validated dataset. Compute is a
	// pure function — no RNG, no wall-clock, no I/O — so it cannot fail.
	reports := report.Compute(ds)

	// Step 4: write all output files.
	if err := writer.Write(ds, reports, cfg, cfg.OutDir); err != nil {
		fmt.Fprintf(os.Stderr, "fbx: write failed: %v\n", err)
		return 2
	}

	return 0
}
