package checker

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// untypedKind classifies an untyped numeric constant expression. Untyped
// constants are a checker-only notion (Go-style): they adopt the type of
// their context instead of defaulting to DINT or LREAL.
type untypedKind int

const (
	untypedNone untypedKind = iota // not an untyped constant
	untypedInt                     // integer literal or integer constant expression
	untypedReal                    // real literal, or an expression involving one
)

// untypedConst reports whether e is an untyped numeric constant expression:
// an integer or real literal without a type prefix, possibly parenthesised,
// negated, or combined with another untyped constant by + - * / MOD.
// For integers, val is the folded value when it is known and fits int64
// (overflow, division by zero and literals above 2^63-1 leave hasVal false).
func untypedConst(e ast.Expr) (k untypedKind, val int64, hasVal bool) {
	switch x := e.(type) {
	case *ast.Literal:
		switch x.LitKind {
		case ast.LitInt:
			val, hasVal = ast.ConstIntValue(x, nil)
			return untypedInt, val, hasVal
		case ast.LitReal:
			return untypedReal, 0, false
		}
	case *ast.ParenExpr:
		return untypedConst(x.Inner)
	case *ast.UnaryExpr:
		k, val, hasVal = untypedConst(x.Operand)
		switch {
		case k == untypedNone:
		case x.Op.Text == "+":
			return k, val, hasVal
		case x.Op.Text == "-":
			if k == untypedInt && hasVal {
				return k, -val, val != math.MinInt64
			}
			return k, 0, false
		}
	case *ast.BinaryExpr:
		op := strings.ToUpper(x.Op.Text)
		switch op {
		case "+", "-", "*", "/", "MOD":
		default:
			return untypedNone, 0, false
		}
		lk, lv, lok := untypedConst(x.Left)
		rk, rv, rok := untypedConst(x.Right)
		if lk == untypedNone || rk == untypedNone {
			return untypedNone, 0, false
		}
		if lk == untypedReal || rk == untypedReal {
			return untypedReal, 0, false
		}
		if !lok || !rok {
			return untypedInt, 0, false
		}
		val, hasVal = foldInt(op, lv, rv)
		return untypedInt, val, hasVal
	}
	return untypedNone, 0, false
}

// foldInt applies an integer operator exactly; ok is false when the result
// does not fit int64 or the divisor is zero. Division truncates toward zero
// as in IEC 61131-3.
func foldInt(op string, a, b int64) (int64, bool) {
	x, y := big.NewInt(a), big.NewInt(b)
	r := new(big.Int)
	switch op {
	case "+":
		r.Add(x, y)
	case "-":
		r.Sub(x, y)
	case "*":
		r.Mul(x, y)
	case "/", "MOD":
		if b == 0 {
			return 0, false
		}
		if op == "/" {
			r.Quo(x, y)
		} else {
			r.Rem(x, y)
		}
	}
	if !r.IsInt64() {
		return 0, false
	}
	return r.Int64(), true
}

// untypedAssignable reports whether the untyped constant e may be stored as
// target. Integer constants fit ANY_INT, the bit strings BYTE..LWORD and
// ANY_REAL; real constants only ANY_REAL. When an integer constant's value
// is known and does not fit the target, ok is false and rangeMsg is
// "constant <n> out of range for <TYPE>". A non-constant e is not assignable
// here (rangeMsg empty); callers fall back to the ordinary type rules.
func untypedAssignable(e ast.Expr, target types.Type) (ok bool, rangeMsg string) {
	k, val, hasVal := untypedConst(e)
	if k == untypedNone || target == nil {
		return false, ""
	}
	tk := target.Kind()
	if types.IsAnyReal(tk) {
		return true, ""
	}
	if k != untypedInt || !isIntOrBitString(tk) {
		return false, ""
	}
	if !hasVal {
		return true, ""
	}
	lo, hi := intRange(tk)
	if val < lo || (val > 0 && uint64(val) > hi) {
		return false, fmt.Sprintf("constant %d out of range for %s", val, tk)
	}
	return true, ""
}

// isIntOrBitString reports whether an untyped integer may take kind k:
// ANY_INT or one of the bit strings BYTE, WORD, DWORD, LWORD.
func isIntOrBitString(k types.TypeKind) bool {
	return types.IsAnyInt(k) || (types.IsAnyBit(k) && k != types.KindBOOL)
}

// intRange returns the value range of the 12 integer and bit-string kinds.
// Any other kind returns (0, 0).
func intRange(k types.TypeKind) (lo int64, hi uint64) {
	switch k {
	case types.KindSINT:
		return math.MinInt8, math.MaxInt8
	case types.KindINT:
		return math.MinInt16, math.MaxInt16
	case types.KindDINT:
		return math.MinInt32, math.MaxInt32
	case types.KindLINT:
		return math.MinInt64, math.MaxInt64
	case types.KindUSINT, types.KindBYTE:
		return 0, math.MaxUint8
	case types.KindUINT, types.KindWORD:
		return 0, math.MaxUint16
	case types.KindUDINT, types.KindDWORD:
		return 0, math.MaxUint32
	case types.KindULINT, types.KindLWORD:
		return 0, math.MaxUint64
	}
	return 0, 0
}
