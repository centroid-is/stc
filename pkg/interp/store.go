package interp

import "github.com/centroid-is/stc/pkg/types"

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
		v = Value{Kind: ValReal, Real: float64(v.Int), IECType: k}
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
