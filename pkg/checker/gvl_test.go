package checker

import (
	"os"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gvlFile is one named source file for a multi-file checker run. The name
// matters: a GVL takes its name from the file basename.
type gvlFile struct{ name, src string }

func parseGVLFiles(t *testing.T, files []gvlFile) []*ast.SourceFile {
	t.Helper()
	var out []*ast.SourceFile
	for _, f := range files {
		r := parser.Parse(f.name, f.src)
		require.Empty(t, r.Diags, "parse diagnostics in %s", f.name)
		out = append(out, r.File)
	}
	return out
}

// runGVL runs resolve, body check and usage analysis over files in order.
func runGVL(t *testing.T, files []gvlFile, opts ...ResolveOpts) ([]diag.Diagnostic, *symbols.Table) {
	t.Helper()
	parsed := parseGVLFiles(t, files)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	NewResolver(table, diags).CollectDeclarations(parsed, opts...)
	NewChecker(table, diags).CheckBodies(parsed)
	CheckUsage(parsed, table, diags)
	return diags.All(), table
}

func errorsOf(ds []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Severity == diag.Error {
			out = append(out, d)
		}
	}
	return out
}

const gvlG = "VAR_GLOBAL\n\tx : INT;\nEND_VAR\n"
const gvlGQualified = "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tx : INT;\nEND_VAR\n"

func progUsing(body string) gvlFile {
	return gvlFile{"main.st", "PROGRAM P\nVAR\n\tb : BOOL;\nEND_VAR\n" + body + "\nEND_PROGRAM\n"}
}

func TestGVL(t *testing.T) {
	t.Run("qualified and bare access to a plain GVL", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{{"G.st", gvlG}, progUsing("G.x := 1; x := 2; b := b;")})
		assert.Empty(t, ds)
		sym := table.LookupGlobal("G")
		require.NotNil(t, sym)
		assert.Equal(t, symbols.KindGVL, sym.Kind)
		assert.Equal(t, "GVL", sym.Kind.String())
		st, ok := sym.Type.(*types.StructType)
		require.True(t, ok)
		assert.Equal(t, "G", st.Name)
	})

	t.Run("qualified_only on the GVL rejects bare access with SEMA033", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", gvlGQualified}, progUsing("G.x := 1; x := 2; b := b;")})
		got := diagsWithCode(ds, CodeGVLQualifiedOnly)
		require.Len(t, got, 1)
		assert.Equal(t, diag.Error, got[0].Severity)
		assert.Equal(t, "GVL 'G' is qualified_only; use G.x", got[0].Message)
		assert.Empty(t, diagsWithCode(ds, CodeUndeclared))
		assert.Len(t, errorsOf(ds), 1)
	})

	t.Run("qualified_only on the VarBlock is honoured", func(t *testing.T) {
		src := "VAR_GLOBAL\n\ty : INT;\nEND_VAR\n{attribute 'qualified_only'}\nVAR_GLOBAL\n\tx : INT;\nEND_VAR\n"
		ds, _ := runGVL(t, []gvlFile{{"G.st", src}, progUsing("G.x := 1; x := 2; b := b;")})
		require.Len(t, diagsWithCode(ds, CodeGVLQualifiedOnly), 1)
	})

	t.Run("qualified_only miss through a call reports SEMA033", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", gvlGQualified}, progUsing("b := x();")})
		require.Len(t, diagsWithCode(ds, CodeGVLQualifiedOnly), 1)
		assert.Empty(t, diagsWithCode(ds, CodeUndeclared))
	})

	t.Run("plain undeclared identifier still reports SEMA010", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", gvlGQualified}, progUsing("b := nothere; b := nocall();")})
		assert.Len(t, diagsWithCode(ds, CodeUndeclared), 2)
		assert.Empty(t, diagsWithCode(ds, CodeGVLQualifiedOnly))
	})

	t.Run("two qualified_only GVLs share a variable name", func(t *testing.T) {
		fb := gvlFile{"fb.st", "FUNCTION_BLOCK FB_Wagon\nVAR_OUTPUT\n\tDone : BOOL;\nEND_VAR\nEND_FUNCTION_BLOCK\n"}
		e := gvlFile{"EPW01.st", "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tWA01 : FB_Wagon;\nEND_VAR\n"}
		f := gvlFile{"FPW01.st", "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tWA01 : FB_Wagon;\nEND_VAR\n"}
		ds, _ := runGVL(t, []gvlFile{fb, e, f, progUsing("b := EPW01.WA01.Done; b := FPW01.WA01.Done;")})
		assert.Empty(t, ds)

		ds, _ = runGVL(t, []gvlFile{fb, e, f, progUsing("b := WA01.Done;")})
		got := diagsWithCode(ds, CodeGVLQualifiedOnly)
		require.Len(t, got, 1)
		assert.Equal(t, "GVL 'EPW01' is qualified_only; use EPW01.WA01 (also declared in FPW01)", got[0].Message)
	})

	t.Run("unknown member reports SEMA024", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", gvlG}, progUsing("G.nope := 1; b := b;")})
		require.Len(t, diagsWithCode(ds, CodeNoMember), 1)
	})

	t.Run("nested struct member through ECT", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/ECT.st")
		require.NoError(t, err)
		ds, _ := runGVL(t, []gvlFile{{"ECT.st", string(data)}, progUsing("b := ECT.X.I1;")})
		assert.Empty(t, ds)
		ds, _ = runGVL(t, []gvlFile{{"ECT.st", string(data)}, progUsing("ECT.X.I1 := 5;")})
		assert.Len(t, diagsWithCode(ds, CodeTypeMismatch), 1, "ECT.X.I1 should type as BOOL")
	})

	t.Run("assigning a VAR_GLOBAL CONSTANT reports SEMA034", func(t *testing.T) {
		src := "VAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		ds, _ := runGVL(t, []gvlFile{{"G.st", src}, progUsing("G.c := 1; c := 1; b := b;")})
		got := diagsWithCode(ds, CodeAssignToConstant)
		require.Len(t, got, 2)
		for _, d := range got {
			assert.Equal(t, diag.Error, d.Severity)
			assert.Equal(t, "cannot assign to constant 'c'", d.Message)
		}
	})

	t.Run("reading a constant and writing a non-constant are fine", func(t *testing.T) {
		src := "VAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\nVAR_GLOBAL\n\tv : INT;\nEND_VAR\n"
		ds, _ := runGVL(t, []gvlFile{{"G.st", src}, progUsing("v := c; G.v := G.c; b := b;")})
		assert.Empty(t, ds)
	})

	t.Run("qualified_only constant through member access reports SEMA034", func(t *testing.T) {
		src := "{attribute 'qualified_only'}\nVAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		ds, _ := runGVL(t, []gvlFile{{"G.st", src}, progUsing("G.c := 1; b := b;")})
		require.Len(t, diagsWithCode(ds, CodeAssignToConstant), 1)
	})

	t.Run("local variable shadowing a GVL constant is assignable", func(t *testing.T) {
		src := "VAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		prog := gvlFile{"main.st", "PROGRAM P\nVAR\n\tc : INT;\n\tG : INT;\nEND_VAR\nc := 1; G := c;\nEND_PROGRAM\n"}
		ds, _ := runGVL(t, []gvlFile{{"G.st", src}, prog})
		assert.Empty(t, diagsWithCode(ds, CodeAssignToConstant))
	})

	t.Run("GVL named from the file like an existing POU only warns", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"main.st", "PROGRAM G\nEND_PROGRAM\n"}, {"G.st", gvlG}})
		got := diagsWithCode(ds, CodeRedeclared)
		require.Len(t, got, 1)
		assert.Equal(t, diag.Warning, got[0].Severity)
		assert.Contains(t, got[0].Message, "--gvl-name")
	})

	t.Run("GVL and POU named after the same file keep the GVL variables", func(t *testing.T) {
		src := "VAR_GLOBAL\n\tgCount : DINT;\nEND_VAR\n\nPROGRAM Main\nVAR\n\tn : DINT;\nEND_VAR\ngCount := gCount + 1;\nn := gCount;\nEND_PROGRAM\n"
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
		require.Len(t, diagsWithCode(ds, CodeRedeclared), 1)
		assert.Equal(t, symbols.KindProgram, table.LookupGlobal("Main").Kind)
		require.NotNil(t, table.LookupGlobal("gCount"))
	})

	t.Run("explicit GVL name like an existing POU is a redeclaration", func(t *testing.T) {
		files := parseGVLFiles(t, []gvlFile{{"main.st", "PROGRAM G\nEND_PROGRAM\n"}, {"other.st", gvlG}})
		require.True(t, ast.SetGVLName(files[1], "G"))
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations(files)
		got := diagsWithCode(diags.All(), CodeRedeclared)
		require.Len(t, got, 1)
		assert.Equal(t, diag.Error, got[0].Severity)
		assert.Nil(t, table.LookupGlobal("x"))
	})

	t.Run("bare variable clash between plain GVLs is a redeclaration", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"A.st", gvlG}, {"B.st", gvlG}})
		require.Len(t, diagsWithCode(ds, CodeRedeclared), 1)
	})

	t.Run("library GVL: duplicates ignored, user GVL overrides", func(t *testing.T) {
		lib := parseGVLFiles(t, []gvlFile{{"G.st", gvlG}, {"G.st", gvlG}})
		ds, table := runGVL(t, []gvlFile{{"G.st", "VAR_GLOBAL\n\tz : INT;\n\tx : INT;\nEND_VAR\n"}, progUsing("G.z := 1; x := 1; b := b;")},
			ResolveOpts{LibraryFiles: lib})
		assert.Empty(t, errorsOf(ds))
		sym := table.LookupGlobal("G")
		require.NotNil(t, sym)
		assert.False(t, sym.IsLibrary)

		_, table = runGVL(t, nil, ResolveOpts{LibraryFiles: lib})
		require.NotNil(t, table.LookupGlobal("G"))
		assert.True(t, table.LookupGlobal("G").IsLibrary)
	})

	t.Run("user GVL overrides a library POU or qualified_only GVL", func(t *testing.T) {
		lib := parseGVLFiles(t, []gvlFile{{"lib.st", "PROGRAM G\nEND_PROGRAM\n"}, {"Q.st", gvlGQualified}})
		ds, table := runGVL(t, []gvlFile{{"G.st", gvlG}, {"Q.st", "VAR_GLOBAL\n\tq1 : INT;\nEND_VAR\n"}}, ResolveOpts{LibraryFiles: lib})
		assert.Empty(t, diagsWithCode(ds, CodeRedeclared))
		assert.Equal(t, symbols.KindGVL, table.LookupGlobal("G").Kind)
		assert.Nil(t, table.LookupPOU("G"))
		assert.False(t, table.LookupGlobal("Q").GVL.QualifiedOnly)
	})

	t.Run("GVLDecl without a name is skipped", func(t *testing.T) {
		file := parser.Parse("G.st", gvlG).File
		file.Declarations[0].(*ast.GVLDecl).Name = nil
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{file})
		assert.Empty(t, diags.All())
		assert.Nil(t, table.LookupGlobal("x"))
	})

	t.Run("AT addresses in a GVL", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", "VAR_GLOBAL\n\tin1 AT %IX0.0 : BOOL;\nEND_VAR\n"}})
		assert.Empty(t, diagsWithCode(ds, CodeATNotAllowedHere))
		assert.Empty(t, diagsWithCode(ds, CodeInvalidATAddress))
		ds, _ = runGVL(t, []gvlFile{{"G.st", "VAR_GLOBAL\n\tin1 AT %IX0.9 : BOOL;\nEND_VAR\n"}})
		assert.Len(t, diagsWithCode(ds, CodeInvalidATAddress), 1)
	})

	t.Run("invalid AT address token is a parse error", func(t *testing.T) {
		r := parser.Parse("G.st", "VAR_GLOBAL\n\tin1 AT %Z1 : BOOL;\nEND_VAR\n")
		assert.NotEmpty(t, r.Diags)
	})

	t.Run("GVL variables never warn as unused", func(t *testing.T) {
		plain := gvlFile{"G.st", "VAR_GLOBAL\n\tspare : INT;\nEND_VAR\n"}
		qual := gvlFile{"Q.st", "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tspare2 : INT;\nEND_VAR\n"}
		ds, _ := runGVL(t, []gvlFile{plain, qual, progUsing("b := b;")})
		assert.Empty(t, diagsWithCode(ds, CodeUnusedVar))
	})

	t.Run("probe gvl1 checks clean", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/gvl1.st")
		require.NoError(t, err)
		ds, _ := runGVL(t, []gvlFile{{"gvl1.st", string(data)}})
		assert.Empty(t, ds)
	})

	t.Run("forward reference to a DUT in a later file", func(t *testing.T) {
		gvl := gvlFile{"A_ECT.st", "VAR_GLOBAL\n\tX : ST_EL1008;\nEND_VAR\n"}
		dut := gvlFile{"Z_types.st", "TYPE ST_EL1008 :\nSTRUCT\n\tI1 AT %I* : BOOL;\nEND_STRUCT\nEND_TYPE\n"}
		prog := gvlFile{"main.st", "PROGRAM P\nVAR\n\tb : BOOL;\nEND_VAR\nb := A_ECT.X.I1;\nEND_PROGRAM\n"}
		ds, table := runGVL(t, []gvlFile{gvl, dut, prog})
		assert.Empty(t, ds)
		st := table.LookupGlobal("A_ECT").Type.(*types.StructType)
		require.Len(t, st.Members, 1)
		member, ok := st.Members[0].Type.(*types.StructType)
		require.True(t, ok, "member type is %T, want *types.StructType", st.Members[0].Type)
		assert.Equal(t, "ST_EL1008", member.Name)
		assert.True(t, strings.EqualFold(st.Members[0].Name, "X"))
	})
}
