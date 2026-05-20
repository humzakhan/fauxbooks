package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
)

// buildFBX compiles the fbx binary into a temporary directory and returns the
// path to the resulting executable. The build is cached per test run via the
// standard Go test binary cache — callers should not cache the path themselves.
// The binary is placed inside t.TempDir() so it is removed when the test ends.
func buildFBX(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fbx")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// TestVersion is a subprocess smoke test that runs "fbx version" and asserts
// exit 0 with stdout "v0.1.0\n".
func TestVersion(t *testing.T) {
	bin := buildFBX(t)

	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatalf("fbx version: %v", err)
	}
	if got := string(out); got != "v0.1.0\n" {
		t.Errorf("fbx version stdout = %q; want %q", got, "v0.1.0\n")
	}
}

// TestGenMonthlyClose is a subprocess smoke test. It builds a real fbx binary
// and runs "fbx gen monthly-close --seed 1 --period 2026-01 --volume small",
// asserting exit 0 and empty stderr.
func TestGenMonthlyClose(t *testing.T) {
	bin := buildFBX(t)

	outDir := t.TempDir()
	cmd := exec.Command(bin,
		"gen", "monthly-close",
		"--seed", "1",
		"--period", "2026-01",
		"--volume", "small",
		"--out", outDir,
	)

	var stderrBuf, stdoutBuf []byte
	stderrPipe, _ := cmd.StderrPipe()
	stdoutPipe, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	// Drain pipes before Wait to avoid blocking on large output.
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			stderrBuf = append(stderrBuf, buf[:n]...)
			if err != nil {
				break
			}
		}
		close(done)
	}()
	buf := make([]byte, 4096)
	for {
		n, err := stdoutPipe.Read(buf)
		stdoutBuf = append(stdoutBuf, buf[:n]...)
		if err != nil {
			break
		}
	}
	<-done

	if err := cmd.Wait(); err != nil {
		t.Fatalf("fbx exited with error: %v\nstderr: %s\nstdout: %s", err, stderrBuf, stdoutBuf)
	}

	if len(stderrBuf) != 0 {
		t.Errorf("expected empty stderr, got: %s", stderrBuf)
	}

	// Verify the process exited 0 (Wait returned nil above).
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}

	// Suppress unused import warnings for os — use it to verify outDir exists.
	if _, err := os.Stat(outDir); err != nil {
		t.Errorf("outDir stat: %v", err)
	}
}

// TestBadPeriod verifies that fbx exits 1 and writes to stderr when the
// --period flag receives an invalid value.
func TestBadPeriod(t *testing.T) {
	bin := buildFBX(t)
	outDir := t.TempDir()

	cmd := exec.Command(bin,
		"gen", "monthly-close",
		"--period", "bad",
		"--seed", "1",
		"--volume", "small",
		"--out", outDir,
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	var stderrBuf []byte
	readBuf := make([]byte, 4096)
	for {
		n, readErr := stderr.Read(readBuf)
		stderrBuf = append(stderrBuf, readBuf[:n]...)
		if readErr != nil {
			break
		}
	}
	_ = cmd.Wait()

	if got := cmd.ProcessState.ExitCode(); got != 1 {
		t.Errorf("exit code: got %d, want 1", got)
	}
	if len(stderrBuf) == 0 {
		t.Error("expected non-empty stderr, got empty")
	}
}

// TestUnknownSubcommand verifies that fbx exits 1 and writes to stderr when
// given a subcommand that does not exist.
func TestUnknownSubcommand(t *testing.T) {
	bin := buildFBX(t)

	cmd := exec.Command(bin, "unknown")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	var stderrBuf []byte
	readBuf := make([]byte, 4096)
	for {
		n, readErr := stderr.Read(readBuf)
		stderrBuf = append(stderrBuf, readBuf[:n]...)
		if readErr != nil {
			break
		}
	}
	_ = cmd.Wait()

	if got := cmd.ProcessState.ExitCode(); got != 1 {
		t.Errorf("exit code: got %d, want 1", got)
	}
	if len(stderrBuf) == 0 {
		t.Error("expected non-empty stderr, got empty")
	}
}

// TestInvariantAbort verifies that invariant.Check detects an unbalanced
// Dataset and returns a gen.InvariantError with the expected invariant number.
//
// We call invariant.Check directly (in-process) rather than spawning a
// subprocess because the alternative — making an output directory unwritable
// via chmod — is unreliable when tests run as root in CI containers.
func TestInvariantAbort(t *testing.T) {
	// Construct a Dataset with an unbalanced journal entry (invariant 1 violated):
	// one entry with a debit of 100 but no corresponding credit.
	// Monetary amounts use decimal.Decimal per the no-float64-for-money rule.
	hundred := decimal.NewFromInt(100)
	ds := gen.Dataset{
		Entries: []gen.JournalEntry{
			{EntryID: "E-0001"},
		},
		Lines: []gen.JournalLine{
			{
				LineID:      "L-0001",
				EntryID:     "E-0001",
				AccountCode: "1000",
				Debit:       hundred,
				Credit:      decimal.Zero,
			},
			// Deliberately omit the balancing credit line to violate invariant 1.
		},
		CoA: map[string]gen.Account{
			"1000": {Code: "1000", Name: "Cash", Type: gen.AccountTypeAsset, NormalBalance: "debit"},
		},
		// Truth is nil so invariant.Check is not skipped (see runGen condition).
	}

	// invariant.Check must return an InvariantError for invariant 1.
	err := invariant.Check(ds)
	if err == nil {
		t.Fatal("invariant.Check: expected InvariantError, got nil")
	}
	var ie gen.InvariantError
	if !errors.As(err, &ie) {
		t.Fatalf("invariant.Check: expected gen.InvariantError, got %T: %v", err, err)
	}
	if ie.Invariant != 1 {
		t.Errorf("expected invariant 1 violation, got invariant %d", ie.Invariant)
	}
}
