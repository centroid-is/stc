package interp

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// bitWidth returns the number of addressable bits of an integer or
// bit-string type, or 0 for any other type.
func bitWidth(k types.TypeKind) int {
	switch k {
	case types.KindBYTE, types.KindSINT, types.KindUSINT:
		return 8
	case types.KindWORD, types.KindINT, types.KindUINT:
		return 16
	case types.KindDWORD, types.KindDINT, types.KindUDINT:
		return 32
	case types.KindLWORD, types.KindLINT, types.KindULINT:
		return 64
	default:
		return 0
	}
}

// isUnsignedBits reports whether k is an unsigned integer or bit-string kind.
func isUnsignedBits(k types.TypeKind) bool {
	switch k {
	case types.KindBYTE, types.KindUSINT, types.KindWORD, types.KindUINT,
		types.KindDWORD, types.KindUDINT, types.KindLWORD, types.KindULINT:
		return true
	default:
		return false
	}
}

// valueBitWidth returns the bit width of an integer value. An integer whose
// IEC type is unknown is treated as 64 bits wide. A non-integer value is an
// error: BOOL, REAL, STRING, struct and FB values have no addressable bits.
func valueBitWidth(v Value, pos ast.Pos) (int, error) {
	if v.Kind != ValInt {
		return 0, &RuntimeError{Msg: fmt.Sprintf("bit access on non-integer value of type %s", v.Kind), Pos: pos}
	}
	if w := bitWidth(v.IECType); w > 0 {
		return w, nil
	}
	return 64, nil
}

// checkBitRange fails with a RuntimeError when n is not a valid bit index for
// a value of the given width. It runs before any shift (T-20-13).
func checkBitRange(n int64, width int, pos ast.Pos) error {
	if n < 0 || n >= int64(width) {
		return &RuntimeError{Msg: fmt.Sprintf("bit index %d out of range for %d-bit value", n, width), Pos: pos}
	}
	return nil
}

// bitIndex evaluates a bit index expression to an integer. The range check
// against the target's width is done by the caller.
func (interp *Interpreter) bitIndex(env *Env, idx ast.Expr) (int64, error) {
	v, err := interp.evalExpr(env, idx)
	if err != nil {
		return 0, err
	}
	if v.Kind != ValInt {
		return 0, &RuntimeError{Msg: fmt.Sprintf("bit index must be an integer, got %s", v.Kind), Pos: idx.Span().Start}
	}
	return v.Int, nil
}

// readBit returns bit n of v as a BOOL value. Bit 0 is the least
// significant bit (DIAL-04).
func readBit(v Value, n int64, pos ast.Pos) (Value, error) {
	width, err := valueBitWidth(v, pos)
	if err != nil {
		return Value{}, err
	}
	if err := checkBitRange(n, width, pos); err != nil {
		return Value{}, err
	}
	return BoolValue(uint64(v.Int)&(uint64(1)<<uint(n)) != 0), nil
}

// evalBitAccess evaluates x.N.
func (interp *Interpreter) evalBitAccess(env *Env, e *ast.BitAccessExpr) (Value, error) {
	target, err := interp.evalExpr(env, e.Target)
	if err != nil {
		return Value{}, err
	}
	n, err := interp.bitIndex(env, e.Index)
	if err != nil {
		return Value{}, err
	}
	return readBit(target, n, e.Span().Start)
}

// assignBit handles x.N := val.
func (interp *Interpreter) assignBit(env *Env, target *ast.BitAccessExpr, val Value) error {
	cur, err := interp.evalExpr(env, target.Target)
	if err != nil {
		return err
	}
	n, err := interp.bitIndex(env, target.Index)
	if err != nil {
		return err
	}
	return interp.assignBitAt(env, target.Target, cur, n, val)
}

// assignBitAt sets or clears bit n of cur, the current value of targetExpr,
// and writes the result back through assignToTarget so identifier,
// reference, member, array element and GVL targets all reuse their existing
// write paths. The result is stored through storeAs against cur, so the IEC
// type of cur is kept and the result wraps to its width (masked for unsigned
// kinds, sign-extended for signed kinds).
func (interp *Interpreter) assignBitAt(env *Env, targetExpr ast.Expr, cur Value, n int64, val Value) error {
	pos := targetExpr.Span().Start
	width, err := valueBitWidth(cur, pos)
	if err != nil {
		return err
	}
	if err := checkBitRange(n, width, pos); err != nil {
		return err
	}
	bits := uint64(cur.Int)
	mask := uint64(1) << uint(n)
	if val.IsTruthy() {
		bits |= mask
	} else {
		bits &^= mask
	}
	next := cur
	next.Int = int64(bits)
	return interp.assignToTarget(env, targetExpr, storeAs(cur, next))
}

// constBitIndex reports the bit index named by the member of x.member when
// the member resolves to an integer symbol in scope (ruling A3: a constant
// identifier used as a bit index parses as member access). Callers only use
// it when x is an integer value, which has no members of its own, so the
// lookup never shadows a struct or FB member. The checker enforces that the
// symbol is a CONSTANT.
func constBitIndex(env *Env, e *ast.MemberAccessExpr) (int64, bool) {
	if e.Member == nil {
		return 0, false
	}
	v, ok := env.Get(e.Member.Name)
	if !ok || v.Kind != ValInt {
		return 0, false
	}
	return v.Int, true
}
