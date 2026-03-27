package interp

import (
	"fmt"
	"math"
	"strconv"

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

	// DINT_TO_LREAL -- integer to LREAL
	StdlibFunctions["DINT_TO_LREAL"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "DINT_TO_LREAL requires 1 argument"}
		}
		return Value{Kind: ValReal, Real: float64(args[0].Int), IECType: types.KindLREAL}, nil
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

	// INT_TO_DINT, DINT_TO_INT -- integer width changes (same backing int64)
	for _, entry := range []struct {
		name    string
		iecType types.TypeKind
	}{
		{"INT_TO_DINT", types.KindDINT},
		{"DINT_TO_INT", types.KindINT},
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
		s := strconv.FormatFloat(args[0].Real, 'G', -1, 64)
		return Value{Kind: ValString, Str: s, IECType: types.KindSTRING}, nil
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
