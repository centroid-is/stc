package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
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
