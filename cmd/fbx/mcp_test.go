package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// mcpSession wraps an fbx-mcp subprocess with helper methods for sending
// JSON-RPC requests and reading responses.
type mcpSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

// newMCPSession starts "fbx mcp" as a subprocess and returns a session.
// The caller must call session.close() when done.
func newMCPSession(t *testing.T, bin string) *mcpSession {
	t.Helper()
	cmd := exec.Command(bin, "mcp")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	// Drain stderr to prevent subprocess from blocking on a full pipe.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	sc := bufio.NewScanner(stdoutPipe)
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	return &mcpSession{t: t, cmd: cmd, stdin: stdin, stdout: sc}
}

// send writes a JSON-RPC request line to the subprocess stdin.
func (s *mcpSession) send(req interface{}) {
	s.t.Helper()
	b, err := json.Marshal(req)
	if err != nil {
		s.t.Fatalf("json.Marshal request: %v", err)
	}
	if _, err := fmt.Fprintf(s.stdin, "%s\n", b); err != nil {
		s.t.Fatalf("write to stdin: %v", err)
	}
}

// readLine reads one response line from the subprocess stdout.
// It uses a 2-second deadline to catch missing-flush bugs.
func (s *mcpSession) readLine() string {
	s.t.Helper()
	done := make(chan string, 1)
	go func() {
		if s.stdout.Scan() {
			done <- s.stdout.Text()
		} else {
			done <- ""
		}
	}()
	select {
	case line := <-done:
		return line
	case <-time.After(2 * time.Second):
		s.t.Fatal("timeout waiting for MCP response (missing Flush?)")
		return ""
	}
}

// close closes stdin (causes the subprocess to exit on EOF) and waits.
func (s *mcpSession) close() {
	s.stdin.Close()
	_ = s.cmd.Wait()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// parseResponse unmarshals a JSON-RPC response line into the given target.
func parseResponse(t *testing.T, line string, out interface{}) {
	t.Helper()
	if err := json.Unmarshal([]byte(line), out); err != nil {
		t.Fatalf("json.Unmarshal response: %v\nline: %s", err, line)
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestMCPInitialize sends an initialize request and asserts protocolVersion and
// serverInfo.name == "fbx".
func TestMCPInitialize(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	line := s.readLine()
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	parseResponse(t, line, &resp)

	if resp.Result.ProtocolVersion == "" {
		t.Errorf("protocolVersion is empty")
	}
	if resp.Result.ServerInfo.Name != "fbx" {
		t.Errorf("serverInfo.name = %q; want %q", resp.Result.ServerInfo.Name, "fbx")
	}
}

// TestMCPToolsList sends tools/list and asserts exactly 4 tools in alphabetical order,
// each with name, description, and inputSchema.
func TestMCPToolsList(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})

	line := s.readLine()
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	parseResponse(t, line, &resp)

	wantNames := []string{"fbx_bundle_list", "fbx_docs_list", "fbx_gen", "fbx_verify"}
	if len(resp.Result.Tools) != len(wantNames) {
		t.Fatalf("tools count = %d; want %d", len(resp.Result.Tools), len(wantNames))
	}
	for i, tool := range resp.Result.Tools {
		if tool.Name != wantNames[i] {
			t.Errorf("tools[%d].name = %q; want %q", i, tool.Name, wantNames[i])
		}
		if tool.Description == "" {
			t.Errorf("tools[%d].description is empty", i)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("tools[%d].inputSchema is empty", i)
		}
	}
}

// TestMCPToolsListDeterministic sends tools/list twice and asserts byte-identical
// responses.
func TestMCPToolsListDeterministic(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	s.send(req)
	line1 := s.readLine()

	s.send(req)
	line2 := s.readLine()

	if line1 != line2 {
		t.Errorf("tools/list responses differ:\nfirst:  %s\nsecond: %s", line1, line2)
	}
}

// TestMCPFbxGen calls fbx_gen with valid params and asserts ok=true and the
// out_dir contains expected output files.
func TestMCPFbxGen(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	outDir := t.TempDir()
	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "fbx_gen",
			"arguments": map[string]interface{}{
				"bundle":  "monthly-close",
				"seed":    1,
				"period":  "2026-01",
				"volume":  "small",
				"out_dir": outDir,
			},
		},
	})

	line := s.readLine()
	var resp struct {
		Result struct {
			OK     bool   `json:"ok"`
			OutDir string `json:"out_dir"`
			Error  string `json:"error"`
		} `json:"result"`
	}
	parseResponse(t, line, &resp)

	if !resp.Result.OK {
		t.Fatalf("fbx_gen returned ok=false: %s", resp.Result.Error)
	}

	// Assert the output directory contains expected files.
	for _, name := range []string{"gl.csv", "manifest.json", "profit-loss.csv"} {
		path := outDir + "/" + name
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected output file %q not found: %v", name, err)
		}
	}
}

// TestMCPFbxGenInvalidBundle calls fbx_gen with a nonexistent bundle and asserts
// ok=false (not a server crash).
func TestMCPFbxGenInvalidBundle(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "fbx_gen",
			"arguments": map[string]interface{}{
				"bundle":  "nonexistent",
				"seed":    1,
				"period":  "2026-01",
				"volume":  "small",
				"out_dir": t.TempDir(),
			},
		},
	})

	line := s.readLine()
	var resp struct {
		Result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"result"`
	}
	parseResponse(t, line, &resp)

	if resp.Result.OK {
		t.Error("fbx_gen with invalid bundle returned ok=true; want ok=false")
	}
	if resp.Result.Error == "" {
		t.Error("fbx_gen with invalid bundle returned empty error message")
	}
}

// TestMCPFbxVerify round-trips: gen then verify the same dir, assert ok=true.
func TestMCPFbxVerify(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	outDir := t.TempDir()

	// Step 1: generate.
	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "fbx_gen",
			"arguments": map[string]interface{}{
				"bundle":  "monthly-close",
				"seed":    1,
				"period":  "2026-01",
				"volume":  "small",
				"out_dir": outDir,
			},
		},
	})
	genLine := s.readLine()
	var genResp struct {
		Result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"result"`
	}
	parseResponse(t, genLine, &genResp)
	if !genResp.Result.OK {
		t.Fatalf("fbx_gen failed: %s", genResp.Result.Error)
	}

	// Step 2: verify.
	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "fbx_verify",
			"arguments": map[string]interface{}{
				"dir": outDir,
			},
		},
	})
	verLine := s.readLine()
	var verResp struct {
		Result struct {
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
			Error  string `json:"error"`
		} `json:"result"`
	}
	parseResponse(t, verLine, &verResp)
	if !verResp.Result.OK {
		t.Errorf("fbx_verify returned ok=false: detail=%s error=%s", verResp.Result.Detail, verResp.Result.Error)
	}
}

// TestMCPFbxVerifyNonexistent calls fbx_verify on /nonexistent and asserts ok=false.
func TestMCPFbxVerifyNonexistent(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "fbx_verify",
			"arguments": map[string]interface{}{
				"dir": "/nonexistent/path/that/does/not/exist",
			},
		},
	})

	line := s.readLine()
	var resp struct {
		Result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"result"`
	}
	parseResponse(t, line, &resp)

	if resp.Result.OK {
		t.Error("fbx_verify on nonexistent dir returned ok=true; want ok=false")
	}
}

// TestMCPUnknownMethod sends an unknown method and asserts JSON-RPC error -32601.
func TestMCPUnknownMethod(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "foo/bar",
	})

	line := s.readLine()
	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	parseResponse(t, line, &resp)

	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d; want -32601", resp.Error.Code)
	}
	if resp.Error.Message == "" {
		t.Error("error message is empty")
	}
}

// TestMCPFlushTimeout sends a request and asserts the response arrives within
// 2 seconds. This catches missing-Flush bugs.
func TestMCPFlushTimeout(t *testing.T) {
	bin := buildFBX(t)
	s := newMCPSession(t, bin)
	defer s.close()

	s.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	// readLine already enforces a 2-second timeout internally.
	line := s.readLine()
	if !strings.Contains(line, "protocolVersion") {
		t.Errorf("unexpected response: %s", line)
	}
}
