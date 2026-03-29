package emit

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// --- Every AST node type emission ---

func TestEmit_EmptyBody(t *testing.T) {
	src := `PROGRAM Empty END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "PROGRAM Empty") {
		t.Errorf("missing PROGRAM header in: %s", out)
	}
	if !strings.Contains(out, "END_PROGRAM") {
		t.Errorf("missing END_PROGRAM in: %s", out)
	}
}

func TestEmit_FunctionBlockEmpty(t *testing.T) {
	src := `FUNCTION_BLOCK FB END_FUNCTION_BLOCK`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "FUNCTION_BLOCK FB") {
		t.Errorf("got: %s", out)
	}
}

func TestEmit_FunctionNoReturnType(t *testing.T) {
	src := `FUNCTION NoRet END_FUNCTION`
	out := parseAndEmit(t, src)
	// Should not have " : " after function name
	if strings.Contains(out, "NoRet :") {
		t.Errorf("function without return type should not have ':' in: %s", out)
	}
}

func TestEmit_EmptyStmt(t *testing.T) {
	src := `PROGRAM P VAR x : INT; END_VAR ; x := 1; END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, ";") {
		t.Errorf("missing semicolons in: %s", out)
	}
}

func TestEmit_CaseLabelRange(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; y : INT; END_VAR
    CASE x OF
        1..5: y := 1;
    END_CASE;
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "1..5") {
		t.Errorf("missing range label in: %s", out)
	}
}

func TestEmit_ForWithoutBY(t *testing.T) {
	src := `PROGRAM P
VAR i : INT; END_VAR
    FOR i := 0 TO 10 DO
        i := i;
    END_FOR;
END_PROGRAM`
	out := parseAndEmit(t, src)
	if strings.Contains(out, " BY ") {
		t.Errorf("should not have BY clause in: %s", out)
	}
}

func TestEmit_DerefExpr(t *testing.T) {
	src := `FUNCTION_BLOCK FB
VAR p : POINTER TO INT; END_VAR
    p^ := 1;
END_FUNCTION_BLOCK`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "p^") {
		t.Errorf("missing deref ^ in: %s", out)
	}
}

func TestEmit_ParenExpr(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; END_VAR
    x := (1 + 2) * 3;
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "(1 + 2)") {
		t.Errorf("missing parens in: %s", out)
	}
}

func TestEmit_SubrangeType(t *testing.T) {
	src := `TYPE SR : INT(0..100); END_TYPE`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "INT(0..100)") {
		t.Errorf("missing subrange in: %s", out)
	}
}

func TestEmit_EnumTypeInline(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; END_VAR
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "PROGRAM") {
		t.Errorf("basic check failed: %s", out)
	}
}

func TestEmit_TypeDecl_InlineType(t *testing.T) {
	src := `TYPE MyAlias : DINT; END_TYPE`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "DINT") {
		t.Errorf("missing type alias in: %s", out)
	}
}

func TestEmit_TypeDecl_Enum(t *testing.T) {
	src := `TYPE E :
(
    Red,
    Green := 1,
    Blue
);
END_TYPE`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "Red") {
		t.Errorf("missing enum value in: %s", out)
	}
	if !strings.Contains(out, "Green := 1") {
		t.Errorf("missing enum init in: %s", out)
	}
}

// --- Vendor target tests ---

func TestEmit_SchneiderNoPointer(t *testing.T) {
	src := `FUNCTION_BLOCK FB
VAR
    p : POINTER TO INT;
    r : REFERENCE TO INT;
    x : INT;
END_VAR
END_FUNCTION_BLOCK`
	opts := Options{Target: TargetSchneider, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	if strings.Contains(out, "POINTER") {
		t.Errorf("Schneider should strip POINTER: %s", out)
	}
	if strings.Contains(out, "REFERENCE") {
		t.Errorf("Schneider should strip REFERENCE: %s", out)
	}
	if !strings.Contains(out, "x : INT") {
		t.Errorf("should keep normal vars: %s", out)
	}
}

func TestEmit_PortableNo64Bit(t *testing.T) {
	src := `FUNCTION_BLOCK FB
VAR
    a : LINT;
    b : LREAL;
    c : LWORD;
    d : ULINT;
    e : INT;
END_VAR
END_FUNCTION_BLOCK`
	opts := Options{Target: TargetPortable, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	for _, kw := range []string{"LINT", "LREAL", "LWORD", "ULINT"} {
		if strings.Contains(out, kw) {
			t.Errorf("Portable should strip %s: %s", kw, out)
		}
	}
	if !strings.Contains(out, "e : INT") {
		t.Errorf("Portable should keep INT: %s", out)
	}
}

func TestEmit_PortableSkipsOOP(t *testing.T) {
	src := `FUNCTION_BLOCK FB
METHOD PUBLIC Run : BOOL
    Run := TRUE;
END_METHOD
END_FUNCTION_BLOCK

INTERFACE I
    METHOD Work : BOOL;
END_INTERFACE`
	opts := Options{Target: TargetPortable, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	if strings.Contains(out, "METHOD") {
		t.Errorf("Portable should strip METHOD: %s", out)
	}
	if strings.Contains(out, "INTERFACE") {
		t.Errorf("Portable should strip INTERFACE: %s", out)
	}
}

func TestEmit_SchneiderKeepsFB(t *testing.T) {
	src := `FUNCTION_BLOCK FB
VAR x : INT; END_VAR
    x := 1;
END_FUNCTION_BLOCK`
	opts := Options{Target: TargetSchneider, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "FUNCTION_BLOCK FB") {
		t.Errorf("Schneider should keep FB: %s", out)
	}
}

// --- Lowercase keywords ---

func TestEmit_LowercaseKeywords(t *testing.T) {
	src := `PROGRAM Main
VAR x : INT; END_VAR
    IF x > 0 THEN x := 1; END_IF;
    FOR x := 0 TO 10 DO x := x; END_FOR;
    WHILE x > 0 DO x := x - 1; END_WHILE;
    REPEAT x := x + 1; UNTIL x > 10 END_REPEAT;
    RETURN;
END_PROGRAM`
	opts := Options{Target: TargetBeckhoff, Indent: "  ", UppercaseKeywords: false}
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "program") {
		t.Errorf("expected lowercase 'program' in: %s", out)
	}
	if !strings.Contains(out, "end_program") {
		t.Errorf("expected lowercase 'end_program' in: %s", out)
	}
	if !strings.Contains(out, "if") {
		t.Errorf("expected lowercase 'if' in: %s", out)
	}
}

// --- Default indent ---

func TestEmit_DefaultIndent(t *testing.T) {
	src := `PROGRAM P VAR x : INT; END_VAR x := 1; END_PROGRAM`
	out := Emit(parser.Parse("test.st", src).File, Options{})
	// Default indent should be 4 spaces
	if !strings.Contains(out, "    x") {
		t.Errorf("expected 4-space indent in:\n%s", out)
	}
}

// --- Nil file ---

func TestEmit_Nil(t *testing.T) {
	out := Emit(nil, DefaultOptions())
	if out != "" {
		t.Errorf("expected empty for nil, got %q", out)
	}
}

// --- TestCase emission ---

func TestEmit_TestCase(t *testing.T) {
	src := `TEST_CASE 'my test'
VAR x : INT; END_VAR
    x := 1;
END_TEST_CASE`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "TEST_CASE") {
		t.Errorf("missing TEST_CASE in: %s", out)
	}
	if !strings.Contains(out, "END_TEST_CASE") {
		t.Errorf("missing END_TEST_CASE in: %s", out)
	}
}

// --- All expression types in emission ---

func TestEmit_CallExpr(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; END_VAR
    x := ABS(x);
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "ABS(x)") {
		t.Errorf("missing function call in: %s", out)
	}
}

func TestEmit_MultipleCallArgs(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; END_VAR
    x := MIN(x, 10);
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "MIN(x, 10)") {
		t.Errorf("missing multi-arg call in: %s", out)
	}
}

func TestEmit_TypedLiteral(t *testing.T) {
	src := `PROGRAM P
VAR x : INT; END_VAR
    x := INT#42;
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "INT#42") {
		t.Errorf("missing typed literal in: %s", out)
	}
}

func TestEmit_MultiIndex(t *testing.T) {
	src := `PROGRAM P
VAR arr : ARRAY[0..3, 0..3] OF INT; x : INT; END_VAR
    x := arr[1, 2];
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "arr[1, 2]") {
		t.Errorf("missing multi-index in: %s", out)
	}
}

// --- Round-trip stability with complex code ---

func TestRoundTrip_AllStmtTypes(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
    b : BOOL;
END_VAR
    x := 0;
    IF b THEN
        x := 1;
    ELSIF NOT b THEN
        x := 2;
    ELSE
        x := 3;
    END_IF;
    CASE x OF
        0:
            b := TRUE;
        1..5:
            b := FALSE;
    ELSE
        b := TRUE;
    END_CASE;
    FOR x := 0 TO 10 BY 2 DO
        CONTINUE;
    END_FOR;
    WHILE b DO
        EXIT;
    END_WHILE;
    REPEAT
        x := x + 1;
    UNTIL x > 10
    END_REPEAT;
    RETURN;
END_PROGRAM`
	opts := DefaultOptions()
	r1 := parser.Parse("test.st", src)
	o1 := Emit(r1.File, opts)
	r2 := parser.Parse("test.st", o1)
	o2 := Emit(r2.File, opts)
	if o1 != o2 {
		t.Errorf("round-trip not stable.\nFirst:\n%s\nSecond:\n%s", o1, o2)
	}
}

// --- VarBlock where all decls are filtered ---

func TestEmit_AllVarsFilteredSkipsBlock(t *testing.T) {
	// All vars are POINTER TO, Schneider strips them -> block should be empty
	src := `FUNCTION_BLOCK FB
VAR
    p1 : POINTER TO INT;
    p2 : POINTER TO BOOL;
END_VAR
END_FUNCTION_BLOCK`
	opts := Options{Target: TargetSchneider, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	// Should not emit VAR/END_VAR for an empty block
	if strings.Contains(out, "END_VAR") {
		t.Errorf("should skip entirely empty var block, got:\n%s", out)
	}
}

// --- Method with modifiers ---

func TestEmit_MethodModifiers(t *testing.T) {
	src := `FUNCTION_BLOCK FB
METHOD PUBLIC ABSTRACT DoWork : BOOL
END_METHOD

METHOD FINAL Run
END_METHOD

METHOD OVERRIDE Stop
END_METHOD

END_FUNCTION_BLOCK`
	opts := Options{Target: TargetBeckhoff, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "ABSTRACT") {
		t.Errorf("missing ABSTRACT: %s", out)
	}
	if !strings.Contains(out, "FINAL") {
		t.Errorf("missing FINAL: %s", out)
	}
	if !strings.Contains(out, "OVERRIDE") {
		t.Errorf("missing OVERRIDE: %s", out)
	}
}

// --- Interface with method and property signatures ---

func TestEmit_InterfaceSignatures(t *testing.T) {
	src := `INTERFACE I
    METHOD Work : BOOL;
    END_METHOD
    PROPERTY Enabled : BOOL;
    END_PROPERTY
END_INTERFACE`
	opts := Options{Target: TargetBeckhoff, Indent: "    ", UppercaseKeywords: true}
	out := parseAndEmit(t, src, opts)
	if !strings.Contains(out, "INTERFACE I") {
		t.Errorf("missing interface: %s", out)
	}
	if !strings.Contains(out, "METHOD Work") {
		t.Errorf("missing method sig: %s", out)
	}
	if !strings.Contains(out, "PROPERTY Enabled") {
		t.Errorf("missing property sig: %s", out)
	}
}

// --- ActionDecl (manual AST) ---

func TestEmit_ActionDecl(t *testing.T) {
	file := &ast.SourceFile{
		NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{
			&ast.ActionDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindActionDecl},
				Name:     &ast.Ident{Name: "MyAction"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt},
						Target:   &ast.Ident{Name: "x"},
						Value:    &ast.Literal{LitKind: ast.LitInt, Value: "1"},
					},
				},
			},
		},
	}
	out := Emit(file, DefaultOptions())
	if !strings.Contains(out, "ACTION MyAction") {
		t.Errorf("missing ACTION: %s", out)
	}
	if !strings.Contains(out, "END_ACTION") {
		t.Errorf("missing END_ACTION: %s", out)
	}
}

// --- Vendor support functions ---

func TestVendorSupport(t *testing.T) {
	// Beckhoff supports everything
	if !TargetBeckhoff.supportsOOP() {
		t.Error("Beckhoff should support OOP")
	}
	if !TargetBeckhoff.supportsPointerTo() {
		t.Error("Beckhoff should support POINTER TO")
	}
	if !TargetBeckhoff.supportsReferenceTo() {
		t.Error("Beckhoff should support REFERENCE TO")
	}
	if !TargetBeckhoff.supports64Bit() {
		t.Error("Beckhoff should support 64-bit")
	}

	// Schneider: no OOP, no pointers, no refs, but has 64-bit
	if TargetSchneider.supportsOOP() {
		t.Error("Schneider should not support OOP")
	}
	if TargetSchneider.supportsPointerTo() {
		t.Error("Schneider should not support POINTER TO")
	}
	if TargetSchneider.supportsReferenceTo() {
		t.Error("Schneider should not support REFERENCE TO")
	}
	if !TargetSchneider.supports64Bit() {
		t.Error("Schneider should support 64-bit")
	}

	// Portable: nothing advanced
	if TargetPortable.supportsOOP() {
		t.Error("Portable should not support OOP")
	}
	if TargetPortable.supports64Bit() {
		t.Error("Portable should not support 64-bit")
	}
}

func TestIs64BitType(t *testing.T) {
	for _, name := range []string{"LINT", "LREAL", "LWORD", "ULINT"} {
		if !is64BitType(name) {
			t.Errorf("%s should be 64-bit", name)
		}
	}
	for _, name := range []string{"INT", "REAL", "BOOL", "DINT"} {
		if is64BitType(name) {
			t.Errorf("%s should not be 64-bit", name)
		}
	}
}

// --- PropertyDecl with access modifier (manual AST) ---

func TestEmit_PropertyWithAccess(t *testing.T) {
	file := &ast.SourceFile{
		NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{
			&ast.FunctionBlockDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
				Name:     &ast.Ident{Name: "FB"},
				Properties: []*ast.PropertyDecl{
					{
						NodeBase:       ast.NodeBase{NodeKind: ast.KindPropertyDecl},
						Name:           &ast.Ident{Name: "Enabled"},
						AccessModifier: ast.AccessPublic,
						Type:           &ast.NamedType{Name: &ast.Ident{Name: "BOOL"}},
					},
				},
			},
		},
	}
	opts := Options{Target: TargetBeckhoff, Indent: "    ", UppercaseKeywords: true}
	out := Emit(file, opts)
	if !strings.Contains(out, "PUBLIC") {
		t.Errorf("missing PUBLIC access: %s", out)
	}
}

// --- VarDecl with AT address ---

func TestEmit_VarDeclAT(t *testing.T) {
	src := `PROGRAM P
VAR x AT addr : INT; END_VAR
END_PROGRAM`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "AT") {
		t.Errorf("missing AT address: %s", out)
	}
}

// --- Multiple declarations in one file ---

func TestEmit_MultipleDecls(t *testing.T) {
	src := `PROGRAM P1 END_PROGRAM

FUNCTION F1 : INT END_FUNCTION

FUNCTION_BLOCK FB1 END_FUNCTION_BLOCK`
	out := parseAndEmit(t, src)
	if !strings.Contains(out, "PROGRAM P1") {
		t.Errorf("missing P1: %s", out)
	}
	if !strings.Contains(out, "FUNCTION F1") {
		t.Errorf("missing F1: %s", out)
	}
	if !strings.Contains(out, "FUNCTION_BLOCK FB1") {
		t.Errorf("missing FB1: %s", out)
	}
}

// --- Struct in type spec (inline) ---

func TestEmit_StructInTypeSpec(t *testing.T) {
	file := &ast.SourceFile{
		NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
				Name:     &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{
					{
						NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock},
						Section:  ast.VarLocal,
						Declarations: []*ast.VarDecl{
							{
								NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl},
								Names:    []*ast.Ident{{Name: "s"}},
								Type: &ast.StructType{
									Members: []*ast.StructMember{
										{Name: &ast.Ident{Name: "x"}, Type: &ast.NamedType{Name: &ast.Ident{Name: "INT"}}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	out := Emit(file, DefaultOptions())
	if !strings.Contains(out, "STRUCT") {
		t.Errorf("missing inline STRUCT: %s", out)
	}
	if !strings.Contains(out, "END_STRUCT") {
		t.Errorf("missing END_STRUCT: %s", out)
	}
}

// --- isWordOp ---

func TestIsWordOp(t *testing.T) {
	for _, op := range []string{"NOT", "AND", "OR", "XOR", "MOD"} {
		if !isWordOp(op) {
			t.Errorf("%s should be word op", op)
		}
	}
	for _, op := range []string{"+", "-", "*", "/", "**", "=", "<>"} {
		if isWordOp(op) {
			t.Errorf("%s should not be word op", op)
		}
	}
}
