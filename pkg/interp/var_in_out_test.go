package interp

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// setupInOut parses src, registers its TYPE and FUNCTION_BLOCK declarations on
// a fresh interpreter, and returns the interpreter together with the first
// PROGRAM declaration found.
func setupInOut(t *testing.T, src string) (*Interpreter, *ast.ProgramDecl) {
	t.Helper()
	result := parser.Parse("var_in_out_test.st", src)
	for _, d := range result.Diags {
		if d.Severity == 0 { // error
			t.Fatalf("parse error: %s", d.Message)
		}
	}

	interp := New()
	interp.TypeDecls = map[string]ast.TypeSpec{}
	interp.FBDecls = map[string]*ast.FunctionBlockDecl{}
	var prog *ast.ProgramDecl
	for _, decl := range result.File.Declarations {
		switch d := decl.(type) {
		case *ast.TypeDecl:
			if d.Name != nil {
				interp.TypeDecls[strings.ToUpper(d.Name.Name)] = d.Type
			}
		case *ast.FunctionBlockDecl:
			if d.Name != nil {
				interp.FBDecls[strings.ToUpper(d.Name.Name)] = d
			}
		case *ast.ProgramDecl:
			if prog == nil {
				prog = d
			}
		}
	}
	if prog == nil {
		t.Fatal("no PROGRAM declaration found")
	}
	return interp, prog
}

// runInOutProgram builds an env from the program's VAR blocks (instantiating
// user-defined FBs) and executes the program body once.
func runInOutProgram(t *testing.T, interp *Interpreter, prog *ast.ProgramDecl) *Env {
	t.Helper()
	env := NewEnv(nil)
	resolve := interp.TypeResolverFunc()
	for _, vb := range prog.VarBlocks {
		for _, vd := range vb.Declarations {
			typeName := typeNameFromSpec(vd.Type)
			if fbDecl, ok := interp.FBDecls[strings.ToUpper(typeName)]; ok {
				for _, n := range vd.Names {
					inst := NewUserFBInstance(typeName, fbDecl, interp, env)
					env.Define(n.Name, Value{Kind: ValFBInstance, FBRef: inst})
				}
				continue
			}
			val := ZeroFromTypeSpecWith(vd.Type, resolve)
			if vd.InitValue != nil {
				iv, err := interp.evalExpr(env, vd.InitValue)
				if err != nil {
					t.Fatalf("init value for %v: %v", vd.Names, err)
				}
				val = iv
			}
			for _, n := range vd.Names {
				env.Define(n.Name, val.Clone())
			}
		}
	}
	if err := interp.execStatements(env, prog.Body); err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	return env
}

func TestVarInOut_ScalarWritesBack(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Scalar
VAR_IN_OUT
    n : DINT;
END_VAR
    n := n + 1;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Scalar;
    n : DINT := 5;
END_VAR
    fb(n := n);
    fb(n := n);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	v, ok := env.Get("n")
	if !ok {
		t.Fatal("n not found in env")
	}
	if v.Int != 7 {
		t.Fatalf("expected n=7 after two calls, got %d", v.Int)
	}
}

func TestVarInOut_StructWritesBack(t *testing.T) {
	src := `
TYPE ST_IO :
STRUCT
    cmd : BOOL;
    fbk : DINT;
END_STRUCT;
END_TYPE

FUNCTION_BLOCK FB_Struct
VAR_IN_OUT
    io : ST_IO;
END_VAR
    IF io.cmd THEN
        io.fbk := io.fbk + 1;
        io.cmd := FALSE;
    END_IF;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Struct;
    io : ST_IO;
END_VAR
    io.cmd := TRUE;
    fb(io := io);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	v, ok := env.Get("io")
	if !ok {
		t.Fatal("io not found in env")
	}
	if v.Kind != ValStruct {
		t.Fatalf("expected io to be a struct, got %s", v.Kind)
	}
	if v.Struct["CMD"].Bool {
		t.Fatal("expected io.cmd to be FALSE after the call")
	}
	if v.Struct["FBK"].Int != 1 {
		t.Fatalf("expected io.fbk=1 after the call, got %d", v.Struct["FBK"].Int)
	}
}

// The caller's struct must not stay aliased to the FB env after write-back:
// the copy is a copy, so a later call that does not run its body leaves the
// caller's value alone.
func TestVarInOut_StructWriteBackIsACopy(t *testing.T) {
	src := `
TYPE ST_IO :
STRUCT
    cmd : BOOL;
    fbk : DINT;
END_STRUCT;
END_TYPE

FUNCTION_BLOCK FB_Struct
VAR_IN_OUT
    io : ST_IO;
END_VAR
    io.fbk := io.fbk + 1;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Struct;
    a : ST_IO;
    b : ST_IO;
END_VAR
    fb(io := a);
    fb(io := b);
    fb(io := b);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	a, _ := env.Get("a")
	if a.Struct["FBK"].Int != 1 {
		t.Fatalf("expected a.fbk=1 (one call), got %d", a.Struct["FBK"].Int)
	}
	b, _ := env.Get("b")
	if b.Struct["FBK"].Int != 2 {
		t.Fatalf("expected b.fbk=2 (two calls), got %d", b.Struct["FBK"].Int)
	}
}

func TestVarInOut_ArrayElementArgWritesBack(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Scalar
VAR_IN_OUT
    n : DINT;
END_VAR
    n := n + 3;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Scalar;
    a : ARRAY[0..3] OF DINT;
END_VAR
    a[1] := 1;
    fb(n := a[1]);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	v, ok := env.Get("a")
	if !ok || v.Kind != ValArray {
		t.Fatalf("expected a to be an array, got %v", v.Kind)
	}
	if v.Array[1].Int != 4 {
		t.Fatalf("expected a[1]=4, got %d", v.Array[1].Int)
	}
	if v.Array[0].Int != 0 || v.Array[2].Int != 0 {
		t.Fatalf("write-back touched the wrong element: %v", v.Array)
	}
}

func TestVarInOut_StructMemberArgWritesBack(t *testing.T) {
	src := `
TYPE ST_OUTER :
STRUCT
    count : DINT;
    other : DINT;
END_STRUCT;
END_TYPE

FUNCTION_BLOCK FB_Scalar
VAR_IN_OUT
    n : DINT;
END_VAR
    n := n + 10;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Scalar;
    o : ST_OUTER;
END_VAR
    o.count := 1;
    fb(n := o.count);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	v, _ := env.Get("o")
	if v.Struct["COUNT"].Int != 11 {
		t.Fatalf("expected o.count=11, got %d", v.Struct["COUNT"].Int)
	}
	if v.Struct["OTHER"].Int != 0 {
		t.Fatalf("expected o.other untouched, got %d", v.Struct["OTHER"].Int)
	}
}

// A non-assignable argument -- here a literal -- has nowhere to write back to.
// It must be skipped silently rather than aborting the scan.
func TestVarInOut_LiteralArgIsIgnored(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Scalar
VAR_IN_OUT
    n : DINT;
END_VAR
    n := n + 1;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Scalar;
    r : DINT;
END_VAR
    fb(n := 5);
    r := 42;
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	if v, _ := env.Get("r"); v.Int != 42 {
		t.Fatalf("expected r=42, got %d", v.Int)
	}
}

// A computed expression is equally non-assignable and must not disturb the
// operands it was built from.
func TestVarInOut_ExpressionArgIsIgnored(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Scalar
VAR_IN_OUT
    n : DINT;
END_VAR
    n := n + 1;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Scalar;
    a : DINT := 2;
    b : DINT := 3;
END_VAR
    fb(n := a + b);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	if v, _ := env.Get("a"); v.Int != 2 {
		t.Fatalf("expected a=2, got %d", v.Int)
	}
	if v, _ := env.Get("b"); v.Int != 3 {
		t.Fatalf("expected b=3, got %d", v.Int)
	}
}

// VAR_INPUT and VAR_OUTPUT parameters keep their existing behaviour: only
// VAR_IN_OUT names are written back into the caller's argument expression.
func TestVarInOut_InputArgNotWrittenBack(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Mixed
VAR_INPUT
    a : DINT;
END_VAR
VAR_IN_OUT
    b : DINT;
END_VAR
VAR_OUTPUT
    c : DINT;
END_VAR
    a := a + 100;
    b := b + 1;
    c := a + b;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Mixed;
    x : DINT := 1;
    y : DINT := 2;
END_VAR
    fb(a := x, b := y);
END_PROGRAM
`
	interp, prog := setupInOut(t, src)
	env := runInOutProgram(t, interp, prog)

	if v, _ := env.Get("x"); v.Int != 1 {
		t.Fatalf("VAR_INPUT arg must not be written back: expected x=1, got %d", v.Int)
	}
	if v, _ := env.Get("y"); v.Int != 3 {
		t.Fatalf("expected y=3, got %d", v.Int)
	}
}

func TestFBInstance_IsInOutAndGetInOut(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Mixed
VAR_INPUT
    a : DINT;
END_VAR
VAR_IN_OUT
    b : DINT;
END_VAR
VAR_OUTPUT
    c : DINT;
END_VAR
    c := a + b;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    fb : FB_Mixed;
END_VAR
END_PROGRAM
`
	interp, _ := setupInOut(t, src)
	inst := NewUserFBInstance("FB_Mixed", interp.FBDecls["FB_MIXED"], interp, nil)

	if !inst.IsInOut("b") || !inst.IsInOut("B") {
		t.Fatal("expected b to be recognised as VAR_IN_OUT, case-insensitively")
	}
	if inst.IsInOut("a") || inst.IsInOut("c") {
		t.Fatal("only VAR_IN_OUT names should report true")
	}
	if _, ok := inst.GetInOut("a"); ok {
		t.Fatal("GetInOut should reject a non-VAR_IN_OUT name")
	}
	if _, ok := inst.GetInOut("b"); !ok {
		t.Fatal("GetInOut should resolve a VAR_IN_OUT name")
	}

	// Stdlib FBs have no env and no VAR_IN_OUT parameters.
	stdlib := &FBInstance{TypeName: "TON"}
	if stdlib.IsInOut("IN") {
		t.Fatal("a stdlib FB has no VAR_IN_OUT parameters")
	}
	if _, ok := stdlib.GetInOut("IN"); ok {
		t.Fatal("GetInOut should return false for an instance without an env")
	}
}
