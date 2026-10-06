package interp

import "github.com/centroid-is/stc/pkg/types"

// IEC 61131-3 bit shift and rotate functions (Phase 26-04, needed by
// FB_EcDeviceDiag): SHL(IN, N), SHR(IN, N), ROL(IN, N), ROR(IN, N).
// The width is that of IN's type (BYTE 8 .. LWORD 64); untyped or unknown
// inputs use 32 bits. The result keeps IN's type and is masked to its width.
func init() {
	for name, op := range map[string]func(x uint64, n, w uint) uint64{
		"SHL": func(x uint64, n, w uint) uint64 { return x << n },
		"SHR": func(x uint64, n, w uint) uint64 { return x >> n },
		"ROL": func(x uint64, n, w uint) uint64 { n %= w; return x<<n | x>>(w-n) },
		"ROR": func(x uint64, n, w uint) uint64 { n %= w; return x>>n | x<<(w-n) },
	} {
		name, op := name, op
		StdlibFunctions[name] = func(args []Value) (Value, error) {
			if len(args) != 2 || args[0].Kind != ValInt || args[1].Kind != ValInt {
				return Value{}, &RuntimeError{Msg: name + " requires an integer IN and N"}
			}
			if args[1].Int < 0 {
				return Value{}, &RuntimeError{Msg: name + ": N must not be negative"}
			}
			w, _ := intWidth(args[0].IECType)
			if w == 0 {
				w = 32
			}
			mask := uint64(1)<<uint(w) - 1
			if w == 64 {
				mask = ^uint64(0)
			}
			x := uint64(args[0].Int) & mask
			n := uint(args[1].Int)
			var r uint64
			if n >= uint(w) && (name == "SHL" || name == "SHR") {
				r = 0
			} else {
				r = op(x, n, uint(w)) & mask
			}
			kind := args[0].IECType
			if kind == types.KindInvalid {
				kind = types.KindDWORD
			}
			return Value{Kind: ValInt, Int: int64(r), IECType: kind}, nil
		}
	}
}
