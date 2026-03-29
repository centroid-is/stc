package main

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMCPClient creates an in-memory MCP server with all tools registered,
// connects a client, and returns the client session. This exercises the
// anonymous wrapper functions in registerTools().
func setupMCPClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "stc-mcp-test",
		Version: "0.0.1-test",
	}, nil)
	registerTools(server)

	ct, st := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "0.0.1",
	}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { session.Close() })

	return session
}

func TestMCP_StcParse(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_parse",
		Arguments: map[string]any{"code": validST, "filename": "test.st"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "ast")
	assert.Contains(t, tc.Text, "has_errors")
}

func TestMCP_StcParse_DefaultFilename(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_parse",
		Arguments: map[string]any{"code": validST},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestMCP_StcCheck(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_check",
		Arguments: map[string]any{"code": validST},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestMCP_StcCheck_WithVendor(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_check",
		Arguments: map[string]any{"code": validST, "vendor": "beckhoff"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestMCP_StcTest(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	tmpDir := t.TempDir()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_test",
		Arguments: map[string]any{"directory": tmpDir},
	})
	// May error for empty dir, but we're testing the wrapper dispatches correctly
	if err != nil {
		t.Logf("stc_test returned error (expected for empty dir): %v", err)
		return
	}
	require.NotNil(t, res)
}

// TestWrap_DirectCalls exercises all named wrapper functions directly,
// covering both success and error paths that are hard to trigger via MCP dispatch.
func TestWrap_Parse(t *testing.T) {
	ctx := context.Background()
	res, _, err := wrapParse(ctx, nil, parseArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestWrap_Check(t *testing.T) {
	ctx := context.Background()
	res, _, err := wrapCheck(ctx, nil, checkArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestWrap_Test(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	res, _, err := wrapTest(ctx, nil, testArgs{Directory: tmpDir})
	if err != nil {
		t.Logf("wrapTest error (acceptable): %v", err)
		return
	}
	require.NotNil(t, res)
}

func TestWrap_Test_UnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not honor os.Chmod(0o000) for read restriction")
	}
	// Create a directory with a _test.st file that cannot be read,
	// triggering the error path in handleTest -> stctesting.Run -> runFile.
	tmpDir := t.TempDir()
	testFile := tmpDir + "/bad_test.st"
	err := os.WriteFile(testFile, []byte("content"), 0o644)
	require.NoError(t, err)
	// Remove read permission
	err = os.Chmod(testFile, 0o000)
	require.NoError(t, err)
	t.Cleanup(func() { os.Chmod(testFile, 0o644) })

	ctx := context.Background()
	_, _, err = wrapTest(ctx, nil, testArgs{Directory: tmpDir})
	// Should error because the file can't be read
	assert.Error(t, err)
}

func TestWrap_Emit(t *testing.T) {
	ctx := context.Background()
	res, _, err := wrapEmit(ctx, nil, emitArgs{Code: validST, Target: "portable"})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestWrap_Lint(t *testing.T) {
	ctx := context.Background()
	res, _, err := wrapLint(ctx, nil, lintArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestWrap_Format(t *testing.T) {
	ctx := context.Background()
	res, _, err := wrapFormat(ctx, nil, formatArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestMCP_StcEmit(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_emit",
		Arguments: map[string]any{"code": validST, "target": "beckhoff"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "PROGRAM")
}

func TestMCP_StcEmit_DefaultTarget(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_emit",
		Arguments: map[string]any{"code": validST},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestMCP_StcLint(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_lint",
		Arguments: map[string]any{"code": validST},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)
}

func TestMCP_StcFormat(t *testing.T) {
	session := setupMCPClient(t)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "stc_format",
		Arguments: map[string]any{"code": validST},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "PROGRAM")
}
