package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execFBX runs the pre-built fbx binary with args, drains stdout and stderr,
// waits for completion, and returns (stderr string, exit code). It never calls
// t.Fatal — the caller handles error checking.
func execFBX(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	// Drain both pipes concurrently to avoid blocking.
	stderrDone := make(chan []byte, 1)
	go func() {
		var buf []byte
		tmp := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if err != nil {
				break
			}
		}
		stderrDone <- buf
	}()
	// Drain stdout (discard).
	stdoutBuf := make([]byte, 4096)
	for {
		_, err := stdoutPipe.Read(stdoutBuf)
		if err != nil {
			break
		}
	}
	stderrBytes := <-stderrDone

	_ = cmd.Wait()
	return string(stderrBytes), cmd.ProcessState.ExitCode()
}

// TestVerifyRoundTrip is a subprocess round-trip test:
// generate a dataset via fbx gen, then verify it via fbx verify, assert exit 0.
func TestVerifyRoundTrip(t *testing.T) {
	bin := buildFBX(t)
	outDir := t.TempDir()

	// Step 1: generate.
	_, genCode := execFBX(t, bin,
		"gen", "monthly-close",
		"--seed", "1",
		"--period", "2026-01",
		"--volume", "small",
		"--out", outDir,
	)
	if genCode != 0 {
		t.Fatalf("fbx gen exited %d", genCode)
	}

	// Step 2: verify — expect exit 0.
	stderr, code := execFBX(t, bin, "verify", outDir)
	if code != 0 {
		t.Errorf("fbx verify exit code: got %d, want 0\nstderr: %s", code, stderr)
	}
}

// TestVerifyCorruptedGL verifies that when gl.csv has a corrupted debit value
// (with 0.000000001 added to the first non-zero debit), fbx verify exits 2 and
// stderr contains "invariant".
func TestVerifyCorruptedGL(t *testing.T) {
	bin := buildFBX(t)
	outDir := t.TempDir()

	// Step 1: generate.
	_, genCode := execFBX(t, bin,
		"gen", "monthly-close",
		"--seed", "1",
		"--period", "2026-01",
		"--volume", "small",
		"--out", outDir,
	)
	if genCode != 0 {
		t.Fatalf("fbx gen exited %d", genCode)
	}

	// Step 2: corrupt gl.csv.
	glPath := filepath.Join(outDir, "gl.csv")
	if err := corruptGLFirstDebit(glPath); err != nil {
		t.Fatalf("corruptGLFirstDebit: %v", err)
	}

	// Step 3: verify — expect exit 2 with "invariant" in stderr.
	stderr, code := execFBX(t, bin, "verify", outDir)
	if code != 2 {
		t.Errorf("fbx verify exit code: got %d, want 2\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "invariant") {
		t.Errorf("fbx verify stderr missing 'invariant': %s", stderr)
	}
}

// TestVerifyNonexistentDir verifies that fbx verify /nonexistent exits 1.
func TestVerifyNonexistentDir(t *testing.T) {
	bin := buildFBX(t)
	_, code := execFBX(t, bin, "verify", "/nonexistent/path/that/does/not/exist")
	if code != 1 {
		t.Errorf("fbx verify exit code: got %d, want 1", code)
	}
}

// corruptGLFirstDebit reads gl.csv, finds the first data row with a non-zero
// debit column, and modifies its last decimal digit to corrupt the value by
// exactly 0.000000001, then writes the file back.
func corruptGLFirstDebit(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	f.Close()
	if err := sc.Err(); err != nil {
		return err
	}

	// lines[0] is the header; look for first data row with non-zero debit.
	for i := 1; i < len(lines); i++ {
		parts := strings.Split(lines[i], ",")
		if len(parts) < 7 {
			continue
		}
		debit := parts[5]
		if debit == "0.000000000" {
			continue
		}
		// Corrupt: flip the last digit (9th decimal place) by incrementing it.
		// Decimal values have format like "1234.123456789".
		// We bump the last character: if it's '9', wrap to '0', else +1.
		b := []byte(debit)
		last := b[len(b)-1]
		if last == '9' {
			b[len(b)-1] = '0'
		} else {
			b[len(b)-1] = last + 1
		}
		parts[5] = string(b)
		lines[i] = strings.Join(parts, ",")
		break
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}
