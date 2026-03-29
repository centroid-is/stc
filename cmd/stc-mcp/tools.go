package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/emit"
	"github.com/centroid-is/stc/pkg/format"
	"github.com/centroid-is/stc/pkg/lint"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	stctesting "github.com/centroid-is/stc/pkg/testing"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Arg types for MCP tool handlers ---

type parseArgs struct {
	Code     string `json:"code" jsonschema:"IEC 61131-3 ST source code to parse"`
	Filename string `json:"filename,omitempty" jsonschema:"source filename (default: input.st)"`
}

type checkArgs struct {
	Code   string `json:"code" jsonschema:"IEC 61131-3 ST source code to type-check"`
	Vendor string `json:"vendor,omitempty" jsonschema:"vendor target: beckhoff schneider or portable"`
}

type testArgs struct {
	Directory string `json:"directory" jsonschema:"directory containing *_test.st files"`
}

type emitArgs struct {
	Code   string `json:"code" jsonschema:"IEC 61131-3 ST source code to emit"`
	Target string `json:"target,omitempty" jsonschema:"emit target: beckhoff schneider or portable (default: portable)"`
}

type lintArgs struct {
	Code string `json:"code" jsonschema:"IEC 61131-3 ST source code to lint"`
}

type formatArgs struct {
	Code string `json:"code" jsonschema:"IEC 61131-3 ST source code to format"`
}

// --- Tool descriptions (must be under 100 tokens each per MCP-07) ---

const (
	descParse  = "Parse IEC 61131-3 Structured Text source code and return the AST with diagnostics."
	descCheck  = "Type-check IEC 61131-3 Structured Text source code and return semantic diagnostics."
	descTest   = "Run IEC 61131-3 Structured Text unit tests in a directory and return results as JSON."
	descEmit   = "Emit vendor-specific Structured Text from source code for Beckhoff, Schneider, or portable targets."
	descLint   = "Lint IEC 61131-3 Structured Text source code for coding standard violations."
	descFormat = "Format IEC 61131-3 Structured Text source code with consistent style."
)

// toolDef is used for testing tool metadata.
type toolDef struct {
	name        string
	description string
}

// allToolDefinitions returns metadata for all registered tools (for testing).
func allToolDefinitions() []toolDef {
	return []toolDef{
		{name: "stc_parse", description: descParse},
		{name: "stc_check", description: descCheck},
		{name: "stc_test", description: descTest},
		{name: "stc_emit", description: descEmit},
		{name: "stc_lint", description: descLint},
		{name: "stc_format", description: descFormat},
	}
}

// --- Internal result types for testability ---

// textContent wraps a text string for tool results.
type textContent struct {
	Text string
}

// callToolResult wraps the content returned by a tool handler.
type callToolResult struct {
	Content []interface{}
}

// mustMarshalJSON marshals v to JSON, panicking on failure.
// json.Marshal only fails on unmarshalable types (channels, funcs, etc.),
// which our well-typed structs never contain.
func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("json.Marshal: %v", err))
	}
	return b
}

// --- Handler functions (testable without MCP transport) ---

func handleParse(_ context.Context, args parseArgs) (*callToolResult, error) {
	filename := args.Filename
	if filename == "" {
		filename = "input.st"
	}

	result := pipeline.Parse(filename, args.Code, nil)

	astJSON, err := ast.MarshalNode(result.File)
	if err != nil {
		return nil, fmt.Errorf("marshaling AST: %w", err)
	}

	diagJSON := mustMarshalJSON(result.Diags)

	hasErrors := false
	for _, d := range result.Diags {
		if d.Severity == diag.Error {
			hasErrors = true
			break
		}
	}

	output := fmt.Sprintf(`{"ast":%s,"diagnostics":%s,"has_errors":%t}`, astJSON, diagJSON, hasErrors)
	return &callToolResult{Content: []interface{}{&textContent{Text: output}}}, nil
}

func handleCheck(_ context.Context, args checkArgs) (*callToolResult, error) {
	result := pipeline.Parse("input.st", args.Code, nil)

	var cfg *project.Config
	if args.Vendor != "" {
		cfg = &project.Config{}
		cfg.Build.VendorTarget = args.Vendor
	}

	analysis := analyzer.Analyze([]*ast.SourceFile{result.File}, cfg)

	// Combine parse + analysis diagnostics
	allDiags := make([]diag.Diagnostic, 0, len(result.Diags)+len(analysis.Diags))
	allDiags = append(allDiags, result.Diags...)
	allDiags = append(allDiags, analysis.Diags...)

	diagJSON := mustMarshalJSON(allDiags)
	return &callToolResult{Content: []interface{}{&textContent{Text: string(diagJSON)}}}, nil
}

func handleTest(_ context.Context, args testArgs) (*callToolResult, error) {
	runResult, err := stctesting.Run(args.Directory)
	if err != nil {
		return nil, fmt.Errorf("running tests: %w", err)
	}

	jsonBytes, err := stctesting.FormatJSON(runResult)
	if err != nil {
		return nil, fmt.Errorf("formatting test results: %w", err)
	}

	return &callToolResult{Content: []interface{}{&textContent{Text: string(jsonBytes)}}}, nil
}

func handleEmit(_ context.Context, args emitArgs) (*callToolResult, error) {
	target := args.Target
	if target == "" {
		target = "portable"
	}

	result := pipeline.Parse("input.st", args.Code, nil)
	output := emit.Emit(result.File, emit.Options{
		Target:            emit.LookupTarget(target),
		Indent:            "    ",
		UppercaseKeywords: true,
	})

	return &callToolResult{Content: []interface{}{&textContent{Text: output}}}, nil
}

func handleLint(_ context.Context, args lintArgs) (*callToolResult, error) {
	result := pipeline.Parse("input.st", args.Code, nil)
	lintResult := lint.LintFile(result.File, lint.DefaultLintOptions())

	diagJSON := mustMarshalJSON(lintResult.Diags)
	return &callToolResult{Content: []interface{}{&textContent{Text: string(diagJSON)}}}, nil
}

func handleFormat(_ context.Context, args formatArgs) (*callToolResult, error) {
	result := pipeline.Parse("input.st", args.Code, nil)
	formatted := format.Format(result.File, format.FormatOptions{
		Indent:            "    ",
		UppercaseKeywords: true,
	})

	return &callToolResult{Content: []interface{}{&textContent{Text: formatted}}}, nil
}

// --- MCP wrapper functions (named for testability) ---

func wrapParse(ctx context.Context, _ *mcp.CallToolRequest, args parseArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleParse(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

func wrapCheck(ctx context.Context, _ *mcp.CallToolRequest, args checkArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleCheck(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

func wrapTest(ctx context.Context, _ *mcp.CallToolRequest, args testArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleTest(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

func wrapEmit(ctx context.Context, _ *mcp.CallToolRequest, args emitArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleEmit(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

func wrapLint(ctx context.Context, _ *mcp.CallToolRequest, args lintArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleLint(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

func wrapFormat(ctx context.Context, _ *mcp.CallToolRequest, args formatArgs) (*mcp.CallToolResult, any, error) {
	r, err := handleFormat(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	return toMCPResult(r), nil, nil
}

// --- MCP registration ---

func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_parse",
		Description: descParse,
	}, wrapParse)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_check",
		Description: descCheck,
	}, wrapCheck)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_test",
		Description: descTest,
	}, wrapTest)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_emit",
		Description: descEmit,
	}, wrapEmit)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_lint",
		Description: descLint,
	}, wrapLint)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_format",
		Description: descFormat,
	}, wrapFormat)
}

// toMCPResult converts our internal result to MCP SDK result type.
func toMCPResult(r *callToolResult) *mcp.CallToolResult {
	var content []mcp.Content
	for _, c := range r.Content {
		if tc, ok := c.(*textContent); ok {
			content = append(content, &mcp.TextContent{Text: tc.Text})
		}
	}
	return &mcp.CallToolResult{Content: content}
}
