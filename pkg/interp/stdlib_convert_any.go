package interp

import "github.com/centroid-is/stc/pkg/types"

// Typed integer and bit-string conversions X_TO_Y (Phase 26-04): every pair
// of SINT..ULINT, BYTE..LWORD, BOOL as source and REAL/LREAL as target that
// has no dedicated implementation goes through convertTo and wraps to the
// target width, so results are right inside expressions too. FB_ATV320 needs
// UINT_TO_WORD; the rest of the family comes along for free.
func init() {
	ints := []types.TypeKind{
		types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT,
		types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT,
		types.KindBYTE, types.KindWORD, types.KindDWORD, types.KindLWORD,
	}
	srcs := append([]types.TypeKind{types.KindBOOL}, ints...)
	dsts := append(append([]types.TypeKind{}, ints...), types.KindREAL, types.KindLREAL)
	for _, src := range srcs {
		for _, d := range dsts {
			if src == d {
				continue
			}
			kind := d
			name := src.String() + "_TO_" + kind.String()
			if _, ok := StdlibFunctions[name]; ok {
				continue
			}
			StdlibFunctions[name] = func(args []Value) (Value, error) {
				if len(args) != 1 {
					return Value{}, &RuntimeError{Msg: name + " requires 1 argument"}
				}
				v, err := convertTo(name, kind, args[0])
				if err == nil && v.Kind == ValInt {
					v.Int = wrapInt(v.Int, kind)
				}
				return v, err
			}
		}
	}
}
