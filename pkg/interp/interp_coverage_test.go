package interp

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
)

// helper: parse program code and run interp, return the env
func runProgram(t *testing.T, code string) *Env {
	t.Helper()
	res := parser.Parse("test.st", code)
	if len(res.Diags) > 0 {
		t.Fatalf("parse errors: %v", res.Diags)
	}
	prog, ok := res.File.Declarations[0].(*ast.ProgramDecl)
	if !ok {
		t.Fatal("expected ProgramDecl")
	}
	engine := NewScanCycleEngine(prog)
	err := engine.Tick(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("tick error: %v", err)
	}
	return engine.env
}

// --- Value tests ---

func TestValue_String(t *testing.T) {
	tests := []struct {
		v    Value
		want string
	}{
		{BoolValue(true), "TRUE"},
		{BoolValue(false), "FALSE"},
		{IntValue(42), "42"},
		{RealValue(3.14), "3.14"},
		{StringValue("hello"), "'hello'"},
		{TimeValue(5 * time.Second), "T#5s"},
		{Value{Kind: ValArray, Array: []Value{{}, {}, {}}}, "[3 elements]"},
		{Value{Kind: ValStruct, Struct: map[string]Value{"X": IntValue(1)}}, "{1 fields}"},
		{Value{Kind: ValFBInstance}, "FB_INSTANCE"},
		{Value{Kind: ValDate}, "Value(Date)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.v.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValue_IsTruthy(t *testing.T) {
	tests := []struct {
		v    Value
		want bool
	}{
		{BoolValue(true), true},
		{BoolValue(false), false},
		{IntValue(1), true},
		{IntValue(0), false},
		{RealValue(1.0), true},
		{RealValue(0.0), false},
		{StringValue("hello"), true},
		{StringValue(""), false},
		{Value{Kind: ValArray}, false}, // default case
		{Value{Kind: ValStruct}, false},
	}
	for _, tt := range tests {
		got := tt.v.IsTruthy()
		if got != tt.want {
			t.Errorf("%v.IsTruthy() = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func TestValue_MarshalJSON(t *testing.T) {
	tests := []struct {
		v       Value
		wantStr string
	}{
		{BoolValue(true), "true"},
		{BoolValue(false), "false"},
		{IntValue(42), "42"},
		{RealValue(3.14), "3.14"},
		{StringValue("hello"), `"hello"`},
		{TimeValue(1 * time.Second), `"1s"`},
		{Value{Kind: ValArray, Array: []Value{{}, {}}}, `"[2 elements]"`},
	}
	for _, tt := range tests {
		b, err := tt.v.MarshalJSON()
		if err != nil {
			t.Errorf("MarshalJSON error: %v", err)
			continue
		}
		if string(b) != tt.wantStr {
			t.Errorf("got %s, want %s", string(b), tt.wantStr)
		}
	}
}

func TestValueKind_String(t *testing.T) {
	if s := ValBool.String(); s != "Bool" {
		t.Errorf("ValBool.String() = %q", s)
	}
	// Out of range
	invalid := ValueKind(999)
	s := invalid.String()
	if s == "" {
		t.Error("out-of-range kind should produce non-empty string")
	}
}

func TestZero_AllTypes(t *testing.T) {
	tests := []struct {
		kind types.TypeKind
		vk   ValueKind
	}{
		{types.KindBOOL, ValBool},
		{types.KindSINT, ValInt},
		{types.KindINT, ValInt},
		{types.KindDINT, ValInt},
		{types.KindLINT, ValInt},
		{types.KindUSINT, ValInt},
		{types.KindUINT, ValInt},
		{types.KindUDINT, ValInt},
		{types.KindULINT, ValInt},
		{types.KindBYTE, ValInt},
		{types.KindWORD, ValInt},
		{types.KindDWORD, ValInt},
		{types.KindLWORD, ValInt},
		{types.KindREAL, ValReal},
		{types.KindLREAL, ValReal},
		{types.KindSTRING, ValString},
		{types.KindWSTRING, ValString},
		{types.KindTIME, ValTime},
		{types.KindDATE, ValDate},
		{types.KindDT, ValDateTime},
		{types.KindTOD, ValTod},
		{types.KindEnum, ValInt}, // default fallback
	}
	for _, tt := range tests {
		v := Zero(tt.kind)
		if v.Kind != tt.vk {
			t.Errorf("Zero(%s).Kind = %v, want %v", tt.kind, v.Kind, tt.vk)
		}
	}
}

// --- Env tests ---

func TestEnv_ScopeChain(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("x", IntValue(10))
	child := NewEnv(parent)

	// Child can read from parent
	v, ok := child.Get("x")
	if !ok || v.Int != 10 {
		t.Error("child should inherit from parent")
	}

	// Child defines its own x
	child.Define("x", IntValue(20))
	v, _ = child.Get("x")
	if v.Int != 20 {
		t.Error("child's own x should shadow parent")
	}

	// Parent unchanged
	v, _ = parent.Get("x")
	if v.Int != 10 {
		t.Error("parent x should be unchanged")
	}
}

func TestEnv_SetWalksParent(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("counter", IntValue(0))
	child := NewEnv(parent)

	// Set in child should update parent's var
	ok := child.Set("counter", IntValue(99))
	if !ok {
		t.Error("Set should find var in parent")
	}
	v, _ := parent.Get("counter")
	if v.Int != 99 {
		t.Errorf("parent counter = %d, want 99", v.Int)
	}
}

func TestEnv_SetNotFound(t *testing.T) {
	env := NewEnv(nil)
	ok := env.Set("nonexistent", IntValue(1))
	if ok {
		t.Error("Set should return false for undefined variable")
	}
}

func TestEnv_CaseInsensitive(t *testing.T) {
	env := NewEnv(nil)
	env.Define("myVar", IntValue(42))

	v, ok := env.Get("MYVAR")
	if !ok || v.Int != 42 {
		t.Error("Get should be case-insensitive")
	}
}

func TestEnv_AllVars(t *testing.T) {
	env := NewEnv(nil)
	env.Define("a", IntValue(1))
	env.Define("b", IntValue(2))
	vars := env.AllVars()
	if len(vars) != 2 {
		t.Errorf("AllVars len = %d, want 2", len(vars))
	}
}

func TestEnv_Parent(t *testing.T) {
	root := NewEnv(nil)
	if root.Parent() != nil {
		t.Error("root env parent should be nil")
	}
	child := NewEnv(root)
	if child.Parent() != root {
		t.Error("child parent should be root")
	}
}

// --- Expression evaluation ---

func TestEvalBinary_AllIntOps(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : INT; b : INT; c : INT; d : INT; e : INT; f : INT; g : INT; h : BOOL; i : BOOL; j : BOOL; k : BOOL; l : BOOL; m : BOOL; END_VAR
    a := 10 + 3;
    b := 10 - 3;
    c := 10 * 3;
    d := 10 / 3;
    e := 10 MOD 3;
    h := 10 = 10;
    i := 10 <> 3;
    j := 3 < 10;
    k := 10 > 3;
    l := 3 <= 3;
    m := 3 >= 3;
END_PROGRAM`)
	assertInt(t, env, "A", 13)
	assertInt(t, env, "B", 7)
	assertInt(t, env, "C", 30)
	assertInt(t, env, "D", 3)
	assertInt(t, env, "E", 1)
	assertBool(t, env, "H", true)
	assertBool(t, env, "I", true)
	assertBool(t, env, "J", true)
	assertBool(t, env, "K", true)
	assertBool(t, env, "L", true)
	assertBool(t, env, "M", true)
}

func TestEvalBinary_RealOps(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : REAL; b : REAL; c : REAL; d : REAL; e : BOOL; f : BOOL; END_VAR
    a := 1.5 + 2.5;
    b := 5.0 - 1.5;
    c := 2.0 * 3.0;
    d := 10.0 / 4.0;
    e := 1.5 = 1.5;
    f := 1.5 <> 2.5;
END_PROGRAM`)
	assertReal(t, env, "A", 4.0)
	assertReal(t, env, "B", 3.5)
	assertReal(t, env, "C", 6.0)
	assertReal(t, env, "D", 2.5)
	assertBool(t, env, "E", true)
	assertBool(t, env, "F", true)
}

func TestEvalBinary_BoolOps(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; d : BOOL; e : BOOL; END_VAR
    a := TRUE AND FALSE;
    b := TRUE OR FALSE;
    c := TRUE XOR TRUE;
    d := TRUE = TRUE;
    e := TRUE <> FALSE;
END_PROGRAM`)
	assertBool(t, env, "A", false)
	assertBool(t, env, "B", true)
	assertBool(t, env, "C", false)
	assertBool(t, env, "D", true)
	assertBool(t, env, "E", true)
}

func TestEvalBinary_StringOps(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : STRING; b : BOOL; c : BOOL; d : BOOL; e : BOOL; f : BOOL; g : BOOL; END_VAR
    a := 'hello' + ' world';
    b := 'abc' = 'abc';
    c := 'abc' <> 'xyz';
    d := 'abc' < 'abd';
    e := 'abd' > 'abc';
    f := 'abc' <= 'abc';
    g := 'abc' >= 'abc';
END_PROGRAM`)
	v, _ := env.Get("A")
	if v.Str != "hello world" {
		t.Errorf("string concat = %q", v.Str)
	}
	assertBool(t, env, "B", true)
	assertBool(t, env, "C", true)
	assertBool(t, env, "D", true)
	assertBool(t, env, "E", true)
	assertBool(t, env, "F", true)
	assertBool(t, env, "G", true)
}

func TestEvalBinary_TimeOps(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : TIME; b : TIME; c : BOOL; d : BOOL; END_VAR
    a := T#5s + T#3s;
    b := T#5s - T#3s;
    c := T#5s > T#3s;
    d := T#5s = T#5s;
END_PROGRAM`)
	v, _ := env.Get("A")
	if v.Time != 8*time.Second {
		t.Errorf("T#+T# = %v, want 8s", v.Time)
	}
	v, _ = env.Get("B")
	if v.Time != 2*time.Second {
		t.Errorf("T#-T# = %v, want 2s", v.Time)
	}
	assertBool(t, env, "C", true)
	assertBool(t, env, "D", true)
}

func TestEvalBinary_Power(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : REAL; END_VAR
    x := 2 ** 10;
END_PROGRAM`)
	v, _ := env.Get("X")
	if v.Kind != ValReal || v.Real != 1024.0 {
		t.Errorf("2**10 = %v, want 1024.0", v)
	}
}

func TestEvalBinary_MixedIntReal(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : REAL; END_VAR
    x := 10 + 0.5;
END_PROGRAM`)
	assertReal(t, env, "X", 10.5)
}

func TestEvalBinary_DivisionByZero(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(10))
	env.Define("y", IntValue(0))

	res := parser.Parse("test.st", `PROGRAM Main
VAR x : INT; y : INT; END_VAR
    x := x / y;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	err := interp.execStatements(env, prog.Body)
	if err == nil {
		t.Error("expected division by zero error")
	}
}

func TestEvalBinary_ModByZero(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(10))
	env.Define("y", IntValue(0))

	res := parser.Parse("test.st", `PROGRAM Main
VAR x : INT; y : INT; END_VAR
    x := x MOD y;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	err := interp.execStatements(env, prog.Body)
	if err == nil {
		t.Error("expected division by zero error")
	}
}

func TestEvalBinary_RealDivByZero(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", RealValue(10.0))
	env.Define("y", RealValue(0.0))

	res := parser.Parse("test.st", `PROGRAM Main
VAR x : REAL; y : REAL; END_VAR
    x := x / y;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	err := interp.execStatements(env, prog.Body)
	if err == nil {
		t.Error("expected division by zero error for real")
	}
}

func TestEvalUnary_Ops(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR a : BOOL; b : INT; c : REAL; END_VAR
    a := NOT TRUE;
    b := -5;
    c := -3.14;
END_PROGRAM`)
	assertBool(t, env, "A", false)
	assertInt(t, env, "B", -5)
	v, _ := env.Get("C")
	if math.Abs(v.Real-(-3.14)) > 0.001 {
		t.Errorf("unary real = %v, want -3.14", v.Real)
	}
}

// --- Statement tests ---

func TestExecIf_ElsIfElse(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; result : INT; END_VAR
    x := 5;
    IF x > 10 THEN
        result := 1;
    ELSIF x > 3 THEN
        result := 2;
    ELSE
        result := 3;
    END_IF;
END_PROGRAM`)
	assertInt(t, env, "RESULT", 2)
}

func TestExecCase_Branches(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR state : INT; result : INT; END_VAR
    state := 2;
    CASE state OF
        1: result := 10;
        2: result := 20;
        3: result := 30;
    ELSE
        result := 99;
    END_CASE;
END_PROGRAM`)
	assertInt(t, env, "RESULT", 20)
}

func TestExecCase_ElseBranch(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR state : INT; result : INT; END_VAR
    state := 99;
    CASE state OF
        1: result := 10;
    ELSE
        result := 0;
    END_CASE;
END_PROGRAM`)
	assertInt(t, env, "RESULT", 0)
}

func TestExecFor_Normal(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR i : INT; sum : INT; END_VAR
    sum := 0;
    FOR i := 1 TO 10 DO
        sum := sum + i;
    END_FOR;
END_PROGRAM`)
	assertInt(t, env, "SUM", 55)
}

func TestExecFor_WithExit(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR i : INT; sum : INT; END_VAR
    sum := 0;
    FOR i := 1 TO 100 DO
        IF i > 5 THEN EXIT; END_IF;
        sum := sum + i;
    END_FOR;
END_PROGRAM`)
	assertInt(t, env, "SUM", 15)
}

func TestExecFor_WithContinue(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR i : INT; sum : INT; END_VAR
    sum := 0;
    FOR i := 1 TO 10 DO
        IF i MOD 2 = 0 THEN CONTINUE; END_IF;
        sum := sum + i;
    END_FOR;
END_PROGRAM`)
	assertInt(t, env, "SUM", 25) // 1+3+5+7+9
}

func TestExecFor_NegativeStep(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR i : INT; sum : INT; END_VAR
    sum := 0;
    FOR i := 10 TO 1 BY -1 DO
        sum := sum + i;
    END_FOR;
END_PROGRAM`)
	assertInt(t, env, "SUM", 55)
}

func TestExecWhile_Normal(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
    WHILE x < 10 DO
        x := x + 1;
    END_WHILE;
END_PROGRAM`)
	assertInt(t, env, "X", 10)
}

func TestExecWhile_WithExit(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
    WHILE TRUE DO
        x := x + 1;
        IF x = 5 THEN EXIT; END_IF;
    END_WHILE;
END_PROGRAM`)
	assertInt(t, env, "X", 5)
}

func TestExecWhile_WithContinue(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; sum : INT; END_VAR
    x := 0;
    sum := 0;
    WHILE x < 10 DO
        x := x + 1;
        IF x MOD 2 = 0 THEN CONTINUE; END_IF;
        sum := sum + x;
    END_WHILE;
END_PROGRAM`)
	assertInt(t, env, "SUM", 25)
}

func TestExecRepeat_Normal(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
    REPEAT
        x := x + 1;
    UNTIL x >= 5 END_REPEAT;
END_PROGRAM`)
	assertInt(t, env, "X", 5)
}

func TestExecRepeat_WithExit(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
    REPEAT
        x := x + 1;
        IF x = 3 THEN EXIT; END_IF;
    UNTIL x >= 100 END_REPEAT;
END_PROGRAM`)
	assertInt(t, env, "X", 3)
}

func TestExecRepeat_WithContinue(t *testing.T) {
	env := runProgram(t, `PROGRAM Main
VAR x : INT; sum : INT; END_VAR
    x := 0;
    sum := 0;
    REPEAT
        x := x + 1;
        IF x MOD 2 = 0 THEN CONTINUE; END_IF;
        sum := sum + x;
    UNTIL x >= 10 END_REPEAT;
END_PROGRAM`)
	assertInt(t, env, "SUM", 25)
}

func TestExecReturn_StopsExecution(t *testing.T) {
	res := parser.Parse("test.st", `PROGRAM Main
VAR x : INT; END_VAR
    x := 1;
    RETURN;
    x := 99;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	engine := NewScanCycleEngine(prog)
	err := engine.Tick(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("tick error: %v", err)
	}
	v, _ := engine.env.Get("X")
	if v.Int != 1 {
		t.Errorf("RETURN should stop execution, x = %d", v.Int)
	}
}

// --- Literal parsing ---

func TestParseLitTime_Variants(t *testing.T) {
	interp := New()
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"T#5s", 5 * time.Second},
		{"T#1h30m", 1*time.Hour + 30*time.Minute},
		{"T#100ms", 100 * time.Millisecond},
		{"T#1d", 24 * time.Hour},
		{"TIME#2h", 2 * time.Hour},
	}
	for _, tt := range tests {
		v, err := interp.parseLitTime(tt.input)
		if err != nil {
			t.Errorf("parseLitTime(%q) error: %v", tt.input, err)
			continue
		}
		if v.Time != tt.want {
			t.Errorf("parseLitTime(%q) = %v, want %v", tt.input, v.Time, tt.want)
		}
	}
}

func TestParseLitTyped_Variants(t *testing.T) {
	interp := New()
	tests := []struct {
		value  string
		prefix string
		wantOK bool
	}{
		{"5", "INT", true},
		{"10", "UINT", true},
		{"3.14", "REAL", true},
		{"1.0", "LREAL", true},
		{"TRUE", "BOOL", true},
		{"5", "UNKNOWN", false},
	}
	for _, tt := range tests {
		v, err := interp.parseLitTyped(tt.value, tt.prefix)
		if tt.wantOK && err != nil {
			t.Errorf("parseLitTyped(%q, %q) error: %v", tt.value, tt.prefix, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("parseLitTyped(%q, %q) should error, got %v", tt.value, tt.prefix, v)
		}
	}
}

func TestParseLitInt_BasePrefixed(t *testing.T) {
	interp := New()
	tests := []struct {
		input string
		want  int64
	}{
		{"16#FF", 255},
		{"2#1010", 10},
		{"8#77", 63},
		{"1_000", 1000},
	}
	for _, tt := range tests {
		v, err := interp.parseLitInt(tt.input)
		if err != nil {
			t.Errorf("parseLitInt(%q) error: %v", tt.input, err)
			continue
		}
		if v.Int != tt.want {
			t.Errorf("parseLitInt(%q) = %d, want %d", tt.input, v.Int, tt.want)
		}
	}
}

func TestParseLitString_QuoteStripping(t *testing.T) {
	interp := New()
	v, _ := interp.parseLitString("'hello'")
	if v.Str != "hello" {
		t.Errorf("got %q, want %q", v.Str, "hello")
	}
	v, _ = interp.parseLitString(`"world"`)
	if v.Str != "world" {
		t.Errorf("got %q, want %q", v.Str, "world")
	}
	v, _ = interp.parseLitString("x")
	if v.Str != "x" {
		t.Errorf("single char %q", v.Str)
	}
}

// --- Array index ---

func TestEvalIndex_OutOfBounds(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{Kind: ValArray, Array: []Value{IntValue(10), IntValue(20)}})

	expr := &ast.IndexExpr{
		Object:  &ast.Ident{Name: "arr"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "5"}},
	}
	_, err := interp.evalIndex(env, expr)
	if err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestEvalIndex_NonArray(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(42))

	expr := &ast.IndexExpr{
		Object:  &ast.Ident{Name: "x"},
		Indices: []ast.Expr{&ast.Literal{LitKind: ast.LitInt, Value: "0"}},
	}
	_, err := interp.evalIndex(env, expr)
	if err == nil {
		t.Error("expected 'cannot index' error")
	}
}

// --- Scan cycle ---

func TestScanCycle_MultipleTicks(t *testing.T) {
	res := parser.Parse("test.st", `PROGRAM Main
VAR
    counter : INT;
END_VAR
    counter := counter + 1;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	engine := NewScanCycleEngine(prog)

	for i := 0; i < 5; i++ {
		if err := engine.Tick(10 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}

	v, _ := engine.env.Get("COUNTER")
	if v.Int != 5 {
		t.Errorf("counter = %d after 5 ticks, want 5", v.Int)
	}
	if engine.Clock() != 50*time.Millisecond {
		t.Errorf("clock = %v, want 50ms", engine.Clock())
	}
}

func TestScanCycle_IO(t *testing.T) {
	res := parser.Parse("test.st", `PROGRAM Main
VAR_INPUT
    sensor : INT;
END_VAR
VAR_OUTPUT
    result : INT;
END_VAR
    result := sensor * 2;
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	engine := NewScanCycleEngine(prog)
	engine.SetInput("sensor", IntValue(21))
	engine.Tick(10 * time.Millisecond)

	out := engine.GetOutput("result")
	if out.Int != 42 {
		t.Errorf("result = %d, want 42", out.Int)
	}
}

func TestScanCycle_InputOutputNames(t *testing.T) {
	res := parser.Parse("test.st", `PROGRAM Main
VAR_INPUT in1 : INT; END_VAR
VAR_OUTPUT out1 : INT; END_VAR
END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	engine := NewScanCycleEngine(prog)
	engine.Initialize()

	if len(engine.InputNames()) != 1 || engine.InputNames()[0] != "IN1" {
		t.Errorf("InputNames = %v", engine.InputNames())
	}
	if len(engine.OutputNames()) != 1 || engine.OutputNames()[0] != "OUT1" {
		t.Errorf("OutputNames = %v", engine.OutputNames())
	}
}

func TestScanCycle_GetOutputUnknown(t *testing.T) {
	res := parser.Parse("test.st", `PROGRAM Main END_PROGRAM`)
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	engine := NewScanCycleEngine(prog)
	engine.Initialize()
	v := engine.GetOutput("nonexistent")
	if v.Kind != 0 {
		t.Error("unknown output should return zero Value")
	}
}

// --- Error types ---

func TestRuntimeError_WithPos(t *testing.T) {
	err := &RuntimeError{Msg: "test error", Pos: ast.Pos{File: "test.st", Line: 5, Col: 3}}
	s := err.Error()
	if s != "test.st:5:3: runtime error: test error" {
		t.Errorf("got %q", s)
	}
}

func TestRuntimeError_WithoutPos(t *testing.T) {
	err := &RuntimeError{Msg: "test error"}
	s := err.Error()
	if s != "runtime error: test error" {
		t.Errorf("got %q", s)
	}
}

func TestControlFlowErrors(t *testing.T) {
	if e := (&ErrReturn{}).Error(); e != "RETURN" {
		t.Errorf("ErrReturn = %q", e)
	}
	if e := (&ErrExit{}).Error(); e != "EXIT" {
		t.Errorf("ErrExit = %q", e)
	}
	if e := (&ErrContinue{}).Error(); e != "CONTINUE" {
		t.Errorf("ErrContinue = %q", e)
	}
}

// --- Assertions ---

func TestAssertions_EQ_Pass(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	fn := interp.LocalFunctions["ASSERT_EQ"]
	fn([]Value{IntValue(42), IntValue(42)}, ast.Pos{})
	if collector.HasFailures() {
		t.Error("ASSERT_EQ(42, 42) should pass")
	}
}

func TestAssertions_EQ_Fail(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	fn := interp.LocalFunctions["ASSERT_EQ"]
	fn([]Value{IntValue(42), IntValue(99)}, ast.Pos{})
	if !collector.HasFailures() {
		t.Error("ASSERT_EQ(42, 99) should fail")
	}
	failures := collector.Failures()
	if len(failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(failures))
	}
}

func TestAssertions_EQ_CustomMsg(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	fn := interp.LocalFunctions["ASSERT_EQ"]
	fn([]Value{IntValue(1), IntValue(2), StringValue("custom")}, ast.Pos{})
	if len(collector.Failures()) != 1 || collector.Failures()[0].Message != "custom" {
		t.Error("custom message not used")
	}
}

func TestAssertions_TRUE_FALSE(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	interp.LocalFunctions["ASSERT_TRUE"]([]Value{BoolValue(true)}, ast.Pos{})
	interp.LocalFunctions["ASSERT_FALSE"]([]Value{BoolValue(false)}, ast.Pos{})
	if collector.HasFailures() {
		t.Error("TRUE/FALSE assertions should pass")
	}

	interp.LocalFunctions["ASSERT_TRUE"]([]Value{BoolValue(false)}, ast.Pos{})
	if !collector.HasFailures() {
		t.Error("ASSERT_TRUE(FALSE) should fail")
	}
}

func TestAssertions_TRUE_CustomMsg(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	interp.LocalFunctions["ASSERT_TRUE"]([]Value{BoolValue(false), StringValue("custom true")}, ast.Pos{})
	if collector.Failures()[0].Message != "custom true" {
		t.Error("custom message not used for ASSERT_TRUE")
	}
}

func TestAssertions_FALSE_CustomMsg(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	interp.LocalFunctions["ASSERT_FALSE"]([]Value{BoolValue(true), StringValue("custom false")}, ast.Pos{})
	if collector.Failures()[0].Message != "custom false" {
		t.Error("custom message not used for ASSERT_FALSE")
	}
}

func TestAssertions_NEAR(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	fn := interp.LocalFunctions["ASSERT_NEAR"]
	fn([]Value{RealValue(3.14), RealValue(3.15), RealValue(0.02)}, ast.Pos{})
	if collector.HasFailures() {
		t.Error("ASSERT_NEAR should pass within epsilon")
	}

	fn([]Value{RealValue(3.14), RealValue(4.0), RealValue(0.01)}, ast.Pos{})
	if !collector.HasFailures() {
		t.Error("ASSERT_NEAR should fail outside epsilon")
	}
}

func TestAssertions_NEAR_CustomMsg(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	fn := interp.LocalFunctions["ASSERT_NEAR"]
	fn([]Value{RealValue(0), RealValue(100), RealValue(0.01), StringValue("custom near")}, ast.Pos{})
	if collector.Failures()[0].Message != "custom near" {
		t.Error("custom message not used for ASSERT_NEAR")
	}
}

func TestAssertions_TooFewArgs(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)

	_, err := interp.LocalFunctions["ASSERT_TRUE"]([]Value{}, ast.Pos{})
	if err == nil {
		t.Error("expected error for too few args")
	}
	_, err = interp.LocalFunctions["ASSERT_FALSE"]([]Value{}, ast.Pos{})
	if err == nil {
		t.Error("expected error for too few args")
	}
	_, err = interp.LocalFunctions["ASSERT_EQ"]([]Value{IntValue(1)}, ast.Pos{})
	if err == nil {
		t.Error("expected error for too few args")
	}
	_, err = interp.LocalFunctions["ASSERT_NEAR"]([]Value{RealValue(1), RealValue(2)}, ast.Pos{})
	if err == nil {
		t.Error("expected error for too few args")
	}
}

func TestAdvanceTime(t *testing.T) {
	interp := New()
	var tickedDuration time.Duration
	interp.RegisterAdvanceTime(func(d time.Duration) {
		tickedDuration = d
	})

	fn := interp.LocalFunctions["ADVANCE_TIME"]
	_, err := fn([]Value{TimeValue(5 * time.Second)}, ast.Pos{})
	if err != nil {
		t.Fatalf("ADVANCE_TIME error: %v", err)
	}
	if tickedDuration != 5*time.Second {
		t.Errorf("tickedDuration = %v, want 5s", tickedDuration)
	}

	// Wrong type
	_, err = fn([]Value{IntValue(5)}, ast.Pos{})
	if err == nil {
		t.Error("ADVANCE_TIME(INT) should error")
	}

	// No args
	_, err = fn([]Value{}, ast.Pos{})
	if err == nil {
		t.Error("ADVANCE_TIME() should error")
	}
}

// --- Stdlib math edge cases ---

func TestStdlib_ABS_Negative(t *testing.T) {
	fn := StdlibFunctions["ABS"]
	v, err := fn([]Value{IntValue(-42)})
	if err != nil || v.Int != 42 {
		t.Errorf("ABS(-42) = %v, err=%v", v, err)
	}
	v, _ = fn([]Value{RealValue(-3.14)})
	if math.Abs(v.Real-3.14) > 0.001 {
		t.Errorf("ABS(-3.14) = %v", v.Real)
	}
	v, _ = fn([]Value{IntValue(5)})
	if v.Int != 5 {
		t.Errorf("ABS(5) = %v", v.Int)
	}
}

func TestStdlib_ABS_Error(t *testing.T) {
	fn := StdlibFunctions["ABS"]
	_, err := fn([]Value{StringValue("x")})
	if err == nil {
		t.Error("ABS(string) should error")
	}
	_, err = fn([]Value{})
	if err == nil {
		t.Error("ABS() should error")
	}
}

func TestStdlib_MinMax(t *testing.T) {
	minFn := StdlibFunctions["MIN"]
	maxFn := StdlibFunctions["MAX"]

	v, _ := minFn([]Value{IntValue(3), IntValue(7)})
	if v.Int != 3 {
		t.Errorf("MIN(3,7) = %d", v.Int)
	}
	v, _ = maxFn([]Value{IntValue(3), IntValue(7)})
	if v.Int != 7 {
		t.Errorf("MAX(3,7) = %d", v.Int)
	}

	// Real args
	v, _ = minFn([]Value{RealValue(1.5), RealValue(2.5)})
	if v.Real != 1.5 {
		t.Errorf("MIN(1.5,2.5) = %v", v.Real)
	}
	v, _ = maxFn([]Value{RealValue(1.5), RealValue(2.5)})
	if v.Real != 2.5 {
		t.Errorf("MAX(1.5,2.5) = %v", v.Real)
	}

	// b > a for min real
	v, _ = minFn([]Value{RealValue(5.0), RealValue(2.0)})
	if v.Real != 2.0 {
		t.Errorf("MIN(5.0, 2.0) = %v", v.Real)
	}
	v, _ = maxFn([]Value{RealValue(2.0), RealValue(5.0)})
	if v.Real != 5.0 {
		t.Errorf("MAX(2.0, 5.0) = %v", v.Real)
	}
}

func TestStdlib_LIMIT(t *testing.T) {
	fn := StdlibFunctions["LIMIT"]
	v, _ := fn([]Value{IntValue(0), IntValue(50), IntValue(100)})
	if v.Int != 50 {
		t.Errorf("LIMIT(0,50,100) = %d", v.Int)
	}
	v, _ = fn([]Value{IntValue(0), IntValue(-5), IntValue(100)})
	if v.Int != 0 {
		t.Errorf("LIMIT(0,-5,100) = %d", v.Int)
	}
	v, _ = fn([]Value{IntValue(0), IntValue(200), IntValue(100)})
	if v.Int != 100 {
		t.Errorf("LIMIT(0,200,100) = %d", v.Int)
	}
}

func TestStdlib_SEL(t *testing.T) {
	fn := StdlibFunctions["SEL"]
	v, _ := fn([]Value{BoolValue(false), IntValue(10), IntValue(20)})
	if v.Int != 10 {
		t.Errorf("SEL(FALSE,10,20) = %d", v.Int)
	}
	v, _ = fn([]Value{BoolValue(true), IntValue(10), IntValue(20)})
	if v.Int != 20 {
		t.Errorf("SEL(TRUE,10,20) = %d", v.Int)
	}
}

func TestStdlib_MUX(t *testing.T) {
	fn := StdlibFunctions["MUX"]
	v, _ := fn([]Value{IntValue(0), IntValue(10), IntValue(20), IntValue(30)})
	if v.Int != 10 {
		t.Errorf("MUX(0,...) = %d", v.Int)
	}
	v, _ = fn([]Value{IntValue(2), IntValue(10), IntValue(20), IntValue(30)})
	if v.Int != 30 {
		t.Errorf("MUX(2,...) = %d", v.Int)
	}
	_, err := fn([]Value{IntValue(10), IntValue(1)})
	if err == nil {
		t.Error("MUX out-of-range should error")
	}
}

func TestStdlib_MOVE(t *testing.T) {
	fn := StdlibFunctions["MOVE"]
	v, _ := fn([]Value{IntValue(42)})
	if v.Int != 42 {
		t.Errorf("MOVE(42) = %d", v.Int)
	}
}

func TestStdlib_EXPT(t *testing.T) {
	fn := StdlibFunctions["EXPT"]
	v, _ := fn([]Value{RealValue(2.0), RealValue(10.0)})
	if v.Real != 1024.0 {
		t.Errorf("EXPT(2,10) = %v", v.Real)
	}
}

// --- Stdlib string ---

func TestStdlib_String_EdgeCases(t *testing.T) {
	// LEFT with 0 length
	v, _ := StdlibFunctions["LEFT"]([]Value{StringValue("hello"), IntValue(0)})
	if v.Str != "" {
		t.Errorf("LEFT(hello,0) = %q", v.Str)
	}
	// LEFT with length > string
	v, _ = StdlibFunctions["LEFT"]([]Value{StringValue("hi"), IntValue(10)})
	if v.Str != "hi" {
		t.Errorf("LEFT(hi,10) = %q", v.Str)
	}

	// RIGHT with 0
	v, _ = StdlibFunctions["RIGHT"]([]Value{StringValue("hello"), IntValue(0)})
	if v.Str != "" {
		t.Errorf("RIGHT(hello,0) = %q", v.Str)
	}
	// RIGHT excess
	v, _ = StdlibFunctions["RIGHT"]([]Value{StringValue("hi"), IntValue(10)})
	if v.Str != "hi" {
		t.Errorf("RIGHT(hi,10) = %q", v.Str)
	}

	// MID edge cases
	v, _ = StdlibFunctions["MID"]([]Value{StringValue("hello"), IntValue(3), IntValue(2)})
	if v.Str != "ell" {
		t.Errorf("MID(hello,3,2) = %q", v.Str)
	}
	v, _ = StdlibFunctions["MID"]([]Value{StringValue("hello"), IntValue(0), IntValue(1)})
	if v.Str != "" {
		t.Errorf("MID with L=0 = %q", v.Str)
	}
	v, _ = StdlibFunctions["MID"]([]Value{StringValue("hi"), IntValue(3), IntValue(10)})
	if v.Str != "" {
		t.Errorf("MID past end = %q", v.Str)
	}

	// FIND not found
	v, _ = StdlibFunctions["FIND"]([]Value{StringValue("hello"), StringValue("xyz")})
	if v.Int != 0 {
		t.Errorf("FIND not found = %d", v.Int)
	}
	v, _ = StdlibFunctions["FIND"]([]Value{StringValue("hello"), StringValue("ell")})
	if v.Int != 2 { // 1-based
		t.Errorf("FIND(ell) = %d, want 2", v.Int)
	}

	// INSERT
	v, _ = StdlibFunctions["INSERT"]([]Value{StringValue("hlo"), StringValue("el"), IntValue(2)})
	if v.Str != "hello" {
		t.Errorf("INSERT = %q", v.Str)
	}
	// INSERT at position < 1
	v, _ = StdlibFunctions["INSERT"]([]Value{StringValue("abc"), StringValue("x"), IntValue(0)})
	if v.Str != "abc" {
		t.Errorf("INSERT(pos=0) = %q", v.Str)
	}

	// DELETE
	v, _ = StdlibFunctions["DELETE"]([]Value{StringValue("hello"), IntValue(2), IntValue(2)})
	if v.Str != "hlo" {
		t.Errorf("DELETE = %q", v.Str)
	}
	v, _ = StdlibFunctions["DELETE"]([]Value{StringValue("abc"), IntValue(0), IntValue(1)})
	if v.Str != "abc" {
		t.Errorf("DELETE(L=0) = %q", v.Str)
	}

	// REPLACE
	v, _ = StdlibFunctions["REPLACE"]([]Value{StringValue("hello"), StringValue("a"), IntValue(3), IntValue(2)})
	if v.Str != "hao" {
		t.Errorf("REPLACE = %q", v.Str)
	}
}

// --- Stdlib convert ---

func TestStdlib_Convert_StringToInt_Invalid(t *testing.T) {
	fn := StdlibFunctions["STRING_TO_INT"]
	_, err := fn([]Value{StringValue("not_a_number")})
	if err == nil {
		t.Error("STRING_TO_INT(bad) should error")
	}
}

func TestStdlib_Convert_StringToReal_Invalid(t *testing.T) {
	fn := StdlibFunctions["STRING_TO_REAL"]
	_, err := fn([]Value{StringValue("not_a_float")})
	if err == nil {
		t.Error("STRING_TO_REAL(bad) should error")
	}
}

func TestStdlib_Convert_BoolToInt(t *testing.T) {
	fn := StdlibFunctions["BOOL_TO_INT"]
	v, _ := fn([]Value{BoolValue(true)})
	if v.Int != 1 {
		t.Errorf("BOOL_TO_INT(TRUE) = %d", v.Int)
	}
	v, _ = fn([]Value{BoolValue(false)})
	if v.Int != 0 {
		t.Errorf("BOOL_TO_INT(FALSE) = %d", v.Int)
	}
}

func TestStdlib_Convert_IntToBool(t *testing.T) {
	fn := StdlibFunctions["INT_TO_BOOL"]
	v, _ := fn([]Value{IntValue(42)})
	if !v.Bool {
		t.Error("INT_TO_BOOL(42) should be TRUE")
	}
	v, _ = fn([]Value{IntValue(0)})
	if v.Bool {
		t.Error("INT_TO_BOOL(0) should be FALSE")
	}
}

func TestStdlib_Convert_BoolToString(t *testing.T) {
	fn := StdlibFunctions["BOOL_TO_STRING"]
	v, _ := fn([]Value{BoolValue(true)})
	if v.Str != "TRUE" {
		t.Errorf("got %q", v.Str)
	}
	v, _ = fn([]Value{BoolValue(false)})
	if v.Str != "FALSE" {
		t.Errorf("got %q", v.Str)
	}
}

func TestStdlib_Convert_ByteInt(t *testing.T) {
	v, _ := StdlibFunctions["BYTE_TO_INT"]([]Value{IntValue(0x1FF)})
	if v.Int != 0xFF {
		t.Errorf("BYTE_TO_INT(0x1FF) = %d, want 255", v.Int)
	}
	v, _ = StdlibFunctions["INT_TO_BYTE"]([]Value{IntValue(0x1FF)})
	if v.Int != 0xFF {
		t.Errorf("INT_TO_BYTE(0x1FF) = %d, want 255", v.Int)
	}
}

// --- valuesEqual ---

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		a, b Value
		want bool
	}{
		{IntValue(5), IntValue(5), true},
		{IntValue(5), IntValue(6), false},
		{BoolValue(true), BoolValue(true), true},
		{BoolValue(true), BoolValue(false), false},
		{RealValue(1.5), RealValue(1.5), true},
		{StringValue("a"), StringValue("a"), true},
		{StringValue("a"), StringValue("b"), false},
		{TimeValue(time.Second), TimeValue(time.Second), true},
		{IntValue(5), RealValue(5.0), true}, // cross-type
		{IntValue(5), StringValue("5"), false}, // incompatible
		{Value{Kind: ValArray}, Value{Kind: ValArray}, false}, // default
	}
	for _, tt := range tests {
		got := valuesEqual(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestValuesInRange(t *testing.T) {
	if !valuesInRange(IntValue(5), IntValue(1), IntValue(10)) {
		t.Error("5 should be in [1,10]")
	}
	if valuesInRange(IntValue(0), IntValue(1), IntValue(10)) {
		t.Error("0 should not be in [1,10]")
	}
}

// --- toFloat ---

func TestToFloat(t *testing.T) {
	if f := toFloat(IntValue(5)); f != 5.0 {
		t.Errorf("toFloat(int 5) = %v", f)
	}
	if f := toFloat(RealValue(3.14)); f != 3.14 {
		t.Errorf("toFloat(real) = %v", f)
	}
	if f := toFloat(BoolValue(true)); f != 1.0 {
		t.Errorf("toFloat(true) = %v", f)
	}
	if f := toFloat(BoolValue(false)); f != 0.0 {
		t.Errorf("toFloat(false) = %v", f)
	}
	if f := toFloat(StringValue("x")); f != 0.0 {
		t.Errorf("toFloat(string) = %v", f)
	}
}

// --- DerefExpr ---

func TestEvalDeref_NotImplemented(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalExpr(env, &ast.DerefExpr{Operand: &ast.Literal{LitKind: ast.LitInt, Value: "1"}})
	if err == nil {
		t.Error("DerefExpr should return not-implemented error")
	}
}

// --- FBInstance ---

func TestFBInstance_UserDefined(t *testing.T) {
	res := parser.Parse("test.st", `FUNCTION_BLOCK FB_Counter
VAR_INPUT inc : INT; END_VAR
VAR_OUTPUT count : INT; END_VAR
    count := count + inc;
END_FUNCTION_BLOCK`)
	fbDecl := res.File.Declarations[0].(*ast.FunctionBlockDecl)
	interp := New()
	inst := NewUserFBInstance("FB_Counter", fbDecl, interp, nil)

	inst.SetInput("inc", IntValue(5))
	inst.Execute(10*time.Millisecond, interp)
	out := inst.GetOutput("count")
	if out.Int != 5 {
		t.Errorf("count = %d, want 5", out.Int)
	}

	inst.Execute(10*time.Millisecond, interp)
	out = inst.GetOutput("count")
	if out.Int != 10 {
		t.Errorf("count = %d after 2nd exec, want 10", out.Int)
	}
}

func TestFBInstance_GetMember(t *testing.T) {
	res := parser.Parse("test.st", `FUNCTION_BLOCK FB_Test
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT; END_VAR
VAR local : INT; END_VAR
    y := x * 2;
    local := 99;
END_FUNCTION_BLOCK`)
	fbDecl := res.File.Declarations[0].(*ast.FunctionBlockDecl)
	interp := New()
	inst := NewUserFBInstance("FB_Test", fbDecl, interp, nil)
	inst.SetInput("x", IntValue(21))
	inst.Execute(0, interp)

	// GetMember should find output, input, and local vars
	if v := inst.GetMember("y"); v.Int != 42 {
		t.Errorf("GetMember(y) = %d, want 42", v.Int)
	}
	if v := inst.GetMember("local"); v.Int != 99 {
		t.Errorf("GetMember(local) = %d, want 99", v.Int)
	}
}

// --- helpers ---

func assertInt(t *testing.T, env *Env, name string, want int64) {
	t.Helper()
	v, ok := env.Get(name)
	if !ok {
		t.Errorf("variable %s not found", name)
		return
	}
	if v.Int != want {
		t.Errorf("%s = %d, want %d", name, v.Int, want)
	}
}

func assertBool(t *testing.T, env *Env, name string, want bool) {
	t.Helper()
	v, ok := env.Get(name)
	if !ok {
		t.Errorf("variable %s not found", name)
		return
	}
	if v.Bool != want {
		t.Errorf("%s = %v, want %v", name, v.Bool, want)
	}
}

func assertReal(t *testing.T, env *Env, name string, want float64) {
	t.Helper()
	v, ok := env.Get(name)
	if !ok {
		t.Errorf("variable %s not found", name)
		return
	}
	if math.Abs(v.Real-want) > 0.001 {
		t.Errorf("%s = %v, want %v", name, v.Real, want)
	}
}
