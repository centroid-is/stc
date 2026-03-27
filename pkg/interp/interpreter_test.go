package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a literal expression
func intLit(v string) *ast.Literal {
	return &ast.Literal{
		NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
		LitKind:  ast.LitInt,
		Value:    v,
	}
}

func realLit(v string) *ast.Literal {
	return &ast.Literal{
		NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
		LitKind:  ast.LitReal,
		Value:    v,
	}
}

func boolLit(v string) *ast.Literal {
	return &ast.Literal{
		NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
		LitKind:  ast.LitBool,
		Value:    v,
	}
}

func strLit(v string) *ast.Literal {
	return &ast.Literal{
		NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
		LitKind:  ast.LitString,
		Value:    v,
	}
}

func timeLit(v string) *ast.Literal {
	return &ast.Literal{
		NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
		LitKind:  ast.LitTime,
		Value:    v,
	}
}

func ident(name string) *ast.Ident {
	return &ast.Ident{
		NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
		Name:     name,
	}
}

func binExpr(left ast.Expr, op string, right ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindBinaryExpr},
		Left:     left,
		Op:       ast.Token{Text: op},
		Right:    right,
	}
}

func unaryExpr(op string, operand ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindUnaryExpr},
		Op:       ast.Token{Text: op},
		Operand:  operand,
	}
}

func parenExpr(inner ast.Expr) *ast.ParenExpr {
	return &ast.ParenExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindParenExpr},
		Inner:    inner,
	}
}

func assignStmt(target string, value ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt},
		Target:   ident(target),
		Value:    value,
	}
}

// --- Literal evaluation tests ---

func TestEvalLiteralInt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, intLit("42"))
	require.NoError(t, err)
	assert.Equal(t, ValInt, v.Kind)
	assert.Equal(t, int64(42), v.Int)
}

func TestEvalLiteralReal(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, realLit("3.14"))
	require.NoError(t, err)
	assert.Equal(t, ValReal, v.Kind)
	assert.InDelta(t, 3.14, v.Real, 0.001)
}

func TestEvalLiteralBoolTrue(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, boolLit("TRUE"))
	require.NoError(t, err)
	assert.Equal(t, ValBool, v.Kind)
	assert.True(t, v.Bool)
}

func TestEvalLiteralBoolFalse(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, boolLit("FALSE"))
	require.NoError(t, err)
	assert.Equal(t, ValBool, v.Kind)
	assert.False(t, v.Bool)
}

func TestEvalLiteralString(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, strLit("'hello'"))
	require.NoError(t, err)
	assert.Equal(t, ValString, v.Kind)
	assert.Equal(t, "hello", v.Str)
}

func TestEvalLiteralTime(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, timeLit("T#5s"))
	require.NoError(t, err)
	assert.Equal(t, ValTime, v.Kind)
	assert.Equal(t, 5*time.Second, v.Time)
}

func TestEvalLiteralTimeCompound(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, timeLit("T#1h2m3s"))
	require.NoError(t, err)
	assert.Equal(t, ValTime, v.Kind)
	expected := 1*time.Hour + 2*time.Minute + 3*time.Second
	assert.Equal(t, expected, v.Time)
}

func TestEvalLiteralTimeMilliseconds(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, timeLit("T#100ms"))
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, v.Time)
}

func TestEvalLiteralHexInt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, intLit("16#FF"))
	require.NoError(t, err)
	assert.Equal(t, int64(255), v.Int)
}

func TestEvalLiteralBinaryInt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, intLit("2#1010"))
	require.NoError(t, err)
	assert.Equal(t, int64(10), v.Int)
}

func TestEvalLiteralOctalInt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, intLit("8#77"))
	require.NoError(t, err)
	assert.Equal(t, int64(63), v.Int)
}

// --- Binary expression tests ---

func TestEvalBinaryAdd(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("3"), "+", intLit("4")))
	require.NoError(t, err)
	assert.Equal(t, int64(7), v.Int)
}

func TestEvalBinarySub(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("10"), "-", intLit("3")))
	require.NoError(t, err)
	assert.Equal(t, int64(7), v.Int)
}

func TestEvalBinaryMul(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("3"), "*", intLit("4")))
	require.NoError(t, err)
	assert.Equal(t, int64(12), v.Int)
}

func TestEvalBinaryIntDiv(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("10"), "/", intLit("3")))
	require.NoError(t, err)
	assert.Equal(t, int64(3), v.Int)
}

func TestEvalBinaryRealDiv(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(realLit("10.0"), "/", realLit("3.0")))
	require.NoError(t, err)
	assert.InDelta(t, 3.333, v.Real, 0.01)
}

func TestEvalBinaryMod(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("10"), "MOD", intLit("3")))
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.Int)
}

func TestEvalBinaryPower(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("2"), "**", intLit("3")))
	require.NoError(t, err)
	assert.Equal(t, ValReal, v.Kind)
	assert.InDelta(t, 8.0, v.Real, 0.001)
}

func TestEvalBinaryGreater(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("5"), ">", intLit("3")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryLess(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("3"), "<", intLit("5")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryEqual(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("5"), "=", intLit("5")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryNotEqual(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("5"), "<>", intLit("3")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryLessEqual(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("5"), "<=", intLit("5")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryGreaterEqual(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(intLit("5"), ">=", intLit("5")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryAnd(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(boolLit("TRUE"), "AND", boolLit("FALSE")))
	require.NoError(t, err)
	assert.False(t, v.Bool)
}

func TestEvalBinaryOr(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(boolLit("TRUE"), "OR", boolLit("FALSE")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBinaryXor(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(boolLit("TRUE"), "XOR", boolLit("TRUE")))
	require.NoError(t, err)
	assert.False(t, v.Bool)
}

// --- Unary expression tests ---

func TestEvalUnaryNot(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, unaryExpr("NOT", boolLit("TRUE")))
	require.NoError(t, err)
	assert.False(t, v.Bool)
}

func TestEvalUnaryNegate(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, unaryExpr("-", intLit("5")))
	require.NoError(t, err)
	assert.Equal(t, int64(-5), v.Int)
}

func TestEvalUnaryNegateReal(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, unaryExpr("-", realLit("3.14")))
	require.NoError(t, err)
	assert.InDelta(t, -3.14, v.Real, 0.001)
}

// --- Identifier tests ---

func TestEvalIdent(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("X", IntValue(10))
	v, err := interp.evalExpr(env, ident("X"))
	require.NoError(t, err)
	assert.Equal(t, int64(10), v.Int)
}

func TestEvalIdentUndefined(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalExpr(env, ident("MISSING"))
	assert.Error(t, err)
}

// --- Paren expression test ---

func TestEvalParen(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, parenExpr(intLit("42")))
	require.NoError(t, err)
	assert.Equal(t, int64(42), v.Int)
}

// --- Statement tests ---

func TestExecAssign(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	err := interp.execStatements(env, []ast.Statement{
		assignStmt("x", intLit("5")),
	})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(5), v.Int)
}

func TestExecIfTrue(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	stmt := &ast.IfStmt{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
		Condition: boolLit("TRUE"),
		Then:      []ast.Statement{assignStmt("x", intLit("1"))},
		Else:      []ast.Statement{assignStmt("x", intLit("2"))},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(1), v.Int)
}

func TestExecIfFalse(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	stmt := &ast.IfStmt{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
		Condition: boolLit("FALSE"),
		Then:      []ast.Statement{assignStmt("x", intLit("1"))},
		Else:      []ast.Statement{assignStmt("x", intLit("2"))},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(2), v.Int)
}

func TestExecIfElsIf(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))
	stmt := &ast.IfStmt{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
		Condition: boolLit("FALSE"),
		Then:      []ast.Statement{assignStmt("x", intLit("1"))},
		ElsIfs: []*ast.ElsIf{
			{
				Condition: boolLit("TRUE"),
				Body:      []ast.Statement{assignStmt("x", intLit("3"))},
			},
		},
		Else: []ast.Statement{assignStmt("x", intLit("2"))},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(3), v.Int)
}

func TestExecForLoop(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("i", IntValue(0))
	env.Define("sum", IntValue(0))

	// sum := sum + i for i := 1 to 5
	stmt := &ast.ForStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindForStmt},
		Variable: ident("i"),
		From:     intLit("1"),
		To:       intLit("5"),
		Body: []ast.Statement{
			assignStmt("sum", binExpr(ident("sum"), "+", ident("i"))),
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("sum")
	assert.Equal(t, int64(15), v.Int)
}

func TestExecWhileLoop(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	stmt := &ast.WhileStmt{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindWhileStmt},
		Condition: binExpr(ident("x"), "<", intLit("5")),
		Body: []ast.Statement{
			assignStmt("x", binExpr(ident("x"), "+", intLit("1"))),
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(5), v.Int)
}

func TestExecRepeatLoop(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	stmt := &ast.RepeatStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindRepeatStmt},
		Body: []ast.Statement{
			assignStmt("x", binExpr(ident("x"), "+", intLit("1"))),
		},
		Condition: binExpr(ident("x"), ">=", intLit("5")),
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(5), v.Int)
}

func TestExecForWithExit(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("i", IntValue(0))
	env.Define("sum", IntValue(0))

	// sum := sum + i; if i = 3 then EXIT
	stmt := &ast.ForStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindForStmt},
		Variable: ident("i"),
		From:     intLit("1"),
		To:       intLit("10"),
		Body: []ast.Statement{
			assignStmt("sum", binExpr(ident("sum"), "+", ident("i"))),
			&ast.IfStmt{
				NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
				Condition: binExpr(ident("i"), "=", intLit("3")),
				Then: []ast.Statement{
					&ast.ExitStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindExitStmt}},
				},
			},
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("sum")
	assert.Equal(t, int64(6), v.Int) // 1+2+3 = 6
}

func TestExecForWithContinue(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("i", IntValue(0))
	env.Define("sum", IntValue(0))

	// for i := 1 to 5: if i = 3 then continue; sum := sum + i
	stmt := &ast.ForStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindForStmt},
		Variable: ident("i"),
		From:     intLit("1"),
		To:       intLit("5"),
		Body: []ast.Statement{
			&ast.IfStmt{
				NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
				Condition: binExpr(ident("i"), "=", intLit("3")),
				Then: []ast.Statement{
					&ast.ContinueStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindContinueStmt}},
				},
			},
			assignStmt("sum", binExpr(ident("sum"), "+", ident("i"))),
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("sum")
	assert.Equal(t, int64(12), v.Int) // 1+2+4+5 = 12
}

func TestExecDivisionByZero(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	_, err := interp.evalExpr(env, binExpr(intLit("10"), "/", intLit("0")))
	assert.Error(t, err)
	var rtErr *RuntimeError
	assert.ErrorAs(t, err, &rtErr)
	assert.Contains(t, rtErr.Msg, "division by zero")
}

func TestExecCaseStmt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	stmt := &ast.CaseStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCaseStmt},
		Expr:     intLit("2"),
		Branches: []*ast.CaseBranch{
			{
				Labels: []ast.CaseLabel{
					&ast.CaseLabelValue{Value: intLit("1")},
				},
				Body: []ast.Statement{assignStmt("x", intLit("10"))},
			},
			{
				Labels: []ast.CaseLabel{
					&ast.CaseLabelValue{Value: intLit("2")},
				},
				Body: []ast.Statement{assignStmt("x", intLit("20"))},
			},
		},
		ElseBranch: []ast.Statement{assignStmt("x", intLit("99"))},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(20), v.Int)
}

func TestExecCaseStmtElse(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	stmt := &ast.CaseStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCaseStmt},
		Expr:     intLit("99"),
		Branches: []*ast.CaseBranch{
			{
				Labels: []ast.CaseLabel{
					&ast.CaseLabelValue{Value: intLit("1")},
				},
				Body: []ast.Statement{assignStmt("x", intLit("10"))},
			},
		},
		ElseBranch: []ast.Statement{assignStmt("x", intLit("42"))},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(42), v.Int)
}

func TestExecCaseStmtRange(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	stmt := &ast.CaseStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCaseStmt},
		Expr:     intLit("5"),
		Branches: []*ast.CaseBranch{
			{
				Labels: []ast.CaseLabel{
					&ast.CaseLabelRange{Low: intLit("1"), High: intLit("10")},
				},
				Body: []ast.Statement{assignStmt("x", intLit("77"))},
			},
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("x")
	assert.Equal(t, int64(77), v.Int)
}

func TestEvalMixedArithmetic(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	// int + real should promote to real
	v, err := interp.evalExpr(env, binExpr(intLit("3"), "+", realLit("1.5")))
	require.NoError(t, err)
	assert.Equal(t, ValReal, v.Kind)
	assert.InDelta(t, 4.5, v.Real, 0.001)
}

func TestExecForWithBy(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("i", IntValue(0))
	env.Define("sum", IntValue(0))

	stmt := &ast.ForStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindForStmt},
		Variable: ident("i"),
		From:     intLit("0"),
		To:       intLit("10"),
		By:       intLit("2"),
		Body: []ast.Statement{
			assignStmt("sum", binExpr(ident("sum"), "+", ident("i"))),
		},
	}
	err := interp.execStatements(env, []ast.Statement{stmt})
	require.NoError(t, err)
	v, _ := env.Get("sum")
	assert.Equal(t, int64(30), v.Int) // 0+2+4+6+8+10 = 30
}

func TestExecEmptyStmt(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	err := interp.execStatements(env, []ast.Statement{
		&ast.EmptyStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindEmptyStmt}},
	})
	require.NoError(t, err)
}

func TestExecReturn(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(0))

	err := interp.execStatements(env, []ast.Statement{
		assignStmt("x", intLit("1")),
		&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt}},
		assignStmt("x", intLit("2")),
	})
	// Return should propagate as ErrReturn
	var retErr *ErrReturn
	assert.ErrorAs(t, err, &retErr)
	v, _ := env.Get("x")
	assert.Equal(t, int64(1), v.Int) // second assign not executed
}

func TestEvalStringComparison(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(strLit("'abc'"), "=", strLit("'abc'")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalBoolComparison(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	v, err := interp.evalExpr(env, binExpr(boolLit("TRUE"), "=", boolLit("TRUE")))
	require.NoError(t, err)
	assert.True(t, v.Bool)
}

func TestEvalIndexExpr(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("arr", Value{
		Kind:  ValArray,
		Array: []Value{IntValue(10), IntValue(20), IntValue(30)},
	})

	expr := &ast.IndexExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindIndexExpr},
		Object:   ident("arr"),
		Indices:  []ast.Expr{intLit("1")},
	}
	v, err := interp.evalExpr(env, expr)
	require.NoError(t, err)
	assert.Equal(t, int64(20), v.Int) // 0-based index: arr[1] = 20
}
