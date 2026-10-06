package main

import (
	"bytes"
	"math"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"github.com/centroid-is/stc/pkg/interp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	liveFixture = filepath.Join("testdata", "live")
	liveIO      = filepath.Join("..", "..", "tests", "ecat_fixtures", "Demo Device 1.xml")
)

const (
	pStart   = "GVL_Live.conveyor.HMI.p_cmd_Start"
	pRunning = "GVL_Live.conveyor.HMI.p_stat_Running"
	pSensor  = "GVL_Live.xSensor"
	pSensorH = "GVL_Live.conveyor.HMI.p_stat_Sensor"
	pCount   = "GVL_Live.nRunCycles"
)

func liveConfig() simConfig {
	return simConfig{Project: liveFixture, IO: []string{liveIO}}
}

func newLive(t *testing.T, cfg simConfig) *simSession {
	t.Helper()
	s, err := newSimSession(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func values(entries []readEntry) []any {
	out := make([]any, len(entries))
	for i, e := range entries {
		out[i] = e.Value
	}
	return out
}

func TestSimSessionLoad(t *testing.T) {
	s := newLive(t, liveConfig())
	r := s.Read([]string{pCount})
	assert.Equal(t, []readEntry{{Path: pCount, Value: int64(0)}}, r)
	assert.Equal(t, "", s.Endpoint())
	assert.Equal(t, 0, s.cycles)
}

func TestSimSessionStep(t *testing.T) {
	s := newLive(t, liveConfig())
	for _, n := range []int{0, -1, maxStepCycles + 1} {
		_, err := s.Step(n)
		assert.Error(t, err, "n=%d", n)
	}
	res, err := s.Step(3)
	require.NoError(t, err)
	assert.Equal(t, stepResult{Stepped: 3, Cycles: 3, SimTimeMS: 30, ScenarioFailures: []string{}}, res)
	res, err = s.Step(2)
	require.NoError(t, err)
	assert.Equal(t, 5, res.Cycles)
	assert.Equal(t, int64(50), res.SimTimeMS)
}

func TestSimSessionWriteRuntimeSet(t *testing.T) {
	s := newLive(t, liveConfig())
	w, err := s.Write(pStart, true)
	require.NoError(t, err)
	assert.Equal(t, writeResult{Path: pStart, Route: "runtime_set", Value: true}, w)
	assert.Equal(t, []any{true, false}, values(s.Read([]string{pStart, pRunning})))
	_, err = s.Step(2)
	require.NoError(t, err)
	assert.Equal(t, []any{false, true, int64(2)}, values(s.Read([]string{pStart, pRunning, pCount})))

	// Numbers and strings go through the Phase 22 coercion.
	_, err = s.Write("GVL_Live.conveyor.HMI.p_cfg_Speed", float64(75))
	require.NoError(t, err)
	_, err = s.Write("GVL_Live.conveyor.HMI.p_cfg_Speed", "INT#80")
	require.NoError(t, err)
	assert.Equal(t, []any{int64(80)}, values(s.Read([]string{"GVL_Live.conveyor.HMI.p_cfg_Speed"})))

	for _, tc := range []struct {
		path  string
		value any
	}{
		{pStart, "maybe"},
		{"GVL_Live.conveyor.HMI.p_cfg_Speed", float64(70000)},
		{"GVL_Live.cMax", float64(3)},
		{"GVL_Live.nope", true},
	} {
		_, err := s.Write(tc.path, tc.value)
		require.Error(t, err, tc.path)
		assert.Contains(t, err.Error(), tc.path)
	}
}

func TestSimSessionWriteForcedInput(t *testing.T) {
	s := newLive(t, liveConfig())
	_, err := s.Step(1)
	require.NoError(t, err)
	w, err := s.Write(pSensor, true)
	require.NoError(t, err)
	assert.Equal(t, "input_force", w.Route)
	assert.Equal(t, []any{true}, values(s.Read([]string{pSensor})))
	// The force survives the input copy of every following scan.
	_, err = s.Step(3)
	require.NoError(t, err)
	assert.Equal(t, []any{true, true}, values(s.Read([]string{pSensor, pSensorH})))
	_, err = s.Write(pSensor, false)
	require.NoError(t, err)
	_, err = s.Step(1)
	require.NoError(t, err)
	assert.Equal(t, []any{false, false}, values(s.Read([]string{pSensor, pSensorH})))
}

func TestSimSessionUnforcedInputFollowsImage(t *testing.T) {
	// Without a force, a plain Set of a linked input is overwritten by the
	// terminal's zero on the next scan; Write must therefore force it.
	s := newLive(t, liveConfig())
	require.NoError(t, s.rt.Set(pSensor, true))
	_, err := s.Step(1)
	require.NoError(t, err)
	assert.Equal(t, []any{false}, values(s.Read([]string{pSensor})))
}

func TestSimSessionReadUnknownPath(t *testing.T) {
	s := newLive(t, liveConfig())
	r := s.Read([]string{pRunning, "GVL_Live.missing", pCount})
	require.Len(t, r, 3)
	assert.Equal(t, false, r[0].Value)
	assert.NotEmpty(t, r[1].Error)
	assert.Nil(t, r[1].Value)
	assert.Equal(t, int64(0), r[2].Value)
}

func TestSimSessionConcurrentSteps(t *testing.T) {
	s := newLive(t, liveConfig())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Step(5)
			assert.NoError(t, err)
			_ = s.Read([]string{pCount})
		}()
	}
	wg.Wait()
	res, err := s.Step(1)
	require.NoError(t, err)
	assert.Equal(t, 41, res.Cycles)
}

func TestSimSessionOPCUA(t *testing.T) {
	cfg := liveConfig()
	cfg.OPCUA = freeAddr(t)
	cfg.PKIDir = t.TempDir()
	s := newLive(t, cfg)
	assert.Contains(t, s.Endpoint(), "opc.tcp://")
	// An OPC UA write is queued and applied by the next Step.
	require.NoError(t, s.src.Write(pStart, true))
	assert.Equal(t, []any{false}, values(s.Read([]string{pStart})))
	_, err := s.Step(2)
	require.NoError(t, err)
	assert.Equal(t, []any{true}, values(s.Read([]string{pRunning})))
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
	assert.Equal(t, "", s.Endpoint())
}

func TestSimSessionOPCUABadAddress(t *testing.T) {
	cfg := liveConfig()
	cfg.OPCUA = "256.0.0.1:x"
	cfg.PKIDir = t.TempDir()
	_, err := newSimSession(cfg)
	assert.Error(t, err)
}

func TestSimSessionLoadErrors(t *testing.T) {
	for name, cfg := range map[string]simConfig{
		"no project": {},
		"scenario":   {Project: liveFixture, Scenario: "x.yaml"},
		"missing":    {Project: filepath.Join(t.TempDir(), "missing.st")},
		"bad io":     {Project: liveFixture, IO: []string{filepath.Join(t.TempDir(), "none.xml")}},
		"wrong io":   {Project: liveFixture, IO: []string{filepath.Join("..", "..", "tests", "ecat_fixtures", "Demo Device 2.xml")}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newSimSession(cfg)
			assert.Error(t, err)
		})
	}
}

func TestSimSessionHost(t *testing.T) {
	h := &simHost{}
	_, err := h.session()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start stc-mcp with --project <path> [--io ...]")
	require.NoError(t, h.close())

	h = &simHost{cfg: simConfig{Project: filepath.Join(t.TempDir(), "missing.st")}}
	_, err = h.session()
	assert.ErrorContains(t, err, "loading simulation")

	h = &simHost{cfg: liveConfig()}
	s1, err := h.session()
	require.NoError(t, err)
	s2, err := h.session()
	require.NoError(t, err)
	assert.Same(t, s1, s2)
	require.NoError(t, h.close())
}

func TestSimSessionRawBits(t *testing.T) {
	for _, tc := range []struct {
		v      interp.Value
		bitLen int
		want   uint64
	}{
		{interp.Value{Kind: interp.ValBool, Bool: true}, 1, 1},
		{interp.Value{Kind: interp.ValBool}, 1, 0},
		{interp.Value{Kind: interp.ValInt, Int: -1}, 16, math.MaxUint64},
		{interp.Value{Kind: interp.ValReal, Real: 1.5}, 32, uint64(math.Float32bits(1.5))},
		{interp.Value{Kind: interp.ValReal, Real: 1.5}, 64, math.Float64bits(1.5)},
	} {
		got, err := rawBits(tc.v, tc.bitLen)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
	_, err := rawBits(interp.Value{Kind: interp.ValString}, 8)
	assert.Error(t, err)
}

func TestParseFlags(t *testing.T) {
	var errb bytes.Buffer
	cfg, err := parseFlags([]string{"--project", liveFixture, "--io", filepath.Join("..", "..", "tests", "ecat_fixtures", "Demo Device *.xml"), "--opcua", ":4841"}, &errb)
	require.NoError(t, err)
	assert.Equal(t, liveFixture, cfg.Project)
	assert.Len(t, cfg.IO, 2)
	assert.Equal(t, ":4841", cfg.OPCUA)

	cfg, err = parseFlags([]string{"--io", "a.xml,b.xml", "--io", "c.xml", "--project", "p"}, &errb)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.xml", "b.xml", "c.xml"}, cfg.IO)

	cfg, err = parseFlags(nil, &errb)
	require.NoError(t, err)
	assert.Equal(t, simConfig{}, cfg)

	_, err = parseFlags([]string{"--opcua", ":1"}, &errb)
	assert.Error(t, err)
	_, err = parseFlags([]string{"stray"}, &errb)
	assert.Error(t, err)
	_, err = parseFlags([]string{"--bogus"}, &errb)
	assert.Error(t, err)
	var l stringList
	require.NoError(t, l.Set("x, ,y"))
	assert.Equal(t, "x,y", l.String())
}
