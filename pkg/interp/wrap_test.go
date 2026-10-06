package interp

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exprEngine runs one scan of a program declaring decls, so the variables
// are initialised, and returns the engine.
func exprEngine(t *testing.T, decls string) *ScanCycleEngine {
	t.Helper()
	return bitRun(t, "P.st", "PROGRAM P\nVAR\n"+decls+"\nEND_VAR\nEND_PROGRAM\n")
}

// parseExpr parses src as the right-hand side of an assignment.
func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	res := parser.Parse("E.st", "PROGRAM E\nx := "+src+";\nEND_PROGRAM\n")
	require.Empty(t, res.Diags, "parse diagnostics")
	prog := res.File.Declarations[0].(*ast.ProgramDecl)
	return prog.Body[0].(*ast.AssignStmt).Value
}

// evalIn evaluates expression src in the environment of eng.
func evalIn(t *testing.T, eng *ScanCycleEngine, src string) (Value, error) {
	t.Helper()
	return eng.interp.evalExpr(eng.env, parseExpr(t, src))
}

func mustEval(t *testing.T, eng *ScanCycleEngine, src string) Value {
	t.Helper()
	v, err := evalIn(t, eng, src)
	require.NoError(t, err, src)
	return v
}

func TestIntegerWrap(t *testing.T) {
	tests := []struct {
		kind     string
		k        types.TypeKind
		max, min string
		wantMax  int64 // max + 1
		wantMin  int64 // min - 1
	}{
		{"SINT", types.KindSINT, "127", "-128", -128, 127},
		{"USINT", types.KindUSINT, "255", "0", 0, 255},
		{"BYTE", types.KindBYTE, "255", "0", 0, 255},
		{"INT", types.KindINT, "32767", "-32768", -32768, 32767},
		{"UINT", types.KindUINT, "65535", "0", 0, 65535},
		{"WORD", types.KindWORD, "65535", "0", 0, 65535},
		{"DINT", types.KindDINT, "2147483647", "-2147483648", -2147483648, 2147483647},
		{"UDINT", types.KindUDINT, "4294967295", "0", 0, 4294967295},
		{"DWORD", types.KindDWORD, "4294967295", "0", 0, 4294967295},
		{"LINT", types.KindLINT, "16#7FFF_FFFF_FFFF_FFFF", "16#8000_0000_0000_0000", math.MinInt64, math.MaxInt64},
		{"ULINT", types.KindULINT, "16#FFFF_FFFF_FFFF_FFFF", "0", 0, -1},
		{"LWORD", types.KindLWORD, "16#FFFF_FFFF_FFFF_FFFF", "0", 0, -1},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			eng := exprEngine(t, "hi : "+tt.kind+" := "+tt.max+"; lo : "+tt.kind+" := "+tt.min+";")
			up := mustEval(t, eng, "hi + 1")
			assert.Equal(t, tt.wantMax, up.Int, "max + 1")
			assert.Equal(t, tt.k, up.IECType)
			down := mustEval(t, eng, "lo - 1")
			assert.Equal(t, tt.wantMin, down.Int, "min - 1")
			assert.Equal(t, tt.k, down.IECType)
		})
	}

	t.Run("store wraps after arithmetic", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR i : INT := 32767; u : UINT; END_VAR
i := i + 1;
u := u - 1;
END_PROGRAM
`)
		assert.Equal(t, int64(-32768), progVar(t, eng, "i").Int)
		assert.Equal(t, int64(65535), progVar(t, eng, "u").Int)
	})
}

func TestTypedBinary(t *testing.T) {
	eng := exprEngine(t, `i : INT := 32767; m : INT := -32768; u : UINT; s : SINT := 100;
d : DINT := 7; w : WORD := 16#FFFF; r : REAL := 1.5; e : INT;`)

	t.Run("untyped literal adopts the typed operand kind", func(t *testing.T) {
		for _, src := range []string{"i + 1", "1 + i", "i + (1)", "i - -1", "(i) + +1"} {
			v := mustEval(t, eng, src)
			assert.Equal(t, int64(-32768), v.Int, src)
			assert.Equal(t, types.KindINT, v.IECType, src)
		}
		v := mustEval(t, eng, "u - 1")
		assert.Equal(t, int64(65535), v.Int)
		assert.Equal(t, types.KindUINT, v.IECType)
	})

	t.Run("typed operands use the common type", func(t *testing.T) {
		v := mustEval(t, eng, "s + i")
		assert.Equal(t, types.KindINT, v.IECType)
		assert.Equal(t, int64(-32768+99), v.Int)
		v = mustEval(t, eng, "w + d")
		assert.Equal(t, types.KindDINT, v.IECType, "wider kind when no common type")
		assert.Equal(t, int64(65542), v.Int)
		v = mustEval(t, eng, "i + INT#1")
		assert.Equal(t, int64(-32768), v.Int)
	})

	t.Run("unary minus keeps the operand kind", func(t *testing.T) {
		v := mustEval(t, eng, "-m")
		assert.Equal(t, int64(-32768), v.Int)
		assert.Equal(t, types.KindINT, v.IECType)
		v = mustEval(t, eng, "-5")
		assert.Equal(t, types.KindDINT, v.IECType)
	})

	t.Run("untyped constant expressions do not wrap", func(t *testing.T) {
		v := mustEval(t, eng, "100000 * 100000")
		assert.Equal(t, int64(10000000000), v.Int)
		assert.Equal(t, types.KindDINT, v.IECType)
	})

	t.Run("comparisons yield BOOL", func(t *testing.T) {
		v := mustEval(t, eng, "i + 1 < i")
		assert.Equal(t, ValBool, v.Kind)
		assert.True(t, v.Bool)
	})

	t.Run("integer division by zero is an error", func(t *testing.T) {
		_, err := evalIn(t, eng, "i / e")
		assert.ErrorContains(t, err, "division by zero")
		_, err = evalIn(t, eng, "i MOD 0")
		assert.ErrorContains(t, err, "division by zero")
	})

	t.Run("reals are unchanged", func(t *testing.T) {
		v := mustEval(t, eng, "r + i")
		assert.Equal(t, ValReal, v.Kind)
		assert.Equal(t, 32768.5, v.Real)
		v = mustEval(t, eng, "i ** 2")
		assert.Equal(t, types.KindLREAL, v.IECType)
	})
}

func TestResultIntKind(t *testing.T) {
	typed := func(k types.TypeKind) Value { return Value{Kind: ValInt, IECType: k} }
	lit := &ast.Literal{LitKind: ast.LitInt, Value: "1"}
	tests := []struct {
		name   string
		le, re ast.Expr
		l, r   Value
		want   types.TypeKind
	}{
		{"both untyped", lit, lit, IntValue(1), IntValue(2), types.KindInvalid},
		{"left untyped", lit, nil, IntValue(1), typed(types.KindBYTE), types.KindBYTE},
		{"right untyped", nil, lit, typed(types.KindUSINT), IntValue(1), types.KindUSINT},
		{"common type", nil, nil, typed(types.KindSINT), typed(types.KindDINT), types.KindDINT},
		{"INT and UINT tie picks unsigned", nil, nil, typed(types.KindINT), typed(types.KindUINT), types.KindUINT},
		{"UINT and INT tie picks unsigned", nil, nil, typed(types.KindUINT), typed(types.KindINT), types.KindUINT},
		{"LINT and ULINT", nil, nil, typed(types.KindLINT), typed(types.KindULINT), types.KindULINT},
		{"BYTE and DINT picks wider", nil, nil, typed(types.KindBYTE), typed(types.KindDINT), types.KindDINT},
		{"DINT and BYTE picks wider", nil, nil, typed(types.KindDINT), typed(types.KindBYTE), types.KindDINT},
		{"unknown left", nil, nil, Value{Kind: ValInt}, typed(types.KindINT), types.KindINT},
		{"unknown right", nil, nil, typed(types.KindWORD), Value{Kind: ValInt}, types.KindWORD},
		{"both unknown", nil, nil, Value{Kind: ValInt}, Value{Kind: ValInt}, types.KindInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resultIntKind(tt.le, tt.re, tt.l, tt.r))
		})
	}
}

func TestIsUntypedIntLiteral(t *testing.T) {
	assert.True(t, isUntypedIntLiteral(parseExpr(t, "5")))
	assert.True(t, isUntypedIntLiteral(parseExpr(t, "-(5)")))
	assert.False(t, isUntypedIntLiteral(parseExpr(t, "INT#5")))
	assert.False(t, isUntypedIntLiteral(parseExpr(t, "5.0")))
	assert.False(t, isUntypedIntLiteral(parseExpr(t, "NOT 5")))
	assert.False(t, isUntypedIntLiteral(parseExpr(t, "x")))
	assert.False(t, isUntypedIntLiteral(nil))
}

func TestUnsigned64(t *testing.T) {
	for _, kind := range []string{"ULINT", "LWORD"} {
		t.Run(kind, func(t *testing.T) {
			eng := exprEngine(t, "u : "+kind+" := 16#FFFF_FFFF_FFFF_FFFF; z : "+kind+"; one : "+kind+" := 1;")
			assert.True(t, mustEval(t, eng, "u > 0").Bool)
			assert.True(t, mustEval(t, eng, "u >= one").Bool)
			assert.False(t, mustEval(t, eng, "u < 1").Bool)
			assert.False(t, mustEval(t, eng, "u <= one").Bool)
			assert.True(t, mustEval(t, eng, "u = 16#FFFF_FFFF_FFFF_FFFF").Bool)
			assert.True(t, mustEval(t, eng, "u <> 0").Bool)
			half := mustEval(t, eng, "u / 2")
			assert.Equal(t, int64(math.MaxInt64), half.Int)
			assert.Equal(t, int64(5), mustEval(t, eng, "u MOD 10").Int)
			_, err := evalIn(t, eng, "u / z")
			assert.ErrorContains(t, err, "division by zero")
			_, err = evalIn(t, eng, "u MOD z")
			assert.ErrorContains(t, err, "division by zero")

			u := mustEval(t, eng, "u")
			assert.Equal(t, "18446744073709551615", u.String())
			b, err := json.Marshal(u)
			require.NoError(t, err)
			assert.Equal(t, "18446744073709551615", string(b))
			assert.Equal(t, "18446744073709551615", anyToString(u))
		})
	}

	t.Run("decimal literal above 2^63", func(t *testing.T) {
		eng := exprEngine(t, "u : ULINT := 18446744073709551615;")
		assert.Equal(t, int64(-1), progVar(t, eng, "u").Int)
	})

	t.Run("signed 64-bit values still print signed", func(t *testing.T) {
		assert.Equal(t, "-1", Value{Kind: ValInt, Int: -1, IECType: types.KindLINT}.String())
	})
}
