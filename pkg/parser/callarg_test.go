package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func firstCallStmt(t *testing.T, f *ast.SourceFile) *ast.CallStmt {
	t.Helper()
	prog := f.Declarations[0].(*ast.ProgramDecl)
	for _, s := range prog.Body {
		if cs, ok := s.(*ast.CallStmt); ok {
			return cs
		}
	}
	t.Fatal("no CallStmt in body")
	return nil
}

func TestEmptyArg(t *testing.T) {
	const decls = "PROGRAM P\nVAR\n\tb : BOOL;\n\tt : TON;\nEND_VAR\n"

	t.Run("spaced empty args", func(t *testing.T) {
		f := parseClean(t, decls+"t(IN := b, PT := , Q => , ET => );\nEND_PROGRAM\n")
		cs := firstCallStmt(t, f)
		require.Len(t, cs.Args, 4)
		require.NotNil(t, cs.Args[0].Value)
		for i, name := range []string{"PT", "Q", "ET"} {
			a := cs.Args[i+1]
			require.Equal(t, name, a.Name.Name)
			require.True(t, a.Value == nil, "arg %s Value must be an untyped nil interface", name)
		}
		require.False(t, cs.Args[1].IsOutput)
		require.True(t, cs.Args[2].IsOutput)
		require.True(t, cs.Args[3].IsOutput)
	})

	t.Run("unspaced empty args", func(t *testing.T) {
		f := parseClean(t, decls+"t(PT :=, Q =>);\nEND_PROGRAM\n")
		cs := firstCallStmt(t, f)
		require.Len(t, cs.Args, 2)
		require.True(t, cs.Args[0].Value == nil)
		require.False(t, cs.Args[0].IsOutput)
		require.True(t, cs.Args[1].Value == nil)
		require.True(t, cs.Args[1].IsOutput)
	})

	t.Run("empty arg span ends at operator", func(t *testing.T) {
		f := parseClean(t, decls+"t(PT :=);\nEND_PROGRAM\n")
		a := firstCallStmt(t, f).Args[0]
		require.Equal(t, a.Name.Span().Start.Line, a.Span().End.Line)
		require.Greater(t, a.Span().End.Col, a.Name.Span().End.Col)
	})

	t.Run("prog fixture has no errors on AT and call lines", func(t *testing.T) {
		r := Parse("prog.st", readProbe(t, "prog.st"))
		for _, d := range r.Diags {
			require.NotContains(t, []int{3, 4, 10, 11}, d.Pos.Line, "unexpected diagnostic: %v", d)
		}
	})
}
