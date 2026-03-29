package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

// --- Error recovery tests ---

func TestParse_MultipleErrors(t *testing.T) {
	// Multiple broken statements in one program
	src := `PROGRAM Main
VAR x : INT; END_VAR
    @ ;
    $ ;
    x := 1;
END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.NotEmpty(t, result.Diags, "should have errors for @ and $")

	// Program should still be parsed
	require.Len(t, result.File.Declarations, 1)
	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	require.Equal(t, "Main", prog.Name.Name)
}

func TestParse_InvalidDeclarationRecovery(t *testing.T) {
	src := `WHILE TRUE DO END_WHILE;
PROGRAM Main
END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.NotEmpty(t, result.Diags)
	// Should recover and still find the program
	foundProgram := false
	for _, d := range result.File.Declarations {
		if _, ok := d.(*ast.ProgramDecl); ok {
			foundProgram = true
		}
	}
	require.True(t, foundProgram, "should recover and find ProgramDecl")
}

// --- Every declaration type ---

func TestParse_ProgramMinimal(t *testing.T) {
	result := Parse("test.st", `PROGRAM P END_PROGRAM`)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags)
	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	require.Equal(t, "P", prog.Name.Name)
	require.Empty(t, prog.Body)
	require.Empty(t, prog.VarBlocks)
}

func TestParse_FunctionBlockMinimal(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	require.Equal(t, "FB", fb.Name.Name)
}

func TestParse_FunctionMinimal(t *testing.T) {
	result := Parse("test.st", `FUNCTION F : INT END_FUNCTION`)
	require.Empty(t, result.Diags)
	fn, ok := result.File.Declarations[0].(*ast.FunctionDecl)
	require.True(t, ok)
	require.Equal(t, "F", fn.Name.Name)
	require.NotNil(t, fn.ReturnType)
}

func TestParse_FunctionNoReturnType(t *testing.T) {
	result := Parse("test.st", `FUNCTION F END_FUNCTION`)
	require.Empty(t, result.Diags)
	fn, ok := result.File.Declarations[0].(*ast.FunctionDecl)
	require.True(t, ok)
	require.Nil(t, fn.ReturnType)
}

func TestParse_InterfaceMinimal(t *testing.T) {
	result := Parse("test.st", `INTERFACE I END_INTERFACE`)
	require.Empty(t, result.Diags)
	iface, ok := result.File.Declarations[0].(*ast.InterfaceDecl)
	require.True(t, ok)
	require.Equal(t, "I", iface.Name.Name)
}

func TestParse_InterfaceWithExtends(t *testing.T) {
	result := Parse("test.st", `INTERFACE I EXTENDS IBase, IOther
    METHOD DoWork : BOOL; END_METHOD
    PROPERTY Prop : INT; END_PROPERTY
END_INTERFACE`)
	require.Empty(t, result.Diags)
	iface, ok := result.File.Declarations[0].(*ast.InterfaceDecl)
	require.True(t, ok)
	require.Len(t, iface.Extends, 2)
	require.Len(t, iface.Methods, 1)
	require.Len(t, iface.Properties, 1)
}

func TestParse_TypeDecl_Enum(t *testing.T) {
	result := Parse("test.st", `TYPE E : (A, B := 2, C); END_TYPE`)
	require.Empty(t, result.Diags)
	td, ok := result.File.Declarations[0].(*ast.TypeDecl)
	require.True(t, ok)
	enum, ok := td.Type.(*ast.EnumType)
	require.True(t, ok)
	require.Len(t, enum.Values, 3)
	require.NotNil(t, enum.Values[1].Value) // B has init value
}

func TestParse_TypeDecl_Struct(t *testing.T) {
	result := Parse("test.st", `TYPE S :
STRUCT
    x : INT := 0;
    y : REAL;
END_STRUCT
END_TYPE`)
	require.Empty(t, result.Diags)
	td := result.File.Declarations[0].(*ast.TypeDecl)
	st, ok := td.Type.(*ast.StructType)
	require.True(t, ok)
	require.Len(t, st.Members, 2)
	require.NotNil(t, st.Members[0].InitValue)
}

func TestParse_TypeDecl_Array(t *testing.T) {
	result := Parse("test.st", `TYPE A : ARRAY[0..9] OF INT; END_TYPE`)
	require.Empty(t, result.Diags)
	td := result.File.Declarations[0].(*ast.TypeDecl)
	arr, ok := td.Type.(*ast.ArrayType)
	require.True(t, ok)
	require.Len(t, arr.Ranges, 1)
}

func TestParse_TypeDecl_MultiDimArray(t *testing.T) {
	result := Parse("test.st", `TYPE A : ARRAY[0..2, 0..3] OF INT; END_TYPE`)
	require.Empty(t, result.Diags)
	td := result.File.Declarations[0].(*ast.TypeDecl)
	arr, ok := td.Type.(*ast.ArrayType)
	require.True(t, ok)
	require.Len(t, arr.Ranges, 2)
}

func TestParse_TypeDecl_Subrange(t *testing.T) {
	result := Parse("test.st", `TYPE SR : INT(0..100); END_TYPE`)
	require.Empty(t, result.Diags)
	td := result.File.Declarations[0].(*ast.TypeDecl)
	_, ok := td.Type.(*ast.SubrangeType)
	require.True(t, ok)
}

func TestParse_TypeDecl_PointerRef(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB
VAR
    p : POINTER TO INT;
    r : REFERENCE TO REAL;
END_VAR
END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	_, ok := fb.VarBlocks[0].Declarations[0].Type.(*ast.PointerType)
	require.True(t, ok)
	_, ok = fb.VarBlocks[0].Declarations[1].Type.(*ast.ReferenceType)
	require.True(t, ok)
}

func TestParse_TypeDecl_StringTypes(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB
VAR
    s1 : STRING;
    s2 : STRING(80);
    w1 : WSTRING;
    w2 : WSTRING(100);
END_VAR
END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	// STRING without length
	st1, ok := fb.VarBlocks[0].Declarations[0].Type.(*ast.StringType)
	require.True(t, ok)
	require.False(t, st1.IsWide)
	require.Nil(t, st1.Length)
	// STRING(80)
	st2, ok := fb.VarBlocks[0].Declarations[1].Type.(*ast.StringType)
	require.True(t, ok)
	require.NotNil(t, st2.Length)
	// WSTRING
	st3, ok := fb.VarBlocks[0].Declarations[2].Type.(*ast.StringType)
	require.True(t, ok)
	require.True(t, st3.IsWide)
}

// --- Expression precedence ---

func TestParse_ExprPrecedence_MulOverAdd(t *testing.T) {
	result := Parse("test.st", `PROGRAM P VAR x : INT; END_VAR x := 1 + 2 * 3; END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	add, ok := assign.Value.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, "+", add.Op.Text)
	_, ok = add.Right.(*ast.BinaryExpr) // right should be * node
	require.True(t, ok)
}

func TestParse_ExprPrecedence_PowerRightAssoc(t *testing.T) {
	result := Parse("test.st", `PROGRAM P VAR x : INT; END_VAR x := 2 ** 3 ** 4; END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	pow, ok := assign.Value.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, "**", pow.Op.Text)
	// Right-associative: right should also be **
	pow2, ok := pow.Right.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, "**", pow2.Op.Text)
}

func TestParse_ExprPrecedence_AndOrXor(t *testing.T) {
	result := Parse("test.st", `PROGRAM P VAR x : BOOL; END_VAR x := TRUE OR FALSE AND TRUE XOR FALSE; END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	// OR should be at top (lowest precedence of these three)
	or, ok := assign.Value.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, "OR", or.Op.Text)
}

func TestParse_ExprPrecedence_Comparison(t *testing.T) {
	result := Parse("test.st", `PROGRAM P VAR x : BOOL; a : INT; b : INT; END_VAR x := a + 1 > b - 1; END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	cmp, ok := assign.Value.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, ">", cmp.Op.Text)
	// Both sides should be + and -
	_, ok = cmp.Left.(*ast.BinaryExpr)
	require.True(t, ok)
	_, ok = cmp.Right.(*ast.BinaryExpr)
	require.True(t, ok)
}

// --- Edge cases ---

func TestParse_EmptyInput(t *testing.T) {
	result := Parse("test.st", "")
	require.NotNil(t, result.File)
	require.Empty(t, result.File.Declarations)
}

func TestParse_DeeplyNestedIf(t *testing.T) {
	// 20 levels deep
	src := "PROGRAM P\nVAR x : INT; END_VAR\n"
	for i := 0; i < 20; i++ {
		src += "IF x > 0 THEN\n"
	}
	src += "x := 1;\n"
	for i := 0; i < 20; i++ {
		src += "END_IF;\n"
	}
	src += "END_PROGRAM\n"
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags)
}

func TestParse_CaseWithRange(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT; y : INT; END_VAR
    CASE x OF
        1..5: y := 1;
        10, 20, 30: y := 2;
    ELSE
        y := 0;
    END_CASE;
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	cs := prog.Body[0].(*ast.CaseStmt)
	// First branch: range label
	_, ok := cs.Branches[0].Labels[0].(*ast.CaseLabelRange)
	require.True(t, ok)
	// Second branch: 3 value labels
	require.Len(t, cs.Branches[1].Labels, 3)
}

func TestParse_PragmaSkipped(t *testing.T) {
	result := Parse("test.st", `{attribute 'qualified_only'}
{attribute 'symbol'}
PROGRAM Main END_PROGRAM`)
	require.Empty(t, result.Diags)
	require.Len(t, result.File.Declarations, 1)
}

// --- All var section types ---

func TestParse_AllVarSections(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB
VAR x : INT; END_VAR
VAR_INPUT a : INT; END_VAR
VAR_OUTPUT b : INT; END_VAR
VAR_IN_OUT c : INT; END_VAR
VAR_TEMP d : INT; END_VAR
VAR_GLOBAL e : INT; END_VAR
VAR_EXTERNAL f : INT; END_VAR
VAR_CONFIG g : INT; END_VAR
VAR_ACCESS h : INT; END_VAR
END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.Len(t, fb.VarBlocks, 9)

	expected := []ast.VarSection{
		ast.VarLocal, ast.VarInput, ast.VarOutput, ast.VarInOut,
		ast.VarTemp, ast.VarGlobal, ast.VarExternal, ast.VarConfig, ast.VarAccess,
	}
	for i, vb := range fb.VarBlocks {
		if vb.Section != expected[i] {
			t.Errorf("block %d: got %v, want %v", i, vb.Section, expected[i])
		}
	}
}

func TestParse_VarModifiers(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR CONSTANT x : INT := 1; END_VAR
VAR RETAIN y : INT; END_VAR
VAR PERSISTENT z : INT; END_VAR
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, prog.VarBlocks[0].IsConstant)
	require.True(t, prog.VarBlocks[1].IsRetain)
	require.True(t, prog.VarBlocks[2].IsPersistent)
}

func TestParse_VarMultipleNames(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR a, b, c : INT; END_VAR
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.Len(t, prog.VarBlocks[0].Declarations[0].Names, 3)
}

func TestParse_VarWithInit(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT := 42; END_VAR
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.NotNil(t, prog.VarBlocks[0].Declarations[0].InitValue)
}

func TestParse_VarWithAT(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x AT addr : INT; END_VAR
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.NotNil(t, prog.VarBlocks[0].Declarations[0].AtAddress)
}

// --- Statement types ---

func TestParse_ReturnExitContinue(t *testing.T) {
	result := Parse("test.st", `FUNCTION F : INT
VAR i : INT; END_VAR
    FOR i := 0 TO 10 DO
        IF i = 3 THEN CONTINUE; END_IF;
        IF i = 7 THEN EXIT; END_IF;
    END_FOR;
    RETURN;
END_FUNCTION`)
	require.Empty(t, result.Diags)
	fn := result.File.Declarations[0].(*ast.FunctionDecl)
	require.NotEmpty(t, fn.Body)
}

func TestParse_EmptyStatement(t *testing.T) {
	result := Parse("test.st", `PROGRAM P VAR x : INT; END_VAR ; ; x := 1; END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	// Should have empty stmts + assignment
	require.GreaterOrEqual(t, len(prog.Body), 2)
}

func TestParse_CallStmtNamedArgs(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR fb : FB_Timer; done : BOOL; END_VAR
    fb(IN := TRUE, PT := T#5s, Q => done);
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	cs, ok := prog.Body[0].(*ast.CallStmt)
	require.True(t, ok)
	require.Len(t, cs.Args, 3)
	require.False(t, cs.Args[0].IsOutput)
	require.True(t, cs.Args[2].IsOutput)
}

// --- Literal types ---

func TestParse_AllLiteralTypes(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR a : INT; b : REAL; c : BOOL; d : STRING; e : TIME; f : INT; END_VAR
    a := 42;
    b := 3.14;
    c := TRUE;
    d := 'hello';
    e := T#5s;
    f := INT#10;
END_PROGRAM`)
	require.Empty(t, result.Diags)
}

// --- Deref expression ---

func TestParse_DerefExpr(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB
VAR p : POINTER TO INT; END_VAR
    p^ := 42;
END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	assign := fb.Body[0].(*ast.AssignStmt)
	_, ok := assign.Target.(*ast.DerefExpr)
	require.True(t, ok)
}

// --- Index expression ---

func TestParse_IndexExpr(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR arr : ARRAY[0..9] OF INT; x : INT; END_VAR
    x := arr[0];
    arr[5] := 42;
END_PROGRAM`)
	require.Empty(t, result.Diags)
}

// --- Member access ---

func TestParse_MemberAccessExpr(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR fb : FB_Motor; END_VAR
    fb.speed := 100;
END_PROGRAM`)
	require.Empty(t, result.Diags)
}

// --- Paren expression ---

func TestParse_ParenExpr(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT; END_VAR
    x := (1 + 2) * 3;
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	mul, ok := assign.Value.(*ast.BinaryExpr)
	require.True(t, ok)
	require.Equal(t, "*", mul.Op.Text)
	_, ok = mul.Left.(*ast.ParenExpr)
	require.True(t, ok)
}

// --- Unary NOT and minus ---

func TestParse_UnaryNot(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : BOOL; END_VAR
    x := NOT TRUE;
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	unary, ok := assign.Value.(*ast.UnaryExpr)
	require.True(t, ok)
	require.Equal(t, "NOT", unary.Op.Text)
}

func TestParse_UnaryMinus(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT; END_VAR
    x := -5;
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	unary, ok := assign.Value.(*ast.UnaryExpr)
	require.True(t, ok)
	require.Equal(t, "-", unary.Op.Text)
}

// --- FB with methods and properties ---

func TestParse_FBMethods(t *testing.T) {
	result := Parse("test.st", `FUNCTION_BLOCK FB
METHOD PUBLIC Run : BOOL
VAR_INPUT speed : INT; END_VAR
    Run := TRUE;
END_METHOD

METHOD OVERRIDE Stop
    ;
END_METHOD

PROPERTY Running : BOOL
END_PROPERTY

END_FUNCTION_BLOCK`)
	require.Empty(t, result.Diags)
	fb := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.Len(t, fb.Methods, 2)
	require.Equal(t, ast.AccessPublic, fb.Methods[0].AccessModifier)
	require.True(t, fb.Methods[1].IsOverride)
	require.Len(t, fb.Properties, 1)
}

// --- REPEAT without END_REPEAT (optional) ---

func TestParse_RepeatWithoutEndRepeat(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT; END_VAR
    REPEAT x := x + 1; UNTIL x > 10;
END_PROGRAM`)
	// Should parse (may have diags but should not crash)
	require.NotNil(t, result.File)
}

// --- TestCase declaration ---

func TestParse_TestCase(t *testing.T) {
	result := Parse("test.st", `TEST_CASE 'my test'
VAR x : INT; END_VAR
    x := 1;
END_TEST_CASE`)
	require.Empty(t, result.Diags)
	tc, ok := result.File.Declarations[0].(*ast.TestCaseDecl)
	require.True(t, ok)
	require.Equal(t, "my test", tc.Name)
	require.NotEmpty(t, tc.Body)
}

// --- Function call expression (not FB call stmt) ---

func TestParse_FunctionCallExpr(t *testing.T) {
	result := Parse("test.st", `PROGRAM P
VAR x : INT; END_VAR
    x := ABS(x);
END_PROGRAM`)
	require.Empty(t, result.Diags)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	assign := prog.Body[0].(*ast.AssignStmt)
	call, ok := assign.Value.(*ast.CallExpr)
	require.True(t, ok)
	require.Len(t, call.Args, 1)
}
