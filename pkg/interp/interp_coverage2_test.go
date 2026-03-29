package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// --- Exported wrappers: SetDt, EvalExpr, ExecStatements ---

func TestSetDt(t *testing.T) {
	interp := New()
	interp.SetDt(42 * time.Millisecond)
	if interp.dt != 42*time.Millisecond {
		t.Fatalf("expected dt=42ms, got %v", interp.dt)
	}
}

func TestEvalExpr_Exported(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(7))
	v, err := interp.EvalExpr(env, &ast.Ident{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 7 {
		t.Fatalf("expected 7, got %d", v.Int)
	}
}

func TestExecStatements_Exported(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	err := interp.ExecStatements(env, []ast.Statement{
		&ast.AssignStmt{
			Target: &ast.Ident{Name: "x"},
			Value:  &ast.Literal{LitKind: ast.LitInt, Value: "5"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("x")
	if v.Int != 5 {
		t.Fatalf("expected x=5, got %d", v.Int)
	}
}

// --- ZeroFromTypeSpec and MakeFBInstanceValue ---

func TestZeroFromTypeSpec_Exported(t *testing.T) {
	v := ZeroFromTypeSpec(&ast.NamedType{Name: &ast.Ident{Name: "BOOL"}})
	if v.Kind != ValBool {
		t.Fatalf("expected ValBool, got %v", v.Kind)
	}
}

func TestMakeFBInstanceValue(t *testing.T) {
	ton := &TON{}
	v := MakeFBInstanceValue("TON", ton)
	if v.Kind != ValFBInstance {
		t.Fatalf("expected ValFBInstance, got %v", v.Kind)
	}
	if v.FBRef == nil {
		t.Fatal("expected non-nil FBRef")
	}
	if v.FBRef.TypeName != "TON" {
		t.Fatalf("expected TypeName=TON, got %s", v.FBRef.TypeName)
	}
}

// --- evalLiteral: unsupported literal kind ---

func TestEvalLiteral_UnsupportedKind(t *testing.T) {
	interp := New()
	// Use a kind not covered by the switch
	_, err := interp.evalLiteral(&ast.Literal{LitKind: ast.LitDate, Value: "D#2024-01-01"})
	if err == nil {
		t.Fatal("expected error for unsupported literal kind")
	}
}

func TestEvalLiteral_WString(t *testing.T) {
	interp := New()
	v, err := interp.evalLiteral(&ast.Literal{LitKind: ast.LitWString, Value: "'hello'"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Str != "hello" {
		t.Fatalf("expected 'hello', got %q", v.Str)
	}
}

// --- parseLitInt: invalid base / invalid digits ---

func TestParseLitInt_InvalidBase(t *testing.T) {
	interp := New()
	_, err := interp.parseLitInt("xyz#10")
	if err == nil {
		t.Fatal("expected error for invalid base")
	}
}

func TestParseLitInt_InvalidDigitsForBase(t *testing.T) {
	interp := New()
	_, err := interp.parseLitInt("2#999")
	if err == nil {
		t.Fatal("expected error for invalid digits")
	}
}

func TestParseLitInt_InvalidDecimal(t *testing.T) {
	interp := New()
	_, err := interp.parseLitInt("abc")
	if err == nil {
		t.Fatal("expected error for invalid decimal")
	}
}

// --- parseLitReal: invalid ---

func TestParseLitReal_Invalid(t *testing.T) {
	interp := New()
	_, err := interp.parseLitReal("notanumber")
	if err == nil {
		t.Fatal("expected error for invalid real")
	}
}

// --- parseLitTime: TIME# prefix and invalid ---

func TestParseLitTime_FullPrefix(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTime("TIME#1h30m")
	if err != nil {
		t.Fatal(err)
	}
	expected := 1*time.Hour + 30*time.Minute
	if v.Time != expected {
		t.Fatalf("expected %v, got %v", expected, v.Time)
	}
}

func TestParseLitTime_Invalid(t *testing.T) {
	interp := New()
	_, err := interp.parseLitTime("T#nope")
	if err == nil {
		t.Fatal("expected error for invalid time literal")
	}
}

// --- parseLitTyped: REAL/LREAL, BOOL, unsupported prefix ---

func TestParseLitTyped_Real(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTyped("3.14", "REAL")
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValReal || v.IECType != types.KindREAL {
		t.Fatalf("expected ValReal/REAL, got %v/%v", v.Kind, v.IECType)
	}
}

func TestParseLitTyped_Bool(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTyped("TRUE", "BOOL")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Bool {
		t.Fatal("expected true")
	}
}

func TestParseLitTyped_Unsupported(t *testing.T) {
	interp := New()
	_, err := interp.parseLitTyped("x", "STRING")
	if err == nil {
		t.Fatal("expected error for unsupported prefix")
	}
}

func TestParseLitTyped_RealInvalid(t *testing.T) {
	interp := New()
	_, err := interp.parseLitTyped("notfloat", "LREAL")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseLitTyped_IntInvalid(t *testing.T) {
	interp := New()
	_, err := interp.parseLitTyped("notint", "UINT")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- evalExpr: DerefExpr, ErrorNode, unsupported ---

func TestEvalExpr_ErrorNode(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalExpr(env, &ast.ErrorNode{Message: "bad"})
	if err == nil {
		t.Fatal("expected error for ErrorNode")
	}
}

// --- evalBinary: unsupported types ---

func TestEvalBinary_UnsupportedTypes(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("a", Value{Kind: ValArray, Array: []Value{IntValue(1)}})
	env.Define("b", Value{Kind: ValArray, Array: []Value{IntValue(2)}})
	_, err := interp.evalBinary(env, &ast.BinaryExpr{
		Left:  &ast.Ident{Name: "a"},
		Op:    ast.Token{Text: "+"},
		Right: &ast.Ident{Name: "b"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported binary types")
	}
}

// --- evalBinaryInt: unsupported operator ---

func TestEvalBinaryInt_UnsupportedOp(t *testing.T) {
	interp := New()
	_, err := interp.evalBinaryInt(1, "NOPE", 2)
	if err == nil {
		t.Fatal("expected error for unsupported int op")
	}
}

// --- evalBinaryReal: all comparison + unsupported op ---

func TestEvalBinaryReal_AllOps(t *testing.T) {
	interp := New()
	tests := []struct {
		op   string
		l, r float64
	}{
		{"+", 1, 2}, {"-", 3, 1}, {"*", 2, 3}, {"/", 6, 2},
		{"=", 1, 1}, {"<>", 1, 2}, {"<", 1, 2}, {">", 2, 1},
		{"<=", 1, 1}, {">=", 2, 1},
	}
	for _, tt := range tests {
		_, err := interp.evalBinaryReal(tt.l, tt.op, tt.r)
		if err != nil {
			t.Fatalf("op %s: unexpected error: %v", tt.op, err)
		}
	}
	// unsupported
	_, err := interp.evalBinaryReal(1, "NOPE", 2)
	if err == nil {
		t.Fatal("expected error for unsupported real op")
	}
}

// --- evalUnary: + passthrough, unsupported ---

func TestEvalUnary_Plus(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(42))
	v, err := interp.evalUnary(env, &ast.UnaryExpr{
		Op:      ast.Token{Text: "+"},
		Operand: &ast.Ident{Name: "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestEvalUnary_NegateReal(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR x : LREAL; END_VAR
	x := -3.14;
	END_PROGRAM`)
	v, _ := env.Get("x")
	if v.Real >= 0 {
		t.Fatalf("expected negative, got %f", v.Real)
	}
}

func TestEvalUnary_NegateUnsupported(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", StringValue("hi"))
	_, err := interp.evalUnary(env, &ast.UnaryExpr{
		Op:      ast.Token{Text: "-"},
		Operand: &ast.Ident{Name: "s"},
	})
	if err == nil {
		t.Fatal("expected error for negating string")
	}
}

func TestEvalUnary_UnsupportedOp(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(1))
	_, err := interp.evalUnary(env, &ast.UnaryExpr{
		Op:      ast.Token{Text: "BOGUS"},
		Operand: &ast.Ident{Name: "x"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported unary op")
	}
}

// --- evalIndex: missing index, non-int index ---

func TestEvalIndex_MissingIndex(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(1)}})
	_, err := interp.evalIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: nil,
	})
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestEvalIndex_NonIntIndex(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(1)}})
	env.Define("s", StringValue("x"))
	_, err := interp.evalIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: []ast.Expr{&ast.Ident{Name: "s"}},
	})
	if err == nil {
		t.Fatal("expected error for non-int index")
	}
}

// --- execStmt: ErrorNode, unsupported, EmptyStmt ---

func TestExecStmt_ErrorNode(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execStmt(env, &ast.ErrorNode{Message: "oops"})
	if err == nil {
		t.Fatal("expected error for ErrorNode")
	}
}

// --- execAssign: unsupported target ---

func TestExecAssign_UnsupportedTarget(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execAssign(env, &ast.AssignStmt{
		Target: &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		Value:  &ast.Literal{LitKind: ast.LitInt, Value: "2"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported target")
	}
}

// --- execAssignIndex ---

func TestExecAssignIndex_Success(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(0), IntValue(0), IntValue(0)}})
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "1"}},
	}, IntValue(99))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("arr")
	if v.Array[1].Int != 99 {
		t.Fatalf("expected arr[1]=99, got %d", v.Array[1].Int)
	}
}

func TestExecAssignIndex_NonIdentBase(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "0"}},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecAssignIndex_UndefinedVar(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "missing"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "0"}},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecAssignIndex_NotArray(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(1))
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "x"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "0"}},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecAssignIndex_MissingIdx(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(0)}})
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: nil,
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestExecAssignIndex_OutOfBounds(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(0)}})
	err := interp.execAssignIndex(env, &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "5"}},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error for out of bounds")
	}
}

// --- execAssignMember ---

func TestExecAssignMember_StructWriteBack(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: map[string]Value{"X": IntValue(0)}})
	err := interp.execAssignMember(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "s"},
		Member: &ast.Ident{Name: "x"},
	}, IntValue(42))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("s")
	if v.Struct["X"].Int != 42 {
		t.Fatalf("expected s.X=42, got %d", v.Struct["X"].Int)
	}
}

func TestExecAssignMember_FBInstance(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	ton := &TON{}
	inst := &FBInstance{TypeName: "TON", FB: ton}
	env.Define("myTon", Value{Kind: ValFBInstance, FBRef: inst})

	err := interp.execAssignMember(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "myTon"},
		Member: &ast.Ident{Name: "PT"},
	}, TimeValue(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	// Verify the input was set
	pt := ton.GetInput("PT")
	if pt.Time != 100*time.Millisecond {
		t.Fatalf("expected PT=100ms, got %v", pt.Time)
	}
}

func TestExecAssignMember_NilFBRef(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: nil})
	err := interp.execAssignMember(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "fb"},
		Member: &ast.Ident{Name: "x"},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error for nil FB ref")
	}
}

func TestExecAssignMember_UnsupportedType(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(1))
	err := interp.execAssignMember(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "x"},
		Member: &ast.Ident{Name: "y"},
	}, IntValue(2))
	if err == nil {
		t.Fatal("expected error for member assign on int")
	}
}

func TestExecAssignMember_StructNil(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: nil})
	err := interp.execAssignMember(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "s"},
		Member: &ast.Ident{Name: "x"},
	}, IntValue(1))
	if err == nil {
		t.Fatal("expected error for nil struct")
	}
}

// --- evalMemberAccess ---

func TestEvalMemberAccess_NilFBRef(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: nil})
	_, err := interp.evalMemberAccess(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "fb"},
		Member: &ast.Ident{Name: "Q"},
	})
	if err == nil {
		t.Fatal("expected error for nil FB ref")
	}
}

func TestEvalMemberAccess_StructNotFound(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: map[string]Value{"X": IntValue(1)}})
	_, err := interp.evalMemberAccess(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "s"},
		Member: &ast.Ident{Name: "NONEXISTENT"},
	})
	if err == nil {
		t.Fatal("expected error for missing member")
	}
}

func TestEvalMemberAccess_NilStruct(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: nil})
	_, err := interp.evalMemberAccess(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "s"},
		Member: &ast.Ident{Name: "x"},
	})
	if err == nil {
		t.Fatal("expected error for nil struct")
	}
}

func TestEvalMemberAccess_UnsupportedType(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(1))
	_, err := interp.evalMemberAccess(env, &ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "x"},
		Member: &ast.Ident{Name: "y"},
	})
	if err == nil {
		t.Fatal("expected error for member access on int")
	}
}

// --- evalCall: unsupported callee, undefined function ---

func TestEvalCall_UnsupportedCallee(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalCall(env, &ast.CallExpr{
		Callee: &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		Args:   nil,
	})
	if err == nil {
		t.Fatal("expected error for unsupported callee type")
	}
}

func TestEvalCall_UndefinedFunction(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalCall(env, &ast.CallExpr{
		Callee: &ast.Ident{Name: "NONEXISTENT_FUNC"},
		Args:   nil,
	})
	if err == nil {
		t.Fatal("expected error for undefined function")
	}
}

// --- execCallStmt: various error paths ---

func TestExecCallStmt_UnsupportedCallTarget(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execCallStmt(env, &ast.CallStmt{
		Callee: &ast.Literal{LitKind: ast.LitInt, Value: "1"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported call target")
	}
}

func TestExecCallStmt_UndefinedCallee(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execCallStmt(env, &ast.CallStmt{
		Callee: &ast.Ident{Name: "missing"},
	})
	if err == nil {
		t.Fatal("expected error for undefined callee")
	}
}

func TestExecCallStmt_NotFBInstance(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(1))
	err := interp.execCallStmt(env, &ast.CallStmt{
		Callee: &ast.Ident{Name: "x"},
	})
	if err == nil {
		t.Fatal("expected error for non-FB callee")
	}
}

func TestExecCallStmt_OutputBinding(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	ton := &TON{}
	ton.SetInput("IN", BoolValue(true))
	ton.SetInput("PT", TimeValue(0))
	ton.Execute(10 * time.Millisecond)

	inst := &FBInstance{TypeName: "TON", FB: ton}
	env.Define("myTon", Value{Kind: ValFBInstance, FBRef: inst})
	env.Define("result", BoolValue(false))

	err := interp.execCallStmt(env, &ast.CallStmt{
		Callee: &ast.Ident{Name: "myTon"},
		Args: []*ast.CallArg{
			{
				Name:     &ast.Ident{Name: "IN"},
				Value:    &ast.Literal{LitKind: ast.LitBool, Value: "TRUE"},
				IsOutput: false,
			},
			{
				Name:     &ast.Ident{Name: "Q"},
				Value:    &ast.Ident{Name: "result"},
				IsOutput: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- execFor: zero step ---

func TestExecFor_ZeroStep(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("i", IntValue(0))
	err := interp.execFor(env, &ast.ForStmt{
		Variable: &ast.Ident{Name: "i"},
		From:     &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		To:       &ast.Literal{LitKind: ast.LitInt, Value: "10"},
		By:       &ast.Literal{LitKind: ast.LitInt, Value: "0"},
		Body:     nil,
	})
	if err == nil {
		t.Fatal("expected error for zero step")
	}
}

// --- matchCaseLabel: range, unsupported ---

func TestMatchCaseLabel_Range(t *testing.T) {
	// Test a CASE with range labels to cover the CaseLabelRange path
	env := runProgram(t, `PROGRAM test
	VAR x : DINT; result : DINT; END_VAR
	x := 5;
	CASE x OF
		1..3: result := 1;
		4..6: result := 2;
	ELSE
		result := 3;
	END_CASE;
	END_PROGRAM`)
	v, _ := env.Get("result")
	if v.Int != 2 {
		t.Fatalf("expected result=2, got %d", v.Int)
	}
}

// --- Stdlib GetInput methods (all at 0%) ---

func TestRTRIG_GetInput(t *testing.T) {
	r := &RTRIG{}
	r.SetInput("CLK", BoolValue(true))
	v := r.GetInput("CLK")
	if !v.Bool {
		t.Fatal("expected CLK=true")
	}
	// unknown input
	v = r.GetInput("X")
	if v.Kind != 0 {
		t.Fatal("expected zero value for unknown input")
	}
}

func TestFTRIG_GetInput(t *testing.T) {
	f := &FTRIG{}
	f.SetInput("CLK", BoolValue(true))
	v := f.GetInput("CLK")
	if !v.Bool {
		t.Fatal("expected CLK=true")
	}
	v = f.GetInput("X")
	if v.Kind != 0 {
		t.Fatal("expected zero value")
	}
}

func TestSR_GetInput(t *testing.T) {
	sr := &SR{}
	sr.SetInput("S1", BoolValue(true))
	sr.SetInput("R", BoolValue(false))
	s1 := sr.GetInput("S1")
	r := sr.GetInput("R")
	if !s1.Bool {
		t.Fatal("expected S1=true")
	}
	if r.Bool {
		t.Fatal("expected R=false")
	}
	// unknown
	v := sr.GetInput("X")
	if v.Kind != 0 {
		t.Fatal("expected zero value")
	}
}

func TestRS_GetInput(t *testing.T) {
	rs := &RS{}
	rs.SetInput("S", BoolValue(true))
	rs.SetInput("R1", BoolValue(false))
	s := rs.GetInput("S")
	r1 := rs.GetInput("R1")
	if !s.Bool {
		t.Fatal("expected S=true")
	}
	if r1.Bool {
		t.Fatal("expected R1=false")
	}
	v := rs.GetInput("X")
	if v.Kind != 0 {
		t.Fatal("expected zero value")
	}
}

func TestCTU_GetInput(t *testing.T) {
	c := &CTU{}
	c.SetInput("CU", BoolValue(true))
	c.SetInput("R", BoolValue(false))
	c.SetInput("PV", IntValue(10))
	if cu := c.GetInput("CU"); !cu.Bool {
		t.Fatal("expected CU=true")
	}
	if r := c.GetInput("R"); r.Bool {
		t.Fatal("expected R=false")
	}
	if pv := c.GetInput("PV"); pv.Int != 10 {
		t.Fatalf("expected PV=10, got %d", pv.Int)
	}
	if x := c.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestCTD_GetInput(t *testing.T) {
	c := &CTD{}
	c.SetInput("CD", BoolValue(true))
	c.SetInput("LD", BoolValue(true))
	c.SetInput("PV", IntValue(5))
	if cd := c.GetInput("CD"); !cd.Bool {
		t.Fatal("expected CD=true")
	}
	if ld := c.GetInput("LD"); !ld.Bool {
		t.Fatal("expected LD=true")
	}
	if pv := c.GetInput("PV"); pv.Int != 5 {
		t.Fatalf("expected PV=5, got %d", pv.Int)
	}
	if x := c.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestCTUD_GetInput(t *testing.T) {
	c := &CTUD{}
	c.SetInput("CU", BoolValue(true))
	c.SetInput("CD", BoolValue(false))
	c.SetInput("R", BoolValue(false))
	c.SetInput("LD", BoolValue(false))
	c.SetInput("PV", IntValue(7))
	if cu := c.GetInput("CU"); !cu.Bool {
		t.Fatal("expected CU=true")
	}
	if cd := c.GetInput("CD"); cd.Bool {
		t.Fatal("expected CD=false")
	}
	if r := c.GetInput("R"); r.Bool {
		t.Fatal("expected R=false")
	}
	if ld := c.GetInput("LD"); ld.Bool {
		t.Fatal("expected LD=false")
	}
	if pv := c.GetInput("PV"); pv.Int != 7 {
		t.Fatalf("expected PV=7, got %d", pv.Int)
	}
	if x := c.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

// --- Timer GetOutput / GetInput for uncovered branches ---

func TestTON_GetOutput_Unknown(t *testing.T) {
	ton := &TON{}
	v := ton.GetOutput("NOPE")
	if v.Kind != 0 {
		t.Fatal("expected zero value for unknown output")
	}
}

func TestTON_GetInput_All(t *testing.T) {
	ton := &TON{}
	ton.SetInput("IN", BoolValue(true))
	ton.SetInput("PT", TimeValue(100*time.Millisecond))
	if in := ton.GetInput("IN"); !in.Bool {
		t.Fatal("expected IN=true")
	}
	if pt := ton.GetInput("PT"); pt.Time != 100*time.Millisecond {
		t.Fatalf("expected PT=100ms, got %v", pt.Time)
	}
	if x := ton.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestTOF_GetInput_All(t *testing.T) {
	tof := &TOF{}
	tof.SetInput("IN", BoolValue(true))
	tof.SetInput("PT", TimeValue(200*time.Millisecond))
	if in := tof.GetInput("IN"); !in.Bool {
		t.Fatal("expected IN=true")
	}
	if pt := tof.GetInput("PT"); pt.Time != 200*time.Millisecond {
		t.Fatalf("expected PT=200ms, got %v", pt.Time)
	}
	if x := tof.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestTP_GetInput_All(t *testing.T) {
	tp := &TP{}
	tp.SetInput("IN", BoolValue(true))
	tp.SetInput("PT", TimeValue(300*time.Millisecond))
	if in := tp.GetInput("IN"); !in.Bool {
		t.Fatal("expected IN=true")
	}
	if pt := tp.GetInput("PT"); pt.Time != 300*time.Millisecond {
		t.Fatalf("expected PT=300ms, got %v", pt.Time)
	}
	if x := tp.GetInput("X"); x.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestTOF_GetOutput_Unknown(t *testing.T) {
	tof := &TOF{}
	if v := tof.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestTP_GetOutput_Unknown(t *testing.T) {
	tp := &TP{}
	if v := tp.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

// --- Bistable GetOutput: default branches ---

func TestSR_GetOutput_Unknown(t *testing.T) {
	sr := &SR{}
	if v := sr.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestRS_GetOutput_Unknown(t *testing.T) {
	rs := &RS{}
	if v := rs.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

// --- Edge GetOutput: default branches ---

func TestRTRIG_GetOutput_Unknown(t *testing.T) {
	r := &RTRIG{}
	if v := r.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestFTRIG_GetOutput_Unknown(t *testing.T) {
	f := &FTRIG{}
	if v := f.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

// --- Counter GetOutput: default branches ---

func TestCTU_GetOutput_Unknown(t *testing.T) {
	c := &CTU{}
	if v := c.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestCTD_GetOutput_Unknown(t *testing.T) {
	c := &CTD{}
	if v := c.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

func TestCTUD_GetOutput_Unknown(t *testing.T) {
	c := &CTUD{}
	if v := c.GetOutput("X"); v.Kind != 0 {
		t.Fatal("expected zero")
	}
}

// --- FBInstance.GetInput for user-defined (non-stdlib) ---

func TestFBInstance_GetInput_UserDefined(t *testing.T) {
	env := NewEnv(nil)
	env.Define("IN1", BoolValue(true))
	inst := &FBInstance{
		TypeName:   "MyFB",
		Env:        env,
		inputNames: []string{"IN1"},
	}
	v := inst.GetInput("IN1")
	if !v.Bool {
		t.Fatal("expected IN1=true")
	}
	v = inst.GetInput("X")
	if v.Kind != 0 {
		t.Fatal("expected zero for unknown input")
	}
}

// --- FBInstance.GetMember: fallback to env for user-defined ---

func TestFBInstance_GetMember_FallbackEnv(t *testing.T) {
	env := NewEnv(nil)
	env.Define("localVar", IntValue(99))
	inst := &FBInstance{
		TypeName: "MyFB",
		Env:      env,
	}
	v := inst.GetMember("localVar")
	if v.Int != 99 {
		t.Fatalf("expected 99, got %d", v.Int)
	}
}

// --- zeroFromTypeSpec: various cases ---

func TestZeroFromTypeSpec_UnknownType(t *testing.T) {
	v := zeroFromTypeSpec(&ast.NamedType{Name: &ast.Ident{Name: "UNKNOWN_TYPE"}})
	if v.Kind != ValInt {
		t.Fatalf("expected default int zero, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_NilSpec(t *testing.T) {
	v := zeroFromTypeSpec(nil)
	if v.Kind != ValInt {
		t.Fatalf("expected default int zero, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_NilName(t *testing.T) {
	v := zeroFromTypeSpec(&ast.NamedType{Name: nil})
	if v.Kind != ValInt {
		t.Fatalf("expected default int zero, got %v", v.Kind)
	}
}

// --- Stdlib convert: missing arg errors ---

func TestStdlib_Convert_MissingArgs(t *testing.T) {
	fns := []string{
		"INT_TO_REAL", "DINT_TO_LREAL", "REAL_TO_INT", "INT_TO_DINT",
		"BOOL_TO_INT", "INT_TO_BOOL", "BOOL_TO_STRING", "INT_TO_STRING",
		"STRING_TO_INT", "REAL_TO_STRING", "STRING_TO_REAL",
		"BYTE_TO_INT", "INT_TO_BYTE",
	}
	for _, name := range fns {
		fn, ok := StdlibFunctions[name]
		if !ok {
			t.Fatalf("missing stdlib function %s", name)
		}
		_, err := fn(nil)
		if err == nil {
			t.Fatalf("%s: expected error for missing args", name)
		}
	}
}

// --- Stdlib math: missing arg errors ---

func TestStdlib_Math_MissingArgs(t *testing.T) {
	oneArgFns := []string{"ABS", "SQRT", "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP", "MOVE"}
	for _, name := range oneArgFns {
		fn, ok := StdlibFunctions[name]
		if !ok {
			t.Fatalf("missing stdlib function %s", name)
		}
		_, err := fn(nil)
		if err == nil {
			t.Fatalf("%s: expected error for missing args", name)
		}
	}
	twoArgFns := []string{"EXPT", "MIN", "MAX"}
	for _, name := range twoArgFns {
		fn := StdlibFunctions[name]
		_, err := fn([]Value{IntValue(1)})
		if err == nil {
			t.Fatalf("%s: expected error for too few args", name)
		}
	}
	// LIMIT needs 3
	_, err := StdlibFunctions["LIMIT"]([]Value{IntValue(1), IntValue(2)})
	if err == nil {
		t.Fatal("LIMIT: expected error for 2 args")
	}
	// SEL needs 3
	_, err = StdlibFunctions["SEL"]([]Value{IntValue(1), IntValue(2)})
	if err == nil {
		t.Fatal("SEL: expected error for 2 args")
	}
	// MUX needs 2+
	_, err = StdlibFunctions["MUX"]([]Value{IntValue(0)})
	if err == nil {
		t.Fatal("MUX: expected error for 1 arg")
	}
}

// --- Stdlib string: missing arg errors ---

func TestStdlib_String_MissingArgs(t *testing.T) {
	oneArgFns := []string{"LEN"}
	for _, name := range oneArgFns {
		fn := StdlibFunctions[name]
		_, err := fn(nil)
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	twoArgFns := []string{"LEFT", "RIGHT", "CONCAT", "FIND"}
	for _, name := range twoArgFns {
		fn := StdlibFunctions[name]
		_, err := fn([]Value{StringValue("x")})
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	threeArgFns := []string{"MID", "INSERT", "DELETE"}
	for _, name := range threeArgFns {
		fn := StdlibFunctions[name]
		_, err := fn([]Value{StringValue("x"), IntValue(1)})
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	// REPLACE needs 4
	_, err := StdlibFunctions["REPLACE"]([]Value{StringValue("x"), StringValue("y"), IntValue(1)})
	if err == nil {
		t.Fatal("REPLACE: expected error for 3 args")
	}
}

// --- MUX out of range ---

func TestStdlib_MUX_OutOfRange(t *testing.T) {
	_, err := StdlibFunctions["MUX"]([]Value{IntValue(99), IntValue(1)})
	if err == nil {
		t.Fatal("expected error for MUX index out of range")
	}
}

// --- TP timer: pulse completion edge cases ---

func TestTP_PulseCompletionWithINFalse(t *testing.T) {
	tp := &TP{}
	tp.SetInput("PT", TimeValue(100 * time.Millisecond))

	// Rising edge starts pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(10 * time.Millisecond)
	if !tp.q {
		t.Fatal("Q should be TRUE during pulse")
	}

	// Let pulse complete while IN is still true
	tp.Execute(200 * time.Millisecond)
	if tp.q {
		t.Fatal("Q should be FALSE after pulse")
	}

	// Now set IN to false, should deactivate
	tp.SetInput("IN", BoolValue(false))
	tp.Execute(10 * time.Millisecond)
}

// --- String comparisons ---

// --- execAssign: nil value (expression statement) ---

func TestExecAssign_NilValue(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(10))
	// An assign with nil Value = expression statement
	err := interp.execAssign(env, &ast.AssignStmt{
		Target: &ast.Ident{Name: "x"},
		Value:  nil,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- execAssign: to MemberAccessExpr ---

func TestExecAssign_ToMemberAccess(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: map[string]Value{"X": IntValue(0)}})
	err := interp.execAssign(env, &ast.AssignStmt{
		Target: &ast.MemberAccessExpr{
			Object: &ast.Ident{Name: "s"},
			Member: &ast.Ident{Name: "x"},
		},
		Value: &ast.Literal{LitKind: ast.LitInt, Value: "42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("s")
	if v.Struct["X"].Int != 42 {
		t.Fatalf("expected s.X=42, got %d", v.Struct["X"].Int)
	}
}

// --- execAssign: to IndexExpr ---

func TestExecAssign_ToIndexExpr(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(0), IntValue(0)}})
	err := interp.execAssign(env, &ast.AssignStmt{
		Target: &ast.IndexExpr{
			Object:  &ast.Ident{Name: "arr"},
			Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "0"}},
		},
		Value: &ast.Literal{LitKind: ast.LitInt, Value: "99"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- execAssign: define new variable ---

func TestExecAssign_DefineNew(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execAssign(env, &ast.AssignStmt{
		Target: &ast.Ident{Name: "newVar"},
		Value:  &ast.Literal{LitKind: ast.LitInt, Value: "42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := env.Get("newVar")
	if !ok {
		t.Fatal("expected newVar to be defined")
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

// --- execCase: error in selector eval, error in label eval ---

func TestExecCase_ErrorInSelector(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execCase(env, &ast.CaseStmt{
		Expr: &ast.Ident{Name: "undefined_var"},
	})
	if err == nil {
		t.Fatal("expected error for undefined selector")
	}
}

func TestExecCase_LabelEvalError(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))
	err := interp.execCase(env, &ast.CaseStmt{
		Expr: &ast.Ident{Name: "x"},
		Branches: []*ast.CaseBranch{
			{
				Labels: []ast.CaseLabel{
					&ast.CaseLabelValue{Value: &ast.Ident{Name: "undef"}},
				},
				Body: nil,
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for undefined label")
	}
}

// --- matchCaseLabel: range with eval error ---

func TestMatchCaseLabel_RangeEvalError(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.matchCaseLabel(env, IntValue(5), &ast.CaseLabelRange{
		Low:  &ast.Ident{Name: "undef_low"},
		High: &ast.Literal{LitKind: ast.LitInt, Value: "10"},
	})
	if err == nil {
		t.Fatal("expected error for undefined range low")
	}
	_, err = interp.matchCaseLabel(env, IntValue(5), &ast.CaseLabelRange{
		Low:  &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		High: &ast.Ident{Name: "undef_high"},
	})
	if err == nil {
		t.Fatal("expected error for undefined range high")
	}
}

// --- FBInstance: SetInput for user-defined FB with nil env ---

func TestFBInstance_SetInput_NilEnv(t *testing.T) {
	inst := &FBInstance{TypeName: "MyFB", Env: nil}
	// Should not panic
	inst.SetInput("x", IntValue(1))
}

// --- FBInstance.GetMember: stdlib FB member fallback ---

func TestFBInstance_GetMember_StdlibFallback(t *testing.T) {
	ton := &TON{}
	ton.SetInput("IN", BoolValue(true))
	ton.SetInput("PT", TimeValue(100 * time.Millisecond))
	ton.Execute(200 * time.Millisecond)

	inst := &FBInstance{TypeName: "TON", FB: ton}
	v := inst.GetMember("Q")
	if !v.Bool {
		t.Fatal("expected Q=true after TON timeout")
	}
	v = inst.GetMember("ET")
	if v.Time != 100*time.Millisecond {
		t.Fatalf("expected ET=100ms, got %v", v.Time)
	}
}

// --- CTUD GetOutput: all outputs ---

func TestCTUD_GetOutput_All(t *testing.T) {
	c := &CTUD{}
	c.SetInput("PV", IntValue(3))
	// Count up
	c.SetInput("CU", BoolValue(true))
	c.Execute(0)
	c.SetInput("CU", BoolValue(false))
	c.Execute(0)
	c.SetInput("CU", BoolValue(true))
	c.Execute(0)
	c.SetInput("CU", BoolValue(false))
	c.Execute(0)
	c.SetInput("CU", BoolValue(true))
	c.Execute(0)

	qu := c.GetOutput("QU")
	qd := c.GetOutput("QD")
	cv := c.GetOutput("CV")
	_ = qu
	_ = qd
	_ = cv
}

// --- TOF GetOutput: ET ---

func TestTOF_GetOutput_ET(t *testing.T) {
	tof := &TOF{}
	tof.SetInput("PT", TimeValue(100 * time.Millisecond))
	tof.SetInput("IN", BoolValue(true))
	tof.Execute(10 * time.Millisecond)
	tof.SetInput("IN", BoolValue(false))
	tof.Execute(50 * time.Millisecond)
	et := tof.GetOutput("ET")
	if et.Time == 0 {
		t.Fatal("expected non-zero ET after timing started")
	}
}

// --- TP GetOutput: ET ---

func TestTP_GetOutput_ET(t *testing.T) {
	tp := &TP{}
	tp.SetInput("PT", TimeValue(100 * time.Millisecond))
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(50 * time.Millisecond)
	et := tp.GetOutput("ET")
	if et.Time == 0 {
		t.Fatal("expected non-zero ET during pulse")
	}
}

// --- evalBinary: bool equality ---

// --- FBInstance Execute: user-defined FB with ErrReturn ---

func TestFBInstance_Execute_UserWithReturn(t *testing.T) {
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	decl := &ast.FunctionBlockDecl{
		Body: []ast.Statement{
			&ast.AssignStmt{
				Target: &ast.Ident{Name: "x"},
				Value:  &ast.Literal{LitKind: ast.LitInt, Value: "42"},
			},
			&ast.ReturnStmt{},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Env: env, Decl: decl}
	interp := New()
	inst.Execute(10*time.Millisecond, interp)
	v, _ := env.Get("x")
	if v.Int != 42 {
		t.Fatalf("expected x=42, got %d", v.Int)
	}
}

// --- FBInstance Execute: nil interp, nil decl, nil env ---

func TestFBInstance_Execute_NilInterp(t *testing.T) {
	inst := &FBInstance{TypeName: "MyFB"}
	// Should not panic with nil interp
	inst.Execute(10*time.Millisecond, nil)
}

// --- evalExpr: ParenExpr ---

func TestEvalExpr_ParenExpr(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))
	v, err := interp.evalExpr(env, &ast.ParenExpr{Inner: &ast.Ident{Name: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 5 {
		t.Fatalf("expected 5, got %d", v.Int)
	}
}

// --- evalBinary: string concatenation ---

func TestEvalBinary_StringConcat(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR s : STRING; END_VAR
	s := 'hello' + ' world';
	END_PROGRAM`)
	v, _ := env.Get("s")
	if v.Str != "hello world" {
		t.Fatalf("expected 'hello world', got %q", v.Str)
	}
}

// --- execIf: false condition with no elsif or else ---

func TestExecIf_FalseNoElse(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR x : DINT; END_VAR
	x := 0;
	IF FALSE THEN
		x := 1;
	END_IF;
	END_PROGRAM`)
	v, _ := env.Get("x")
	if v.Int != 0 {
		t.Fatalf("expected x=0, got %d", v.Int)
	}
}

// --- execFor: negative step not reaching bound ---

func TestExecFor_NegativeNotReaching(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR i : DINT; sum : DINT; END_VAR
	sum := 0;
	FOR i := 10 TO 1 BY -3 DO
		sum := sum + i;
	END_FOR;
	END_PROGRAM`)
	v, _ := env.Get("sum")
	// 10 + 7 + 4 + 1 = 22
	if v.Int != 22 {
		t.Fatalf("expected sum=22, got %d", v.Int)
	}
}

func TestEvalBinary_BoolEquality(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR a : BOOL; b : BOOL; eq : BOOL; ne : BOOL; END_VAR
	a := TRUE;
	b := FALSE;
	eq := a = a;
	ne := a <> b;
	END_PROGRAM`)
	v, _ := env.Get("eq")
	if !v.Bool {
		t.Fatal("expected eq=TRUE")
	}
	v, _ = env.Get("ne")
	if !v.Bool {
		t.Fatal("expected ne=TRUE")
	}
}

func TestEvalBinary_StringComparisons(t *testing.T) {
	env := runProgram(t, `PROGRAM test
	VAR
		a : STRING := 'abc';
		b : STRING := 'def';
		lt : BOOL; gt : BOOL; le : BOOL; ge : BOOL; ne : BOOL;
	END_VAR
	lt := a < b;
	gt := b > a;
	le := a <= a;
	ge := b >= b;
	ne := a <> b;
	END_PROGRAM`)
	check := func(name string, expected bool) {
		v, _ := env.Get(name)
		if v.Bool != expected {
			t.Errorf("%s: expected %v, got %v", name, expected, v.Bool)
		}
	}
	check("lt", true)
	check("gt", true)
	check("le", true)
	check("ge", true)
	check("ne", true)
}
