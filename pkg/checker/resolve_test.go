package checker

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseFile parses an ST source string and returns the AST.
func parseFile(src string) *ast.SourceFile {
	result := parser.Parse("test.st", src)
	return result.File
}

// parseTestdata reads and parses a testdata file.
func parseTestdata(t *testing.T, name string) *ast.SourceFile {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return parseFile(string(data))
}

func TestResolveProgram(t *testing.T) {
	file := parseTestdata(t, "valid_program.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())

	// Check POU registered
	sym := table.LookupGlobal("Main")
	require.NotNil(t, sym, "Main program should be registered")
	assert.Equal(t, symbols.KindProgram, sym.Kind)

	// Check POU scope has variables
	pouScope := table.LookupPOU("Main")
	require.NotNil(t, pouScope, "Main POU scope should exist")

	xSym := pouScope.LookupLocal("x")
	require.NotNil(t, xSym, "variable x should be registered in Main scope")
	assert.Equal(t, symbols.KindVariable, xSym.Kind)
	xType, ok := xSym.Type.(types.Type)
	require.True(t, ok, "x.Type should be a types.Type")
	assert.Equal(t, types.KindINT, xType.Kind())

	ySym := pouScope.LookupLocal("y")
	require.NotNil(t, ySym, "variable y should be registered")
	yType, ok := ySym.Type.(types.Type)
	require.True(t, ok, "y.Type should be a types.Type")
	assert.Equal(t, types.KindREAL, yType.Kind())

	zSym := pouScope.LookupLocal("z")
	require.NotNil(t, zSym, "variable z should be registered")
	zType, ok := zSym.Type.(types.Type)
	require.True(t, ok, "z.Type should be a types.Type")
	assert.Equal(t, types.KindBOOL, zType.Kind())
}

func TestResolveForwardRef(t *testing.T) {
	file := parseTestdata(t, "forward_ref.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())

	// Both FBs should be registered
	pumpSym := table.LookupGlobal("FB_Pump")
	require.NotNil(t, pumpSym, "FB_Pump should be registered")
	assert.Equal(t, symbols.KindFunctionBlock, pumpSym.Kind)

	motorSym := table.LookupGlobal("FB_Motor")
	require.NotNil(t, motorSym, "FB_Motor should be registered")
	assert.Equal(t, symbols.KindFunctionBlock, motorSym.Kind)

	// FB_Pump's scope should have a variable 'motor'
	pumpScope := table.LookupPOU("FB_Pump")
	require.NotNil(t, pumpScope)
	motorVar := pumpScope.LookupLocal("motor")
	require.NotNil(t, motorVar, "motor variable should be in FB_Pump scope")

	// FB_Motor's scope should have a variable 'speed'
	motorScope := table.LookupPOU("FB_Motor")
	require.NotNil(t, motorScope)
	speedVar := motorScope.LookupLocal("speed")
	require.NotNil(t, speedVar, "speed variable should be in FB_Motor scope")
}

func TestResolveTypeDecl(t *testing.T) {
	src := `
TYPE E_Color : (Red, Green, Blue);
END_TYPE

TYPE S_Point :
STRUCT
    x : REAL;
    y : REAL;
END_STRUCT;
END_TYPE
`
	file := parseFile(src)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())

	// Check enum type registered
	colorSym := table.LookupGlobal("E_Color")
	require.NotNil(t, colorSym, "E_Color type should be registered")
	assert.Equal(t, symbols.KindType, colorSym.Kind)
	colorType, ok := colorSym.Type.(*types.EnumType)
	require.True(t, ok, "E_Color should be EnumType")
	assert.Equal(t, "E_Color", colorType.Name)
	assert.Equal(t, []string{"Red", "Green", "Blue"}, colorType.Values)

	// Check enum values registered globally
	redSym := table.LookupGlobal("Red")
	require.NotNil(t, redSym, "Red enum value should be registered")
	assert.Equal(t, symbols.KindEnumValue, redSym.Kind)

	// Check struct type registered
	pointSym := table.LookupGlobal("S_Point")
	require.NotNil(t, pointSym, "S_Point type should be registered")
	assert.Equal(t, symbols.KindType, pointSym.Kind)
	pointType, ok := pointSym.Type.(*types.StructType)
	require.True(t, ok, "S_Point should be StructType")
	assert.Equal(t, "S_Point", pointType.Name)
	require.Len(t, pointType.Members, 2)
	assert.Equal(t, "x", pointType.Members[0].Name)
	assert.Equal(t, types.KindREAL, pointType.Members[0].Type.Kind())
}

func TestResolveRedeclaration(t *testing.T) {
	src := `
PROGRAM Main
VAR
    x : INT;
END_VAR
END_PROGRAM

PROGRAM Main
VAR
    y : INT;
END_VAR
END_PROGRAM
`
	file := parseFile(src)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	require.True(t, diags.HasErrors(), "expected redeclaration error")
	errors := diags.Errors()
	require.Len(t, errors, 1)
	assert.Equal(t, CodeRedeclared, errors[0].Code)
	assert.Contains(t, errors[0].Message, "Main")
}
