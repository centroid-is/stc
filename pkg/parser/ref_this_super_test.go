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
