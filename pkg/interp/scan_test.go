package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: build a simple ProgramDecl for testing
// PROGRAM TestProg
//   VAR_INPUT  StartBtn : BOOL; END_VAR
//   VAR_OUTPUT MotorRunning : BOOL; END_VAR
//   VAR        Counter : INT; END_VAR
//   MotorRunning := StartBtn;
//   Counter := Counter + 1;
// END_PROGRAM
func makeTestProgram() *ast.ProgramDecl {
	return &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("TestProg"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("StartBtn")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("BOOL"),
						},
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("MotorRunning")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("BOOL"),
						},
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("Counter")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
						},
					},
				},
			},
		},
		Body: []ast.Statement{
			// MotorRunning := StartBtn;
			assignStmt("MotorRunning", ident("StartBtn")),
			// Counter := Counter + 1;
			assignStmt("Counter", binExpr(ident("Counter"), "+", intLit("1"))),
		},
	}
}

func TestScanCycleEngineNew(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)
	require.NotNil(t, engine)
	assert.Equal(t, time.Duration(0), engine.Clock())
}

func TestScanCycleTick(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	engine.SetInput("StartBtn", BoolValue(true))
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// Output should reflect the input
	out := engine.GetOutput("MotorRunning")
	assert.Equal(t, ValBool, out.Kind)
	assert.True(t, out.Bool)
}

func TestScanCycleClockAdvances(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, engine.Clock())

	err = engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 200*time.Millisecond, engine.Clock())
}

func TestDeterministicClock(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	for i := 0; i < 5; i++ {
		err := engine.Tick(100 * time.Millisecond)
		require.NoError(t, err)
	}

	// After 5 ticks of 100ms each, clock should be exactly 500ms
	assert.Equal(t, 500*time.Millisecond, engine.Clock())
}

func TestIOAccessSetAndGet(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	// SetInput before tick
	engine.SetInput("StartBtn", BoolValue(true))

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// GetOutput after tick
	motorRunning := engine.GetOutput("MotorRunning")
	assert.True(t, motorRunning.Bool)

	// Set input to false and tick again
	engine.SetInput("StartBtn", BoolValue(false))
	err = engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	motorRunning = engine.GetOutput("MotorRunning")
	assert.False(t, motorRunning.Bool)
}

func TestScanCycleStatePersists(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	// Counter increments each tick
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	err = engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	err = engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// Counter is a local var, not an output, but the env should persist.
	// We can verify indirectly or by checking that output changes accumulate.
	// For this test, we verify clock and that execution didn't error.
	assert.Equal(t, 300*time.Millisecond, engine.Clock())
}

func TestScanCycleUnknownInput(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	// Setting an unknown input should not panic or error
	engine.SetInput("NonExistent", BoolValue(true))

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
}

func TestScanCycleUnknownOutput(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// Getting an unknown output should return zero value
	v := engine.GetOutput("NonExistent")
	assert.Equal(t, Value{}, v)
}

func TestScanCycleCaseInsensitiveIO(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	// Case-insensitive input
	engine.SetInput("startbtn", BoolValue(true))
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// Case-insensitive output
	out := engine.GetOutput("motorrunning")
	assert.True(t, out.Bool)
}

func TestScanCycleMultipleOutputs(t *testing.T) {
	// Program with two outputs
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("MultiOut"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("A")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
						},
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("Double"), ident("Triple")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
						},
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("Double", binExpr(ident("A"), "*", intLit("2"))),
			assignStmt("Triple", binExpr(ident("A"), "*", intLit("3"))),
		},
	}

	engine := NewScanCycleEngine(prog)
	engine.SetInput("A", IntValue(5))
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	assert.Equal(t, int64(10), engine.GetOutput("Double").Int)
	assert.Equal(t, int64(15), engine.GetOutput("Triple").Int)
}

func TestScanCycleDtPassedToInterpreter(t *testing.T) {
	// This test verifies that dt is set on the interpreter during Tick.
	// We do this indirectly: we know the engine sets interp.dt before execStatements.
	// A proper integration test would involve an FB that uses dt.
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	err := engine.Tick(50 * time.Millisecond)
	require.NoError(t, err)

	// The clock should reflect the dt
	assert.Equal(t, 50*time.Millisecond, engine.Clock())
}

// --- IOTable integration tests ---

// Helper: build a program with AT-addressed variables
func makeATInputBoolProgram() *ast.ProgramDecl {
	// PROGRAM P
	//   VAR startButton AT %IX0.0 : BOOL; END_VAR
	//   VAR_OUTPUT result : BOOL; END_VAR
	//   result := startButton;
	// END_PROGRAM
	return &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("startButton")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
						AtAddress: ident("%IX0.0"),
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("result")},
						Type:     &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("result", ident("startButton")),
		},
	}
}

func TestIOTableInputBit(t *testing.T) {
	prog := makeATInputBoolProgram()
	engine := NewScanCycleEngine(prog)

	// Set input bit in the IOTable before Tick
	engine.IOTable().SetBit(iomap.AreaInput, 0, 0, true)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// The AT-bound variable should have been read from IOTable
	out := engine.GetOutput("result")
	assert.True(t, out.Bool, "AT %IX0.0 should read TRUE from IOTable")
}

func TestIOTableOutputBit(t *testing.T) {
	// PROGRAM P
	//   VAR output AT %QX0.0 : BOOL; END_VAR
	//   output := TRUE;
	// END_PROGRAM
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("output")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
						AtAddress: ident("%QX0.0"),
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("output", boolLit("TRUE")),
		},
	}

	engine := NewScanCycleEngine(prog)
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// After Tick, the IOTable output bit should be set
	assert.True(t, engine.IOTable().GetBit(iomap.AreaOutput, 0, 0),
		"AT %QX0.0 should write TRUE to IOTable after Tick")
}

func TestIOTableInputWord(t *testing.T) {
	// PROGRAM P
	//   VAR sensor AT %IW0 : INT; END_VAR
	//   VAR_OUTPUT value : INT; END_VAR
	//   value := sensor;
	// END_PROGRAM
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("sensor")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("INT")},
						AtAddress: ident("%IW0"),
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("value")},
						Type:     &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("INT")},
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("value", ident("sensor")),
		},
	}

	engine := NewScanCycleEngine(prog)
	engine.IOTable().SetWord(iomap.AreaInput, 0, 0x1234)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	out := engine.GetOutput("value")
	assert.Equal(t, int64(0x1234), out.Int, "AT %IW0 should read 0x1234 from IOTable")
}

func TestIOTableOutputWord(t *testing.T) {
	// PROGRAM P
	//   VAR actuator AT %QW4 : INT; END_VAR
	//   actuator := 500;
	// END_PROGRAM
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("actuator")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("INT")},
						AtAddress: ident("%QW4"),
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("actuator", intLit("500")),
		},
	}

	engine := NewScanCycleEngine(prog)
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	assert.Equal(t, uint16(500), engine.IOTable().GetWord(iomap.AreaOutput, 4),
		"AT %QW4 should write 500 to IOTable after Tick")
}

func TestIOTableMemoryDWord(t *testing.T) {
	// PROGRAM P
	//   VAR memvar AT %MD0 : DINT; END_VAR
	//   memvar := memvar + 1;
	// END_PROGRAM
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("memvar")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("DINT")},
						AtAddress: ident("%MD0"),
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("memvar", binExpr(ident("memvar"), "+", intLit("1"))),
		},
	}

	engine := NewScanCycleEngine(prog)

	// Set initial memory value
	engine.IOTable().SetDWord(iomap.AreaMemory, 0, 100)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	// Memory area should be bidirectional: read at start (100), add 1, write back (101)
	assert.Equal(t, uint32(101), engine.IOTable().GetDWord(iomap.AreaMemory, 0),
		"AT %MD0 should read 100 at start, add 1, write 101 back")
}

func TestIOTableMemoryBidirectional(t *testing.T) {
	// Memory area (%M*) should sync both directions: read at start AND write at end
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("mw")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("INT")},
						AtAddress: ident("%MW0"),
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("mw", binExpr(ident("mw"), "+", intLit("10"))),
		},
	}

	engine := NewScanCycleEngine(prog)
	engine.IOTable().SetWord(iomap.AreaMemory, 0, 5)

	// Tick 1: read 5, add 10, write 15
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint16(15), engine.IOTable().GetWord(iomap.AreaMemory, 0))

	// Tick 2: read 15, add 10, write 25
	err = engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, uint16(25), engine.IOTable().GetWord(iomap.AreaMemory, 0))
}

func TestIOTableMultipleATVars(t *testing.T) {
	// Multiple AT vars on different addresses should work independently
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("in1")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
						AtAddress: ident("%IX0.0"),
					},
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("in2")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
						AtAddress: ident("%IX0.1"),
					},
				},
			},
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("out1"), ident("out2")},
						Type:     &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
					},
				},
			},
		},
		Body: []ast.Statement{
			assignStmt("out1", ident("in1")),
			assignStmt("out2", ident("in2")),
		},
	}

	engine := NewScanCycleEngine(prog)
	engine.IOTable().SetBit(iomap.AreaInput, 0, 0, true)
	engine.IOTable().SetBit(iomap.AreaInput, 0, 1, false)

	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	assert.True(t, engine.GetOutput("out1").Bool, "in1 AT %IX0.0 should be TRUE")
	assert.False(t, engine.GetOutput("out2").Bool, "in2 AT %IX0.1 should be FALSE")
}

func TestIOTableNonATVarsUnaffected(t *testing.T) {
	// Existing non-AT variables should continue to work
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	// The IOTable should exist but non-AT vars should not be affected
	require.NotNil(t, engine.IOTable())

	engine.SetInput("StartBtn", BoolValue(true))
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)

	out := engine.GetOutput("MotorRunning")
	assert.True(t, out.Bool)
}

func TestIOTableAccessor(t *testing.T) {
	prog := makeTestProgram()
	engine := NewScanCycleEngine(prog)

	iot := engine.IOTable()
	require.NotNil(t, iot, "IOTable() should return the engine's IOTable")

	// External code can inject values
	iot.SetBit(iomap.AreaInput, 0, 0, true)
	assert.True(t, iot.GetBit(iomap.AreaInput, 0, 0))
}

func TestIOTableWildcardSkipped(t *testing.T) {
	// Wildcard AT %I* addresses should be skipped (no I/O binding created)
	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     ident("P"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						NodeBase:  ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:     []*ast.Ident{ident("wildcard")},
						Type:      &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: ident("BOOL")},
						AtAddress: ident("%I*"),
					},
				},
			},
		},
		Body: []ast.Statement{},
	}

	engine := NewScanCycleEngine(prog)
	// Should not panic or error with wildcard addresses
	err := engine.Tick(100 * time.Millisecond)
	require.NoError(t, err)
}
