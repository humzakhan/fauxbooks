// Command fbx is the fauxbooks CLI for generating synthetic financial datasets.
//
// Usage:
//
//	fbx gen [<bundle>] --seed N --period YYYY-MM --volume small|medium|large [--out DIR] [--format csv|jsonl|xlsx|pdf] [--company-config PATH] [--bundle-config PATH] [--templates-config PATH]
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
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/reader"
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
	case "verify":
		return runVerify(args[1:])
	case "convert":
		return runConvert(args[1:])
	case "version":
		fmt.Println(Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "fbx: unknown subcommand %q\n", args[0])
		return 1
	}
}

// runVerify handles the "fbx verify <dir>" subcommand.
// Exit codes:
//
//	0 — all invariants pass
//	1 — configuration or I/O error (directory not found, required CSV missing, parse error)
//	2 — at least one invariant violated (detail printed to stderr)
func runVerify(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: fbx verify <dir>")
		return 1
	}
	dir := args[0]

	ds, err := reader.Load(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fbx: %v\n", err)
		return 1
	}

	// invariant.Check is always run; ds.Truth is left nil by reader.Load.
	// Verifying recon-stress-test output (which has deliberately injected
	// mismatches) will produce exit 2 — this is expected and documented.
	if err := invariant.Check(ds); err != nil {
		var ie gen.InvariantError
		if errors.As(err, &ie) {
			fmt.Fprintf(os.Stderr, "fbx: invariant %d violated: %s\n", ie.Invariant, ie.Detail)
		} else {
			fmt.Fprintf(os.Stderr, "fbx: invariant check failed: %v\n", err)
		}
		return 2
	}

	return 0
}

// runGen handles the "fbx gen [<bundle>] [flags]" subcommand.
// When --bundle-config is provided, the positional <bundle> argument is optional;
// the bundle name is taken from the YAML file. When absent, <bundle> is required.
func runGen(args []string) int {
	// Determine whether the first argument is a bundle name or a flag.
	// A bundle name never starts with '-'. When --bundle-config is supplied
	// the caller may omit the positional bundle argument entirely.
	bundle := ""
	rest := args
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		bundle = args[0]
		rest = args[1:]
	}

	fs := flag.NewFlagSet("fbx gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		seed            = fs.Int64("seed", 0, "RNG seed (required for deterministic output)")
		period          = fs.String("period", "", "target month in YYYY-MM format (required)")
		volume          = fs.String("volume", "small", "size tier: small|medium|large")
		out             = fs.String("out", ".", "output directory")
		format          = fs.String("format", "csv", "output format: csv|jsonl|xlsx|pdf")
		companyConfig   = fs.String("company-config", "", "path to YAML company config file for PDF output")
		bundleConfig    = fs.String("bundle-config", "", "path to YAML bundle config file (optional; overrides built-in bundle)")
		templatesConfig = fs.String("templates-config", "", "path to YAML templates config file (optional)")
	)

	if err := fs.Parse(rest); err != nil {
		// fs already wrote the error to stderr.
		return 1
	}

	// Load YAML bundle if --bundle-config is provided.
	// This path bypasses ParseConfig's built-in bundle name validation.
	var customBundle *gen.BundleDefinition
	if *bundleConfig != "" {
		bd, err := gen.LoadBundleConfig(*bundleConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fbx: bundle-config: %v\n", err)
			return 1
		}
		customBundle = &bd
		// Use the bundle name from YAML if no positional arg was given.
		if bundle == "" {
			bundle = bd.Name
		}
	}

	// Load YAML templates if --templates-config is provided.
	// In v0.2 these are validated but not yet wired into the Expand path;
	// the flag is accepted and validated to surface errors early.
	if *templatesConfig != "" {
		if _, err := gen.LoadTemplatesConfig(*templatesConfig); err != nil {
			fmt.Fprintf(os.Stderr, "fbx: templates-config: %v\n", err)
			return 1
		}
	}

	// For custom bundles, validate period/volume/format without bundle name check.
	// For built-in bundles, ParseConfig validates all fields including bundle name.
	var cfg gen.Config
	var cfgErr error
	if customBundle != nil {
		cfg, cfgErr = gen.ParseConfigNoBundle(*seed, *period, *volume, bundle, *out, *format)
	} else {
		if bundle == "" {
			fmt.Fprintln(os.Stderr, "usage: fbx gen <bundle> [flags]")
			return 1
		}
		cfg, cfgErr = gen.ParseConfig(*seed, *period, *volume, bundle, *out, *format)
	}
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "fbx: config error: %v\n", cfgErr)
		return 1
	}
	cfg.Version = Version

	// Ensure output directory exists before generating anything.
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fbx: cannot create output directory: %v\n", err)
		return 2
	}

	// Step 1: run the full generation pipeline.
	var ds gen.Dataset
	var genErr error
	if customBundle != nil {
		ds, genErr = gen.RunWithBundle(cfg, *customBundle)
	} else {
		ds, genErr = gen.Run(cfg)
	}
	if genErr != nil {
		fmt.Fprintf(os.Stderr, "fbx: generation failed: %v\n", genErr)
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

	// periodEnd is the last nanosecond of the period month.
	// cfg.Period is the first instant of the month (YYYY-MM → UTC midnight day 1).
	periodEnd := cfg.Period.AddDate(0, 1, 0).Add(-time.Nanosecond)

	// Compute the three new v0.2 derived reports and assign them to the Reports struct.
	reports.Aging = report.ComputeAging(ds, periodEnd)
	reports.CustomerStatements = report.ComputeStatements(ds)
	reports.CheckRegister = report.ComputeCheckRegister(ds)

	// Step 4: write all output files.
	// Dispatch based on format; each format is a complete substitute for the others.
	var writeErr error
	switch cfg.Format {
	case "xlsx":
		writeErr = writer.WriteXLSX(ds, reports, cfg, cfg.OutDir)
	case "pdf":
		var companyCfg writer.CompanyConfig
		if *companyConfig != "" {
			companyCfg, writeErr = writer.LoadCompanyConfig(*companyConfig)
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "fbx: company-config: %v\n", writeErr)
				return 1
			}
		} else {
			companyCfg = writer.DefaultCompanyConfig()
		}
		writeErr = writer.WritePDF(reports, cfg, companyCfg, cfg.OutDir)
	default:
		writeErr = writer.Write(ds, reports, cfg, cfg.OutDir)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "fbx: write failed: %v\n", writeErr)
		return 2
	}

	return 0
}
