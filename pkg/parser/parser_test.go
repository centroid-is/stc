package parser

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return string(data)
}

func TestParse_ProgramBasic(t *testing.T) {
	src := readTestdata(t, "program_basic.st")
	result := Parse("program_basic.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	require.Len(t, result.File.Declarations, 1)
	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok, "expected ProgramDecl")
	require.Equal(t, "Main", prog.Name.Name)

	require.Len(t, prog.VarBlocks, 1)
	require.Len(t, prog.VarBlocks[0].Declarations, 3)

	// Verify variable names
	require.Equal(t, "x", prog.VarBlocks[0].Declarations[0].Names[0].Name)
	require.Equal(t, "y", prog.VarBlocks[0].Declarations[1].Names[0].Name)
	require.Equal(t, "name", prog.VarBlocks[0].Declarations[2].Names[0].Name)

	// Body has 2 assignment statements
	require.Len(t, prog.Body, 2)
}

func TestParse_FunctionBlockOOP(t *testing.T) {
	src := readTestdata(t, "function_block_oop.st")
	result := Parse("function_block_oop.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	require.Len(t, result.File.Declarations, 2)

	// First decl: InterfaceDecl
	iface, ok := result.File.Declarations[0].(*ast.InterfaceDecl)
	require.True(t, ok, "expected InterfaceDecl, got %T", result.File.Declarations[0])
	require.Equal(t, "IMotor", iface.Name.Name)
	require.Len(t, iface.Methods, 2)
	require.Equal(t, "Start", iface.Methods[0].Name.Name)
	require.Equal(t, "Stop", iface.Methods[1].Name.Name)

	// Second decl: FunctionBlockDecl
	fb, ok := result.File.Declarations[1].(*ast.FunctionBlockDecl)
	require.True(t, ok, "expected FunctionBlockDecl, got %T", result.File.Declarations[1])
	require.Equal(t, "FB_Motor", fb.Name.Name)

	// EXTENDS
	require.NotNil(t, fb.Extends)
	require.Equal(t, "FB_Base", fb.Extends.Name)

	// IMPLEMENTS
	require.Len(t, fb.Implements, 1)
	require.Equal(t, "IMotor", fb.Implements[0].Name)

	// Methods
	require.Len(t, fb.Methods, 2)
	require.Equal(t, "Start", fb.Methods[0].Name.Name)
	require.Equal(t, ast.AccessPublic, fb.Methods[0].AccessModifier)
	require.Equal(t, "Stop", fb.Methods[1].Name.Name)
	require.True(t, fb.Methods[1].IsOverride)

	// Properties
	require.Len(t, fb.Properties, 1)
	require.Equal(t, "Running", fb.Properties[0].Name.Name)
}

func TestParse_ControlFlow(t *testing.T) {
	src := readTestdata(t, "control_flow.st")
	result := Parse("control_flow.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)

	// Expect: IfStmt, CaseStmt, ForStmt, WhileStmt, RepeatStmt
	require.GreaterOrEqual(t, len(prog.Body), 5)

	// IF with ElsIf and Else
	ifStmt, ok := prog.Body[0].(*ast.IfStmt)
	require.True(t, ok, "expected IfStmt, got %T", prog.Body[0])
	require.Len(t, ifStmt.ElsIfs, 1, "expected 1 ELSIF branch")
	require.NotEmpty(t, ifStmt.Else, "expected ELSE branch")

	// CASE with range and list labels
	caseStmt, ok := prog.Body[1].(*ast.CaseStmt)
	require.True(t, ok, "expected CaseStmt, got %T", prog.Body[1])
	require.GreaterOrEqual(t, len(caseStmt.Branches), 3)
	// Branch 1..5 should be a range label
	rangeLabel, ok := caseStmt.Branches[1].Labels[0].(*ast.CaseLabelRange)
	require.True(t, ok, "expected CaseLabelRange for 1..5")
	require.NotNil(t, rangeLabel.Low)
	require.NotNil(t, rangeLabel.High)
	// Branch 10, 20, 30 has 3 labels
	require.Len(t, caseStmt.Branches[2].Labels, 3, "expected 3 labels in branch")
	// ELSE branch
	require.NotEmpty(t, caseStmt.ElseBranch)

	// FOR with BY clause
	forStmt, ok := prog.Body[2].(*ast.ForStmt)
	require.True(t, ok, "expected ForStmt")
	require.NotNil(t, forStmt.By, "expected BY clause")

	// WHILE
	_, ok = prog.Body[3].(*ast.WhileStmt)
	require.True(t, ok, "expected WhileStmt")

	// REPEAT
	_, ok = prog.Body[4].(*ast.RepeatStmt)
	require.True(t, ok, "expected RepeatStmt")
}

func TestParse_TypeDeclarations(t *testing.T) {
	src := readTestdata(t, "type_declarations.st")
	result := Parse("type_declarations.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	require.Len(t, result.File.Declarations, 5)

	// Enum
	td0, ok := result.File.Declarations[0].(*ast.TypeDecl)
	require.True(t, ok)
	require.Equal(t, "MyEnum", td0.Name.Name)
	enumType, ok := td0.Type.(*ast.EnumType)
	require.True(t, ok, "expected EnumType")
	require.Len(t, enumType.Values, 3)
	require.Equal(t, "Red", enumType.Values[0].Name.Name)
	require.Nil(t, enumType.Values[0].Value, "Red should have no init")
	require.Equal(t, "Green", enumType.Values[1].Name.Name)
	require.NotNil(t, enumType.Values[1].Value, "Green should have init=1")

	// Struct
	td1, ok := result.File.Declarations[1].(*ast.TypeDecl)
	require.True(t, ok)
	require.Equal(t, "MyStruct", td1.Name.Name)
	structType, ok := td1.Type.(*ast.StructType)
	require.True(t, ok, "expected StructType")
	require.Len(t, structType.Members, 3)

	// Array
	td2, ok := result.File.Declarations[2].(*ast.TypeDecl)
	require.True(t, ok)
	require.Equal(t, "MyArray", td2.Name.Name)
	arrayType, ok := td2.Type.(*ast.ArrayType)
	require.True(t, ok, "expected ArrayType")
	require.Len(t, arrayType.Ranges, 1)

	// Subrange
	td3, ok := result.File.Declarations[3].(*ast.TypeDecl)
	require.True(t, ok)
	require.Equal(t, "MySubrange", td3.Name.Name)
	subrangeType, ok := td3.Type.(*ast.SubrangeType)
	require.True(t, ok, "expected SubrangeType")
	require.NotNil(t, subrangeType.BaseType)

	// Alias
	td4, ok := result.File.Declarations[4].(*ast.TypeDecl)
	require.True(t, ok)
	require.Equal(t, "MyAlias", td4.Name.Name)
	namedType, ok := td4.Type.(*ast.NamedType)
	require.True(t, ok, "expected NamedType for alias")
	require.Equal(t, "DINT", namedType.Name.Name)
}

func TestParse_ErrorRecovery(t *testing.T) {
	src := readTestdata(t, "error_recovery.st")
	result := Parse("error_recovery.st", src)

	// File must NOT be nil (partial AST produced)
	require.NotNil(t, result.File)

	// Diagnostics must NOT be empty (errors reported)
	require.NotEmpty(t, result.Diags, "expected diagnostics for broken code")

	// AST still has the FunctionBlockDecl
	require.NotEmpty(t, result.File.Declarations)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok, "expected FunctionBlockDecl, got %T", result.File.Declarations[0])

	// The z := 42 assignment should be recovered
	foundAssign := false
	for _, stmt := range fb.Body {
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			if ident, ok := assign.Target.(*ast.Ident); ok && ident.Name == "z" {
				foundAssign = true
				break
			}
		}
	}
	require.True(t, foundAssign, "expected recovered z := 42 assignment")

	// Diagnostics have positions
	for _, d := range result.Diags {
		require.NotEmpty(t, d.Pos.File, "diagnostic should have file")
		require.Greater(t, d.Pos.Line, 0, "diagnostic should have line > 0")
	}
}

func TestParse_VarSections(t *testing.T) {
	src := readTestdata(t, "var_sections.st")
	result := Parse("var_sections.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)

	require.Len(t, fb.VarBlocks, 6)

	require.Equal(t, ast.VarInput, fb.VarBlocks[0].Section)
	require.Equal(t, ast.VarOutput, fb.VarBlocks[1].Section)
	require.Equal(t, ast.VarInOut, fb.VarBlocks[2].Section)
	require.Equal(t, ast.VarTemp, fb.VarBlocks[3].Section)

	// CONSTANT
	require.Equal(t, ast.VarLocal, fb.VarBlocks[4].Section)
	require.True(t, fb.VarBlocks[4].IsConstant, "expected CONSTANT modifier")

	// RETAIN
	require.Equal(t, ast.VarLocal, fb.VarBlocks[5].Section)
	require.True(t, fb.VarBlocks[5].IsRetain, "expected RETAIN modifier")
}

func TestParse_Expressions(t *testing.T) {
	src := readTestdata(t, "expressions.st")
	result := Parse("expressions.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)

	// a := 1 + 2 * 3 => BinaryExpr(+, 1, BinaryExpr(*, 2, 3))
	assign0, ok := prog.Body[0].(*ast.AssignStmt)
	require.True(t, ok)
	add, ok := assign0.Value.(*ast.BinaryExpr)
	require.True(t, ok, "expected BinaryExpr(+), got %T", assign0.Value)
	require.Equal(t, "+", add.Op.Text)
	_, ok = add.Left.(*ast.Literal)
	require.True(t, ok, "left of + should be literal")
	mul, ok := add.Right.(*ast.BinaryExpr)
	require.True(t, ok, "right of + should be BinaryExpr(*)")
	require.Equal(t, "*", mul.Op.Text)

	// c := 2 ** 3 ** 2 => BinaryExpr(**, 2, BinaryExpr(**, 3, 2)) (right-assoc)
	assign2, ok := prog.Body[2].(*ast.AssignStmt)
	require.True(t, ok)
	pow1, ok := assign2.Value.(*ast.BinaryExpr)
	require.True(t, ok, "expected BinaryExpr(**)")
	require.Equal(t, "**", pow1.Op.Text)
	_, ok = pow1.Left.(*ast.Literal)
	require.True(t, ok, "left of ** should be literal 2")
	pow2, ok := pow1.Right.(*ast.BinaryExpr)
	require.True(t, ok, "right of ** should be BinaryExpr(**) for right-associative")
	require.Equal(t, "**", pow2.Op.Text)

	// f := a > 0 AND b < 100 OR c = d
	// Precedence: (a > 0) AND (b < 100) grouped first, then OR
	assign4, ok := prog.Body[4].(*ast.AssignStmt)
	require.True(t, ok)
	orExpr, ok := assign4.Value.(*ast.BinaryExpr)
	require.True(t, ok, "expected OR at top level, got %T", assign4.Value)
	require.Equal(t, "OR", orExpr.Op.Text)
	andExpr, ok := orExpr.Left.(*ast.BinaryExpr)
	require.True(t, ok, "left of OR should be AND")
	require.Equal(t, "AND", andExpr.Op.Text)
}

func TestParse_PointersRefs(t *testing.T) {
	src := readTestdata(t, "pointers_refs.st")
	result := Parse("pointers_refs.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)

	require.Len(t, fb.VarBlocks, 1)
	decls := fb.VarBlocks[0].Declarations
	require.Len(t, decls, 3)

	// px : POINTER TO INT
	ptrType, ok := decls[0].Type.(*ast.PointerType)
	require.True(t, ok, "expected PointerType for px, got %T", decls[0].Type)
	namedBase, ok := ptrType.BaseType.(*ast.NamedType)
	require.True(t, ok)
	require.Equal(t, "INT", namedBase.Name.Name)

	// rx : REFERENCE TO REAL
	refType, ok := decls[1].Type.(*ast.ReferenceType)
	require.True(t, ok, "expected ReferenceType for rx, got %T", decls[1].Type)
	namedBase, ok = refType.BaseType.(*ast.NamedType)
	require.True(t, ok)
	require.Equal(t, "REAL", namedBase.Name.Name)

	// arr : ARRAY[0..9] OF POINTER TO BOOL
	arrType, ok := decls[2].Type.(*ast.ArrayType)
	require.True(t, ok, "expected ArrayType for arr")
	ptrElem, ok := arrType.ElementType.(*ast.PointerType)
	require.True(t, ok, "expected PointerType element")
	namedBase, ok = ptrElem.BaseType.(*ast.NamedType)
	require.True(t, ok)
	require.Equal(t, "BOOL", namedBase.Name.Name)

	// px^ := 42
	require.GreaterOrEqual(t, len(fb.Body), 1)
	assign, ok := fb.Body[0].(*ast.AssignStmt)
	require.True(t, ok)
	deref, ok := assign.Target.(*ast.DerefExpr)
	require.True(t, ok, "expected DerefExpr target, got %T", assign.Target)
	ident, ok := deref.Operand.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "px", ident.Name)
}

func TestParse_Pragmas(t *testing.T) {
	src := `{attribute 'qualified_only'}
FUNCTION_BLOCK FB_Test
VAR
    x : INT;
END_VAR
END_FUNCTION_BLOCK
`
	result := Parse("test.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags)

	require.Len(t, result.File.Declarations, 1)
	fb, ok := result.File.Declarations[0].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	require.Equal(t, "FB_Test", fb.Name.Name)
}

func TestParse_EmptyProgram(t *testing.T) {
	src := `PROGRAM Empty END_PROGRAM`
	result := Parse("test.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags)

	require.Len(t, result.File.Declarations, 1)
	prog, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	require.Equal(t, "Empty", prog.Name.Name)
	require.Empty(t, prog.Body)
	require.Empty(t, prog.VarBlocks)
}

func TestParse_MultiplePOUs(t *testing.T) {
	src := `PROGRAM P1
END_PROGRAM

FUNCTION_BLOCK FB1
END_FUNCTION_BLOCK

FUNCTION F1 : INT
END_FUNCTION
`
	result := Parse("test.st", src)

	require.NotNil(t, result.File)
	require.Empty(t, result.Diags)

	require.Len(t, result.File.Declarations, 3)
	_, ok := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	_, ok = result.File.Declarations[1].(*ast.FunctionBlockDecl)
	require.True(t, ok)
	_, ok = result.File.Declarations[2].(*ast.FunctionDecl)
	require.True(t, ok)
}
