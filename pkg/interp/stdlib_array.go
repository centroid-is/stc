package interp

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/types"
)

// Array introspection builtins.
//
// UPPER_BOUND(arr, dim) reports the highest valid index of an array, which is
// what lets a loop follow the declaration instead of repeating its size:
//
//	FOR i := 1 TO UPPER_BOUND(stConveyor.arBatches, 1) DO
//
// The bound is exact rather than inferred. Arrays are allocated with
// high+1 elements and indexed directly (see zeroFromArrayType), so the
// declared upper bound is len-1 whatever the declared lower bound was.
//
// LOWER_BOUND is deliberately absent. The lower bound is discarded at
// allocation, so it cannot be recovered here, and a LOWER_BOUND that
// guessed 0 or 1 would quietly mislead code that relied on it. Better to
// fail to compile than to loop over the wrong range.
func init() {
	StdlibFunctions["UPPER_BOUND"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "UPPER_BOUND requires an array"}
		}
		if args[0].Kind != ValArray {
			return Value{}, &RuntimeError{
				Msg: fmt.Sprintf("UPPER_BOUND expects an array, got %s", args[0].Kind),
			}
		}
		// The dimension argument is accepted for source compatibility with
		// TwinCAT/CODESYS. Only one dimension is representable here, so
		// anything other than 1 is a mistake worth reporting rather than
		// silently answering about dimension 1.
		if len(args) > 1 {
			if args[1].Kind != ValInt {
				return Value{}, &RuntimeError{Msg: "UPPER_BOUND dimension must be an integer"}
			}
			if args[1].Int != 1 {
				return Value{}, &RuntimeError{
					Msg: fmt.Sprintf("UPPER_BOUND: only dimension 1 is supported, got %d", args[1].Int),
				}
			}
		}
		return Value{
			Kind:    ValInt,
			Int:     int64(len(args[0].Array) - 1),
			IECType: types.KindDINT,
		}, nil
	}
}
