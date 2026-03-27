package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock StandardFB for testing ---

// mockFB is a trivial StandardFB that accumulates elapsed time and echoes input.
type mockFB struct {
	inputs  map[string]Value
	outputs map[string]Value
	elapsed time.Duration
}

func newMockFB() *mockFB {
	return &mockFB{
		inputs:  make(map[string]Value),
		outputs: make(map[string]Value),
	}
}

func (m *mockFB) Execute(dt time.Duration) {
	m.elapsed += dt
	m.outputs["ET"] = TimeValue(m.elapsed)
	// Echo the IN input as Q output
	if in, ok := m.inputs["IN"]; ok {
		m.outputs["Q"] = in
	}
}

func (m *mockFB) SetInput(name string, v Value) {
	m.inputs[name] = v
}

func (m *mockFB) GetOutput(name string) Value {
	if v, ok := m.outputs[name]; ok {
		return v
	}
	return Value{}
}

func (m *mockFB) GetInput(name string) Value {
	if v, ok := m.inputs[name]; ok {
		return v
	}
	return Value{}
}

// --- Tests ---

func TestFBInstanceStdlib(t *testing.T) {
	fb := newMockFB()
	inst := &FBInstance{
		TypeName: "MockFB",
		FB:       fb,
	}

	// Set input
	inst.SetInput("IN", BoolValue(true))
	assert.True(t, inst.GetInput("IN").Bool)

	// Execute
	inst.Execute(100*time.Millisecond, nil)

	// Read output
	q := inst.GetOutput("Q")
	assert.Equal(t, ValBool, q.Kind)
	assert.True(t, q.Bool)

	// ET should be 100ms
	et := inst.GetOutput("ET")
	assert.Equal(t, 100*time.Millisecond, et.Time)
}

func TestFBInstanceStatePersists(t *testing.T) {
	fb := newMockFB()
	inst := &FBInstance{
		TypeName: "MockFB",
		FB:       fb,
	}

	inst.SetInput("IN", BoolValue(true))
	inst.Execute(100*time.Millisecond, nil)
	inst.Execute(100*time.Millisecond, nil)

	// ET should accumulate to 200ms
	et := inst.GetOutput("ET")
	assert.Equal(t, 200*time.Millisecond, et.Time)
}

func TestFBInstanceUserDefined(t *testing.T) {
	// Build a simple FB declaration:
	// FUNCTION_BLOCK MyCounter
	//   VAR_INPUT  Enable : BOOL; END_VAR
	//   VAR_OUTPUT Count  : INT;  END_VAR
	//   VAR        Internal : INT; END_VAR
	//   IF Enable THEN Internal := Internal + 1; END_IF
	//   Count := Internal;
	decl := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     ident("MyCounter"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("Enable")},
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
						Names:    []*ast.Ident{ident("Count")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
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
						Names:    []*ast.Ident{ident("Internal")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
						},
					},
				},
			},
		},
		Body: []ast.Statement{
			// IF Enable THEN Internal := Internal + 1; END_IF
			&ast.IfStmt{
				NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
				Condition: ident("Enable"),
				Then: []ast.Statement{
					assignStmt("Internal", binExpr(ident("Internal"), "+", intLit("1"))),
				},
			},
			// Count := Internal;
			assignStmt("Count", ident("Internal")),
		},
	}

	interp := New()
	inst := NewUserFBInstance("MyCounter", decl, interp, nil)
	require.NotNil(t, inst)

	// Set input and execute twice
	inst.SetInput("Enable", BoolValue(true))
	inst.Execute(100*time.Millisecond, interp)
	inst.Execute(100*time.Millisecond, interp)

	// Count should be 2 (state persists across calls)
	count := inst.GetOutput("Count")
	assert.Equal(t, ValInt, count.Kind)
	assert.Equal(t, int64(2), count.Int)
}

func TestUserFBInstanceDisabled(t *testing.T) {
	// Same FB as above, but Enable=false -> Count stays 0
	decl := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     ident("MyCounter"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("Enable")},
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
						Names:    []*ast.Ident{ident("Count")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
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
						Names:    []*ast.Ident{ident("Internal")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("INT"),
						},
					},
				},
			},
		},
		Body: []ast.Statement{
			&ast.IfStmt{
				NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
				Condition: ident("Enable"),
				Then: []ast.Statement{
					assignStmt("Internal", binExpr(ident("Internal"), "+", intLit("1"))),
				},
			},
			assignStmt("Count", ident("Internal")),
		},
	}

	interp := New()
	inst := NewUserFBInstance("MyCounter", decl, interp, nil)

	// Enable is false by default (zero value of BOOL)
	inst.Execute(100*time.Millisecond, interp)
	inst.Execute(100*time.Millisecond, interp)

	count := inst.GetOutput("Count")
	assert.Equal(t, int64(0), count.Int)
}

func TestFBInstanceMemberAccess(t *testing.T) {
	// Test that member access on an FB instance resolves to output/input values
	fb := newMockFB()
	inst := &FBInstance{
		TypeName: "MockFB",
		FB:       fb,
	}
	inst.SetInput("IN", BoolValue(true))
	inst.Execute(100*time.Millisecond, nil)

	// MemberAccess should resolve: first check outputs, then inputs
	q := inst.GetOutput("Q")
	assert.True(t, q.Bool)

	et := inst.GetOutput("ET")
	assert.Equal(t, 100*time.Millisecond, et.Time)
}

func TestFBCallStmt(t *testing.T) {
	// Test that CallStmt on an FB instance sets inputs, executes, and copies outputs.
	// Build a program that calls an FB:
	//   fbInst(IN := TRUE);
	//   result := fbInst.Q;
	interp := New()
	env := NewEnv(nil)

	fb := newMockFB()
	inst := &FBInstance{
		TypeName: "MockFB",
		FB:       fb,
	}

	// Define the FB instance in env
	env.Define("fbInst", Value{
		Kind:  ValFBInstance,
		FBRef: inst,
	})
	env.Define("result", BoolValue(false))

	// CallStmt: fbInst(IN := TRUE)
	callStmt := &ast.CallStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallStmt},
		Callee:   ident("fbInst"),
		Args: []*ast.CallArg{
			{
				Name:  ident("IN"),
				Value: boolLit("TRUE"),
			},
		},
	}

	err := interp.execStmt(env, callStmt)
	require.NoError(t, err)

	// Now access fbInst.Q via MemberAccessExpr
	memberExpr := &ast.MemberAccessExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindMemberAccessExpr},
		Object:   ident("fbInst"),
		Member:   ident("Q"),
	}
	v, err := interp.evalExpr(env, memberExpr)
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestFBMemberAccessInput(t *testing.T) {
	// Member access on FB instance should fall back to inputs if not found in outputs
	fb := newMockFB()
	inst := &FBInstance{
		TypeName: "MockFB",
		FB:       fb,
	}
	inst.SetInput("PT", TimeValue(500*time.Millisecond))

	// GetMember should find PT in inputs
	v := inst.GetMember("PT")
	assert.Equal(t, ValTime, v.Kind)
	assert.Equal(t, 500*time.Millisecond, v.Time)
}

func TestStdlibFBFactory(t *testing.T) {
	// Factory should be an empty map by default
	assert.NotNil(t, StdlibFBFactory)
	assert.Empty(t, StdlibFBFactory)
}

func TestUserFBInitializesVarsFromType(t *testing.T) {
	// Verify that vars are initialized with zero values based on their type name
	decl := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     ident("TestFB"),
		VarBlocks: []*ast.VarBlock{
			{
				NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
				Section:  ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
						Names:    []*ast.Ident{ident("TimeIn")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("TIME"),
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
						Names:    []*ast.Ident{ident("RealOut")},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     ident("REAL"),
						},
					},
				},
			},
		},
		Body: []ast.Statement{},
	}

	interp := New()
	inst := NewUserFBInstance("TestFB", decl, interp, nil)

	// TIME input should be zero duration
	timeIn := inst.GetInput("TimeIn")
	assert.Equal(t, ValTime, timeIn.Kind)
	assert.Equal(t, time.Duration(0), timeIn.Time)
	assert.Equal(t, types.KindTIME, timeIn.IECType)

	// REAL output should be zero float
	realOut := inst.GetOutput("RealOut")
	assert.Equal(t, ValReal, realOut.Kind)
	assert.Equal(t, 0.0, realOut.Real)
	assert.Equal(t, types.KindREAL, realOut.IECType)
}
