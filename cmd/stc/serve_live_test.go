package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var liveFixture = filepath.Join("..", "..", "tests", "opcua_live")

// liveSub collects data-change notifications of one subscription from a
// client Publish loop, per client handle.
type liveSub struct {
	mu   sync.Mutex
	got  map[uint32][]any
	done chan struct{}
}

func subscribeLive(t *testing.T, c *client.Client, ids map[uint32]ua.NodeID) *liveSub {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	sub, err := c.CreateSubscription(ctx, &ua.CreateSubscriptionRequest{
		RequestedPublishingInterval: 100, RequestedMaxKeepAliveCount: 30,
		RequestedLifetimeCount: 120, PublishingEnabled: true,
	})
	require.NoError(t, err)
	var items []ua.MonitoredItemCreateRequest
	for h, id := range ids {
		items = append(items, ua.MonitoredItemCreateRequest{
			ItemToMonitor:  ua.ReadValueID{NodeID: id, AttributeID: ua.AttributeIDValue},
			MonitoringMode: ua.MonitoringModeReporting,
			RequestedParameters: ua.MonitoringParameters{
				ClientHandle: h, SamplingInterval: 50, QueueSize: 10, DiscardOldest: true,
			},
		})
	}
	res, err := c.CreateMonitoredItems(ctx, &ua.CreateMonitoredItemsRequest{
		SubscriptionID: sub.SubscriptionID, TimestampsToReturn: ua.TimestampsToReturnBoth, ItemsToCreate: items,
	})
	require.NoError(t, err)
	for _, r := range res.Results {
		require.Equal(t, ua.Good, r.StatusCode)
	}
	s := &liveSub{got: map[uint32][]any{}, done: make(chan struct{})}
	loopCtx, stop := context.WithCancel(context.Background())
	go func() {
		defer close(s.done)
		var acks []ua.SubscriptionAcknowledgement
		for loopCtx.Err() == nil {
			r, err := c.Publish(loopCtx, &ua.PublishRequest{SubscriptionAcknowledgements: acks})
			if err != nil {
				return
			}
			acks = nil
			if len(r.NotificationMessage.NotificationData) > 0 {
				acks = []ua.SubscriptionAcknowledgement{{SubscriptionID: r.SubscriptionID, SequenceNumber: r.NotificationMessage.SequenceNumber}}
			}
			s.mu.Lock()
			for _, nd := range r.NotificationMessage.NotificationData {
				var dcn ua.DataChangeNotification
				switch n := nd.(type) {
				case ua.DataChangeNotification:
					dcn = n
				case *ua.DataChangeNotification:
					dcn = *n
				default:
					continue
				}
				for _, mi := range dcn.MonitoredItems {
					s.got[mi.ClientHandle] = append(s.got[mi.ClientHandle], mi.Value.Value)
				}
			}
			s.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		stop()
		<-s.done
	})
	return s
}

func (s *liveSub) values(h uint32) []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]any(nil), s.got[h]...)
}

// TestServeLiveHandshake is the end-to-end half of D-03 plus OPCUA-08
// against the live scan of `stc serve`: a subscribed client sees
// p_stat_Running go FALSE then TRUE when its p_cmd_Start write is consumed
// by the FB, which clears the command between scans.
func TestServeLiveHandshake(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, true, "--project", liveFixture, "--opcua", addr, "--cycle", "10ms")
	assert.Equal(t, "10ms", s.info.Cycle)
	assert.Empty(t, s.info.Diagnostics)
	c := dialServe(t, s.info.Endpoint)
	requireRunning(t, c)

	const hmi = "GVL_Live.conveyor.HMI."
	eventually(t, "state rdy(2)", func() bool { return readNode(t, c, s4(hmi+"p_stat_State")).Value == int32(2) })

	const hRunning, hCmd = 1, 2
	sub := subscribeLive(t, c, map[uint32]ua.NodeID{hRunning: s4(hmi + "p_stat_Running"), hCmd: s4(hmi + "p_cmd_Start")})
	eventually(t, "initial notifications", func() bool {
		return len(sub.values(hRunning)) >= 1 && len(sub.values(hCmd)) >= 1
	})
	require.Equal(t, false, sub.values(hRunning)[0])

	before := readNode(t, c, s4("GVL_Live.nCycle")).Value.(int32)
	require.Equal(t, ua.Good, writeNode(t, c, s4(hmi+"p_cmd_Start"), true))
	eventually(t, "Running notification", func() bool {
		v := sub.values(hRunning)
		return len(v) >= 2 && v[len(v)-1] == true
	})
	assert.Equal(t, []any{false, true}, sub.values(hRunning), "one data change for one start")

	// Consumed by the FB, not lost: cleared, counted once, within 10 cycles.
	assert.Equal(t, false, readNode(t, c, s4(hmi+"p_cmd_Start")).Value)
	assert.Equal(t, int32(1), readNode(t, c, s4(hmi+"p_stat_Starts")).Value)
	acted := readNode(t, c, s4(hmi+"p_stat_StartCycle")).Value.(int32)
	assert.Greater(t, acted, before)
	assert.LessOrEqual(t, acted-before, int32(10))
	cmd := sub.values(hCmd)
	assert.Equal(t, false, cmd[len(cmd)-1], "subscriber sees the cleared command last")
	assert.Equal(t, ua.BadNotWritable, writeNode(t, c, s4(hmi+"p_stat_Running"), false))
	assert.Equal(t, int32(0), readNode(t, c, s4("GVL_Live.nMismatch")).Value)

	s.cancel()
	assert.NoError(t, s.wait(t))
	assert.NotContains(t, s.stderr.String(), "write_error")
}

// TestServeScenario is 29-01's live scenario mode: `stc serve --scenario`
// fires toggle.toml's step as Ticks elapse, and a subscribed OPC UA client
// sees the sensor's HMI status follow the stimulated input exactly once.
func TestServeScenario(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, true, "--project", liveFixture, "--opcua", addr, "--cycle", "10ms",
		"--scenario", filepath.Join(liveFixture, "toggle.toml"))
	c := dialServe(t, s.info.Endpoint)
	requireRunning(t, c)

	const hRaw = 1
	sub := subscribeLive(t, c, map[uint32]ua.NodeID{hRaw: s4("GVL_Live.sensor.HMI.p_stat_xRaw")})
	eventually(t, "xRaw goes TRUE", func() bool {
		v := sub.values(hRaw)
		return len(v) >= 2 && v[len(v)-1] == true
	})
	assert.Equal(t, []any{false, true}, sub.values(hRaw))

	s.cancel()
	assert.NoError(t, s.wait(t))
	assert.Contains(t, s.stderr.String(), `"event":"scenario"`)
	assert.Contains(t, s.stderr.String(), `"passed":true`)
}

// TestServeScenarioInvalid checks a scenario that does not validate stops
// serve before the OPC UA server starts, with its diagnostic.
func TestServeScenarioInvalid(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.toml")
	require.NoError(t, os.WriteFile(bad, []byte("[[step]]\ncycle = 0\nset = { path = \"GVL_Live.nope\", value = 1 }\n"), 0o644))
	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"serve", "--project", liveFixture, "--opcua", freeServeAddr(t), "--scenario", bad})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, stderr.String(), "bad.toml:1:1: error: step 1: set: unknown path")
	assert.Empty(t, stdout.String(), "no server start-up line")
}

// TestServeScenarioText runs a short text-mode serve whose scenario ends
// before its expect is due, so the report shows the failed expect as a
// warning and serve still succeeds.
func TestServeScenarioText(t *testing.T) {
	sc := filepath.Join(t.TempDir(), "late.toml")
	require.NoError(t, os.WriteFile(sc, []byte("[scenario]\nname = \"late\"\n[[step]]\ncycle = 100000\nexpect = { path = \"GVL_Live.xProbe\", value = true }\n"), 0o644))
	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"serve", "--project", liveFixture, "--cycle", "10ms", "--duration", "100ms", "--scenario", sc})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	require.NoError(t, root.Execute())
	assert.Contains(t, stderr.String(), "scenario late:")
	assert.Contains(t, stderr.String(), "late.toml:3:1: warning: step 1 never fired")
}
