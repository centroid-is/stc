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
	"github.com/centroid-is/stc/pkg/parser"
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

// --- Handler functions (testable without MCP transport) ---

func handleParse(_ context.Context, args parseArgs) (*callToolResult, error) {
	filename := args.Filename
	if filename == "" {
		filename = "input.st"
	}

	result := parser.Parse(filename, args.Code)

	astJSON, err := ast.MarshalNode(result.File)
	if err != nil {
		return nil, fmt.Errorf("marshaling AST: %w", err)
	}

	diagJSON, err := json.Marshal(result.Diags)
	if err != nil {
		return nil, fmt.Errorf("marshaling diagnostics: %w", err)
	}

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
	result := parser.Parse("input.st", args.Code)

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

	diagJSON, err := json.Marshal(allDiags)
	if err != nil {
		return nil, fmt.Errorf("marshaling diagnostics: %w", err)
	}

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

	result := parser.Parse("input.st", args.Code)
	output := emit.Emit(result.File, emit.Options{
		Target:            emit.LookupTarget(target),
		Indent:            "    ",
		UppercaseKeywords: true,
	})

	return &callToolResult{Content: []interface{}{&textContent{Text: output}}}, nil
}

func handleLint(_ context.Context, args lintArgs) (*callToolResult, error) {
	result := parser.Parse("input.st", args.Code)
	lintResult := lint.LintFile(result.File, lint.DefaultLintOptions())

	diagJSON, err := json.Marshal(lintResult.Diags)
	if err != nil {
		return nil, fmt.Errorf("marshaling lint diagnostics: %w", err)
	}

	return &callToolResult{Content: []interface{}{&textContent{Text: string(diagJSON)}}}, nil
}

func handleFormat(_ context.Context, args formatArgs) (*callToolResult, error) {
	result := parser.Parse("input.st", args.Code)
	formatted := format.Format(result.File, format.FormatOptions{
		Indent:            "    ",
		UppercaseKeywords: true,
	})

	return &callToolResult{Content: []interface{}{&textContent{Text: formatted}}}, nil
}

// --- MCP registration ---

func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_parse",
		Description: descParse,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args parseArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleParse(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_check",
		Description: descCheck,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args checkArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleCheck(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_test",
		Description: descTest,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args testArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleTest(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_emit",
		Description: descEmit,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emitArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleEmit(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_lint",
		Description: descLint,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args lintArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleLint(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stc_format",
		Description: descFormat,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args formatArgs) (*mcp.CallToolResult, any, error) {
		r, err := handleFormat(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return toMCPResult(r), nil, nil
	})
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
