package scenario

import (
	"fmt"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stLocals are the locals of every ST snippet run by runST.
const stLocals = "VAR n : DINT; b : BOOL; r : REAL; s : STRING; ok : BOOL; END_VAR\n"

// runST runs body as statements on p's interpreter in a fresh env whose
// locals are stLocals, returning the env and the first error.
func runST(t *testing.T, p *Plant, body string) (*interp.Env, error) {
	t.Helper()
	f := parseST(t, "snippet.st", "PROGRAM Snippet\n"+stLocals+body+"\nEND_PROGRAM\n")
	prog := f.Declarations[0].(*ast.ProgramDecl)
	in := p.Runtime().Interpreter()
	env := interp.NewEnv(in.GlobalParent())
	in.SetTestEnv(env)
	env.Define("N", interp.IntValue(0))
	env.Define("B", interp.BoolValue(false))
	env.Define("R", interp.RealValue(0))
	env.Define("S", interp.StringValue(""))
	env.Define("OK", interp.BoolValue(false))
	return env, in.ExecStatements(env, prog.Body)
}

func local(t *testing.T, env *interp.Env, name string) interp.Value {
	t.Helper()
	v, ok := env.Get(name)
	require.True(t, ok, name)
	return v
}

func builtinPlant(t *testing.T) (*Plant, *Session) {
	t.Helper()
	p := fixturePlant(t)
	return p, RegisterBuiltins(p.Runtime().Interpreter(), p)
}

func TestBuiltinsSetGetRun(t *testing.T) {
	p, _ := builtinPlant(t)
	env, err := runST(t, p, `
n := 7;
SET('ECT.A1_01.I1', TRUE);
RUN_CYCLES(1);
b := GET('ECT.A1_01.I1');
n := n + 1;
ok := GET('ECT.A1_02.O1');
r := GET('ECT_Diag.rSetpoint');`)
	require.NoError(t, err)
	assert.True(t, local(t, env, "B").Bool, "forced input reaches the linked variable")
	assert.EqualValues(t, 8, local(t, env, "N").Int, "locals survive RUN_CYCLES")
	assert.True(t, local(t, env, "OK").Bool, "interlock output after one scan")
	assert.Equal(t, p.BaseTick(), p.Clock())
	assert.Equal(t, p.Clock(), p.Project().Clock())
}

func TestBuiltinsTripAndSlaveState(t *testing.T) {
	p, _ := builtinPlant(t)
	env, err := runST(t, p, `
RUN_CYCLES(19);
ok := GET('ECT_Diag.Device_1_Diag[2].p_stat_bOk');
SIM_TRIP('DEMO.A1.03 (EL9222-5500)', 1);
RUN_CYCLES(1);
b := GET('ECT.A1_03.p_stat_Enabled');
SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 'not_present');
RUN_CYCLES(20);
s := 'x';
IF NOT GET('ECT_Diag.Device_1_Diag[2].p_stat_bOk') THEN s := 'pulled'; END_IF`)
	require.NoError(t, err)
	assert.True(t, local(t, env, "OK").Bool, "healthy before the pull")
	assert.False(t, local(t, env, "B").Bool, "tripped channel disabled")
	assert.Equal(t, "pulled", local(t, env, "S").Str)

	_, err = runST(t, p, `SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 8); RUN_CYCLES(1);`)
	require.NoError(t, err)
	st, _ := p.Network().SlaveState("Device 1 (EtherCAT)", 1)
	assert.EqualValues(t, 8, st, "integer state code")
}

func TestBuiltinsDriveAnalogSerialLink(t *testing.T) {
	p, _ := builtinPlant(t)
	env, err := runST(t, p, `
RUN_CYCLES(30);
SIM_DRIVE_FAULT('DEMO.CN01.FD01 (ATV320 EtherCAT)', 16);
RUN_CYCLES(10);
b := GET('MAIN.xDriveFault');
SIM_DRIVE_FAULT('DEMO.CN01.FD01 (ATV320 EtherCAT)', 0);`)
	require.NoError(t, err)
	assert.True(t, local(t, env, "B").Bool)

	link := lookupLink(t, p, "ECT.A1_01.I2")
	_, err = runST(t, p, fmt.Sprintf(`SIM_SET_LINK('%s', TRUE); RUN_CYCLES(1);`, link))
	require.NoError(t, err)
	env, err = runST(t, p, fmt.Sprintf(`b := GET('%s'); ok := GET('ECT.A1_01.I2');`, link))
	require.NoError(t, err)
	assert.True(t, local(t, env, "B").Bool, "GET of a link path reads the slot")
	assert.True(t, local(t, env, "OK").Bool)

	// No analog or serial terminal on Device 1/2: the model check names it.
	_, err = runST(t, p, `SIM_ANALOG('DEMO.A1.01 (EL1008)', 1, 5.0, 'V');`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SIM_ANALOG")
	_, err = runST(t, p, `SIM_SERIAL_PEER('DEMO.A1.01 (EL1008)', 'loopback');`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EL6001")
}

// lookupLink is the TcLinkTo link path of an input variable.
func lookupLink(t *testing.T, p *Plant, path string) string {
	t.Helper()
	bd, ok := p.IOBinder().InputBinding(path)
	require.True(t, ok, path)
	return bd.Var.Link
}

func TestBuiltinsRamp(t *testing.T) {
	p, _ := builtinPlant(t)
	env, err := runST(t, p, `
SIM_RAMP('ECT_Diag.rSetpoint', 0.0, 10.0, T#50ms);
RUN_CYCLES(3);
r := GET('ECT_Diag.rSetpoint');`)
	require.NoError(t, err)
	assert.InDelta(t, 4.0, local(t, env, "R").Real, 1e-9, "value written before the third tick (20 ms)")
	env, err = runST(t, p, `RUN_CYCLES(3); r := GET('ECT_Diag.rSetpoint');`)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, local(t, env, "R").Real, 1e-9, "reaches To once 50 ms elapsed")

	// SET cancels a running ramp on the same path.
	env, err = runST(t, p, `
SIM_RAMP('ECT_Diag.rSetpoint', 0.0, 100.0, T#1s);
RUN_CYCLES(2);
SET('ECT_Diag.rSetpoint', 1.5);
RUN_CYCLES(2);
r := GET('ECT_Diag.rSetpoint');`)
	require.NoError(t, err)
	assert.InDelta(t, 1.5, local(t, env, "R").Real, 1e-9)

	// A ramp that fails while running stops RUN_CYCLES.
	_, err = runST(t, p, `SIM_RAMP('ECT_Diag.rSetpoint', 0.0, 1.0E40, T#20ms); RUN_CYCLES(5);`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ramp of ECT_Diag.rSetpoint")
}

func TestBuiltinsFromScalar(t *testing.T) {
	assert.Equal(t, interp.BoolValue(true), fromScalar(true))
	assert.Equal(t, interp.RealValue(1.5), fromScalar(1.5))
	assert.Equal(t, interp.IntValue(-3), fromScalar(int64(-3)))
}

func TestBuiltinsAdvanceTime(t *testing.T) {
	p, _ := builtinPlant(t)
	_, err := runST(t, p, `ADVANCE_TIME(T#50ms);`)
	require.NoError(t, err)
	assert.Equal(t, 5*p.BaseTick(), p.Clock())
	_, err = runST(t, p, `ADVANCE_TIME(T#15ms);`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple of the base tick")
	_, err = runST(t, p, `ADVANCE_TIME(5);`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADVANCE_TIME(d : TIME)")
}

func TestBuiltinsGetEnumAsString(t *testing.T) {
	src := interp.ProjectSpec{Files: []*ast.SourceFile{parseST(t, "main.st", `TYPE E_Mode : (Idle, Run); END_TYPE
PROGRAM MAIN
VAR mode : E_Mode := E_Mode.Run; n : INT; END_VAR
n := n + 1;
END_PROGRAM
`)}}
	spec, err := BuildPlantSpec(src, nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	RegisterBuiltins(p.Runtime().Interpreter(), p)
	env, err := runST(t, p, `s := GET('MAIN.mode'); RUN_CYCLES(2); n := GET('MAIN.n');`)
	require.NoError(t, err)
	assert.Contains(t, local(t, env, "S").Str, "Run")
	assert.EqualValues(t, 2, local(t, env, "N").Int)

	_, err = runST(t, p, `SIM_TRIP('X', 1);`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no --io network loaded")
	_, err = runST(t, p, `SIM_RAMP('NOPE.x', 0.0, 1.0, T#1s);`)
	require.Error(t, err)
	_, err = runST(t, p, `SIM_RAMP('MAIN.n', 0.0, 1.0, T#25h);`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestBuiltinsErrors(t *testing.T) {
	p, s := builtinPlant(t)
	cases := []struct{ body, want string }{
		{`SET('ECT.A1_01.I1');`, "SET: got 1 argument(s) (signature SET(path : STRING, value : ANY))"},
		{`SET(1, TRUE);`, "argument 1 must be a STRING"},
		{`SIM_SET_LINK(1, TRUE);`, "argument 1 must be a STRING"},
		{`b := GET('ECT.Nope');`, "ECT.Nope"},
		{`b := GET(3);`, "GET: argument 1 must be a STRING"},
		{`b := GET('TIID^nope');`, "GET"},
		{`SIM_TRIP('DEMO.A1.03 (EL9222-5500)', 'one');`, "argument 2 must be an integer"},
		{`SIM_TRIP(1, 1);`, "argument 1 must be a STRING"},
		{`SIM_TRIP('DEMO.A1.03 (EL9222-5500)', 0);`, "channel 0 out of range"},
		{`SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 1.5);`, "a STRING preset or an integer state"},
		{`SIM_SLAVE_STATE(1, 8);`, "argument 1 must be a STRING"},
		{`SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 'weird');`, "unknown slave state"},
		{`SIM_ANALOG('DEMO.A1.01 (EL1008)', 1, 'x');`, "argument 3 must be a number"},
		{`SIM_ANALOG('DEMO.A1.01 (EL1008)', 1, 2, 3);`, "argument 4 must be a STRING"},
		{`SIM_ANALOG(1, 1, 2);`, "argument 1 must be a STRING"},
		{`SIM_DRIVE_FAULT(1, 2);`, "argument 1 must be a STRING"},
		{`SIM_DRIVE_FAULT('DEMO.CN01.FD01 (ATV320 EtherCAT)', TRUE);`, "argument 2 must be an integer"},
		{`SIM_SERIAL_PEER(1, 'x');`, "argument 1 must be a STRING"},
		{`SIM_SERIAL_PEER('x', 2);`, "argument 2 must be a STRING"},
		{`SIM_RAMP(1, 0.0, 1.0, T#1s);`, "argument 1 must be a STRING"},
		{`SIM_RAMP('ECT_Diag.rSetpoint', 'a', 1.0, T#1s);`, "argument 2 must be a number"},
		{`SIM_RAMP('ECT_Diag.rSetpoint', 0, 'b', T#1s);`, "argument 3 must be a number"},
		{`SIM_RAMP('ECT_Diag.rSetpoint', 0, 1, 5);`, "argument 4 must be a TIME"},
		{`RUN_CYCLES(-1);`, "out of range 0..10000000"},
		{`RUN_CYCLES(10000001);`, "out of range 0..10000000"},
		{`RUN_CYCLES(T#1s);`, "argument 1 must be an integer"},
	}
	for _, tc := range cases {
		_, err := runST(t, p, tc.body)
		if assert.Error(t, err, tc.body) {
			assert.Contains(t, err.Error(), tc.want, tc.body)
		}
	}
	assert.NoError(t, s.RunCycles(0))
	assert.Zero(t, p.Clock(), "no failing call ran a tick")
}

func TestBuiltinsTickError(t *testing.T) {
	src := interp.ProjectSpec{Files: []*ast.SourceFile{parseST(t, "main.st",
		"PROGRAM MAIN\nVAR a : ARRAY[1..2] OF INT; i : INT := 1; END_VAR\ni := i + 1;\na[i] := 1;\nEND_PROGRAM\n")}}
	spec, err := BuildPlantSpec(src, nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	s := RegisterBuiltins(p.Runtime().Interpreter(), p)
	err = s.RunCycles(3)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "cycle 2:"), err.Error())
}

// TestBuiltinsAnalogDefaultUnit: SIM_ANALOG without a unit writes the raw
// count, with 'mA' it scales through the terminal model.
func TestBuiltinsAnalogDefaultUnit(t *testing.T) {
	p := mainOnly(t, "Demo Analog.xml")
	val := "TIID^Device 1 (EtherCAT)^DEMO.A2.00 (EK1100)^DEMO.A2.01 (EL3054)^AI Standard Channel 1^Value"
	RegisterBuiltins(p.Runtime().Interpreter(), p)
	_, err := runST(t, p, `SIM_ANALOG('DEMO.A2.01 (EL3054)', 1, 1234.0); RUN_CYCLES(1);`)
	require.NoError(t, err)
	assert.EqualValues(t, 1234, readVal(t, p, val))
	_, err = runST(t, p, `SIM_ANALOG('DEMO.A2.01 (EL3054)', 1, 12.0, 'mA'); RUN_CYCLES(1);`)
	require.NoError(t, err)
	assert.EqualValues(t, 16384, readVal(t, p, val))
}


// The built-ins resolve only in the test env and refuse to run while a
// scan holds the Runtime mutex, instead of deadlocking (review 2 HI-01).
func TestBuiltinsRefuseInsideScan(t *testing.T) {
	src := "PROGRAM MAIN\nVAR y : INT; END_VAR\nPROBE();\nEND_PROGRAM\n"
	f := parseST(t, "main.st", src)
	spec, err := BuildPlantSpec(interp.ProjectSpec{Files: []*ast.SourceFile{f}}, nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	in := p.Runtime().Interpreter()
	s := RegisterBuiltins(in, p)
	var probeErr error
	in.RegisterFunction("PROBE", func(args []interp.Value, pos ast.Pos) (interp.Value, error) {
		require.True(t, p.Runtime().InTick())
		_, probeErr = s.call(builtins[1], []interp.Value{interp.StringValue("MAIN.y")}, pos)
		return interp.BoolValue(true), nil
	})
	require.NoError(t, p.Tick())
	require.Error(t, probeErr)
	require.Contains(t, probeErr.Error(), "cannot run inside a scan")
	require.False(t, p.Runtime().InTick())

	// Outside the test env the built-ins are unknown.
	env := interp.NewEnv(in.GlobalParent())
	body := parseST(t, "b.st", "PROGRAM B\nRUN_CYCLES(1);\nEND_PROGRAM\n").Declarations[0].(*ast.ProgramDecl).Body
	err = in.ExecStatements(env, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined function: RUN_CYCLES")
	in.SetTestEnv(env)
	require.NoError(t, in.ExecStatements(env, body))
}
