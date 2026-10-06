package interp

import (
	"os"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lookupI2 = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 2^Input"

func TestIOBinderLookupEngine(t *testing.T) {
	eng, _, _ := demoEngine(t)
	b := eng.ioBinder

	bd, ok := b.InputBinding("ect.a1_01.i1")
	require.True(t, ok)
	assert.Equal(t, "ECT.A1_01.I1", bd.Var.Path)
	assert.Equal(t, ecat.DirIn, bd.Slot.Dir)
	_, ok = b.InputBinding(" ECT.A1_02.O1 ")
	assert.False(t, ok, "output leaf is not an input binding")
	_, ok = b.InputBinding("ECT.Nope")
	assert.False(t, ok)

	outs := b.OutputBindings()
	require.NotEmpty(t, outs)
	for i := 1; i < len(outs); i++ {
		assert.LessOrEqual(t, outs[i-1].Var.Path, outs[i].Var.Path)
		assert.Equal(t, ecat.DirOut, outs[i].Slot.Dir)
	}
}

func TestIOBinderLookupDetached(t *testing.T) {
	bs := []ecat.Binding{
		{Var: ecat.LinkedVar{Path: "G.b"}, Slot: ecat.Slot{Dir: ecat.DirOut}},
		{Var: ecat.LinkedVar{Path: "G.B"}, Slot: ecat.Slot{Dir: ecat.DirOut}},
		{Var: ecat.LinkedVar{Path: "G.a"}, Slot: ecat.Slot{Dir: ecat.DirOut}},
		{Var: ecat.LinkedVar{Path: "G.In"}, Slot: ecat.Slot{Dir: ecat.DirIn}},
	}
	b := NewIOBinder(bs, nil)
	_, ok := b.InputBinding("g.in")
	assert.True(t, ok)
	outs := b.OutputBindings()
	require.Len(t, outs, 3)
	assert.Equal(t, []string{"G.a", "G.B", "G.b"}, []string{outs[0].Var.Path, outs[1].Var.Path, outs[2].Var.Path})
}

// demoRuntimeFiles parses the Beckhoff stubs, demo types, the ECT GVL and
// the given extra sources.
func demoRuntimeFiles(t *testing.T, extra ...string) (libs, files []*ast.SourceFile) {
	t.Helper()
	for _, n := range []string{"tc2_system.st", "tc2_ethercat.st"} {
		src, err := os.ReadFile(beckhoffStub(n))
		require.NoError(t, err)
		libs = append(libs, parseNoErr(t, n, string(src)))
	}
	for _, n := range []string{"demo_types.st", "demo_ect.st"} {
		src, err := os.ReadFile(ecatFixture(n))
		require.NoError(t, err)
		files = append(files, parseFixture(t, n, string(src)))
	}
	ast.SetGVLName(files[1], "ECT")
	for i, src := range extra {
		files = append(files, parseNoErr(t, "extra"+string(rune('A'+i))+".st", src))
	}
	return libs, files
}

func TestRuntimeNetworkScanOncePerTick(t *testing.T) {
	libs, files := demoRuntimeFiles(t, `
PROGRAM MAIN
VAR fb : FB_EcGetAllSlaveStates; END_VAR
END_PROGRAM
`, `
PROGRAM P2
VAR
	{attribute 'TcLinkTo' := '`+lookupI2+`'}
	x AT %I* : BOOL;
END_VAR
END_PROGRAM
`)
	topo, err := ecat.LoadProject(ecatFixture("Demo Device 1.xml"), ecatFixture("Demo Device 2.xml"))
	require.NoError(t, err)
	net := ecat.NewNetwork(topo, nil)
	rt, err := NewRuntime(files, RuntimeOpts{LibraryFiles: libs, Network: net})
	require.NoError(t, err)
	assert.Same(t, net, rt.Network())
	assert.Nil(t, rt.IOBinder())

	// FB_EcGetAllSlaveStates was instantiated as the Go mock.
	v, err := rt.Get("MAIN.fb")
	require.NoError(t, err)
	require.NotNil(t, v.FBRef)
	assert.NotNil(t, v.FBRef.FB, "mock StandardFB")

	all := append(append([]*ast.SourceFile{}, libs...), files...)
	vars, _ := ecat.CollectLinks(all)
	bindings, diags := ecat.Resolve(topo, vars)
	for _, d := range diags {
		require.NotEqual(t, "error", d.Severity.String(), d.Message)
	}
	b := NewIOBinder(bindings, net)
	rt.SetIOBinder(b)
	assert.Same(t, b, rt.IOBinder())
	require.NoError(t, net.ForceInput(lookupI2, 1))

	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Equal(t, uint64(1), rt.ecat.scan, "two programs, one services scan")
	assert.Empty(t, b.Errors())
	x, err := rt.Get("P2.x")
	require.NoError(t, err)
	assert.True(t, x.Bool, "program-local link resolved through the Runtime")
	_, ok := b.InputBinding("p2.X")
	assert.True(t, ok)

	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Equal(t, uint64(2), rt.ecat.scan)

	rt.SetIOBinder(nil)
	assert.Nil(t, rt.IOBinder())
	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Equal(t, uint64(3), rt.ecat.scan, "mocks step the network without a binder")
}

func TestRuntimeBinderUnknownProgram(t *testing.T) {
	libs, files := demoRuntimeFiles(t, "PROGRAM MAIN\nEND_PROGRAM\n")
	rt, err := NewRuntime(files, RuntimeOpts{LibraryFiles: libs})
	require.NoError(t, err)
	assert.Nil(t, rt.Network())
	bd := ecat.Binding{Var: ecat.LinkedVar{Path: "NOPE.x", Steps: []string{"NOPE", "X"}}, Slot: ecat.Slot{Master: "Device 1 (EtherCAT)"}}
	topo, err := ecat.LoadProject(ecatFixture("Demo Device 1.xml"))
	require.NoError(t, err)
	b := NewIOBinder([]ecat.Binding{bd}, ecat.NewNetwork(topo, nil))
	rt.SetIOBinder(b)
	require.NoError(t, rt.Tick(time.Millisecond))
	require.Len(t, b.Errors(), 1)
	assert.Contains(t, b.Errors()[0].Error(), "unknown GVL or program")
}

func TestEngineSetNetworkNilDetaches(t *testing.T) {
	eng, _, net := demoEngine(t)
	eng.SetNetwork(net)
	_, ok := eng.interp.LocalFunctions["F_CREATEAMSNETID"]
	assert.True(t, ok)
	eng.SetNetwork(nil)
	_, ok = eng.interp.LocalFunctions["F_CREATEAMSNETID"]
	assert.False(t, ok)
	assert.Nil(t, eng.interp.fbOverrides)
}

func TestProjectNetworkScanOncePerTick(t *testing.T) {
	libs, files := demoRuntimeFiles(t, `
PROGRAM MAIN
VAR fb : FB_EcGetAllSlaveStates; END_VAR
END_PROGRAM
`, `
PROGRAM P2
VAR
	{attribute 'TcLinkTo' := '`+lookupI2+`'}
	x AT %I* : BOOL;
END_VAR
END_PROGRAM
`)
	topo, err := ecat.LoadProject(ecatFixture("Demo Device 1.xml"), ecatFixture("Demo Device 2.xml"))
	require.NoError(t, err)
	net := ecat.NewNetwork(topo, nil)
	p, err := LoadProject(ProjectSpec{LibraryFiles: libs, Files: files, Network: net, Tasks: []TaskSpec{
		{Name: "Fast", Cycle: 10 * time.Millisecond, Programs: []string{"MAIN"}},
		{Name: "Slow", Cycle: 20 * time.Millisecond, Programs: []string{"P2"}},
	}})
	require.NoError(t, err)
	v, err := p.Runtime().Get("MAIN.fb")
	require.NoError(t, err)
	require.NotNil(t, v.FBRef)
	assert.NotNil(t, v.FBRef.FB, "Tc2_EtherCAT mock instantiated")

	require.NoError(t, p.Tick())
	assert.Equal(t, uint64(1), p.rt.ecat.scan, "two tasks due, one services scan")

	all := append(append([]*ast.SourceFile{}, libs...), files...)
	vars, _ := ecat.CollectLinks(all)
	bindings, _ := ecat.Resolve(topo, vars)
	b := NewIOBinder(bindings, net)
	p.SetIOBinder(b)
	require.NoError(t, net.ForceInput(lookupI2, 1))
	require.NoError(t, p.Tick())
	require.NoError(t, p.Tick())
	assert.Equal(t, uint64(3), p.rt.ecat.scan)
	assert.Empty(t, b.Errors())
	x, err := p.Runtime().Get("P2.x")
	require.NoError(t, err)
	assert.True(t, x.Bool, "link resolved through the project binder")
}
