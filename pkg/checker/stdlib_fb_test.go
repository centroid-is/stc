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

// stdProg wraps body in a PROGRAM declaring one instance of fb as inst.
func stdProg(fb, body string) string {
	return "PROGRAM P\nVAR\n\tinst : " + fb + ";\n\tb : BOOL;\n\tn : INT;\n\td : TIME;\nEND_VAR\n" +
		body + "\nEND_PROGRAM\n"
}

func TestStdFB(t *testing.T) {
	clean := []struct{ fb, body string }{
		{"TON", "inst(IN := b, PT := T#1s); b := inst.Q; d := inst.ET;"},
		{"TOF", "inst(IN := b, PT := T#1s); b := inst.Q; d := inst.ET;"},
		{"TP", "inst(IN := b, PT := T#1s); b := inst.Q; d := inst.ET;"},
		{"CTU", "inst(CU := b, RESET := b, PV := n); b := inst.Q; n := inst.CV;"},
		{"CTU", "inst(CU := b, R := b, PV := 5);"},
		{"CTD", "inst(CD := b, LOAD := b, PV := n); b := inst.Q; n := inst.CV;"},
		{"CTD", "inst(CD := b, LD := b, PV := n);"},
		{"CTUD", "inst(CU := b, CD := b, RESET := b, LOAD := b, PV := n); b := inst.QU; b := inst.QD; n := inst.CV;"},
		{"CTUD", "inst(CU := b, CD := b, R := b, LD := b, PV := n);"},
		{"R_TRIG", "inst(CLK := b); b := inst.Q;"},
		{"F_TRIG", "inst(CLK := b); b := inst.Q;"},
		{"SR", "inst(SET1 := b, RESET := b); b := inst.Q1;"},
		{"SR", "inst(S1 := b, R := b);"},
		{"RS", "inst(SET := b, RESET1 := b); b := inst.Q1;"},
		{"RS", "inst(S := b, R1 := b);"},
		{"ton", "inst(in := b, pt := T#1s); b := inst.q;"},
	}
	for _, tc := range clean {
		t.Run(tc.fb+" "+tc.body, func(t *testing.T) {
			ds := runAction(t, stdProg(tc.fb, tc.body))
			assert.Empty(t, errorsOf(ds))
		})
	}

	t.Run("wrong parameter name", func(t *testing.T) {
		ds := errorsOf(runAction(t, stdProg("TON", "inst(IN := b, PTT := T#1s);")))
		require.Len(t, ds, 1)
		assert.Equal(t, CodeNoMember, ds[0].Code)
		assert.Contains(t, ds[0].Message, "PTT")
	})

	t.Run("wrong member", func(t *testing.T) {
		ds := errorsOf(runAction(t, stdProg("TON", "b := inst.QQ;")))
		require.Len(t, ds, 1)
		assert.Equal(t, CodeNoMember, ds[0].Code)
		assert.Contains(t, ds[0].Message, "has no member")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		ds := errorsOf(runAction(t, stdProg("TON", "inst(IN := b, PT := n);")))
		require.Len(t, ds, 1)
		assert.Equal(t, CodeWrongArgType, ds[0].Code)
	})

	t.Run("registered as library symbols with a POU scope", func(t *testing.T) {
		_, table := runGVL(t, []gvlFile{{"main.st", stdProg("TON", "inst(IN := b);")}})
		for _, name := range []string{"TON", "TOF", "TP", "CTU", "CTD", "CTUD", "R_TRIG", "F_TRIG", "SR", "RS"} {
			sym := table.LookupGlobal(name)
			require.NotNil(t, sym, name)
			assert.True(t, sym.IsLibrary, name)
			assert.Equal(t, symbols.KindFunctionBlock, sym.Kind, name)
			fb, ok := sym.Type.(*types.FunctionBlockType)
			require.True(t, ok, name)
			scope := table.LookupPOU(name)
			require.NotNil(t, scope, name)
			for _, p := range append(append([]types.Parameter{}, fb.Inputs...), fb.Outputs...) {
				assert.NotNil(t, scope.LookupLocal(p.Name), "%s.%s", name, p.Name)
			}
		}
		// Canonical inputs come first, in Tc2_Standard order.
		ctu := table.LookupGlobal("CTU").Type.(*types.FunctionBlockType)
		assert.Equal(t, []string{"CU", "RESET", "PV", "R"}, paramNames(ctu.Inputs))
	})

	t.Run("user FB named TON overrides the standard one", func(t *testing.T) {
		src := "FUNCTION_BLOCK TON\nVAR_INPUT\n\tgo : BOOL;\nEND_VAR\nEND_FUNCTION_BLOCK\n" +
			stdProg("TON", "inst(go := b);")
		ds := runAction(t, src)
		assert.Empty(t, errorsOf(ds))
		ds = errorsOf(runAction(t, src+"PROGRAM Q\nVAR\n\tt : TON;\nEND_VAR\nt(IN := TRUE);\nEND_PROGRAM\n"))
		require.Len(t, ds, 1, "the user TON has no IN input")
	})

	t.Run("library stub named TON overrides the standard one", func(t *testing.T) {
		lib := parser.Parse("lib.st", "FUNCTION_BLOCK TON\nVAR_INPUT\n\tgo : BOOL;\nEND_VAR\nEND_FUNCTION_BLOCK\n").File
		user := parseFile(stdProg("TON", "inst(go := b);"))
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{user}, ResolveOpts{LibraryFiles: []*ast.SourceFile{lib}})
		NewChecker(table, diags).CheckBodies([]*ast.SourceFile{user})
		assert.Empty(t, errorsOf(diags.All()))
	})

	t.Run("user globals and enum values shadow standard FB names", func(t *testing.T) {
		src := "TYPE E_Flip : (SR, RS, TP); END_TYPE\n" +
			"VAR_GLOBAL\n\tTON : INT;\nEND_VAR\n" +
			"PROGRAM P\nVAR\n\te : E_Flip;\n\ti : INT;\n\tc : CTU;\nEND_VAR\n" +
			"e := TP; e := SR; e := RS; i := TON; c(CU := TRUE, PV := 3); i := c.CV;\nEND_PROGRAM\n"
		ds := errorsOf(runAction(t, src))
		assert.Empty(t, ds)
	})

	t.Run("POU variables and inline enum values shadow standard FB names", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\tTP : INT;\n\tm : (TON, TOF);\nEND_VAR\nTP := 1; m := TOF;\nEND_PROGRAM\n"
		assert.Empty(t, errorsOf(runAction(t, src)))
	})

	t.Run("qualified enum values and GVL names shadow standard FB names", func(t *testing.T) {
		ds := errorsOf(runGVL2(t,
			gvlFile{"TOF.st", "VAR_GLOBAL\n\tx : INT;\nEND_VAR\n"},
			gvlFile{"main.st", "PROGRAM P\nVAR\n\tt : TON;\nEND_VAR\nt(IN := TRUE, PT := T#1s); TOF.x := 1;\nEND_PROGRAM\n"}))
		assert.Empty(t, ds)
	})

	t.Run("library GVL variable named like a standard FB wins", func(t *testing.T) {
		lib := parser.Parse("lib.st", "VAR_GLOBAL\n\tR_TRIG : BOOL;\nEND_VAR\n").File
		user := parseFile("PROGRAM P\nVAR\n\tb : BOOL;\nEND_VAR\nb := R_TRIG;\nEND_PROGRAM\n")
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations([]*ast.SourceFile{user}, ResolveOpts{LibraryFiles: []*ast.SourceFile{lib}})
		NewChecker(table, diags).CheckBodies([]*ast.SourceFile{user})
		assert.Empty(t, errorsOf(diags.All()))
	})

	t.Run("action_inside probe", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/action_inside.st")
		require.NoError(t, err)
		ds := errorsOf(runAction(t, string(data)))
		require.Len(t, ds, 1, "only the Phase 22 literal-typing error remains: %v", ds)
		assert.Equal(t, 13, ds[0].Pos.Line)
		for _, d := range ds {
			for _, word := range []string{"TON", "R_TRIG", "\"IN\"", "\"PT\"", "\"CLK\""} {
				assert.False(t, strings.Contains(d.Message, word), d.Message)
			}
		}
	})
}

func runGVL2(t *testing.T, files ...gvlFile) []diag.Diagnostic {
	t.Helper()
	ds, _ := runGVL(t, files)
	return ds
}

func paramNames(ps []types.Parameter) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}
