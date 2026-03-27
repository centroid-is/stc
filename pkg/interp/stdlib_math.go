package interp

import (
	"fmt"
	"math"

	"github.com/centroid-is/stc/pkg/types"
)

// StdlibFunctions maps IEC 61131-3 standard function names to their
// Go implementations. The interpreter's evalCall dispatches here before
// falling through to user-defined functions.
var StdlibFunctions = map[string]func(args []Value) (Value, error){}

func init() {
	registerMathFunctions()
}

func registerMathFunctions() {
	// ABS: absolute value for int and real
	StdlibFunctions["ABS"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "ABS requires 1 argument"}
		}
		a := args[0]
		switch a.Kind {
		case ValInt:
			if a.Int < 0 {
				return Value{Kind: ValInt, Int: -a.Int, IECType: a.IECType}, nil
			}
			return a, nil
		case ValReal:
			return Value{Kind: ValReal, Real: math.Abs(a.Real), IECType: a.IECType}, nil
		default:
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("ABS: unsupported type %s", a.Kind)}
		}
	}

	// Single-argument real math functions
	realFuncs := map[string]func(float64) float64{
		"SQRT": math.Sqrt,
		"SIN":  math.Sin,
		"COS":  math.Cos,
		"TAN":  math.Tan,
		"ASIN": math.Asin,
		"ACOS": math.Acos,
		"ATAN": math.Atan,
		"LN":   math.Log,
		"LOG":  math.Log10,
		"EXP":  math.Exp,
	}
	for name, fn := range realFuncs {
		fn := fn // capture
		StdlibFunctions[name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: fmt.Sprintf("%s requires 1 argument", name)}
			}
			f := toFloat(args[0])
			return Value{Kind: ValReal, Real: fn(f), IECType: types.KindLREAL}, nil
		}
	}

	// EXPT: power, always returns real
	StdlibFunctions["EXPT"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "EXPT requires 2 arguments"}
		}
		base := toFloat(args[0])
		exp := toFloat(args[1])
		return Value{Kind: ValReal, Real: math.Pow(base, exp), IECType: types.KindLREAL}, nil
	}

	// MIN: returns smaller of two values
	StdlibFunctions["MIN"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "MIN requires 2 arguments"}
		}
		a, b := args[0], args[1]
		if a.Kind == ValReal || b.Kind == ValReal {
			af, bf := toFloat(a), toFloat(b)
			if af <= bf {
				return Value{Kind: ValReal, Real: af, IECType: types.KindLREAL}, nil
			}
			return Value{Kind: ValReal, Real: bf, IECType: types.KindLREAL}, nil
		}
		if a.Int <= b.Int {
			return a, nil
		}
		return b, nil
	}

	// MAX: returns larger of two values
	StdlibFunctions["MAX"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "MAX requires 2 arguments"}
		}
		a, b := args[0], args[1]
		if a.Kind == ValReal || b.Kind == ValReal {
			af, bf := toFloat(a), toFloat(b)
			if af >= bf {
				return Value{Kind: ValReal, Real: af, IECType: types.KindLREAL}, nil
			}
			return Value{Kind: ValReal, Real: bf, IECType: types.KindLREAL}, nil
		}
		if a.Int >= b.Int {
			return a, nil
		}
		return b, nil
	}

	// LIMIT(MN, IN, MX): clamp IN to [MN, MX]
	StdlibFunctions["LIMIT"] = func(args []Value) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "LIMIT requires 3 arguments (MN, IN, MX)"}
		}
		mn, in, mx := args[0], args[1], args[2]
		if mn.Kind == ValReal || in.Kind == ValReal || mx.Kind == ValReal {
			mnf, inf, mxf := toFloat(mn), toFloat(in), toFloat(mx)
			r := math.Max(mnf, math.Min(inf, mxf))
			return Value{Kind: ValReal, Real: r, IECType: types.KindLREAL}, nil
		}
		// Integer path
		v := in.Int
		if v < mn.Int {
			v = mn.Int
		}
		if v > mx.Int {
			v = mx.Int
		}
		return Value{Kind: ValInt, Int: v, IECType: in.IECType}, nil
	}

	// SEL(G, IN0, IN1): if G then IN1 else IN0
	StdlibFunctions["SEL"] = func(args []Value) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "SEL requires 3 arguments (G, IN0, IN1)"}
		}
		if args[0].IsTruthy() {
			return args[2], nil
		}
		return args[1], nil
	}

	// MUX(K, IN0, IN1, ...): select args[K+1]
	StdlibFunctions["MUX"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "MUX requires at least 2 arguments (K, IN0)"}
		}
		k := int(args[0].Int)
		idx := k + 1 // offset: K is at position 0, inputs start at 1
		if idx < 1 || idx >= len(args) {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("MUX index %d out of range (0..%d)", k, len(args)-2)}
		}
		return args[idx], nil
	}

	// MOVE(IN): passthrough
	StdlibFunctions["MOVE"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "MOVE requires 1 argument"}
		}
		return args[0], nil
	}
}
