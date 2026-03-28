package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssertTrue_Passes(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	// ASSERT_TRUE(TRUE) => pass
	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindCallExpr,
			NodeSpan: ast.Span{
				Start: ast.Pos{File: "test.st", Line: 5, Col: 3},
			},
		},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("TRUE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.True(t, collector.Results[0].Passed)
	assert.Equal(t, "test.st", collector.Results[0].Pos.File)
	assert.Equal(t, 5, collector.Results[0].Pos.Line)
	assert.Equal(t, 3, collector.Results[0].Pos.Col)
}

func TestAssertTrue_Fails(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindCallExpr,
			NodeSpan: ast.Span{
				Start: ast.Pos{File: "test.st", Line: 10, Col: 1},
			},
		},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("FALSE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err) // assertions do NOT return errors

	require.Len(t, collector.Results, 1)
	assert.False(t, collector.Results[0].Passed)
	assert.Contains(t, collector.Results[0].Message, "expected TRUE, got FALSE")
	assert.Equal(t, 10, collector.Results[0].Pos.Line)
}

func TestAssertFalse_Passes(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_FALSE",
		},
		Args: []ast.Expr{boolLit("FALSE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.True(t, collector.Results[0].Passed)
}

func TestAssertFalse_Fails(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_FALSE",
		},
		Args: []ast.Expr{boolLit("TRUE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.False(t, collector.Results[0].Passed)
	assert.Contains(t, collector.Results[0].Message, "expected FALSE, got TRUE")
}

func TestAssertEq_Passes(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_EQ",
		},
		Args: []ast.Expr{intLit("5"), intLit("5")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.True(t, collector.Results[0].Passed)
}

func TestAssertEq_Fails(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_EQ",
		},
		Args: []ast.Expr{intLit("5"), intLit("10")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.False(t, collector.Results[0].Passed)
	assert.Contains(t, collector.Results[0].Message, "expected 10, got 5")
}

func TestAssertEq_CustomMessage(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_EQ",
		},
		Args: []ast.Expr{intLit("5"), intLit("10"), strLit("'values must match'")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.False(t, collector.Results[0].Passed)
	assert.Contains(t, collector.Results[0].Message, "values must match")
}

func TestAssertNear_Passes(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_NEAR",
		},
		Args: []ast.Expr{realLit("3.14"), realLit("3.0"), realLit("0.2")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.True(t, collector.Results[0].Passed)
}

func TestAssertNear_Fails(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_NEAR",
		},
		Args: []ast.Expr{realLit("3.14"), realLit("3.0"), realLit("0.01")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	assert.False(t, collector.Results[0].Passed)
}

func TestAssertionFailureDoesNotAbort(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	// First assertion fails
	call1 := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("FALSE")},
	}
	_, err := interp.evalCall(env, call1)
	require.NoError(t, err)

	// Second assertion should still execute
	call2 := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("TRUE")},
	}
	_, err = interp.evalCall(env, call2)
	require.NoError(t, err)

	require.Len(t, collector.Results, 2)
	assert.False(t, collector.Results[0].Passed)
	assert.True(t, collector.Results[1].Passed)
}

func TestAssertionFailureIncludesSourcePos(t *testing.T) {
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindCallExpr,
			NodeSpan: ast.Span{
				Start: ast.Pos{File: "my_test.st", Line: 42, Col: 5},
			},
		},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("FALSE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)

	require.Len(t, collector.Results, 1)
	r := collector.Results[0]
	assert.False(t, r.Passed)
	assert.Equal(t, "my_test.st", r.Pos.File)
	assert.Equal(t, 42, r.Pos.Line)
	assert.Equal(t, 5, r.Pos.Col)
}

func TestAdvanceTime_AdvancesClock(t *testing.T) {
	interp := New()
	var advancedBy time.Duration
	interp.RegisterAdvanceTime(func(d time.Duration) {
		advancedBy = d
	})
	env := NewEnv(nil)

	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ADVANCE_TIME",
		},
		Args: []ast.Expr{timeLit("T#100ms")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, advancedBy)
}

func TestAssertionCollector_HasFailures(t *testing.T) {
	c := &AssertionCollector{}
	assert.False(t, c.HasFailures())

	c.Record(true, "", ast.Pos{})
	assert.False(t, c.HasFailures())

	c.Record(false, "fail", ast.Pos{})
	assert.True(t, c.HasFailures())
}

func TestAssertionCollector_Failures(t *testing.T) {
	c := &AssertionCollector{}
	c.Record(true, "ok", ast.Pos{})
	c.Record(false, "bad", ast.Pos{})
	c.Record(true, "ok2", ast.Pos{})
	c.Record(false, "bad2", ast.Pos{})

	failures := c.Failures()
	require.Len(t, failures, 2)
	assert.Equal(t, "bad", failures[0].Message)
	assert.Equal(t, "bad2", failures[1].Message)
}

func TestLocalFunctions_PriorityOverStdlib(t *testing.T) {
	// Verify LocalFunctions take priority over StdlibFunctions
	interp := New()
	collector := &AssertionCollector{}
	interp.RegisterAssertions(collector)
	env := NewEnv(nil)

	// ASSERT_TRUE should be dispatched to LocalFunctions, not StdlibFunctions
	call := &ast.CallExpr{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr},
		Callee: &ast.Ident{
			NodeBase: ast.NodeBase{NodeKind: ast.KindIdent},
			Name:     "ASSERT_TRUE",
		},
		Args: []ast.Expr{boolLit("TRUE")},
	}
	_, err := interp.evalCall(env, call)
	require.NoError(t, err)
	assert.Len(t, collector.Results, 1)
}
