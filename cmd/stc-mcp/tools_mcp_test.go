package main

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"

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

// --- Live simulation over the MCP protocol (Phase 29 DEVX-02) ---

// callJSON calls a tool and decodes its text payload into out; it returns
// the result's IsError flag.
func callJSON(t *testing.T, s *mcp.ClientSession, name string, args map[string]any, out any) bool {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	if res.IsError {
		if p, ok := out.(*string); ok {
			*p = tc.Text
		}
		return true
	}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), out), tc.Text)
	return false
}

func TestMCP_SimAgentSession(t *testing.T) {
	withSim(t, liveConfig())
	session := setupMCPClient(t)
	ctx := context.Background()

	lt, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tool := range lt.Tools {
		got[tool.Name] = true
	}
	for _, n := range []string{"stc_sim_step", "stc_sim_read", "stc_sim_write", "stc_opcua_browse"} {
		assert.True(t, got[n], n)
	}

	var w writeResult
	require.False(t, callJSON(t, session, "stc_sim_write", map[string]any{"path": pStart, "value": true}, &w))
	assert.Equal(t, "runtime_set", w.Route)
	var st stepResult
	require.False(t, callJSON(t, session, "stc_sim_step", map[string]any{"cycles": 2}, &st))
	assert.Equal(t, 2, st.Cycles)
	var rr readResult
	require.False(t, callJSON(t, session, "stc_sim_read", map[string]any{"paths": []string{pStart, pRunning}}, &rr))
	require.Len(t, rr.Values, 2)
	assert.Equal(t, false, rr.Values[0].Value)
	assert.Equal(t, true, rr.Values[1].Value)

	var root browseNode
	require.False(t, callJSON(t, session, "stc_opcua_browse", map[string]any{}, &root))
	assert.Equal(t, "ns=4;s=PLC1", root.NodeID)
	assert.Equal(t, []string{"GVL_Live"}, names(&root))

	var msg string
	require.True(t, callJSON(t, session, "stc_sim_write", map[string]any{"path": pStart, "value": "maybe"}, &msg))
	assert.Contains(t, msg, pStart)
	require.True(t, callJSON(t, session, "stc_sim_step", map[string]any{"cycles": 0}, &msg))
	assert.Contains(t, msg, "cycles must be between")
}

func TestMCP_SimWithoutProject(t *testing.T) {
	withSim(t, simConfig{})
	session := setupMCPClient(t)
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"stc_sim_step", map[string]any{"cycles": 1}},
		{"stc_sim_read", map[string]any{"paths": []string{"x"}}},
		{"stc_sim_write", map[string]any{"path": "x", "value": 1}},
		{"stc_opcua_browse", map[string]any{}},
	} {
		var msg string
		require.True(t, callJSON(t, session, call.name, call.args, &msg), call.name)
		assert.Contains(t, msg, "start stc-mcp with --project <path> [--io ...]")
	}
	// The other tools still answer.
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "stc_parse", Arguments: map[string]any{"code": validST}})
	require.NoError(t, err)
	assert.False(t, res.IsError)
}

func TestMCP_SimParallelStepsSerialize(t *testing.T) {
	withSim(t, liveConfig())
	session := setupMCPClient(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "stc_sim_step", Arguments: map[string]any{"cycles": n}})
			assert.NoError(t, err)
			if err == nil {
				assert.False(t, res.IsError)
			}
		}(i + 1)
	}
	wg.Wait()
	var st stepResult
	require.False(t, callJSON(t, session, "stc_sim_step", map[string]any{"cycles": 1}, &st))
	assert.Equal(t, 1+2+3+4+5+6+1, st.Cycles)
}

func TestMCP_SimOPCUA(t *testing.T) {
	cfg := liveConfig()
	cfg.OPCUA = freeAddr(t)
	cfg.PKIDir = t.TempDir()
	withSim(t, cfg)
	session := setupMCPClient(t)
	endpoint := "opc.tcp://" + cfg.OPCUA

	// The session (and its server) starts with the first sim tool call.
	var w writeResult
	require.False(t, callJSON(t, session, "stc_sim_write", map[string]any{"path": pStart, "value": true}, &w))
	var st stepResult
	require.False(t, callJSON(t, session, "stc_sim_step", map[string]any{"cycles": 2}, &st))

	var remote browseNode
	require.False(t, callJSON(t, session, "stc_opcua_browse", map[string]any{"endpoint": endpoint, "depth": 1}, &remote))
	assert.Equal(t, "ns=4;s=PLC1", remote.NodeID)
	assert.Contains(t, names(&remote), "GVL_Live")
	gvl := find(&remote, "GVL_Live")
	require.NotNil(t, gvl)
	assert.Empty(t, gvl.Children, "depth 1 stops at the PLC1 children")

	var deep browseNode
	require.False(t, callJSON(t, session, "stc_opcua_browse",
		map[string]any{"endpoint": endpoint, "node": "GVL_Live.conveyor.HMI", "depth": 1}, &deep))
	assert.Equal(t, "ns=4;s=GVL_Live.conveyor.HMI", deep.NodeID)
	running := find(&deep, "p_stat_Running")
	require.NotNil(t, running)
	assert.Equal(t, "Variable", running.NodeClass)

	// The value written and stepped through MCP is readable over OPC UA.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, endpoint, client.WithInsecureSkipVerify(),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURINone, ua.MessageSecurityModeNone))
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()
	resp, err := c.Read(ctx, &ua.ReadRequest{NodesToRead: []ua.ReadValueID{
		{NodeID: ua.ParseNodeID("ns=4;s=" + pRunning), AttributeID: ua.AttributeIDValue},
		{NodeID: ua.ParseNodeID("ns=4;s=" + pStart), AttributeID: ua.AttributeIDValue},
	}})
	require.NoError(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, true, resp.Results[0].Value)
	assert.Equal(t, false, resp.Results[1].Value)

	var msg string
	require.True(t, callJSON(t, session, "stc_opcua_browse", map[string]any{"endpoint": endpoint, "node": "ns=4;s=GVL_Live.none"}, &msg))
	assert.Contains(t, msg, "browsing")
}

func TestMCP_NewServerListsAllTools(t *testing.T) {
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	_, err := newServer().Connect(ctx, st, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer cs.Close()
	lt, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, lt.Tools, len(allToolDefinitions()))
}
