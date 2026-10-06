package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	e2eDev1  = "Device 1 (EtherCAT)"
	e2eA101  = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)"
	e2eCN01  = "TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)"
	e2eV1Out = "TIID^Device 1 (EtherCAT)^DEMO.V1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C1 Output"
)

func ecatFixture(name string) string {
	return filepath.Join("..", "..", "tests", "ecat_fixtures", name)
}

func parseFixture(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	res := pipeline.Parse(name, src, nil)
	require.Empty(t, res.Diags, name)
	return res.File
}

// demoEngine wires demo_types.st, demo_ect.st (GVL ECT) and a program to the
// Demo Device 1/2 topology through an IOBinder.
func demoEngine(t *testing.T) (*ScanCycleEngine, *ecat.Topology, *ecat.Network) {
	t.Helper()
	var files []*ast.SourceFile
	for _, n := range []string{"demo_types.st", "demo_ect.st"} {
		src, err := os.ReadFile(ecatFixture(n))
		require.NoError(t, err)
		files = append(files, parseFixture(t, n, string(src)))
	}
	ast.SetGVLName(files[1], "ECT")
	files = append(files, parseFixture(t, "Main.st", `
PROGRAM Main
ECT.CN01();
ECT.V1_C1 := 16#5A;
END_PROGRAM
`))

	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	typeDecls := map[string]ast.TypeSpec{}
	fbDecls := map[string]*ast.FunctionBlockDecl{}
	for _, f := range files {
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.ProgramDecl:
				prog = d
			case *ast.GVLDecl:
				gvls = append(gvls, d)
			case *ast.TypeDecl:
				typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
			case *ast.FunctionBlockDecl:
				fbDecls[strings.ToUpper(d.Name.Name)] = d
			}
		}
	}
	eng := NewScanCycleEngine(prog)
	eng.interp.TypeDecls = typeDecls
	eng.interp.FBDecls = fbDecls
	eng.SetGlobals(gvls)

	topo, err := ecat.LoadProject(ecatFixture("Demo Device 1.xml"), ecatFixture("Demo Device 2.xml"))
	require.NoError(t, err)
	vars, _ := ecat.CollectLinks(files)
	bindings, diags := ecat.Resolve(topo, vars)
	for _, d := range diags {
		require.NotEqual(t, "error", d.Severity.String(), d.Message)
	}
	net := ecat.NewNetwork(topo, nil)
	b := NewIOBinder(bindings, net)
	eng.SetIOBinder(b)
	t.Cleanup(func() { assert.Empty(t, b.Errors()) })
	return eng, topo, net
}

func slotBuf(t *testing.T, topo *ecat.Topology, net *ecat.Network, path string) (ecat.Slot, []byte) {
	t.Helper()
	s, ok := topo.Slot(path)
	require.True(t, ok, path)
	img := net.Images().Get(s.Master)
	if s.Dir == ecat.DirOut {
		return s, img.Out
	}
	return s, img.In
}

func TestIOBindE2EInputs(t *testing.T) {
	eng, topo, net := demoEngine(t)
	s, buf := slotBuf(t, topo, net, e2eA101+"^Channel 1^Input")
	ecat.WriteBits(buf, s.Byte, s.Bit, s.BitLen, 1)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	assert.True(t, ioRead(t, eng, "ECT", "A1_01", "I1").Bool)
	ecat.WriteBits(buf, s.Byte, s.Bit, s.BitLen, 0)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	assert.False(t, ioRead(t, eng, "ECT", "A1_01", "I1").Bool)
}

func TestIOBindE2EOutputs(t *testing.T) {
	eng, topo, net := demoEngine(t)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	s, buf := slotBuf(t, topo, net, e2eCN01+"^Outputs^CMD")
	assert.Equal(t, uint64(0x000F), ecat.ReadBits(buf, s.Byte, s.Bit, s.BitLen))
	s, buf = slotBuf(t, topo, net, e2eV1Out)
	assert.Equal(t, uint64(0x5A), ecat.ReadBits(buf, s.Byte, s.Bit, s.BitLen))
}

func TestIOBindE2EPseudoInputsAndFaults(t *testing.T) {
	eng, _, net := demoEngine(t)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	assert.Equal(t, int64(10), ioRead(t, eng, "ECT", "Dev1_SlaveCount").Int)
	assert.Equal(t, int64(8), ioRead(t, eng, "ECT", "A1_01_State").Int)
	assert.False(t, ioRead(t, eng, "ECT", "A1_01_WcState").Bool)
	assert.Equal(t, int64(0), ioRead(t, eng, "ECT", "Dev1_DevState").Int)
	assert.Equal(t, int64(0), ioRead(t, eng, "ECT", "Dev1_Frm0State").Int)
	assert.Equal(t, int64(0), ioRead(t, eng, "ECT", "Dev1_Frm0WcState").Int)
	assert.Equal(t, int64(192), ioRead(t, eng, "ECT", "Dev1_AmsNetId").Array[0].Int)

	cn := ioRead(t, eng, "ECT", "CN01")
	ams, ok := cn.FBRef.Env.GetLocal("AMSADDR")
	require.True(t, ok)
	assert.Equal(t, int64(1007), ams.Struct["PORT"].Int)
	assert.Equal(t, int64(192), ams.Struct["NETID"].Array[0].Int)

	require.NoError(t, net.SetSlaveState(e2eDev1, 1, 2))
	require.NoError(t, net.SetWcState(e2eDev1, 1, true))
	require.NoError(t, net.SetDevState(e2eDev1, 0x0001))
	require.NoError(t, eng.Tick(10*time.Millisecond))
	assert.Equal(t, int64(2), ioRead(t, eng, "ECT", "A1_01_State").Int)
	assert.True(t, ioRead(t, eng, "ECT", "A1_01_WcState").Bool)
	assert.Equal(t, int64(1), ioRead(t, eng, "ECT", "Dev1_DevState").Int)
}

func TestIOBindE2ENilBinderUnchanged(t *testing.T) {
	eng, topo, net := demoEngine(t)
	eng.SetIOBinder(nil)
	s, buf := slotBuf(t, topo, net, e2eA101+"^Channel 1^Input")
	ecat.WriteBits(buf, s.Byte, s.Bit, s.BitLen, 1)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	assert.False(t, ioRead(t, eng, "ECT", "A1_01", "I1").Bool)
	s, buf = slotBuf(t, topo, net, e2eCN01+"^Outputs^CMD")
	assert.Equal(t, uint64(0), ecat.ReadBits(buf, s.Byte, s.Bit, s.BitLen))
}
