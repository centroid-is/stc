package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// defaultStringLen is the length of a STRING declared without one. Values
// do not carry their declared length, so SIZEOF assumes it for every string.
const defaultStringLen = 80

// pointerSize is the byte size of POINTER, REFERENCE and PVOID on the
// 64-bit TwinCAT runtime.
const pointerSize = 8

// convKinds are the kinds with X_TO_Y conversions between every pair.
var convKinds = []types.TypeKind{
	types.KindBOOL,
	types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT,
	types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT,
	types.KindBYTE, types.KindWORD, types.KindDWORD, types.KindLWORD,
	types.KindREAL, types.KindLREAL,
}

func init() {
	registerSystemFunctions()
}

// registerSystemFunctions adds SHL, SHR, ROL, ROR and every BOOL, integer,
// bit-string and real X_TO_Y conversion not already defined. SIZEOF and ADR need the
// argument expression and are handled in evalCall.
func registerSystemFunctions() {
	for _, name := range []string{"SHL", "SHR", "ROL", "ROR"} {
		op := name
		StdlibFunctions[op] = func(args []Value) (Value, error) {
			if len(args) != 2 || args[0].Kind != ValInt || args[1].Kind != ValInt {
				return Value{}, &RuntimeError{Msg: op + " requires an ANY_BIT value and an integer count"}
			}
			return shiftOp(op, args[0], args[1].Int), nil
		}
	}
	for _, from := range convKinds {
		for _, to := range convKinds {
			if from == to {
				continue
			}
			name := from.String() + "_TO_" + to.String()
			if _, ok := StdlibFunctions[name]; ok {
				continue
			}
			fn, kind := name, to
			StdlibFunctions[fn] = func(args []Value) (Value, error) {
				if len(args) != 1 {
					return Value{}, &RuntimeError{Msg: fn + " requires 1 argument"}
				}
				return convertWrapped(fn, kind, args[0])
			}
		}
	}
}

// convertWrapped converts v to kind like convertTo, then wraps integer
// results to the target width (UINT_TO_SINT(200) = -56) and maps integers
// to BOOL as non-zero.
func convertWrapped(name string, kind types.TypeKind, v Value) (Value, error) {
	if kind == types.KindBOOL {
		switch v.Kind {
		case ValBool:
			return v, nil
		case ValInt:
			return BoolValue(v.Int != 0), nil
		case ValReal:
			return BoolValue(v.Real != 0), nil
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("%s: cannot convert %s", name, v.Kind)}
	}
	r, err := convertTo(name, kind, v)
	if err != nil || r.Kind != ValInt {
		return r, err
	}
	w := bitWidth(kind)
	mask := ^uint64(0)
	if w < 64 {
		mask = uint64(1)<<uint(w) - 1
	}
	r.Int = signExtend(uint64(r.Int)&mask, w, kind)
	return r, nil
}

// shiftOp applies SHL, SHR, ROL or ROR to v within its IEC bit width
// (64 bits when the type is unknown). Shift counts at or above the width
// give 0 for shifts; rotations use the count modulo the width.
func shiftOp(op string, v Value, n int64) Value {
	w := bitWidth(v.IECType)
	if w == 0 {
		w = 64
	}
	mask := ^uint64(0)
	if w < 64 {
		mask = uint64(1)<<uint(w) - 1
	}
	x := uint64(v.Int) & mask
	var r uint64
	switch op {
	case "SHL", "SHR":
		if n < 0 || n >= int64(w) {
			r = 0
		} else if op == "SHL" {
			r = (x << uint(n)) & mask
		} else {
			r = x >> uint(n)
		}
	default:
		k := uint(((n % int64(w)) + int64(w)) % int64(w))
		if op == "ROR" {
			k = (uint(w) - k) % uint(w)
		}
		r = ((x << k) | (x >> ((uint(w) - k) % uint(w)))) & mask
		if k == 0 {
			r = x
		}
	}
	return Value{Kind: ValInt, Int: signExtend(r, w, v.IECType), IECType: v.IECType}
}

// signExtend reinterprets the low w bits of r as the integer kind k.
func signExtend(r uint64, w int, k types.TypeKind) int64 {
	if isUnsignedBits(k) || w == 64 || bitWidth(k) == 0 {
		return int64(r)
	}
	if r&(uint64(1)<<uint(w-1)) != 0 {
		return int64(r | ^(uint64(1)<<uint(w) - 1))
	}
	return int64(r)
}

// evalSizeof evaluates SIZEOF(x): the byte size of the variable or
// expression x, or of the type when x names a TYPE or FUNCTION_BLOCK and no
// variable of that name is in scope.
func (interp *Interpreter) evalSizeof(env *Env, e *ast.CallExpr) (Value, error) {
	if len(e.Args) != 1 {
		return Value{}, &RuntimeError{Msg: "SIZEOF requires exactly 1 argument", Pos: e.Span().Start}
	}
	if id, ok := e.Args[0].(*ast.Ident); ok && env.FindOwner(id.Name) == nil {
		if n, ok := interp.sizeOfTypeName(env, id.Name); ok {
			return Value{Kind: ValInt, Int: n, IECType: types.KindUDINT}, nil
		}
	}
	v, err := interp.evalExpr(env, e.Args[0])
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValInt, Int: sizeOfValue(v, 0), IECType: types.KindUDINT}, nil
}

// sizeOfTypeName returns the size of a default value of the named type.
func (interp *Interpreter) sizeOfTypeName(env *Env, name string) (int64, bool) {
	upper := strings.ToUpper(name)
	_, decl := interp.TypeDecls[upper]
	_, fb := interp.FBDecls[upper]
	if !decl && !fb {
		return 0, false
	}
	ts := &ast.NamedType{Name: &ast.Ident{Name: name}}
	return sizeOfValue(interp.zeroOf(ts, env), 0), true
}

// sizeOfValue is the TwinCAT byte size of v without alignment padding:
// BOOL/BYTE/SINT/USINT 1, INT/UINT/WORD 2, DINT/UDINT/DWORD/REAL/TIME/DATE/
// TOD/DT 4, 64-bit types and pointers 8, STRING n+1 (n assumed 80),
// WSTRING 2(n+1), arrays, structs and FB instances the sum of their members.
func sizeOfValue(v Value, depth int) int64 {
	if depth > maxSizeofDepth {
		return 0
	}
	switch v.Kind {
	case ValBool:
		return 1
	case ValInt:
		if w := bitWidth(v.IECType); w > 0 {
			return int64(w / 8)
		}
		return 4 // DATE/TOD/DT-like and untyped integers default to 32 bits
	case ValReal:
		if v.IECType == types.KindREAL {
			return 4
		}
		return 8
	case ValTime, ValDate, ValDateTime, ValTod:
		return 4
	case ValString:
		if v.IECType == types.KindWSTRING {
			return 2 * (defaultStringLen + 1)
		}
		return defaultStringLen + 1
	case ValArray:
		var n int64
		for i := v.ArrayLow; i < len(v.Array); i++ {
			n += sizeOfValue(v.Array[i], depth+1)
		}
		return n
	case ValStruct:
		var n int64
		for _, m := range v.Struct {
			n += sizeOfValue(m, depth+1)
		}
		return n
	case ValFBInstance:
		if v.FBRef == nil || v.FBRef.Env == nil {
			return 0
		}
		var n int64
		for _, m := range v.FBRef.Env.vars {
			n += sizeOfValue(m, depth+1)
		}
		return n
	default:
		return pointerSize
	}
}

// maxSizeofDepth bounds SIZEOF recursion through nested values.
const maxSizeofDepth = 64

// evalAdr evaluates ADR(x) for a variable, a GVL or program member, a
// struct member or an array element.
func (interp *Interpreter) evalAdr(env *Env, e *ast.CallExpr) (Value, error) {
	if len(e.Args) != 1 {
		return Value{}, &RuntimeError{Msg: "ADR requires exactly 1 argument"}
	}
	if id, ok := e.Args[0].(*ast.Ident); ok {
		targetEnv := env.FindOwner(id.Name)
		if targetEnv == nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("ADR: undefined variable '%s'", id.Name)}
		}
		return Value{Kind: ValPointer, PtrEnv: targetEnv, PtrVar: strings.ToUpper(id.Name)}, nil
	}
	p, err := interp.buildRefPath(env, e.Args[0])
	if err != nil {
		return Value{}, fmt.Errorf("ADR: %w", err)
	}
	if _, err := readRef(p); err != nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("ADR: %s does not exist", p), Pos: e.Span().Start}
	}
	if len(p.Steps) == 0 {
		return Value{Kind: ValPointer, PtrEnv: p.Env, PtrVar: p.Var}, nil
	}
	return Value{Kind: ValPointer, Ref: p}, nil
}
