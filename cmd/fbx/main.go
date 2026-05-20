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
	"flag"
	"fmt"
	"os"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

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

	// PR-01: ParseConfig success is the full extent of gen dispatch.
	// Full pipeline wiring (gen.Run → invariant.Check → writer.Write) is PR-08.
	_ = cfg
	return 0
}
