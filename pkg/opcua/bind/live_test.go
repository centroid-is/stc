package bind_test

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/projectload"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveTimeout bounds every network operation and every poll of the live
// tests; none of them asserts on a fixed sleep.
const liveTimeout = 5 * time.Second

// liveFixture is the Phase 29 live handshake project.
var liveFixture = filepath.Join("..", "..", "..", "tests", "opcua_live")

// liveServer is the fixture project running free on a 10 ms cycle and
// served over OPC UA in-process, wired exactly like `stc serve`: queued
// writes are applied by ApplyPending in Run's OnTick, between two Ticks.
type liveServer struct {
	p      *interp.Project
	src    *bind.RuntimeSource
	srv    *opcua.Server
	mu     sync.Mutex
	errs   []error // ApplyPending errors
	runErr chan error
}

func startLive(t *testing.T) *liveServer {
	t.Helper()
	spec, res, _, err := projectload.Load([]string{liveFixture}, map[string]bool{"STC_SIM": true})
	require.NoError(t, err)
	p, err := interp.LoadProject(spec)
	require.NoError(t, err)
	require.Equal(t, 10*time.Millisecond, p.BaseTick())
	tree, err := symtree.Build(res)
	require.NoError(t, err)
	ls := &liveServer{p: p, src: bind.NewRuntimeSource(p.Runtime()), runErr: make(chan error, 1)}
	space, diags := opcua.Build(bind.Root(tree), ls.src)
	require.Empty(t, diags)

	var lastErr error
	for attempt := 0; attempt < 3 && ls.srv == nil; attempt++ {
		cfg := opcua.DefaultConfig()
		cfg.Endpoint = freeAddr(t)
		cfg.PKIDir = t.TempDir()
		srv, err := opcua.New(cfg)
		require.NoError(t, err)
		require.NoError(t, srv.Publish(space, ls.src))
		if lastErr = srv.Start(); lastErr != nil {
			_ = srv.Stop()
			continue
		}
		ls.srv = srv
	}
	require.NoError(t, lastErr)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ls.runErr <- p.Run(ctx, interp.RunOpts{OnTick: func(time.Duration) {
			if err := ls.src.ApplyPending(); err != nil {
				ls.mu.Lock()
				ls.errs = append(ls.errs, err)
				ls.mu.Unlock()
			}
		}})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-ls.runErr:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(liveTimeout):
			t.Error("Run did not stop")
		}
		_ = ls.srv.Stop()
		ls.mu.Lock()
		assert.Empty(t, ls.errs, "ApplyPending errors")
		ls.mu.Unlock()
	})
	return ls
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func dialLive(t *testing.T, ls *liveServer) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	c, err := client.Dial(ctx, ls.srv.Endpoint(), client.WithInsecureSkipVerify())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			_ = c.Abort(ctx)
		}
	})
	return c
}

func n4(path string) ua.NodeID { return ua.NewNodeIDString(4, path) }

func read(t *testing.T, c *client.Client, path string) any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	res, err := c.Read(ctx, &ua.ReadRequest{NodesToRead: []ua.ReadValueID{{NodeID: n4(path), AttributeID: ua.AttributeIDValue}}})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	require.Equal(t, ua.Good, res.Results[0].StatusCode, path)
	return res.Results[0].Value
}

func write(t *testing.T, c *client.Client, path string, v any) ua.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	res, err := c.Write(ctx, &ua.WriteRequest{NodesToWrite: []ua.WriteValue{{
		NodeID: n4(path), AttributeID: ua.AttributeIDValue,
		Value: ua.NewDataValue(v, ua.Good, time.Time{}, 0, time.Time{}, 0),
	}}})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	return res.Results[0]
}

// waitFor polls cond until it holds or liveTimeout passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(liveTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", liveTimeout, what)
}

const hmi = "GVL_Live.conveyor.HMI."

// TestLiveHandshake proves the p_cmd_* set-TRUE / FB-clears handshake
// against a free-running Project (D-02, the in-process half of D-03): the
// write is consumed by the FB within 10 cycles, the command reads back
// FALSE and the FB's effect is visible, all through OPC UA.
func TestLiveHandshake(t *testing.T) {
	ls := startLive(t)
	c := dialLive(t, ls)

	waitFor(t, "state rdy(2)", func() bool { return read(t, c, hmi+"p_stat_State") == int32(2) })
	assert.Equal(t, false, read(t, c, hmi+"p_stat_Running"))
	assert.Equal(t, int32(0), read(t, c, hmi+"p_stat_Starts"))

	before := read(t, c, "GVL_Live.nCycle").(int32)
	require.Equal(t, ua.Good, write(t, c, hmi+"p_cmd_Start", true))
	waitFor(t, "p_stat_Running", func() bool { return read(t, c, hmi+"p_stat_Running") == true })

	// Consumed, not lost: the FB counted exactly one start, stamped the
	// cycle it acted in and cleared the command.
	assert.Equal(t, false, read(t, c, hmi+"p_cmd_Start"), "FB clears p_cmd_Start")
	assert.Equal(t, int32(1), read(t, c, hmi+"p_stat_Starts"))
	assert.Equal(t, int32(3), read(t, c, hmi+"p_stat_State"), "run(3)")
	acted := read(t, c, hmi+"p_stat_StartCycle").(int32)
	assert.Greater(t, acted, before)
	assert.LessOrEqual(t, acted-before, int32(10), "consumed within 10 cycles")

	// The same handshake stops it again.
	require.Equal(t, ua.Good, write(t, c, hmi+"p_cmd_Stop", true))
	waitFor(t, "stopped", func() bool { return read(t, c, hmi+"p_stat_Running") == false })
	assert.Equal(t, false, read(t, c, hmi+"p_cmd_Stop"))
	assert.Equal(t, int32(2), read(t, c, hmi+"p_stat_State"))
}

// TestLiveWritesBetweenScans toggles a probe many times while the scan runs.
// MAIN samples it at the start and at the end of its body, so a write
// applied during a Tick would count a mismatch (T-29-01).
func TestLiveWritesBetweenScans(t *testing.T) {
	ls := startLive(t)
	c := dialLive(t, ls)
	for i := 0; i < 60; i++ {
		require.Equal(t, ua.Good, write(t, c, "GVL_Live.xProbe", i%2 == 0))
	}
	require.Equal(t, ua.Good, write(t, c, "GVL_Live.xProbe", true))
	waitFor(t, "probe consumed", func() bool { return read(t, c, "GVL_Live.nProbeTrue").(int32) > 0 })
	waitFor(t, "queue drained", func() bool { return ls.src.Pending() == 0 })
	assert.Equal(t, int32(0), read(t, c, "GVL_Live.nMismatch"), "a write landed in the middle of a Tick")
}

// TestLiveStatusNotWritable checks that p_stat_* members reject writes and
// keep their value.
func TestLiveStatusNotWritable(t *testing.T) {
	ls := startLive(t)
	c := dialLive(t, ls)
	waitFor(t, "state rdy(2)", func() bool { return read(t, c, hmi+"p_stat_State") == int32(2) })
	assert.Equal(t, ua.BadNotWritable, write(t, c, hmi+"p_stat_Running", true))
	assert.Equal(t, ua.BadNotWritable, write(t, c, hmi+"p_stat_State", int32(3)))
	assert.Equal(t, ua.BadNotWritable, write(t, c, "GVL_Live.nMismatch", int32(5)))
	assert.Zero(t, ls.src.Pending())
	start := read(t, c, "GVL_Live.nCycle").(int32)
	waitFor(t, "three more cycles", func() bool { return read(t, c, "GVL_Live.nCycle").(int32) >= start+3 })
	assert.Equal(t, false, read(t, c, hmi+"p_stat_Running"))
	assert.Equal(t, int32(2), read(t, c, hmi+"p_stat_State"))
	assert.Equal(t, int32(0), read(t, c, "GVL_Live.nMismatch"))
}
