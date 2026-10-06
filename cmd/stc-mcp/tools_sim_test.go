package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSim installs a session host for cfg and restores the previous one.
func withSim(t *testing.T, cfg simConfig) {
	t.Helper()
	prev := sim
	sim = &simHost{cfg: cfg}
	t.Cleanup(func() {
		_ = sim.close()
		sim = prev
	})
}

// resultText returns the single text of r and whether it is an error.
func resultText(t *testing.T, r *callToolResult) (string, bool) {
	t.Helper()
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*textContent)
	require.True(t, ok)
	return tc.Text, r.IsError
}

func decodeOK[T any](t *testing.T, r *callToolResult, err error) T {
	t.Helper()
	require.NoError(t, err)
	text, isErr := resultText(t, r)
	require.False(t, isErr, text)
	var v T
	require.NoError(t, json.Unmarshal([]byte(text), &v), text)
	return v
}

func errText(t *testing.T, r *callToolResult, err error) string {
	t.Helper()
	require.NoError(t, err)
	text, isErr := resultText(t, r)
	require.True(t, isErr, text)
	return text
}

type readResult struct {
	Values []struct {
		Path  string `json:"path"`
		Value any    `json:"value"`
		Error string `json:"error"`
	} `json:"values"`
}

func TestSimToolsWriteStepRead(t *testing.T) {
	withSim(t, liveConfig())
	ctx := context.Background()

	r, err := handleSimWrite(ctx, simWriteArgs{Path: pStart, Value: true})
	w := decodeOK[writeResult](t, r, err)
	assert.Equal(t, "runtime_set", w.Route)

	r, err = handleSimStep(ctx, simStepArgs{Cycles: 2})
	st := decodeOK[stepResult](t, r, err)
	assert.Equal(t, 2, st.Cycles)
	assert.Equal(t, int64(20), st.SimTimeMS)
	assert.Equal(t, []string{}, st.ScenarioFailures)

	r, err = handleSimRead(ctx, simReadArgs{Paths: []string{pStart, pRunning, "GVL_Live.nope"}})
	rr := decodeOK[readResult](t, r, err)
	require.Len(t, rr.Values, 3)
	assert.Equal(t, false, rr.Values[0].Value)
	assert.Equal(t, true, rr.Values[1].Value)
	assert.NotEmpty(t, rr.Values[2].Error)

	// A forced linked input reaches the FB after a step.
	r, err = handleSimWrite(ctx, simWriteArgs{Path: pSensor, Value: "TRUE"})
	w = decodeOK[writeResult](t, r, err)
	assert.Equal(t, "input_force", w.Route)
	_, err = handleSimStep(ctx, simStepArgs{Cycles: 1})
	require.NoError(t, err)
	r, err = handleSimRead(ctx, simReadArgs{Paths: []string{pSensorH}})
	rr = decodeOK[readResult](t, r, err)
	assert.Equal(t, true, rr.Values[0].Value)
}

func TestSimToolsErrors(t *testing.T) {
	withSim(t, liveConfig())
	ctx := context.Background()

	r, err := handleSimWrite(ctx, simWriteArgs{Path: pStart, Value: float64(2.5)})
	assert.Contains(t, errText(t, r, err), pStart)
	r, err = handleSimWrite(ctx, simWriteArgs{Path: "GVL_Live.cMax", Value: float64(1)})
	assert.Contains(t, errText(t, r, err), "GVL_Live.cMax")
	r, err = handleSimWrite(ctx, simWriteArgs{Value: true})
	assert.Contains(t, errText(t, r, err), "path is required")
	r, err = handleSimRead(ctx, simReadArgs{})
	assert.Contains(t, errText(t, r, err), "at least one")
	for _, n := range []int{0, -3, 1_000_001} {
		r, err = handleSimStep(ctx, simStepArgs{Cycles: n})
		assert.Contains(t, errText(t, r, err), "cycles must be between")
	}
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Depth: -1})
	assert.Contains(t, errText(t, r, err), "depth")
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Node: "ns=4;s=GVL_Live.missing"})
	assert.Contains(t, errText(t, r, err), "not in the address space")
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Endpoint: "opc.tcp://127.0.0.1:1"})
	assert.Contains(t, errText(t, r, err), "connecting to")
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Endpoint: "opc.tcp://127.0.0.1:1", Depth: -2})
	assert.Contains(t, errText(t, r, err), "depth")
}

func TestSimToolsWithoutSession(t *testing.T) {
	withSim(t, simConfig{})
	ctx := context.Background()
	const want = "start stc-mcp with --project <path> [--io ...]"
	r, err := handleSimStep(ctx, simStepArgs{Cycles: 1})
	assert.Contains(t, errText(t, r, err), want)
	r, err = handleSimRead(ctx, simReadArgs{Paths: []string{"x"}})
	assert.Contains(t, errText(t, r, err), want)
	r, err = handleSimWrite(ctx, simWriteArgs{Path: "x", Value: true})
	assert.Contains(t, errText(t, r, err), want)
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{})
	assert.Contains(t, errText(t, r, err), want)

	// The MCP adapter keeps IsError.
	mr, _, err := wrapSimStep(ctx, nil, simStepArgs{Cycles: 1})
	require.NoError(t, err)
	assert.True(t, mr.IsError)
	mr, _, err = wrapSimRead(ctx, nil, simReadArgs{Paths: []string{"x"}})
	require.NoError(t, err)
	assert.True(t, mr.IsError)
	mr, _, err = wrapSimWrite(ctx, nil, simWriteArgs{Path: "x"})
	require.NoError(t, err)
	assert.True(t, mr.IsError)
	mr, _, err = wrapOpcuaBrowse(ctx, nil, opcuaBrowseArgs{})
	require.NoError(t, err)
	assert.True(t, mr.IsError)
}

// names lists the browse names of n's children.
func names(n *browseNode) []string {
	var out []string
	for _, c := range n.Children {
		out = append(out, c.BrowseName)
	}
	return out
}

func find(n *browseNode, name string) *browseNode {
	for _, c := range n.Children {
		if c.BrowseName == name {
			return c
		}
	}
	return nil
}

func TestOpcuaBrowseSession(t *testing.T) {
	withSim(t, liveConfig())
	ctx := context.Background()

	r, err := handleOpcuaBrowse(ctx, opcuaBrowseArgs{})
	root := decodeOK[browseNode](t, r, err)
	assert.Equal(t, "ns=4;s=PLC1", root.NodeID)
	assert.Equal(t, "PLC1", root.BrowseName)
	assert.Equal(t, []string{"GVL_Live"}, names(&root))
	gvl := root.Children[0]
	assert.Equal(t, "ns=4;s=GVL_Live", gvl.NodeID)
	assert.Equal(t, []string{"conveyor", "xSensor", "nRunCycles"}, names(gvl))
	conv := find(gvl, "conveyor")
	require.NotNil(t, conv)
	assert.Empty(t, conv.Children, "depth 2 stops below the GVL members")
	x := find(gvl, "xSensor")
	assert.Equal(t, "Variable", x.NodeClass)
	assert.Equal(t, "BOOL", x.DataType)
	assert.Equal(t, "read_write", x.Access)

	// Narrowed to a node; the result is deterministic.
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Node: "ns=4;s=GVL_Live.conveyor", Depth: 1})
	conv2 := decodeOK[browseNode](t, r, err)
	assert.Equal(t, "ns=4;s=GVL_Live.conveyor", conv2.NodeID)
	assert.Equal(t, []string{"xSensor", "xRunning", "HMI"}, names(&conv2))
	r2, err := handleOpcuaBrowse(ctx, opcuaBrowseArgs{Node: "GVL_Live.conveyor", Depth: 1})
	require.NoError(t, err)
	t1, _ := resultText(t, r)
	t2, _ := resultText(t, r2)
	assert.Equal(t, t1, t2)

	// Depth is capped at 10 and "PLC1" is the root.
	r, err = handleOpcuaBrowse(ctx, opcuaBrowseArgs{Node: "PLC1", Depth: 99})
	deep := decodeOK[browseNode](t, r, err)
	hmi := find(find(find(&deep, "GVL_Live"), "conveyor"), "HMI")
	require.NotNil(t, hmi)
}

func TestNodePathAndAccess(t *testing.T) {
	assert.Equal(t, "", nodePath(""))
	assert.Equal(t, "", nodePath("ns=4;s=PLC1"))
	assert.Equal(t, "GVL.x", nodePath("ns=4;s=GVL.x"))
	assert.Equal(t, "GVL.x", nodePath(" GVL.x "))
	assert.Equal(t, "read", accessText(1))
	assert.Equal(t, "write", accessText(2))
	assert.Equal(t, "read_write", accessText(3))
	assert.Equal(t, "read_write", accessText(0))
	assert.Equal(t, "Name", stripNS("4:Name"))
	assert.Equal(t, "Name", stripNS("Name"))
	d, err := clampDepth(5)
	require.NoError(t, err)
	assert.Equal(t, 5, d)
}

func TestSimToolDescriptions(t *testing.T) {
	for _, d := range allToolDefinitions() {
		// ~4 characters per token: 100 tokens is well above 400 characters.
		assert.Less(t, len(d.description), 400, d.name)
	}
}
