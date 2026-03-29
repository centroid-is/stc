package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseMethod: various access modifiers and modifiers ---

func TestParse_MethodAccessModifiers(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	PUBLIC METHOD DoStuff : DINT
	END_METHOD
	PRIVATE METHOD DoOther
	END_METHOD
	PROTECTED METHOD DoProtected
	END_METHOD
	INTERNAL METHOD DoInternal
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 4)
}

func TestParse_MethodModifiers(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	ABSTRACT METHOD DoAbstract
	END_METHOD
	FINAL METHOD DoFinal
	END_METHOD
	OVERRIDE METHOD DoOverride
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 3)
}

func TestParse_MethodPreModifiers(t *testing.T) {
	// Access modifier BEFORE METHOD keyword
	src := `FUNCTION_BLOCK MyFB
	PUBLIC ABSTRACT METHOD DoStuff
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 1)
}

// --- isMethodStart: various orderings ---

func TestParse_MethodAfterAccessModifier(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	PUBLIC METHOD DoStuff
	END_METHOD
	PRIVATE METHOD DoOther
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 2)
}

// --- parseInterface: method signatures, property signatures, unexpected tokens ---

func TestParse_InterfaceWithMethodAndProperty(t *testing.T) {
	src := `INTERFACE IMyInterface
	METHOD DoStuff
	END_METHOD
	PROPERTY MyProp : DINT
	END_PROPERTY
	END_INTERFACE`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	iface, ok := result.File.Declarations[0].(*ast.InterfaceDecl)
	require.True(t, ok)
	assert.Len(t, iface.Methods, 1)
	assert.Len(t, iface.Properties, 1)
}

func TestParse_InterfaceUnexpectedToken(t *testing.T) {
	src := `INTERFACE IMyInterface
	42
	END_INTERFACE`
	result := Parse("test.st", src)
	// Should have a parse error but not crash
	assert.True(t, len(result.Diags) > 0)
}

// --- parsePrimaryExpr: all literal types covered ---

func TestParse_WStringLiteral(t *testing.T) {
	src := `PROGRAM P
	VAR s : WSTRING; END_VAR
		s := "hello";
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

func TestParse_DateLiteral(t *testing.T) {
	src := `PROGRAM P
	VAR d : DATE; END_VAR
		d := D#2024-01-01;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

func TestParse_DateTimeLiteral(t *testing.T) {
	src := `PROGRAM P
	VAR dt : DT; END_VAR
		dt := DT#2024-01-01-00:00:00;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

func TestParse_TODLiteral(t *testing.T) {
	src := `PROGRAM P
	VAR t : TOD; END_VAR
		t := TOD#12:00:00;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

func TestParse_TypedLiteral(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		x := INT#42;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- parsePostfix: named arg call ---

func TestParse_NamedArgCall(t *testing.T) {
	src := `PROGRAM P
	VAR r : DINT; END_VAR
		r := MyFunc(a := 1, b := 2);
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- parseTypeSpec: error case ---

func TestParse_TypeSpec_Error(t *testing.T) {
	src := `TYPE MyBad : ; END_TYPE`
	result := Parse("test.st", src)
	assert.True(t, len(result.Diags) > 0)
}

// --- parseCallArg: output binding ---

func TestParse_CallStmt_OutputBinding(t *testing.T) {
	src := `PROGRAM P
	VAR r : BOOL; END_VAR
		myTimer(IN := TRUE, Q => r);
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- parseStructType: multiple fields ---

func TestParse_StructType_MultipleFields(t *testing.T) {
	src := `TYPE MyStruct : STRUCT
		x : DINT;
		y : LREAL;
		name : STRING;
	END_STRUCT;
	END_TYPE`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	td, ok := result.File.Declarations[0].(*ast.TypeDecl)
	require.True(t, ok)
	st, ok := td.Type.(*ast.StructType)
	require.True(t, ok)
	assert.Len(t, st.Members, 3)
}

// --- parseNamedTypeOrSubrange / tryParseSubrange ---

func TestParse_SubrangeType(t *testing.T) {
	src := `TYPE MyRange : INT(1..100); END_TYPE`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- maxInt helper ---

func TestParse_MaxInt(t *testing.T) {
	// This tests the internal maxInt function via subrange parsing
	src := `TYPE R1 : INT(0..255); END_TYPE
	TYPE R2 : INT(100..50); END_TYPE`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- Property parsing ---

func TestParse_Property(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	PROPERTY MyProp : DINT
	VAR
		backing : DINT;
	END_VAR
	END_PROPERTY
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Properties, 1)
}

// --- parseStatements: CASE with multiple branches ---

func TestParse_CaseMultipleBranches(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; END_VAR
		CASE x OF
			1: x := 10;
			2, 3: x := 20;
			4..10: x := 30;
		ELSE
			x := 0;
		END_CASE;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	caseStmt := prog.Body[0].(*ast.CaseStmt)
	assert.Len(t, caseStmt.Branches, 3)
}

// --- isCaseLabelStart: various cases ---

func TestParse_CaseLabelBoolTime(t *testing.T) {
	src := `PROGRAM P
	VAR b : BOOL; END_VAR
		CASE b OF
			TRUE: b := FALSE;
			FALSE: b := TRUE;
		END_CASE;
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- Trivia attachment ---

func TestParse_CommentTrivia(t *testing.T) {
	src := `(* This is a comment *)
	PROGRAM P
	// inline comment
	VAR x : DINT; END_VAR
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- errorAt: used indirectly via various error paths ---

func TestParse_ErrorAt_IndirectlyViaExpect(t *testing.T) {
	// This triggers expect() failure which uses error()
	src := `PROGRAM P
	VAR x : DINT END_VAR
	END_PROGRAM`
	result := Parse("test.st", src)
	assert.True(t, len(result.Diags) > 0)
}

// --- peek/advance at end ---

func TestParse_AtEnd(t *testing.T) {
	src := ``
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- parseCallArgs with positional args ---

func TestParse_CallArgsPositional(t *testing.T) {
	src := `PROGRAM P
	VAR r : DINT; END_VAR
		r := MyFunc(1, 2, 3);
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- FB with IMPLEMENTS clause ---

// --- parseMethod: post-METHOD modifiers ---

func TestParse_MethodPostModifiers(t *testing.T) {
	// Modifiers AFTER METHOD keyword
	src := `FUNCTION_BLOCK MyFB
	METHOD PUBLIC DoStuff
	END_METHOD
	METHOD PRIVATE DoOther
	END_METHOD
	METHOD PROTECTED DoProtected
	END_METHOD
	METHOD INTERNAL DoInternal
	END_METHOD
	METHOD ABSTRACT DoAbstract
	END_METHOD
	METHOD FINAL DoFinal
	END_METHOD
	METHOD OVERRIDE DoOverride
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 7)
}

// --- parseMethod: with return type ---

func TestParse_MethodWithReturnType(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	METHOD DoCalc : DINT
	VAR_INPUT a : DINT; END_VAR
	END_METHOD
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Methods, 1)
}

// --- isNamedArgCall: edge cases ---

func TestParse_FuncCallNotNamed(t *testing.T) {
	src := `PROGRAM P
	VAR r : DINT; END_VAR
		r := ABS(42);
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
}

// --- parsePrimaryExpr: unknown token ---

func TestParse_UnknownPrimaryExpr(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; END_VAR
		x := ;
	END_PROGRAM`
	result := Parse("test.st", src)
	assert.True(t, len(result.Diags) > 0)
}

// --- PropertySignature in interface ---

func TestParse_InterfacePropertySignature(t *testing.T) {
	src := `INTERFACE IDevice
	PROPERTY Name : STRING
	END_PROPERTY
	END_INTERFACE`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	iface := result.File.Declarations[0].(*ast.InterfaceDecl)
	assert.Len(t, iface.Properties, 1)
}

// --- parseDeclaration: unrecognized keyword recovery ---

func TestParse_DeclarationRecovery(t *testing.T) {
	src := `UNIT MyUnit;
	PROGRAM P
	END_PROGRAM`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	assert.True(t, len(result.Diags) > 0)
}

func TestParse_FBWithImplements(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB IMPLEMENTS IMotor
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Implements, 1)
}

func TestParse_FBWithMultipleImplements(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB IMPLEMENTS IMotor, ISensor
	END_FUNCTION_BLOCK`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	assert.Len(t, fb.Implements, 2)
}
