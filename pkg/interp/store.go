package interp

import (
	"strconv"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// storeAs converts v for storage in a slot whose current value is dst. It is
// the single choke point for typed stores:
//
//   - an integer stored into an integer slot wraps to the slot's IEC width
//     and takes the slot's IECType and enum tag (the tag follows the
//     destination's declared type, so an enum value stored into an INT loses
//     it and an integer stored into an enum variable gains it);
//   - an integer stored into a real slot becomes a real of the slot's type;
//   - a real stored into a real slot takes the slot's type.
//
// A slot without a known IECType keeps v's type. Other combinations are
// returned unchanged.
func storeAs(dst, v Value) Value {
	switch {
	case dst.Kind == ValInt && v.Kind == ValInt:
		v.Enum = dst.Enum
		if dst.IECType != types.KindInvalid {
			v.Int = wrapInt(v.Int, dst.IECType)
			v.IECType = dst.IECType
		}
	case dst.Kind == ValReal && v.Kind == ValInt:
		k := dst.IECType
		if k == types.KindInvalid {
			k = types.KindLREAL
		}
		v = Value{Kind: ValReal, Real: toFloat(v), IECType: k}
	case dst.Kind == ValReal && v.Kind == ValReal:
		if dst.IECType != types.KindInvalid {
			v.IECType = dst.IECType
		}
	}
	return v
}

// wrapInt truncates n to the width of the IEC integer kind k with two's
// complement wrap-around. LINT, ULINT and LWORD (and non-integer kinds) keep
// the int64 bit pattern; ULINT and LWORD values are read as uint64.
func wrapInt(n int64, k types.TypeKind) int64 {
	switch k {
	case types.KindSINT:
		return int64(int8(n))
	case types.KindUSINT, types.KindBYTE:
		return int64(uint8(n))
	case types.KindINT:
		return int64(int16(n))
	case types.KindUINT, types.KindWORD:
		return int64(uint16(n))
	case types.KindDINT:
		return int64(int32(n))
	case types.KindUDINT, types.KindDWORD:
		return int64(uint32(n))
	default:
		return n
	}
}

// isUnsigned64 reports whether k is a 64-bit unsigned kind whose int64 bit
// pattern must be read as uint64.
func isUnsigned64(k types.TypeKind) bool {
	return k == types.KindULINT || k == types.KindLWORD
}

// formatInt formats an integer value in decimal, reading ULINT and LWORD
// values as unsigned.
func formatInt(v Value) string {
	if isUnsigned64(v.IECType) {
		return strconv.FormatUint(uint64(v.Int), 10)
	}
	return strconv.FormatInt(v.Int, 10)
}

// isUntypedIntLiteral reports whether e is an integer literal without a type
// prefix, possibly parenthesised or negated.
func isUntypedIntLiteral(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Literal:
		return x.LitKind == ast.LitInt && x.TypePrefix == ""
	case *ast.ParenExpr:
		return isUntypedIntLiteral(x.Inner)
	case *ast.UnaryExpr:
		return x.Op.Text == "-" && isUntypedIntLiteral(x.Operand)
	}
	return false
}

// resultIntKind returns the IEC kind of an integer binary result with
// operand expressions lexpr, rexpr and values l, r:
//
//   - an untyped integer literal adopts the other operand's kind;
//   - two typed operands use types.CommonType when it is an integer kind,
//     else the wider operand's kind, preferring the unsigned kind on a tie
//     (research A3: TwinCAT behaviour for mixed signedness is unverified);
//   - an operand without a known integer kind takes the other's kind.
//
// KindInvalid means the result is untyped (both operands untyped constants):
// it is not wrapped and keeps the default DINT type.
func resultIntKind(lexpr, rexpr ast.Expr, l, r Value) types.TypeKind {
	lk, rk := l.IECType, r.IECType
	if isUntypedIntLiteral(lexpr) || bitWidth(lk) == 0 {
		lk = types.KindInvalid
	}
	if isUntypedIntLiteral(rexpr) || bitWidth(rk) == 0 {
		rk = types.KindInvalid
	}
	switch {
	case lk == types.KindInvalid:
		return rk
	case rk == types.KindInvalid:
		return lk
	}
	if k, ok := types.CommonType(lk, rk); ok && bitWidth(k) > 0 {
		return k
	}
	lw, rw := bitWidth(lk), bitWidth(rk)
	switch {
	case lw > rw:
		return lk
	case rw > lw:
		return rk
	case isUnsignedBits(rk):
		return rk
	default:
		return lk
	}
}
