package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGenMonthlyClose is a subprocess smoke test. It builds a real fbx binary
// and runs "fbx gen monthly-close --seed 1 --period 2026-01 --volume small",
// asserting exit 0 and empty stderr.
func TestGenMonthlyClose(t *testing.T) {
	// Build the binary into a temp directory so the test is hermetic.
	dir := t.TempDir()
	bin := filepath.Join(dir, "fbx")

	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "." // cmd/fbx
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

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
