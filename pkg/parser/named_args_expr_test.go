package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func assignCall(t *testing.T, s ast.Statement) *ast.CallExpr {
	t.Helper()
	as, ok := s.(*ast.AssignStmt)
	require.True(t, ok, "expected AssignStmt, got %T", s)
	call, ok := as.Value.(*ast.CallExpr)
	require.True(t, ok, "expected CallExpr value, got %T", as.Value)
	return call
}

func requireNamed(t *testing.T, a *ast.CallArg, name string, output bool) {
	t.Helper()
	require.Equal(t, ast.KindCallArg, a.Kind())
	if name == "" {
		require.Nil(t, a.Name)
	} else {
		require.NotNil(t, a.Name)
		require.Equal(t, name, a.Name.Name)
	}
	require.Equal(t, output, a.IsOutput)
}

func TestNamedArgsExpr(t *testing.T) {
	t.Run("all named in expression", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "n := F_X(a := 1, b := 2);")[0])
		require.Nil(t, call.Args)
		require.Len(t, call.NamedArgs, 2)
		requireNamed(t, call.NamedArgs[0], "a", false)
		requireNamed(t, call.NamedArgs[1], "b", false)
	})

	t.Run("positional only keeps Args shape", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "n := F_X(1, 2);")[0])
		require.Len(t, call.Args, 2)
		require.Nil(t, call.NamedArgs)
		require.Equal(t, "1", call.Args[0].(*ast.Literal).Value)
	})

	t.Run("empty parens", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "n := F_X();")[0])
		require.Nil(t, call.Args)
		require.Nil(t, call.NamedArgs)
	})

	t.Run("positional then named and output", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "x := f(1, b := 2, c => y);")[0])
		require.Len(t, call.Args, 1)
		require.Len(t, call.NamedArgs, 2)
		requireNamed(t, call.NamedArgs[0], "b", false)
		requireNamed(t, call.NamedArgs[1], "c", true)
	})

	t.Run("positional after named stays in NamedArgs", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "x := transition(current_state := q, transition_action.run, i_x := r);")[0])
		require.Nil(t, call.Args)
		require.Len(t, call.NamedArgs, 3)
		requireNamed(t, call.NamedArgs[0], "current_state", false)
		requireNamed(t, call.NamedArgs[1], "", false)
		_, ok := call.NamedArgs[1].Value.(*ast.MemberAccessExpr)
		require.True(t, ok)
		requireNamed(t, call.NamedArgs[2], "i_x", false)
	})

	t.Run("mixed named positional named", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "x := f(a := 1, x, c := 3);")[0])
		require.Len(t, call.NamedArgs, 3)
		requireNamed(t, call.NamedArgs[1], "", false)
	})

	t.Run("trailing comma after named", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "x := f(a := 1, );")[0])
		require.Len(t, call.NamedArgs, 1)
	})

	t.Run("trailing comma after output arg and newline", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "x := f(a := 1, q => y,\n);")[0])
		require.Len(t, call.NamedArgs, 2)
		requireNamed(t, call.NamedArgs[1], "q", true)
	})

	t.Run("trailing comma in CallStmt", func(t *testing.T) {
		cs, ok := bodyOf(t, "SPB03.speedBatcher(a := x, q_xDropComplete => y,\n);")[0].(*ast.CallStmt)
		require.True(t, ok)
		require.Len(t, cs.Args, 2)
	})

	t.Run("trailing comma after positional", func(t *testing.T) {
		as := bodyOf(t, "f(1, );")[0].(*ast.AssignStmt)
		call := as.Target.(*ast.CallExpr)
		require.Len(t, call.Args, 1)
		call = assignCall(t, bodyOf(t, "x := f(1, );")[0])
		require.Len(t, call.Args, 1)
	})

	t.Run("empty named value in expression", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "k := G(a :=, 7);")[0])
		require.Len(t, call.NamedArgs, 2)
		require.True(t, call.NamedArgs[0].Value == nil)
	})

	t.Run("broken comma lists report errors", func(t *testing.T) {
		for _, src := range []string{"x := f(,);", "x := f(a := 1,,);", "f(,,,);", "x := f(a := 1,"} {
			_, msgs := parseBody(t, src)
			require.NotEmpty(t, msgs, "source %q must report a diagnostic", src)
		}
	})

	t.Run("statement head FB call stays CallStmt", func(t *testing.T) {
		cs, ok := bodyOf(t, "fb(IN := x, PT := T#1s);")[0].(*ast.CallStmt)
		require.True(t, ok)
		require.Len(t, cs.Args, 2)
		for _, a := range cs.Args {
			require.Equal(t, ast.KindCallArg, a.Kind())
		}
	})

	t.Run("statement head method call stays CallStmt", func(t *testing.T) {
		cs, ok := bodyOf(t, "inst.M(a := 1);")[0].(*ast.CallStmt)
		require.True(t, ok)
		_, ok = cs.Callee.(*ast.MemberAccessExpr)
		require.True(t, ok)
	})

	t.Run("statement head positional CallStmt args", func(t *testing.T) {
		cs, ok := bodyOf(t, "fb(a := 1, x);")[0].(*ast.CallStmt)
		require.True(t, ok)
		require.Len(t, cs.Args, 2)
		require.Equal(t, ast.KindCallArg, cs.Args[1].Kind())
	})

	t.Run("named call inside index at statement head", func(t *testing.T) {
		as := bodyOf(t, "a[f(x := 1)] := 2;")[0].(*ast.AssignStmt)
		idx := as.Target.(*ast.IndexExpr)
		call, ok := idx.Indices[0].(*ast.CallExpr)
		require.True(t, ok, "got %T", idx.Indices[0])
		require.Len(t, call.NamedArgs, 1)
	})

	t.Run("named call inside call args at statement head", func(t *testing.T) {
		as := bodyOf(t, "g(f(x := 1));")[0].(*ast.AssignStmt)
		outer := as.Target.(*ast.CallExpr)
		inner, ok := outer.Args[0].(*ast.CallExpr)
		require.True(t, ok)
		require.Len(t, inner.NamedArgs, 1)
	})

	t.Run("named call inside IF condition", func(t *testing.T) {
		ifs := bodyOf(t, "IF f(a := 1) > 0 THEN\n  y := 1;\nEND_IF")[0].(*ast.IfStmt)
		bin := ifs.Condition.(*ast.BinaryExpr)
		call, ok := bin.Left.(*ast.CallExpr)
		require.True(t, ok)
		require.Len(t, call.NamedArgs, 1)
	})

	t.Run("THIS^ method with named args in expression", func(t *testing.T) {
		call := assignCall(t, bodyOf(t, "SendString := THIS^.SendBytes(pData := ADR(str), nLen := LEN(str));")[0])
		require.Len(t, call.NamedArgs, 2)
		m := call.Callee.(*ast.MemberAccessExpr)
		requireDerefOf[*ast.ThisExpr](t, m.Object)
	})

	t.Run("link fixture parses clean", func(t *testing.T) {
		r := Parse("link.st", readProbe(t, "link.st"))
		require.Empty(t, r.Diags, "unexpected diagnostics: %v", r.Diags)
	})
}
