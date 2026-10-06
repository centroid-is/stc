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

	// The variable resolved before FB_Motor's declaration must be the final,
	// filled type object, not a placeholder.
	assert.Same(t, motorSym.Type, motorVar.Type, "forward reference must be pointer-stable")

	t.Run("member access through forward references", func(t *testing.T) {
		file := parseTestdata(t, "forward_ref_members.st")
		table, ds := resolveAndCheck(file)
		assert.Empty(t, errorsOf(ds), "expected no errors")

		main := table.LookupPOU("Main")
		require.NotNil(t, main)
		assert.Same(t, table.LookupGlobal("ST_A").Type, main.LookupLocal("a").Type)
		assert.Same(t, table.LookupGlobal("FB_Pump").Type, main.LookupLocal("pump").Type)

		t1, ok := main.LookupLocal("t1").Type.(types.Type)
		require.True(t, ok)
		assert.Equal(t, types.KindINT, t1.Kind(), "alias chain declared in reverse order")

		ta, ok := main.LookupLocal("ta").Type.(*types.ArrayType)
		require.True(t, ok, "array alias of a later struct")
		assert.Same(t, table.LookupGlobal("ST_B").Type, ta.ElementType)

		i, ok := main.LookupLocal("i").Type.(*types.FunctionBlockType)
		require.True(t, ok, "interface declared after use resolves to its shell")
		assert.Equal(t, "I_Motor", i.Name)
	})

	t.Run("self-referential alias terminates", func(t *testing.T) {
		file := parseFile("TYPE T : T; END_TYPE\nTYPE U : V; END_TYPE\nTYPE V : U; END_TYPE\n")
		_, _ = resolveAndCheck(file)
	})

	t.Run("user type overriding a library type wins for earlier uses", func(t *testing.T) {
		lib := parser.Parse("lib.st", "TYPE ST_X :\nSTRUCT\n    libOnly : INT;\nEND_STRUCT\nEND_TYPE\n"+
			"TYPE ST_X :\nSTRUCT\n    dup : INT;\nEND_STRUCT\nEND_TYPE\n").File
		user := parseFile("PROGRAM P\nVAR\n    x : ST_X;\n    n : INT;\nEND_VAR\nn := x.userOnly;\nEND_PROGRAM\n" +
			"TYPE ST_X :\nSTRUCT\n    userOnly : INT;\nEND_STRUCT\nEND_TYPE\n")
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{user}, ResolveOpts{LibraryFiles: []*ast.SourceFile{lib}})
		NewChecker(table, diags).CheckBodies([]*ast.SourceFile{user})
		assert.Empty(t, errorsOf(diags.All()))
		assert.False(t, table.LookupGlobal("ST_X").IsLibrary)
		assert.Same(t, table.LookupGlobal("ST_X").Type, table.LookupPOU("P").LookupLocal("x").Type)
	})

	t.Run("library-only type used before declaration", func(t *testing.T) {
		lib := parser.Parse("lib.st", "FUNCTION_BLOCK FB_L\nVAR_OUTPUT\n    s : ST_L;\nEND_VAR\nEND_FUNCTION_BLOCK\n"+
			"TYPE ST_L :\nSTRUCT\n    v : INT;\nEND_STRUCT\nEND_TYPE\n").File
		user := parseFile("PROGRAM P\nVAR\n    f : FB_L;\n    n : INT;\nEND_VAR\nn := f.s.v;\nEND_PROGRAM\n")
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{user}, ResolveOpts{LibraryFiles: []*ast.SourceFile{lib}})
		NewChecker(table, diags).CheckBodies([]*ast.SourceFile{user})
		assert.Empty(t, errorsOf(diags.All()))
	})

	t.Run("mock FB overriding a library FB", func(t *testing.T) {
		lib := parser.Parse("lib.st", "FUNCTION_BLOCK FB_M\nVAR_OUTPUT\n    a : INT;\nEND_VAR\nEND_FUNCTION_BLOCK\n").File
		mock := parser.Parse("mock.st", "FUNCTION_BLOCK FB_M\nVAR_OUTPUT\n    b : INT;\nEND_VAR\nEND_FUNCTION_BLOCK\n").File
		user := parseFile("PROGRAM P\nVAR\n    f : FB_M;\n    n : INT;\nEND_VAR\nn := f.b;\nEND_PROGRAM\n")
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{user},
			ResolveOpts{LibraryFiles: []*ast.SourceFile{lib}, MockFiles: []*ast.SourceFile{mock}})
		NewChecker(table, diags).CheckBodies([]*ast.SourceFile{user})
		assert.Empty(t, errorsOf(diags.All()))
	})

	t.Run("GVL member of a forward-declared struct", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{
			{"g.st", "VAR_GLOBAL\n    gs : ST_Later;\nEND_VAR\n"},
			{"main.st", "PROGRAM P\nVAR\n    n : INT;\nEND_VAR\nn := g.gs.v;\nn := gs.v;\nEND_PROGRAM\n" +
				"TYPE ST_Later :\nSTRUCT\n    v : INT;\nEND_STRUCT\nEND_TYPE\n"},
		})
		assert.Empty(t, errorsOf(ds))
	})
}

// resolveAndCheck runs pass 1 and pass 2 over a single file.
func resolveAndCheck(file *ast.SourceFile) (*symbols.Table, []diag.Diagnostic) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{file})
	NewChecker(table, diags).CheckBodies([]*ast.SourceFile{file})
	return table, diags.All()
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

func TestCollectDeclarations_LibraryFilesRegistered(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Position : REAL;
    Velocity : REAL;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
    Busy : BOOL;
    Error : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	userSrc := `PROGRAM Main
VAR
    mover : MC_MoveAbsolute;
END_VAR
END_PROGRAM
`
	libFile := parseFile(libSrc)
	userFile := parseFile(userSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{userFile}, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())

	// MC_MoveAbsolute should be registered
	mcSym := table.LookupGlobal("MC_MoveAbsolute")
	require.NotNil(t, mcSym, "MC_MoveAbsolute should be registered")
	assert.Equal(t, symbols.KindFunctionBlock, mcSym.Kind)

	// Main should reference it without errors
	mainSym := table.LookupGlobal("Main")
	require.NotNil(t, mainSym, "Main should be registered")
}

func TestLibrary_SymbolsMarkedIsLibrary(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Execute : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	libFile := parseFile(libSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations(nil, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())

	sym := table.LookupGlobal("MC_MoveAbsolute")
	require.NotNil(t, sym)
	assert.True(t, sym.IsLibrary, "library symbol should have IsLibrary=true")
}

func TestLibrary_UserOverridesLibrary(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MyFB
VAR_INPUT
    x : INT;
END_VAR
END_FUNCTION_BLOCK
`
	userSrc := `FUNCTION_BLOCK MyFB
VAR_INPUT
    x : INT;
    y : REAL;
END_VAR
END_FUNCTION_BLOCK
`
	libFile := parseFile(libSrc)
	userFile := parseFile(userSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{userFile}, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	// Should NOT produce redeclaration error -- user overrides library
	assert.False(t, diags.HasErrors(), "user override of library should not produce error, got: %v", diags.All())

	sym := table.LookupGlobal("MyFB")
	require.NotNil(t, sym)
	assert.False(t, sym.IsLibrary, "user-overridden symbol should not be marked as library")
}

func TestLibrary_FBParametersCorrectlyTyped(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Position : REAL;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
    Busy : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	libFile := parseFile(libSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations(nil, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	assert.False(t, diags.HasErrors())

	sym := table.LookupGlobal("MC_MoveAbsolute")
	require.NotNil(t, sym)
	fbType, ok := sym.Type.(*types.FunctionBlockType)
	require.True(t, ok, "should be FunctionBlockType")
	assert.Len(t, fbType.Inputs, 3)
	assert.Len(t, fbType.Outputs, 2)
	assert.Equal(t, "Axis", fbType.Inputs[0].Name)
	assert.Equal(t, types.KindINT, fbType.Inputs[0].Type.Kind())
	assert.Equal(t, "Done", fbType.Outputs[0].Name)
	assert.Equal(t, types.KindBOOL, fbType.Outputs[0].Type.Kind())
}

func TestLibrary_DuplicateLibraryFBSilentlyIgnored(t *testing.T) {
	lib1Src := `FUNCTION_BLOCK SharedFB
VAR_INPUT
    x : INT;
END_VAR
END_FUNCTION_BLOCK
`
	lib2Src := `FUNCTION_BLOCK SharedFB
VAR_INPUT
    x : REAL;
END_VAR
END_FUNCTION_BLOCK
`
	lib1File := parseFile(lib1Src)
	lib2File := parseFile(lib2Src)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations(nil, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{lib1File, lib2File},
	})

	// No error -- duplicate library FBs silently ignored
	assert.False(t, diags.HasErrors(), "duplicate library FBs should not produce error, got: %v", diags.All())

	sym := table.LookupGlobal("SharedFB")
	require.NotNil(t, sym)
	assert.True(t, sym.IsLibrary)
	// First library wins -- should have INT input
	fbType, ok := sym.Type.(*types.FunctionBlockType)
	require.True(t, ok)
	require.Len(t, fbType.Inputs, 1)
	assert.Equal(t, types.KindINT, fbType.Inputs[0].Type.Kind())
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

func TestMockFiles_OverrideLibrarySymbol(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	mockSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
    Done := Execute;
END_FUNCTION_BLOCK
`
	userSrc := `PROGRAM Main
VAR
    mover : MC_MoveAbsolute;
END_VAR
END_PROGRAM
`
	libFile := parseFile(libSrc)
	mockFile := parseFile(mockSrc)
	userFile := parseFile(userSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{userFile}, ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
		MockFiles:    []*ast.SourceFile{mockFile},
	})

	// No redeclaration errors
	assert.False(t, diags.HasErrors(), "mock override of library should not produce error, got: %v", diags.All())

	// The symbol should NOT be marked as library (mock is a real implementation)
	sym := table.LookupGlobal("MC_MoveAbsolute")
	require.NotNil(t, sym)
	assert.False(t, sym.IsLibrary, "mock-overridden symbol should not be marked as library")
}

func TestMockFiles_CannotOverrideUserSymbol(t *testing.T) {
	userSrc := `FUNCTION_BLOCK MyFB
VAR_INPUT
    x : INT;
END_VAR
END_FUNCTION_BLOCK
`
	mockSrc := `FUNCTION_BLOCK MyFB
VAR_INPUT
    x : INT;
END_VAR
    ;
END_FUNCTION_BLOCK
`
	mockFile := parseFile(mockSrc)
	userFile := parseFile(userSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{userFile}, ResolveOpts{
		MockFiles: []*ast.SourceFile{mockFile},
	})

	// Should produce redeclaration error -- mock cannot override user code
	assert.True(t, diags.HasErrors(), "mock override of user code should produce error")
}

// TestResolveWithoutPrePass covers the resolveX entry points called directly,
// without CollectDeclarations' pre-pass: each allocates its own type object.
func TestResolveWithoutPrePass(t *testing.T) {
	file := parseFile("PROGRAM P\nEND_PROGRAM\nFUNCTION_BLOCK FB\nEND_FUNCTION_BLOCK\nFUNCTION F : INT\nF := 1;\nEND_FUNCTION\n" +
		"TYPE S :\nSTRUCT\n    a : INT;\nEND_STRUCT\nEND_TYPE\nTYPE E : (one, two); END_TYPE\n")
	table := symbols.NewTable()
	r := NewResolver(table, diag.NewCollector())
	r.fbs = make(map[string]*fbEntry)
	r.resolveProgram(file.Declarations[0].(*ast.ProgramDecl), false)
	r.resolveFunctionBlock(file.Declarations[1].(*ast.FunctionBlockDecl), false)
	r.resolveFunction(file.Declarations[2].(*ast.FunctionDecl), false)
	r.resolveTypeDecl(file.Declarations[3].(*ast.TypeDecl), false)
	r.resolveTypeDecl(file.Declarations[4].(*ast.TypeDecl), false)

	assert.Equal(t, "P", table.LookupGlobal("P").Type.(*types.FunctionBlockType).Name)
	assert.Equal(t, "FB", table.LookupGlobal("FB").Type.(*types.FunctionBlockType).Name)
	assert.Equal(t, types.TypeINT, table.LookupGlobal("F").Type.(*types.FunctionType).ReturnType)
	assert.Equal(t, "S", table.LookupGlobal("S").Type.(*types.StructType).Name)
	assert.Equal(t, "E", table.LookupGlobal("E").Type.(*types.EnumType).Name)
}

// TestResolveLibraryOverrides covers the redeclaration rules of every POU
// kind against library symbols and the standard FBs.
func TestResolveLibraryOverrides(t *testing.T) {
	libSrc := "PROGRAM LP\nEND_PROGRAM\nPROGRAM LP\nEND_PROGRAM\n" +
		"FUNCTION LF : INT\nLF := 1;\nEND_FUNCTION\nFUNCTION LF : INT\nLF := 2;\nEND_FUNCTION\n" +
		"INTERFACE LI\nEND_INTERFACE\nINTERFACE LI\nEND_INTERFACE\n" +
		"FUNCTION TOF : BOOL\nTOF := TRUE;\nEND_FUNCTION\n" +
		"INTERFACE TP\nEND_INTERFACE\n" +
		"PROGRAM CTU\nEND_PROGRAM\n" +
		"TYPE R_TRIG : INT; END_TYPE\n"
	userSrc := "PROGRAM LP\nEND_PROGRAM\nFUNCTION LF : BOOL\nLF := TRUE;\nEND_FUNCTION\nINTERFACE LI\nEND_INTERFACE\n"
	ds, table := runGVL(t, []gvlFile{{"main.st", userSrc}}, ResolveOpts{LibraryFiles: parseGVLFiles(t, []gvlFile{{"lib.st", libSrc}})})
	assert.Empty(t, errorsOf(ds), "library duplicates are ignored; user code replaces library symbols")

	assert.False(t, table.LookupGlobal("LP").IsLibrary)
	assert.Equal(t, types.TypeBOOL, table.LookupGlobal("LF").Type.(*types.FunctionType).ReturnType)
	assert.False(t, table.LookupGlobal("LI").IsLibrary)
	assert.Equal(t, symbols.KindFunction, table.LookupGlobal("TOF").Kind, "library FUNCTION replaces standard TOF")
	assert.Equal(t, symbols.KindInterface, table.LookupGlobal("TP").Kind, "library INTERFACE replaces standard TP")
	assert.Equal(t, symbols.KindProgram, table.LookupGlobal("CTU").Kind, "library PROGRAM replaces standard CTU")
	assert.Equal(t, symbols.KindType, table.LookupGlobal("R_TRIG").Kind, "library TYPE replaces standard R_TRIG")
}

func TestResolveEdgeCases(t *testing.T) {
	t.Run("TYPE declaration without a name is skipped", func(t *testing.T) {
		file := &ast.SourceFile{Declarations: []ast.Declaration{&ast.TypeDecl{Type: &ast.StructType{}}}}
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{file})
		assert.Empty(t, diags.All())
	})

	t.Run("EXTENDS a FUNCTION is ignored", func(t *testing.T) {
		ds := runAction(t, "FUNCTION F : INT\nF := 1;\nEND_FUNCTION\nFUNCTION_BLOCK FB_X EXTENDS F\nEND_FUNCTION_BLOCK\n")
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("namespace-qualified symbol registered by a loader", func(t *testing.T) {
		table := symbols.NewTable()
		qualified := &types.StructType{Name: "Lib.ST_Q"}
		require.NoError(t, table.GlobalScope().Insert(&symbols.Symbol{Name: "Lib.ST_Q", Kind: symbols.KindType, Type: qualified, IsLibrary: true}))
		file := parseFile("PROGRAM P\nVAR\n    q : Lib.ST_Q;\nEND_VAR\nEND_PROGRAM\n")
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{file})
		assert.Empty(t, diags.All())
		assert.Same(t, qualified, table.LookupPOU("P").LookupLocal("q").Type)
	})
}
