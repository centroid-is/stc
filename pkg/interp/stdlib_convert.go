package interp

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

func init() {
	registerConvertFunctions()
}

func registerConvertFunctions() {
	// INT_TO_REAL, SINT_TO_REAL, DINT_TO_REAL -- integer to float64
	for _, name := range []string{"INT_TO_REAL", "SINT_TO_REAL", "DINT_TO_REAL"} {
		StdlibFunctions[name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "conversion requires 1 argument"}
			}
			return Value{Kind: ValReal, Real: float64(args[0].Int), IECType: types.KindREAL}, nil
		}
	}

	// DINT_TO_LREAL, UDINT_TO_LREAL -- integer to LREAL
	for _, name := range []string{"DINT_TO_LREAL", "UDINT_TO_LREAL"} {
		StdlibFunctions[name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "conversion requires 1 argument"}
			}
			return Value{Kind: ValReal, Real: float64(args[0].Int), IECType: types.KindLREAL}, nil
		}
	}

	// UDINT_TO_REAL, UINT_TO_REAL -- unsigned integer to REAL
	for _, name := range []string{"UDINT_TO_REAL", "UINT_TO_REAL", "USINT_TO_REAL"} {
		StdlibFunctions[name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "conversion requires 1 argument"}
			}
			return Value{Kind: ValReal, Real: float64(args[0].Int), IECType: types.KindREAL}, nil
		}
	}

	// REAL_TO_LREAL -- widening float conversion (same float64 backing)
	StdlibFunctions["REAL_TO_LREAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "REAL_TO_LREAL requires 1 argument"}
		}
		return Value{Kind: ValReal, Real: args[0].Real, IECType: types.KindLREAL}, nil
	}

	// LREAL_TO_REAL -- narrowing float conversion (same float64 backing;
	// true single-precision truncation is not modelled)
	StdlibFunctions["LREAL_TO_REAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "LREAL_TO_REAL requires 1 argument"}
		}
		return Value{Kind: ValReal, Real: args[0].Real, IECType: types.KindREAL}, nil
	}

	// TIME_TO_REAL -- duration in milliseconds as REAL, matching TwinCAT
	// semantics (TIME resolution is 1 ms).
	StdlibFunctions["TIME_TO_REAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "TIME_TO_REAL requires 1 argument"}
		}
		return Value{Kind: ValReal, Real: float64(args[0].Time.Milliseconds()), IECType: types.KindREAL}, nil
	}

	// REAL_TO_INT, LREAL_TO_DINT -- float to integer with banker's rounding
	for _, entry := range []struct {
		name    string
		iecType types.TypeKind
	}{
		{"REAL_TO_INT", types.KindINT},
		{"REAL_TO_DINT", types.KindDINT},
		{"LREAL_TO_DINT", types.KindDINT},
	} {
		iecType := entry.iecType
		StdlibFunctions[entry.name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "conversion requires 1 argument"}
			}
			rounded := math.RoundToEven(args[0].Real)
			return Value{Kind: ValInt, Int: int64(rounded), IECType: iecType}, nil
		}
	}

	// TRUNC, TRUNC_INT, TRUNC_DINT -- float to integer, truncating toward zero
	for _, entry := range []struct {
		name    string
		iecType types.TypeKind
	}{
		{"TRUNC", types.KindDINT},
		{"TRUNC_INT", types.KindINT},
		{"TRUNC_DINT", types.KindDINT},
	} {
		iecType := entry.iecType
		StdlibFunctions[entry.name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "TRUNC requires 1 argument"}
			}
			return Value{Kind: ValInt, Int: int64(math.Trunc(toFloat(args[0]))), IECType: iecType}, nil
		}
	}

	// TIME_TO_DINT, TIME_TO_LREAL -- duration expressed in milliseconds
	StdlibFunctions["TIME_TO_DINT"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "TIME_TO_DINT requires 1 argument"}
		}
		return Value{Kind: ValInt, Int: args[0].Time.Milliseconds(), IECType: types.KindDINT}, nil
	}
	StdlibFunctions["TIME_TO_LREAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "TIME_TO_LREAL requires 1 argument"}
		}
		ms := float64(args[0].Time) / float64(time.Millisecond)
		return Value{Kind: ValReal, Real: ms, IECType: types.KindLREAL}, nil
	}

	// INT_TO_DINT, DINT_TO_INT, ... -- integer width/sign changes (same
	// backing int64; overflow wrapping is not modelled)
	for _, entry := range []struct {
		name    string
		iecType types.TypeKind
	}{
		{"INT_TO_DINT", types.KindDINT},
		{"DINT_TO_INT", types.KindINT},
		{"INT_TO_UDINT", types.KindUDINT},
		{"UDINT_TO_INT", types.KindINT},
		{"UDINT_TO_DINT", types.KindDINT},
		{"DINT_TO_UDINT", types.KindUDINT},
		{"UINT_TO_UDINT", types.KindUDINT},
		{"UDINT_TO_UINT", types.KindUINT},
		{"UINT_TO_INT", types.KindINT},
		{"INT_TO_UINT", types.KindUINT},
	} {
		iecType := entry.iecType
		StdlibFunctions[entry.name] = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return Value{}, &RuntimeError{Msg: "conversion requires 1 argument"}
			}
			return Value{Kind: ValInt, Int: args[0].Int, IECType: iecType}, nil
		}
	}

	// BOOL_TO_INT: FALSE -> 0, TRUE -> 1
	StdlibFunctions["BOOL_TO_INT"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "BOOL_TO_INT requires 1 argument"}
		}
		var v int64
		if args[0].Bool {
			v = 1
		}
		return Value{Kind: ValInt, Int: v, IECType: types.KindINT}, nil
	}

	// INT_TO_BOOL: 0 -> FALSE, nonzero -> TRUE
	StdlibFunctions["INT_TO_BOOL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "INT_TO_BOOL requires 1 argument"}
		}
		return Value{Kind: ValBool, Bool: args[0].Int != 0, IECType: types.KindBOOL}, nil
	}

	// BOOL_TO_STRING: TRUE -> "TRUE", FALSE -> "FALSE"
	StdlibFunctions["BOOL_TO_STRING"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "BOOL_TO_STRING requires 1 argument"}
		}
		s := "FALSE"
		if args[0].Bool {
			s = "TRUE"
		}
		return Value{Kind: ValString, Str: s, IECType: types.KindSTRING}, nil
	}

	// INT_TO_STRING
	StdlibFunctions["INT_TO_STRING"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "INT_TO_STRING requires 1 argument"}
		}
		s := strconv.FormatInt(args[0].Int, 10)
		return Value{Kind: ValString, Str: s, IECType: types.KindSTRING}, nil
	}

	// STRING_TO_INT
	StdlibFunctions["STRING_TO_INT"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "STRING_TO_INT requires 1 argument"}
		}
		n, err := strconv.ParseInt(args[0].Str, 10, 64)
		if err != nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("STRING_TO_INT: invalid integer %q", args[0].Str)}
		}
		return Value{Kind: ValInt, Int: n, IECType: types.KindINT}, nil
	}

	// REAL_TO_STRING
	StdlibFunctions["REAL_TO_STRING"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "REAL_TO_STRING requires 1 argument"}
		}
		return Value{Kind: ValString, Str: formatReal(args[0].Real), IECType: types.KindSTRING}, nil
	}

	// TO_STRING: overloaded conversion. The interpreter answers a to_string
	// enum value with its name before reaching this function (see evalCall).
	StdlibFunctions["TO_STRING"] = func(args []Value) (Value, error) {
		if len(args) != 1 {
			return Value{}, &RuntimeError{Msg: "TO_STRING requires 1 argument"}
		}
		return Value{Kind: ValString, Str: anyToString(args[0]), IECType: types.KindSTRING}, nil
	}

	// STRING_TO_REAL
	StdlibFunctions["STRING_TO_REAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "STRING_TO_REAL requires 1 argument"}
		}
		f, err := strconv.ParseFloat(args[0].Str, 64)
		if err != nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("STRING_TO_REAL: invalid real %q", args[0].Str)}
		}
		return Value{Kind: ValReal, Real: f, IECType: types.KindLREAL}, nil
	}

	// BYTE_TO_INT
	StdlibFunctions["BYTE_TO_INT"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "BYTE_TO_INT requires 1 argument"}
		}
		return Value{Kind: ValInt, Int: args[0].Int & 0xFF, IECType: types.KindINT}, nil
	}

	// INT_TO_BYTE (mask to 8 bits)
	StdlibFunctions["INT_TO_BYTE"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "INT_TO_BYTE requires 1 argument"}
		}
		return Value{Kind: ValInt, Int: args[0].Int & 0xFF, IECType: types.KindBYTE}, nil
	}
}

// formatReal is the REAL_TO_STRING text of f.
func formatReal(f float64) string {
	return strconv.FormatFloat(f, 'G', -1, 64)
}

// anyToString formats v for TO_STRING by kind: integers in decimal, BOOL as
// TRUE/FALSE, reals like REAL_TO_STRING, strings unchanged and every other
// kind by its literal form.
func anyToString(v Value) string {
	switch v.Kind {
	case ValInt:
		return strconv.FormatInt(v.Int, 10)
	case ValBool:
		if v.Bool {
			return "TRUE"
		}
		return "FALSE"
	case ValReal:
		return formatReal(v.Real)
	case ValString:
		return v.Str
	default:
		return v.String()
	}
}
