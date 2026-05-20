// MCP stdio server for fbx.
//
// Implements a JSON-RPC 2.0 server over stdin/stdout (one JSON object per line).
// Errors are written as JSON-RPC error objects in the response; server-level
// panics go to os.Stderr. os.Stdout is reserved for JSON-RPC output only.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/humzakhan/fauxbooks/internal/gen"
	"github.com/humzakhan/fauxbooks/internal/invariant"
	"github.com/humzakhan/fauxbooks/internal/reader"
	"github.com/humzakhan/fauxbooks/internal/report"
	"github.com/humzakhan/fauxbooks/internal/writer"
)

// JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// ── wire types ────────────────────────────────────────────────────────────────

// mcpRequest is a JSON-RPC 2.0 request envelope.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// mcpResponse is a JSON-RPC 2.0 success response.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result"`
}

// mcpErrorBody holds a JSON-RPC error code and message.
type mcpErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpErrorResponse is a JSON-RPC 2.0 error response.
type mcpErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   mcpErrorBody    `json:"error"`
}

// ── initialize result ─────────────────────────────────────────────────────────

type mcpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    mcpCapabilities `json:"capabilities"`
	ServerInfo      mcpServerInfo   `json:"serverInfo"`
}

type mcpCapabilities struct {
	Tools map[string]interface{} `json:"tools"`
}

type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ── tools/list result ─────────────────────────────────────────────────────────

type mcpToolsListResult struct {
	Tools []mcpToolDef `json:"tools"`
}

type mcpToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema mcpInputSchema `json:"inputSchema"`
}

type mcpInputSchema struct {
	Type       string                   `json:"type"`
	Properties map[string]mcpSchemaProp `json:"properties"`
	Required   []string                 `json:"required,omitempty"`
}

type mcpSchemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ── tool result types ─────────────────────────────────────────────────────────

type mcpBundleListResult struct {
	Bundles []mcpBundleInfo `json:"bundles"`
}

type mcpBundleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type mcpDocsListResult struct {
	Documents []string `json:"documents"`
}

type mcpGenResult struct {
	OK     bool   `json:"ok"`
	OutDir string `json:"out_dir,omitempty"`
	Error  string `json:"error,omitempty"`
}

type mcpVerifyResult struct {
	OK        bool   `json:"ok"`
	Invariant int    `json:"invariant,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ── tool input param types ────────────────────────────────────────────────────

type mcpGenParams struct {
	Bundle string `json:"bundle"`
	Seed   int64  `json:"seed"`
	Period string `json:"period"`
	Volume string `json:"volume"`
	OutDir string `json:"out_dir"`
	Format string `json:"format"`
}

type mcpVerifyParams struct {
	Dir string `json:"dir"`
}

// ── fixed alphabetical tool definitions ──────────────────────────────────────

// mcpTools returns the four tool definitions in fixed alphabetical order.
// A typed slice ensures deterministic output without map iteration.
func mcpTools() []mcpToolDef {
	return []mcpToolDef{
		{
			Name:        "fbx_bundle_list",
			Description: "List the available built-in generation bundles with a one-line description each.",
			InputSchema: mcpInputSchema{
				Type:       "object",
				Properties: map[string]mcpSchemaProp{},
			},
		},
		{
			Name:        "fbx_docs_list",
			Description: "List the document files produced by a monthly-close generation run.",
			InputSchema: mcpInputSchema{
				Type:       "object",
				Properties: map[string]mcpSchemaProp{},
			},
		},
		{
			Name:        "fbx_gen",
			Description: "Generate a synthetic financial dataset for a given bundle, seed, period, and volume.",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpSchemaProp{
					"bundle":  {Type: "string", Description: "Bundle name (e.g. monthly-close, recon-stress-test)."},
					"seed":    {Type: "integer", Description: "RNG seed for deterministic output."},
					"period":  {Type: "string", Description: "Target month in YYYY-MM format."},
					"volume":  {Type: "string", Description: "Size tier: small, medium, or large."},
					"out_dir": {Type: "string", Description: "Output directory path."},
					"format":  {Type: "string", Description: "Output format: csv (default), jsonl, xlsx, or pdf."},
				},
				Required: []string{"bundle", "seed", "period", "volume", "out_dir"},
			},
		},
		{
			Name:        "fbx_verify",
			Description: "Verify that a directory of fbx output files passes all double-entry invariants.",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpSchemaProp{
					"dir": {Type: "string", Description: "Directory containing fbx CSV output files."},
				},
				Required: []string{"dir"},
			},
		},
	}
}

// ── document list (hardcoded from manifest.go file list, CSV format) ──────────

// mcpDocsList returns the canonical list of CSV document names produced by a
// monthly-close run. Uses a typed slice for deterministic ordering.
func mcpDocsList() []string {
	return []string{
		"coa.csv",
		"customers.csv",
		"vendors.csv",
		"journal_entries.csv",
		"gl.csv",
		"bank_statement.csv",
		"invoices.csv",
		"customer_payments.csv",
		"bills.csv",
		"bill_payments.csv",
		"expense_reports.csv",
		"profit-loss.csv",
		"balance-sheet.csv",
		"cash-flow.csv",
		"trial-balance.csv",
		"ar-aging.csv",
		"ap-aging.csv",
		"customer-statements.csv",
		"check-register.csv",
		"manifest.json",
	}
}

// ── dispatch ──────────────────────────────────────────────────────────────────

// runMCP is the entry point for "fbx mcp". It reads JSON-RPC requests from
// stdin (one per line), dispatches to the appropriate handler, and writes
// responses to stdout (one per line with an explicit Flush after each).
// Exits 0 on stdin EOF. No goroutines are used in the request loop.
func runMCP(_ []string) int {
	sc := bufio.NewScanner(os.Stdin)
	// Bump buffer to 1 MiB to handle large fbx_gen requests.
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	bw := bufio.NewWriter(os.Stdout)

	for sc.Scan() {
		line := sc.Bytes()
		writeResponse(bw, dispatchRequest(line))
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "fbx mcp: scanner error: %v\n", err)
		return 1
	}
	return 0
}

// writeResponse marshals resp to bw, appends a newline, and flushes.
// Flush is explicit after every response to prevent client deadlock.
func writeResponse(bw *bufio.Writer, resp interface{}) {
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		// Last resort: write a minimal parse error to stderr; stdout is corrupted.
		fmt.Fprintf(os.Stderr, "fbx mcp: encode response: %v\n", err)
	}
	// json.Encoder.Encode already appends '\n'; flush immediately.
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "fbx mcp: flush: %v\n", err)
	}
}

// dispatchRequest parses one JSON-RPC line and returns the appropriate response.
func dispatchRequest(line []byte) interface{} {
	var req mcpRequest
	if err := json.Unmarshal(line, &req); err != nil {
		// Parse error: id is unknown.
		return mcpErrorResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   mcpErrorBody{Code: codeParseError, Message: "Parse error: " + err.Error()},
		}
	}

	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpInitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities:    mcpCapabilities{Tools: map[string]interface{}{}},
				ServerInfo:      mcpServerInfo{Name: "fbx", Version: "v0.2"},
			},
		}

	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpToolsListResult{Tools: mcpTools()},
		}

	case "tools/call":
		return handleToolsCall(req)

	default:
		return mcpErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   mcpErrorBody{Code: codeMethodNotFound, Message: "Method not found"},
		}
	}
}

// handleToolsCall dispatches a tools/call request to the named tool handler.
func handleToolsCall(req mcpRequest) interface{} {
	// params shape: {"name":"<tool>","arguments":{...}}
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &call); err != nil {
		return mcpErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   mcpErrorBody{Code: codeInvalidParams, Message: "Invalid params: " + err.Error()},
		}
	}

	switch call.Name {
	case "fbx_bundle_list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpBundleListResult{Bundles: []mcpBundleInfo{
				{Name: "monthly-close", Description: "Standard monthly close with AR, AP, payroll, and bank reconciliation events."},
				{Name: "recon-stress-test", Description: "High-volume stress bundle with deliberately injected reconciliation discrepancies."},
			}},
		}

	case "fbx_docs_list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpDocsListResult{Documents: mcpDocsList()},
		}

	case "fbx_gen":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  toolFbxGen(call.Arguments),
		}

	case "fbx_verify":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  toolFbxVerify(call.Arguments),
		}

	default:
		return mcpErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   mcpErrorBody{Code: codeMethodNotFound, Message: fmt.Sprintf("Unknown tool: %s", call.Name)},
		}
	}
}

// ── tool implementations ──────────────────────────────────────────────────────

// toolFbxGen handles the fbx_gen tool call.
func toolFbxGen(rawArgs json.RawMessage) mcpGenResult {
	var p mcpGenParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return mcpGenResult{OK: false, Error: "invalid params: " + err.Error()}
	}

	if p.Format == "" {
		p.Format = "csv"
	}

	cfg, err := gen.ParseConfig(p.Seed, p.Period, p.Volume, p.Bundle, p.OutDir, p.Format)
	if err != nil {
		return mcpGenResult{OK: false, Error: "config error: " + err.Error()}
	}
	cfg.Version = Version

	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return mcpGenResult{OK: false, Error: "cannot create output directory: " + err.Error()}
	}

	ds, err := gen.Run(cfg)
	if err != nil {
		return mcpGenResult{OK: false, Error: "generation failed: " + err.Error()}
	}

	// Check invariants before writing (skip for stress bundles that inject faults).
	if ds.Truth == nil {
		if err := invariant.Check(ds); err != nil {
			var ie gen.InvariantError
			if errors.As(err, &ie) {
				return mcpGenResult{OK: false, Error: fmt.Sprintf("invariant %d violated: %s", ie.Invariant, ie.Detail)}
			}
			return mcpGenResult{OK: false, Error: "invariant check failed: " + err.Error()}
		}
	}

	// Derive reports.
	reports := report.Compute(ds)
	periodEnd := cfg.Period.AddDate(0, 1, 0).Add(-time.Nanosecond)
	reports.Aging = report.ComputeAging(ds, periodEnd)
	reports.CustomerStatements = report.ComputeStatements(ds)
	reports.CheckRegister = report.ComputeCheckRegister(ds)

	// Write output.
	var writeErr error
	switch cfg.Format {
	case "xlsx":
		writeErr = writer.WriteXLSX(ds, reports, cfg, cfg.OutDir)
	case "pdf":
		writeErr = writer.WritePDF(reports, cfg, writer.DefaultCompanyConfig(), cfg.OutDir)
	default:
		writeErr = writer.Write(ds, reports, cfg, cfg.OutDir)
	}
	if writeErr != nil {
		return mcpGenResult{OK: false, Error: "write failed: " + writeErr.Error()}
	}

	return mcpGenResult{OK: true, OutDir: cfg.OutDir}
}

// toolFbxVerify handles the fbx_verify tool call.
func toolFbxVerify(rawArgs json.RawMessage) mcpVerifyResult {
	var p mcpVerifyParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return mcpVerifyResult{OK: false, Error: "invalid params: " + err.Error()}
	}
	if p.Dir == "" {
		return mcpVerifyResult{OK: false, Error: "dir is required"}
	}

	ds, err := reader.Load(p.Dir)
	if err != nil {
		return mcpVerifyResult{OK: false, Error: err.Error()}
	}

	if err := invariant.Check(ds); err != nil {
		var ie gen.InvariantError
		if errors.As(err, &ie) {
			return mcpVerifyResult{OK: false, Invariant: ie.Invariant, Detail: ie.Detail}
		}
		return mcpVerifyResult{OK: false, Error: err.Error()}
	}

	return mcpVerifyResult{OK: true}
}
