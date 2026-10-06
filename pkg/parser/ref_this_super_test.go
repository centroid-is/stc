package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func requireDerefOf[T ast.Expr](t *testing.T, e ast.Expr) {
	t.Helper()
	d, ok := e.(*ast.DerefExpr)
	require.True(t, ok, "expected DerefExpr, got %T", e)
	_, ok = d.Operand.(T)
	require.True(t, ok, "unexpected deref operand %T", d.Operand)
}

func TestThisSuper(t *testing.T) {
	t.Run("THIS^ member assignment", func(t *testing.T) {
		as := bodyOf(t, "THIS^.x := 1;")[0].(*ast.AssignStmt)
		m, ok := as.Target.(*ast.MemberAccessExpr)
		require.True(t, ok, "got %T", as.Target)
		requireDerefOf[*ast.ThisExpr](t, m.Object)
		th := m.Object.(*ast.DerefExpr).Operand.(*ast.ThisExpr)
		require.Equal(t, ast.KindThisExpr, th.Kind())
	})

	t.Run("THIS^ method call statement", func(t *testing.T) {
		as := bodyOf(t, "THIS^.M();")[0].(*ast.AssignStmt)
		require.Nil(t, as.Value)
		call, ok := as.Target.(*ast.CallExpr)
		require.True(t, ok, "got %T", as.Target)
		m := call.Callee.(*ast.MemberAccessExpr)
		requireDerefOf[*ast.ThisExpr](t, m.Object)
	})

	t.Run("SUPER^ method call statement", func(t *testing.T) {
		as := bodyOf(t, "SUPER^.M();")[0].(*ast.AssignStmt)
		call := as.Target.(*ast.CallExpr)
		m := call.Callee.(*ast.MemberAccessExpr)
		requireDerefOf[*ast.SuperExpr](t, m.Object)
		su := m.Object.(*ast.DerefExpr).Operand.(*ast.SuperExpr)
		require.Equal(t, ast.KindSuperExpr, su.Kind())
	})

	t.Run("SUPER^ body call", func(t *testing.T) {
		as := bodyOf(t, "SUPER^();")[0].(*ast.AssignStmt)
		call, ok := as.Target.(*ast.CallExpr)
		require.True(t, ok, "got %T", as.Target)
		requireDerefOf[*ast.SuperExpr](t, call.Callee)
	})

	t.Run("THIS as a value", func(t *testing.T) {
		as := bodyOf(t, "p := THIS;")[0].(*ast.AssignStmt)
		_, ok := as.Value.(*ast.ThisExpr)
		require.True(t, ok, "got %T", as.Value)
	})

	t.Run("SUPER^ named-arg call is a CallStmt", func(t *testing.T) {
		cs, ok := bodyOf(t, "SUPER^.M(a := 1);")[0].(*ast.CallStmt)
		require.True(t, ok)
		m := cs.Callee.(*ast.MemberAccessExpr)
		requireDerefOf[*ast.SuperExpr](t, m.Object)
	})
}

func TestRefAssign(t *testing.T) {
	requireRef := func(t *testing.T, s ast.Statement) *ast.RefAssignStmt {
		t.Helper()
		ra, ok := s.(*ast.RefAssignStmt)
		require.True(t, ok, "expected RefAssignStmt, got %T", s)
		require.Equal(t, ast.KindRefAssignStmt, ra.Kind())
		return ra
	}

	t.Run("simple rebind", func(t *testing.T) {
		ra := requireRef(t, bodyOf(t, "r REF= n;")[0])
		require.Equal(t, "r", ra.Target.(*ast.Ident).Name)
		require.Equal(t, "n", ra.Value.(*ast.Ident).Name)
		require.Equal(t, 2, ra.Span().Start.Line)
		require.Greater(t, ra.Span().End.Col, ra.Value.Span().Start.Col)
	})

	t.Run("member value", func(t *testing.T) {
		ra := requireRef(t, bodyOf(t, "batches REF= settings.p_stat_Batches;")[0])
		_, ok := ra.Value.(*ast.MemberAccessExpr)
		require.True(t, ok)
	})

	t.Run("indexed member value", func(t *testing.T) {
		ra := requireRef(t, bodyOf(t, "r REF= arr[i].x;")[0])
		_, ok := ra.Value.(*ast.MemberAccessExpr)
		require.True(t, ok)
	})

	t.Run("case-insensitive REF", func(t *testing.T) {
		requireRef(t, bodyOf(t, "ref REF= n;\nr ref= n;\nr Ref= n;")[0])
	})

	t.Run("variable named REF still assigns", func(t *testing.T) {
		as, ok := bodyOf(t, "REF := 1;")[0].(*ast.AssignStmt)
		require.True(t, ok)
		require.Equal(t, "REF", as.Target.(*ast.Ident).Name)
	})

	t.Run("ptr probe parses clean", func(t *testing.T) {
		body := bodyOf(t, "p := ADR(n);\np^ := 5;\nr REF= n;\nr := 7;\nn := SIZEOF(arr);")
		require.Len(t, body, 5)
		requireRef(t, body[2])
	})

	t.Run("broken forms report errors without hanging", func(t *testing.T) {
		for _, src := range []string{"r REF= ;", "r REF n;", "r REF=", "r REF= n"} {
			_, msgs := parseBody(t, src)
			require.NotEmpty(t, msgs, "source %q must report a diagnostic", src)
		}
	})

	t.Run("missing value keeps following statement", func(t *testing.T) {
		body, msgs := parseBody(t, "r REF= ;\nx := 1;")
		require.NotEmpty(t, msgs)
		require.Len(t, body, 2)
		_, ok := body[1].(*ast.AssignStmt)
		require.True(t, ok)
	})
}
