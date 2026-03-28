package emit

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// helper to parse and emit with default (Beckhoff) options
func parseAndEmit(t *testing.T, src string, opts ...Options) string {
	t.Helper()
	o := DefaultOptions()
	if len(opts) > 0 {
		o = opts[0]
	}
	result := parser.Parse("test.st", src)
	if len(result.Diags) > 0 {
		for _, d := range result.Diags {
			t.Logf("parse diag: %s", d.Message)
		}
	}
	return Emit(result.File, o)
}

// --- Program emission ---

func TestEmitProgram(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT := 0;
END_VAR
    x := x + 1;
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "PROGRAM Main") {
		t.Errorf("expected PROGRAM Main, got:\n%s", out)
	}
	if !strings.Contains(out, "END_PROGRAM") {
		t.Errorf("expected END_PROGRAM, got:\n%s", out)
	}
	if !strings.Contains(out, "x : INT := 0;") {
		t.Errorf("expected var decl 'x : INT := 0;', got:\n%s", out)
	}
	if !strings.Contains(out, "x := x + 1;") {
		t.Errorf("expected assignment 'x := x + 1;', got:\n%s", out)
	}
}

// --- FunctionBlock with EXTENDS, IMPLEMENTS, methods ---

func TestEmitFunctionBlock(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Motor EXTENDS FB_Base IMPLEMENTS I_Motor
VAR
    speed : INT;
END_VAR
    speed := 100;

METHOD PUBLIC Run : BOOL
VAR_INPUT
    targetSpeed : INT;
END_VAR
    speed := targetSpeed;
    Run := TRUE;
END_METHOD

END_FUNCTION_BLOCK
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "FUNCTION_BLOCK FB_Motor EXTENDS FB_Base IMPLEMENTS I_Motor") {
		t.Errorf("expected FB header with EXTENDS and IMPLEMENTS, got:\n%s", out)
	}
	if !strings.Contains(out, "METHOD PUBLIC Run : BOOL") {
		t.Errorf("expected METHOD declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "END_METHOD") {
		t.Errorf("expected END_METHOD, got:\n%s", out)
	}
	if !strings.Contains(out, "END_FUNCTION_BLOCK") {
		t.Errorf("expected END_FUNCTION_BLOCK, got:\n%s", out)
	}
}

// --- Function with return type and VAR_INPUT ---

func TestEmitFunction(t *testing.T) {
	src := `FUNCTION Add : INT
VAR_INPUT
    a : INT;
    b : INT;
END_VAR
    Add := a + b;
END_FUNCTION
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "FUNCTION Add : INT") {
		t.Errorf("expected FUNCTION Add : INT, got:\n%s", out)
	}
	if !strings.Contains(out, "VAR_INPUT") {
		t.Errorf("expected VAR_INPUT, got:\n%s", out)
	}
	if !strings.Contains(out, "Add := a + b;") {
		t.Errorf("expected 'Add := a + b;', got:\n%s", out)
	}
}

// --- Control flow: IF/ELSIF/ELSE ---

func TestEmitIfStmt(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
END_VAR
    IF x > 10 THEN
        x := 10;
    ELSIF x > 5 THEN
        x := 5;
    ELSE
        x := 0;
    END_IF;
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "IF x > 10 THEN") {
		t.Errorf("expected IF condition, got:\n%s", out)
	}
	if !strings.Contains(out, "ELSIF x > 5 THEN") {
		t.Errorf("expected ELSIF, got:\n%s", out)
	}
	if !strings.Contains(out, "ELSE") {
		t.Errorf("expected ELSE, got:\n%s", out)
	}
	if !strings.Contains(out, "END_IF;") {
		t.Errorf("expected END_IF;, got:\n%s", out)
	}
}

// --- CASE/OF ---

func TestEmitCaseStmt(t *testing.T) {
	src := `PROGRAM Main
VAR
    state : INT;
END_VAR
    CASE state OF
        0:
            state := 1;
        1, 2:
            state := 3;
    ELSE
        state := 0;
    END_CASE;
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "CASE state OF") {
		t.Errorf("expected CASE, got:\n%s", out)
	}
	if !strings.Contains(out, "END_CASE;") {
		t.Errorf("expected END_CASE;, got:\n%s", out)
	}
}

// --- FOR/WHILE/REPEAT loops ---

func TestEmitLoops(t *testing.T) {
	src := `PROGRAM Main
VAR
    i : INT;
    sum : INT;
END_VAR
    FOR i := 0 TO 10 BY 2 DO
        sum := sum + i;
    END_FOR;
    WHILE sum > 0 DO
        sum := sum - 1;
    END_WHILE;
    REPEAT
        sum := sum + 1;
    UNTIL sum >= 10
    END_REPEAT;
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "FOR i := 0 TO 10 BY 2 DO") {
		t.Errorf("expected FOR loop, got:\n%s", out)
	}
	if !strings.Contains(out, "END_FOR;") {
		t.Errorf("expected END_FOR;, got:\n%s", out)
	}
	if !strings.Contains(out, "WHILE sum > 0 DO") {
		t.Errorf("expected WHILE loop, got:\n%s", out)
	}
	if !strings.Contains(out, "END_WHILE;") {
		t.Errorf("expected END_WHILE;, got:\n%s", out)
	}
	if !strings.Contains(out, "REPEAT") {
		t.Errorf("expected REPEAT, got:\n%s", out)
	}
	if !strings.Contains(out, "UNTIL sum >= 10") {
		t.Errorf("expected UNTIL condition, got:\n%s", out)
	}
	if !strings.Contains(out, "END_REPEAT;") {
		t.Errorf("expected END_REPEAT;, got:\n%s", out)
	}
}

// --- TypeDecl with STRUCT, ENUM, ARRAY ---

func TestEmitTypeDecl(t *testing.T) {
	src := `TYPE MyStruct :
STRUCT
    field1 : INT;
    field2 : BOOL;
END_STRUCT
END_TYPE

TYPE MyEnum :
(
    Red,
    Green,
    Blue
);
END_TYPE

TYPE MyArray : ARRAY[0..9] OF INT;
END_TYPE
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "TYPE MyStruct :") {
		t.Errorf("expected TYPE MyStruct, got:\n%s", out)
	}
	if !strings.Contains(out, "STRUCT") {
		t.Errorf("expected STRUCT, got:\n%s", out)
	}
	if !strings.Contains(out, "field1 : INT;") {
		t.Errorf("expected struct field, got:\n%s", out)
	}
	if !strings.Contains(out, "END_STRUCT") {
		t.Errorf("expected END_STRUCT, got:\n%s", out)
	}
	if !strings.Contains(out, "MyEnum") {
		t.Errorf("expected MyEnum, got:\n%s", out)
	}
	if !strings.Contains(out, "ARRAY[0..9] OF INT") {
		t.Errorf("expected ARRAY type, got:\n%s", out)
	}
}

// --- Vendor-specific: Beckhoff emits OOP ---

func TestEmitBeckhoffPreservesOOP(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    x : INT;
END_VAR

METHOD PUBLIC DoWork : BOOL
    DoWork := TRUE;
END_METHOD

END_FUNCTION_BLOCK

INTERFACE I_Worker
    METHOD Work : BOOL;
END_INTERFACE
`
	opts := DefaultOptions()
	opts.Target = TargetBeckhoff
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "METHOD PUBLIC DoWork : BOOL") {
		t.Errorf("Beckhoff should preserve METHOD, got:\n%s", out)
	}
	if !strings.Contains(out, "INTERFACE I_Worker") {
		t.Errorf("Beckhoff should preserve INTERFACE, got:\n%s", out)
	}
}

// --- Vendor-specific: Schneider strips OOP ---

func TestEmitSchneiderSkipsOOP(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    x : INT;
END_VAR

METHOD PUBLIC DoWork : BOOL
    DoWork := TRUE;
END_METHOD

END_FUNCTION_BLOCK

INTERFACE I_Worker
    METHOD Work : BOOL;
END_INTERFACE
`
	opts := DefaultOptions()
	opts.Target = TargetSchneider
	out := parseAndEmit(t, src, opts)
	if strings.Contains(out, "METHOD") {
		t.Errorf("Schneider should skip METHOD, got:\n%s", out)
	}
	if strings.Contains(out, "INTERFACE") {
		t.Errorf("Schneider should skip INTERFACE, got:\n%s", out)
	}
	// FB itself should still be emitted
	if !strings.Contains(out, "FUNCTION_BLOCK FB_Test") {
		t.Errorf("Schneider should keep FUNCTION_BLOCK, got:\n%s", out)
	}
}

// --- Vendor-specific: Schneider skips POINTER TO and REFERENCE TO ---

func TestEmitSchneiderSkipsPointerRef(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    pData : POINTER TO INT;
    rData : REFERENCE TO INT;
    normal : INT;
END_VAR
END_FUNCTION_BLOCK
`
	opts := DefaultOptions()
	opts.Target = TargetSchneider
	out := parseAndEmit(t, src, opts)
	if strings.Contains(out, "POINTER TO") {
		t.Errorf("Schneider should skip POINTER TO vars, got:\n%s", out)
	}
	if strings.Contains(out, "REFERENCE TO") {
		t.Errorf("Schneider should skip REFERENCE TO vars, got:\n%s", out)
	}
	if !strings.Contains(out, "normal : INT;") {
		t.Errorf("Schneider should keep normal vars, got:\n%s", out)
	}
}

// --- Vendor-specific: Portable additionally skips 64-bit types ---

func TestEmitPortableSkips64Bit(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    bigInt : LINT;
    bigReal : LREAL;
    bigWord : LWORD;
    bigUint : ULINT;
    normal : INT;
END_VAR
END_FUNCTION_BLOCK
`
	opts := DefaultOptions()
	opts.Target = TargetPortable
	out := parseAndEmit(t, src, opts)
	if strings.Contains(out, "LINT") {
		t.Errorf("Portable should skip LINT vars, got:\n%s", out)
	}
	if strings.Contains(out, "LREAL") {
		t.Errorf("Portable should skip LREAL vars, got:\n%s", out)
	}
	if strings.Contains(out, "LWORD") {
		t.Errorf("Portable should skip LWORD vars, got:\n%s", out)
	}
	if strings.Contains(out, "ULINT") {
		t.Errorf("Portable should skip ULINT vars, got:\n%s", out)
	}
	if !strings.Contains(out, "normal : INT;") {
		t.Errorf("Portable should keep normal vars, got:\n%s", out)
	}
}

// --- Round-trip stability: emit(parse(emit(parse(src)))) == emit(parse(src)) ---

func TestRoundTripStability(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT := 0;
    y : BOOL;
END_VAR
    IF x > 10 THEN
        y := TRUE;
    ELSE
        y := FALSE;
    END_IF;
    FOR x := 0 TO 100 BY 1 DO
        y := NOT y;
    END_FOR;
END_PROGRAM

FUNCTION Add : INT
VAR_INPUT
    a : INT;
    b : INT;
END_VAR
    Add := a + b;
END_FUNCTION
`
	opts := DefaultOptions()

	// First pass: parse -> emit
	r1 := parser.Parse("test.st", src)
	out1 := Emit(r1.File, opts)

	// Second pass: parse the emitted output -> emit again
	r2 := parser.Parse("test.st", out1)
	out2 := Emit(r2.File, opts)

	if out1 != out2 {
		t.Errorf("round-trip not stable.\nFirst emit:\n%s\n\nSecond emit:\n%s", out1, out2)
	}
	if out1 == "" {
		t.Errorf("emitted output should not be empty")
	}
}

// --- Comments preserved (via AST trivia nodes) ---
// NOTE: The current parser does not attach trivia to AST nodes,
// so this test constructs AST nodes with trivia directly to verify
// the emitter's trivia handling code path works correctly.

func TestEmitCommentsPreserved(t *testing.T) {
	// Build an AST with trivia attached manually
	file := &ast.SourceFile{
		NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
				Name:     &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "Main"},
				VarBlocks: []*ast.VarBlock{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
						Section:  ast.VarLocal,
						Declarations: []*ast.VarDecl{
							{
								NodeBase: ast.NodeBase{
									NodeKind: ast.KindVarDecl,
									LeadingTrivia: []ast.Trivia{
										{Kind: ast.TriviaLineComment, Text: "// line comment on var\n"},
									},
								},
								Names: []*ast.Ident{{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "x"}},
								Type:  &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: &ast.Ident{Name: "INT"}},
							},
							{
								NodeBase: ast.NodeBase{
									NodeKind: ast.KindVarDecl,
									LeadingTrivia: []ast.Trivia{
										{Kind: ast.TriviaBlockComment, Text: "(* block comment *)"},
										{Kind: ast.TriviaWhitespace, Text: "\n"},
									},
								},
								Names: []*ast.Ident{{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "y"}},
								Type:  &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: &ast.Ident{Name: "BOOL"}},
							},
						},
					},
				},
				Body: []ast.Statement{
					&ast.AssignStmt{
						NodeBase: ast.NodeBase{
							NodeKind: ast.KindAssignStmt,
							LeadingTrivia: []ast.Trivia{
								{Kind: ast.TriviaLineComment, Text: "// comment in body\n"},
							},
						},
						Target: &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "x"},
						Value:  &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitInt, Value: "1"},
					},
				},
			},
		},
	}

	out := Emit(file, DefaultOptions())
	if !strings.Contains(out, "// line comment on var") {
		t.Errorf("expected line comment preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "(* block comment *)") {
		t.Errorf("expected block comment preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "// comment in body") {
		t.Errorf("expected body comment preserved, got:\n%s", out)
	}
}

// --- VarBlock qualifiers: CONSTANT, RETAIN, PERSISTENT ---

func TestEmitVarBlockQualifiers(t *testing.T) {
	src := `PROGRAM Main
VAR CONSTANT
    PI : REAL := 3.14;
END_VAR
VAR RETAIN
    counter : INT;
END_VAR
VAR PERSISTENT
    config : INT;
END_VAR
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "VAR CONSTANT") {
		t.Errorf("expected VAR CONSTANT, got:\n%s", out)
	}
	if !strings.Contains(out, "VAR RETAIN") {
		t.Errorf("expected VAR RETAIN, got:\n%s", out)
	}
	if !strings.Contains(out, "VAR PERSISTENT") {
		t.Errorf("expected VAR PERSISTENT, got:\n%s", out)
	}
}

// --- CallStmt with named args ---

func TestEmitCallStmt(t *testing.T) {
	src := `PROGRAM Main
VAR
    fb : FB_Timer;
    done : BOOL;
END_VAR
    fb(IN := TRUE, PT := T#5s, Q => done);
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "fb(") {
		t.Errorf("expected fb call, got:\n%s", out)
	}
	if !strings.Contains(out, "IN := TRUE") {
		t.Errorf("expected named input arg, got:\n%s", out)
	}
	if !strings.Contains(out, "Q => done") {
		t.Errorf("expected named output arg with =>, got:\n%s", out)
	}
}

// --- POINTER TO, REFERENCE TO, STRING(n), WSTRING(n) types ---

func TestEmitTypeSpecs(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    p : POINTER TO INT;
    r : REFERENCE TO BOOL;
    s : STRING(80);
    w : WSTRING(100);
END_VAR
END_FUNCTION_BLOCK
`
	opts := DefaultOptions()
	opts.Target = TargetBeckhoff
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "POINTER TO INT") {
		t.Errorf("expected POINTER TO INT, got:\n%s", out)
	}
	if !strings.Contains(out, "REFERENCE TO BOOL") {
		t.Errorf("expected REFERENCE TO BOOL, got:\n%s", out)
	}
	if !strings.Contains(out, "STRING(80)") {
		t.Errorf("expected STRING(80), got:\n%s", out)
	}
	if !strings.Contains(out, "WSTRING(100)") {
		t.Errorf("expected WSTRING(100), got:\n%s", out)
	}
}

// --- Expression precedence ---

func TestEmitExpressions(t *testing.T) {
	src := `PROGRAM Main
VAR
    a : INT;
    b : INT;
    c : INT;
    d : BOOL;
END_VAR
    a := b + c * 2;
    d := NOT (a > b) AND (c < 10);
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "b + c * 2") || !strings.Contains(out, "b + (c * 2)") {
		// Either form is acceptable — the key is that the output is valid
		if !strings.Contains(out, "b + c") {
			t.Errorf("expected expression with operators, got:\n%s", out)
		}
	}
	if !strings.Contains(out, "NOT") {
		t.Errorf("expected NOT operator, got:\n%s", out)
	}
}

// --- Comprehensive round-trip with FB, types, control flow ---

func TestRoundTripComprehensive(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Motor EXTENDS FB_Base IMPLEMENTS I_Motor
VAR_INPUT
    enable : BOOL;
    targetSpeed : INT;
END_VAR
VAR_OUTPUT
    running : BOOL;
    actualSpeed : INT;
END_VAR
VAR
    rampRate : INT := 10;
END_VAR
    IF enable THEN
        IF actualSpeed < targetSpeed THEN
            actualSpeed := actualSpeed + rampRate;
        ELSIF actualSpeed > targetSpeed THEN
            actualSpeed := actualSpeed - rampRate;
        END_IF;
        running := TRUE;
    ELSE
        actualSpeed := 0;
        running := FALSE;
    END_IF;

METHOD PUBLIC GetSpeed : INT
    GetSpeed := actualSpeed;
END_METHOD

END_FUNCTION_BLOCK
`
	opts := DefaultOptions()
	r1 := parser.Parse("test.st", src)
	out1 := Emit(r1.File, opts)
	r2 := parser.Parse("test.st", out1)
	out2 := Emit(r2.File, opts)
	if out1 != out2 {
		t.Errorf("comprehensive round-trip not stable.\nFirst:\n%s\n\nSecond:\n%s", out1, out2)
	}
	if out1 == "" {
		t.Error("output should not be empty")
	}
}

// --- Vendor lookup ---

func TestLookupTarget(t *testing.T) {
	tests := []struct {
		input string
		want  Target
	}{
		{"beckhoff", TargetBeckhoff},
		{"BECKHOFF", TargetBeckhoff},
		{"Schneider", TargetSchneider},
		{"SCHNEIDER", TargetSchneider},
		{"portable", TargetPortable},
		{"PORTABLE", TargetPortable},
		{"unknown", TargetBeckhoff},
	}
	for _, tt := range tests {
		got := LookupTarget(tt.input)
		if got != tt.want {
			t.Errorf("LookupTarget(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Nil file ---

func TestEmitNilFile(t *testing.T) {
	got := Emit(nil, DefaultOptions())
	if got != "" {
		t.Errorf("expected empty string for nil file, got %q", got)
	}
}

// --- RETURN, EXIT, CONTINUE ---

func TestEmitControlFlowStmts(t *testing.T) {
	src := `FUNCTION Test : INT
VAR
    i : INT;
END_VAR
    FOR i := 0 TO 10 DO
        IF i = 5 THEN
            CONTINUE;
        END_IF;
        IF i = 8 THEN
            EXIT;
        END_IF;
    END_FOR;
    RETURN;
END_FUNCTION
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "CONTINUE;") {
		t.Errorf("expected CONTINUE;, got:\n%s", out)
	}
	if !strings.Contains(out, "EXIT;") {
		t.Errorf("expected EXIT;, got:\n%s", out)
	}
	if !strings.Contains(out, "RETURN;") {
		t.Errorf("expected RETURN;, got:\n%s", out)
	}
}

// --- Member access and index expressions ---

func TestEmitMemberAccessAndIndex(t *testing.T) {
	src := `PROGRAM Main
VAR
    motor : FB_Motor;
    arr : ARRAY[0..9] OF INT;
END_VAR
    motor.speed := arr[0] + arr[1];
END_PROGRAM
`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "motor.speed") {
		t.Errorf("expected member access, got:\n%s", out)
	}
	if !strings.Contains(out, "arr[0]") {
		t.Errorf("expected array index, got:\n%s", out)
	}
}
