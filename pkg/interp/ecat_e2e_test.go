package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/ecat/devices"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	atvDev   = "Device 3 (EtherCAT)"
	atvAddr  = 1001
	atvScan  = 50 * time.Millisecond
	cfgReady = 6 // FB_MiniATV320 configState cfgReady
)

func beckhoffStub(name string) string {
	return filepath.Join("..", "..", "stdlib", "vendor", "beckhoff", name)
}

// parseNoErr parses src and fails on error diagnostics.
func parseNoErr(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	res := pipeline.Parse(name, src, nil)
	for _, d := range res.Diags {
		require.NotEqual(t, diag.Error, d.Severity, "%s: %s", name, d.Message)
	}
	return res.File
}

// atvRig is a PROGRAM wired to the ATV320 fixture through an IOBinder with
// the Tc2_EtherCAT mocks attached to the same network.
type atvRig struct {
	t   *testing.T
	eng *ScanCycleEngine
	net *ecat.Network
	drv *devices.ATV320
}

// newAtvRig builds the engine from the Beckhoff stubs plus files (the
// first holding the ECT GVL), resolves its TcLinkTo links against
// atv320_device.xml and attaches the mocks before the first Tick.
func newAtvRig(t *testing.T, files ...*ast.SourceFile) *atvRig {
	t.Helper()
	var all []*ast.SourceFile
	for _, n := range []string{"tc2_system.st", "tc2_ethercat.st"} {
		src, err := os.ReadFile(beckhoffStub(n))
		require.NoError(t, err)
		all = append(all, parseNoErr(t, n, string(src)))
	}
	all = append(all, files...)

	var prog *ast.ProgramDecl
	for _, f := range all {
		for _, d := range f.Declarations {
			if p, ok := d.(*ast.ProgramDecl); ok && prog == nil {
				prog = p
			}
		}
	}
	require.NotNil(t, prog)

	topo, err := ecat.LoadProject(ecatFixture("atv320_device.xml"))
	require.NoError(t, err)
	net := ecat.NewNetwork(topo, nil)

	eng := NewScanCycleEngine(prog)
	eng.SetNetwork(net) // before any FB is instantiated
	require.NoError(t, eng.interp.RegisterFiles(all))

	vars, _ := ecat.CollectLinks(all)
	bindings, diags := ecat.Resolve(topo, vars)
	for _, d := range diags {
		require.NotEqual(t, "error", d.Severity.String(), d.Message)
	}
	require.NotEmpty(t, bindings)
	b := NewIOBinder(bindings, net)
	eng.SetIOBinder(b)
	t.Cleanup(func() { assert.Empty(t, b.Errors()) })

	_, dev, err := net.Slave(atvDev, atvAddr)
	require.NoError(t, err)
	drv, ok := dev.(*devices.ATV320)
	require.True(t, ok, "slave 1001 is not the ATV320 model")
	return &atvRig{t: t, eng: eng, net: net, drv: drv}
}

func (r *atvRig) tick(n int) {
	r.t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(r.t, r.eng.Tick(atvScan))
	}
}

// until ticks until cond holds, at most max scans, and returns the scans used.
func (r *atvRig) until(max int, what string, cond func() bool) int {
	r.t.Helper()
	for i := 1; i <= max; i++ {
		r.tick(1)
		if cond() {
			return i
		}
	}
	require.FailNow(r.t, "timeout waiting for "+what)
	return 0
}

// g reads GVL.var[.member...], stepping into FB instances by their env.
func (r *atvRig) g(steps ...string) Value {
	r.t.Helper()
	env, last := r.fbEnv(steps...)
	v, ok := env.Get(strings.ToUpper(last))
	require.True(r.t, ok, strings.Join(steps, "."))
	return v
}

// setG writes GVL.var[.member...] the same way.
func (r *atvRig) setG(v Value, steps ...string) {
	r.t.Helper()
	env, last := r.fbEnv(steps...)
	env.Set(strings.ToUpper(last), v)
}

// fbEnv walks every FB instance step but the last and returns its env.
func (r *atvRig) fbEnv(steps ...string) (*Env, string) {
	r.t.Helper()
	env := r.eng.interp.lookupGVL(steps[0])
	require.NotNil(r.t, env, steps[0])
	for _, s := range steps[1 : len(steps)-1] {
		v, ok := env.Get(strings.ToUpper(s))
		require.True(r.t, ok, s)
		require.NotNil(r.t, v.FBRef, s)
		env = v.FBRef.Env
	}
	return env, steps[len(steps)-1]
}

func (r *atvRig) local(name string) Value {
	r.t.Helper()
	v, ok := r.eng.env.Get(strings.ToUpper(name))
	require.True(r.t, ok, name)
	return v
}

func miniRig(t *testing.T) *atvRig {
	src, err := os.ReadFile(ecatFixture("atv320_mini.st"))
	require.NoError(t, err)
	f := parseNoErr(t, "atv320_mini.st", string(src))
	ast.SetGVLName(f, "ECT")
	return newAtvRig(t, f)
}

func TestEcatE2EConfigureRunFault(t *testing.T) {
	r := miniRig(t)
	assert.Equal(t, ecat.StatePreOp, r.drv.EcState(), "drive boots in PreOp")

	n := r.until(40, "cfgReady", func() bool { return r.g("ECT", "drive", "configState").Int == cfgReady })
	t.Logf("mini configurator reached cfgReady after %d scans", n)
	assert.Equal(t, int64(0), r.g("ECT", "drive", "nLastConfigError").Int)
	assert.Equal(t, ecat.StateOP, r.drv.EcState())
	r.tick(1)
	assert.Equal(t, int64(8), r.g("ECT", "driveState").Int, "InfoData.State")
	assert.True(t, r.g("ECT", "drive", "q_xReady").Bool)
	assert.Equal(t, int64(500), int64(r.drv.HSP), "HSP written over SDO")
	assert.Equal(t, int64(30), int64(r.drv.ACC), "ACC written over SDO")

	// Run forward: ETA reaches Operation enabled, RFR ramps monotonically to LFR.
	r.setG(BoolValue(true), "ECT", "drive", "i_xFwd")
	r.until(10, "operation enabled", func() bool { return r.drv.ETA()&0x6F == 0x27 })
	last := int16(-1)
	r.until(200, "RFR 500", func() bool {
		rfr := r.drv.RFR()
		require.GreaterOrEqual(t, rfr, last, "RFR must rise monotonically")
		last = rfr
		return rfr == 500
	})
	r.tick(2)
	assert.Equal(t, 1, r.drv.SaveCount(), "EEPROM save committed")
	assert.Equal(t, int64(500), r.g("ECT", "drive", "i_iRFR").Int)
	assert.Equal(t, int64(devices.HMISRun), r.g("ECT", "drive", "i_eHMIS").Int)
	assert.Equal(t, uint16(devices.HMISRun), r.drv.HMIS())

	// Fault: the FB sees the fault word and the LFT code.
	r.drv.InjectFault(16)
	r.until(5, "q_xError", func() bool { return r.g("ECT", "drive", "q_xError").Bool })
	r.tick(1)
	assert.Equal(t, int64(16), r.g("ECT", "drive", "i_eLFT").Int)

	// Clear and reset: the drive leaves Fault for Switch on disabled.
	r.drv.ClearFault()
	r.setG(BoolValue(false), "ECT", "drive", "i_xFwd")
	r.setG(BoolValue(true), "ECT", "drive", "i_xReset")
	r.until(10, "switch on disabled", func() bool { return r.drv.ETA()&0x6F == 0x50 || r.drv.ETA()&0x6F == 0x21 })
	r.setG(BoolValue(false), "ECT", "drive", "i_xReset")
	r.tick(3)
	assert.False(t, r.g("ECT", "drive", "q_xError").Bool)
}

func TestEcatE2EDiag(t *testing.T) {
	r := miniRig(t)
	r.until(40, "cfgReady", func() bool { return r.g("ECT", "drive", "configState").Int == cfgReady })
	require.NoError(t, r.net.SetCrcErrors(atvDev, 0, [4]uint32{3, 4, 0, 0}))

	r.eng.env.Set("DIAGGO", BoolValue(true))
	for scan := 1; scan <= 2; scan++ {
		r.tick(1)
		for _, n := range []string{"statesBusy", "masterBusy", "crcBusy"} {
			assert.True(t, r.local(n).Bool, "%s scan %d", n, scan)
		}
	}
	r.tick(1)
	for _, n := range []string{"statesBusy", "masterBusy", "crcBusy"} {
		assert.False(t, r.local(n).Bool, "%s done", n)
	}
	st := r.local("aStates").Array[0]
	assert.Equal(t, int64(8), st.Struct["DEVICESTATE"].Int)
	assert.Equal(t, int64(0), st.Struct["LINKSTATE"].Int)
	assert.Equal(t, int64(8), r.local("masterState").Int)
	assert.Equal(t, int64(7), r.local("aCrc").Array[0].Int)
}
