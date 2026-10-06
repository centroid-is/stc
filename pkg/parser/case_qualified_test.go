package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func caseOf(t *testing.T, branches string) *ast.CaseStmt {
	t.Helper()
	body := bodyOf(t, "CASE e OF\n"+branches+"\nEND_CASE")
	require.Len(t, body, 1)
	cs, ok := body[0].(*ast.CaseStmt)
	require.True(t, ok, "got %T", body[0])
	return cs
}

func labelCounts(cs *ast.CaseStmt) []int {
	var out []int
	for _, b := range cs.Branches {
		out = append(out, len(b.Labels))
	}
	return out
}

func TestCaseQualifiedLabels(t *testing.T) {
	t.Run("qualified enum labels", func(t *testing.T) {
		cs := caseOf(t, "lft_e.no_fault:\n  n := 1;\nlft_e.eef1:\n  n := 2;\nstates.ready_to_switch_on, states.switched_on:\n  n := 3;")
		require.Equal(t, []int{1, 1, 2}, labelCounts(cs))
		v := cs.Branches[1].Labels[0].(*ast.CaseLabelValue)
		m, ok := v.Value.(*ast.MemberAccessExpr)
		require.True(t, ok)
		require.Equal(t, "eef1", m.Member.Name)
		for _, b := range cs.Branches {
			require.Len(t, b.Body, 1)
		}
	})

	t.Run("qualified range", func(t *testing.T) {
		cs := caseOf(t, "E.a:\n  n := 0;\nE.a..E.c:\n  n := 1;")
		require.Equal(t, []int{1, 1}, labelCounts(cs))
		r, ok := cs.Branches[1].Labels[0].(*ast.CaseLabelRange)
		require.True(t, ok)
		_, ok = r.High.(*ast.MemberAccessExpr)
		require.True(t, ok)
	})

	t.Run("namespace qualified label", func(t *testing.T) {
		cs := caseOf(t, "1:\n  n := 0;\nLib.E.v:\n  n := 1;")
		require.Equal(t, []int{1, 1}, labelCounts(cs))
	})

	t.Run("typed enum label", func(t *testing.T) {
		cs := caseOf(t, "1:\n  n := 0;\nE#a:\n  n := 1;\nE#b, E#c:\n  n := 2;")
		require.Equal(t, []int{1, 1, 2}, labelCounts(cs))
		lit := cs.Branches[1].Labels[0].(*ast.CaseLabelValue).Value.(*ast.Literal)
		require.Equal(t, "E", lit.TypePrefix)
	})

	t.Run("typed literal label", func(t *testing.T) {
		cs := caseOf(t, "1:\n  n := 0;\nINT#5 :\n  n := 1;")
		require.Equal(t, []int{1, 1}, labelCounts(cs))
	})

	t.Run("negative label", func(t *testing.T) {
		cs := caseOf(t, "1:\n  n := 0;\n-1:\n  n := 1;\n-3..-2:\n  n := 2;")
		require.Equal(t, []int{1, 1, 1}, labelCounts(cs))
	})

	t.Run("member assignment in body is not a label", func(t *testing.T) {
		cs := caseOf(t, "E.a:\n  a.b := 1;\n  a.b.c := 2;\n  x := 3;\nE.b:\n  n := 1;")
		require.Equal(t, []int{1, 1}, labelCounts(cs))
		require.Len(t, cs.Branches[0].Body, 3)
	})

	t.Run("minus expression in body is not a label", func(t *testing.T) {
		// A statement never starts with '-', but '-' followed by a
		// non-integer must not be treated as a label start.
		_, msgs := parseBody(t, "CASE e OF\n1:\n  n := 0;\n-x:\n  n := 1;\nEND_CASE")
		require.NotEmpty(t, msgs)
	})

	t.Run("case probe parses clean", func(t *testing.T) {
		r := Parse("case.st", "TYPE E_X : (a, b, c);\nEND_TYPE\nPROGRAM MAIN\nVAR e : E_X; n : INT; END_VAR\nCASE e OF\n\tE_X.a: n := 1;\n\tE_X.b, E_X.c: n := 2;\nEND_CASE\nEND_PROGRAM\n")
		require.Empty(t, r.Diags, "unexpected diagnostics: %v", r.Diags)
	})
}
