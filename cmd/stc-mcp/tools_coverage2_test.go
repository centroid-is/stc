package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- handleParse: with custom filename ---

func TestStcParse_CustomFilename(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{
		Code:     "PROGRAM P\nEND_PROGRAM",
		Filename: "custom.st",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Content, 1)
}

// --- handleParse: code with errors (has_errors=true) ---

func TestStcParse_WithErrors(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{
		Code: "PROGRAM P\nVAR x : INT END_VAR\nEND_PROGRAM",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "has_errors")
}

// --- handleCheck: various vendor targets ---

func TestStcCheck_BeckhoffVendor(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{
		Code:   "PROGRAM P\nVAR x : INT; END_VAR\nEND_PROGRAM",
		Vendor: "beckhoff",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcCheck_SchneiderVendor(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{
		Code:   "PROGRAM P\nVAR x : INT; END_VAR\nEND_PROGRAM",
		Vendor: "schneider",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcCheck_PortableVendor(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{
		Code:   "PROGRAM P\nVAR x : INT; END_VAR\nEND_PROGRAM",
		Vendor: "portable",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// --- handleTest: invalid directory ---

func TestStcTest_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := handleTest(context.Background(), testArgs{
		Directory: tmpDir,
	})
	// May succeed with empty results or error
	if err != nil {
		t.Logf("handleTest error (expected for empty dir): %v", err)
	} else {
		require.NotNil(t, result)
	}
}

// --- handleEmit: various targets ---

func TestStcEmit_PortableTarget(t *testing.T) {
	result, err := handleEmit(context.Background(), emitArgs{
		Code:   "PROGRAM P\nVAR x : INT; END_VAR\nEND_PROGRAM",
		Target: "portable",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcEmit_SchneiderTarget(t *testing.T) {
	result, err := handleEmit(context.Background(), emitArgs{
		Code:   "PROGRAM P\nVAR x : INT; END_VAR\nEND_PROGRAM",
		Target: "schneider",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// --- handleLint: code with lint issues ---

func TestStcLint_CodeWithIssues(t *testing.T) {
	result, err := handleLint(context.Background(), lintArgs{
		Code: "PROGRAM p\nVAR X : INT; END_VAR\nEND_PROGRAM",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// --- handleFormat: various codes ---

func TestStcFormat_WithSpacing(t *testing.T) {
	result, err := handleFormat(context.Background(), formatArgs{
		Code: "PROGRAM P\nVAR x : INT; END_VAR\n x := 1 ;\nEND_PROGRAM",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	assert.NotEmpty(t, text)
}

// --- registerTools: verify it doesn't panic ---

func TestRegisterTools_NoPanic(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "0.0.1",
	}, nil)
	// Should not panic
	registerTools(server)
}

// --- toMCPResult: empty content ---

func TestToMCPResult_EmptyContent(t *testing.T) {
	r := &callToolResult{Content: nil}
	result := toMCPResult(r)
	assert.NotNil(t, result)
	assert.Empty(t, result.Content)
}

func TestToMCPResult_MultipleContent(t *testing.T) {
	r := &callToolResult{Content: []interface{}{
		&textContent{Text: "hello"},
		&textContent{Text: "world"},
	}}
	result := toMCPResult(r)
	assert.Len(t, result.Content, 2)
}
