package parser

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

// bodyOf wraps stmts in a PROGRAM, requires a clean parse and returns the body.
func bodyOf(t *testing.T, stmts string) []ast.Statement {
	t.Helper()
	f := parseClean(t, "PROGRAM P\n"+stmts+"\nEND_PROGRAM\n")
	return f.Declarations[0].(*ast.ProgramDecl).Body
}

// parseBody parses stmts in a PROGRAM and returns the body and diagnostics.
func parseBody(t *testing.T, stmts string) ([]ast.Statement, []string) {
	t.Helper()
	r := Parse("t.st", "PROGRAM P\n"+stmts+"\nEND_PROGRAM\n")
	var msgs []string
	for _, d := range r.Diags {
		msgs = append(msgs, d.Message)
	}
	prog, ok := r.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	return prog.Body, msgs
}

func requireBitAccess(t *testing.T, e ast.Expr, index string) *ast.BitAccessExpr {
	t.Helper()
	ba, ok := e.(*ast.BitAccessExpr)
	require.True(t, ok, "expected BitAccessExpr, got %T", e)
	require.Equal(t, ast.KindBitAccessExpr, ba.Kind())
	lit, ok := ba.Index.(*ast.Literal)
	require.True(t, ok, "index must be a Literal, got %T", ba.Index)
	require.Equal(t, ast.LitInt, lit.LitKind)
	require.Equal(t, index, lit.Value)
	return ba
}

func TestBitAccess(t *testing.T) {
	t.Run("simple read", func(t *testing.T) {
		as := bodyOf(t, "b := w.3;")[0].(*ast.AssignStmt)
		ba := requireBitAccess(t, as.Value, "3")
		require.Equal(t, "w", ba.Target.(*ast.Ident).Name)
		require.Equal(t, as.Value.Span().Start, ba.Target.Span().Start)
		require.Greater(t, ba.Span().End.Col, ba.Target.Span().End.Col)
	})

	t.Run("member chain target", func(t *testing.T) {
		as := bodyOf(t, "x := ECT.EPW01_WA01_FD01.q_wDigitalInputs.0;")[0].(*ast.AssignStmt)
		ba := requireBitAccess(t, as.Value, "0")
		m, ok := ba.Target.(*ast.MemberAccessExpr)
		require.True(t, ok, "target must be MemberAccessExpr, got %T", ba.Target)
		require.Equal(t, "q_wDigitalInputs", m.Member.Name)
		inner, ok := m.Object.(*ast.MemberAccessExpr)
		require.True(t, ok)
		require.Equal(t, "EPW01_WA01_FD01", inner.Member.Name)
	})

	t.Run("write over index expr", func(t *testing.T) {
		as := bodyOf(t, "Modbus.brettakerfi_Write[0].0 := b.Q;")[0].(*ast.AssignStmt)
		ba := requireBitAccess(t, as.Target, "0")
		_, ok := ba.Target.(*ast.IndexExpr)
		require.True(t, ok, "target must be IndexExpr, got %T", ba.Target)
		_, ok = as.Value.(*ast.MemberAccessExpr)
		require.True(t, ok)
	})

	t.Run("write with NOT", func(t *testing.T) {
		as := bodyOf(t, "create_cmd.8 := NOT run;")[0].(*ast.AssignStmt)
		requireBitAccess(t, as.Target, "8")
		_, ok := as.Value.(*ast.UnaryExpr)
		require.True(t, ok)
	})

	t.Run("bits in boolean expression", func(t *testing.T) {
		as := bodyOf(t, "x := w.0 OR w.1;")[0].(*ast.AssignStmt)
		bin, ok := as.Value.(*ast.BinaryExpr)
		require.True(t, ok)
		requireBitAccess(t, bin.Left, "0")
		requireBitAccess(t, bin.Right, "1")
	})

	t.Run("modbus array element bit", func(t *testing.T) {
		as := bodyOf(t, "Modbus.arr[0].3 := TRUE;")[0].(*ast.AssignStmt)
		requireBitAccess(t, as.Target, "3")
	})

	t.Run("huge index kept as text", func(t *testing.T) {
		as := bodyOf(t, "b := w.99999999999999999999;")[0].(*ast.AssignStmt)
		requireBitAccess(t, as.Value, "99999999999999999999")
	})

	t.Run("bit on bit is an error", func(t *testing.T) {
		body, msgs := parseBody(t, "b := w.3.1;\nc := 1;")
		require.NotEmpty(t, msgs)
		require.Contains(t, strings.Join(msgs, "\n"), "bit access on a bit")
		as := body[0].(*ast.AssignStmt)
		outer := requireBitAccess(t, as.Value, "1")
		requireBitAccess(t, outer.Target, "3")
		require.Len(t, body, 2, "recovery must continue with the next statement")
	})

	t.Run("bit on bit with int literal", func(t *testing.T) {
		// (w.3).1 is not lexable as Dot IntLiteral twice; use an index between.
		_, msgs := parseBody(t, "b := w.3.1.2;")
		require.Contains(t, strings.Join(msgs, "\n"), "bit access on a bit")
	})

	t.Run("real literal that is not a bit pair", func(t *testing.T) {
		_, msgs := parseBody(t, "b := w.3.1e5;")
		require.NotEmpty(t, msgs)
	})

	t.Run("dot then semicolon reports expected identifier", func(t *testing.T) {
		_, msgs := parseBody(t, "b := w.;")
		require.Contains(t, strings.Join(msgs, "\n"), "expected identifier")
	})

	t.Run("constant index stays member access", func(t *testing.T) {
		as := bodyOf(t, "x := v.cEnable;")[0].(*ast.AssignStmt)
		m, ok := as.Value.(*ast.MemberAccessExpr)
		require.True(t, ok, "got %T", as.Value)
		require.Equal(t, "cEnable", m.Member.Name)
	})

	t.Run("prog fixture parses clean", func(t *testing.T) {
		r := Parse("prog.st", readProbe(t, "prog.st"))
		require.Empty(t, r.Diags, "unexpected diagnostics: %v", r.Diags)
	})
}
