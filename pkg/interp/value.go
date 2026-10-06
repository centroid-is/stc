// Package interp implements a tree-walking interpreter for IEC 61131-3
// Structured Text. It evaluates typed ASTs with PLC scan-cycle semantics.
package interp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// ValueKind identifies the runtime type of a Value.
type ValueKind int

const (
	ValBool       ValueKind = iota // bool
	ValInt                         // int64 backing for all integer types
	ValReal                        // float64 backing for REAL/LREAL
	ValString                      // string
	ValTime                        // time.Duration
	ValDate                        // time.Duration (days since epoch)
	ValDateTime                    // time.Duration
	ValTod                         // time.Duration (time of day)
	ValArray                       // []Value
	ValStruct                      // map[string]Value
	ValFBInstance                  // Reference to a function block instance
	ValPointer                     // Pointer to a variable (env + name)
	ValReference                   // Reference to a variable (auto-deref pointer)
)

var valueKindNames = [...]string{
	ValBool:       "Bool",
	ValInt:        "Int",
	ValReal:       "Real",
	ValString:     "String",
	ValTime:       "Time",
	ValDate:       "Date",
	ValDateTime:   "DateTime",
	ValTod:        "Tod",
	ValArray:      "Array",
	ValStruct:     "Struct",
	ValFBInstance: "FBInstance",
	ValPointer:    "Pointer",
	ValReference:  "Reference",
}

// String returns the human-readable name of a ValueKind.
func (k ValueKind) String() string {
	if int(k) < len(valueKindNames) {
		return valueKindNames[k]
	}
	return fmt.Sprintf("ValueKind(%d)", k)
}

// Value is a tagged union representing a runtime value in the interpreter.
// The Kind field indicates which payload field is active.
type Value struct {
	Kind    ValueKind
	Bool    bool
	Int     int64
	Real    float64
	Str     string
	Time    time.Duration
	Array   []Value
	Struct  map[string]Value
	FBRef   *FBInstance     // Reference to a function block instance
	IECType types.TypeKind // Tracks the precise IEC type for conversions
	PtrEnv  *Env           // For ValPointer/ValReference: the env containing the target
	PtrVar  string         // For ValPointer/ValReference: uppercase variable name
	Enum    string         // Upper-case enum type name for enum values, else empty

	// superDecl is set on the value of SUPER (a ValPointer with FBRef) and
	// of SUPER^ (a ValFBInstance): methods and the body then come from this
	// base declaration instead of the instance's own type.
	superDecl *ast.FunctionBlockDecl
}

// String returns a debug representation of the Value.
func (v Value) String() string {
	switch v.Kind {
	case ValBool:
		if v.Bool {
			return "TRUE"
		}
		return "FALSE"
	case ValInt:
		return fmt.Sprintf("%d", v.Int)
	case ValReal:
		return fmt.Sprintf("%g", v.Real)
	case ValString:
		return fmt.Sprintf("'%s'", v.Str)
	case ValTime:
		return fmt.Sprintf("T#%s", v.Time)
	case ValArray:
		return fmt.Sprintf("[%d elements]", len(v.Array))
	case ValStruct:
		return fmt.Sprintf("{%d fields}", len(v.Struct))
	case ValFBInstance:
		return "FB_INSTANCE"
	case ValPointer:
		return fmt.Sprintf("PTR(%s)", v.PtrVar)
	case ValReference:
		return fmt.Sprintf("REF(%s)", v.PtrVar)
	default:
		return fmt.Sprintf("Value(%v)", v.Kind)
	}
}

// Clone returns a deep copy of the Value.
//
// ARRAY and STRUCT are value types in IEC 61131-3: assigning one to another
// copies the contents, and later writes to the source must not be visible
// through the destination. The interpreter backs them with Go slices and maps,
// which are reference types, so any assignment or copy of an aggregate has to
// clone it explicitly.
//
// FB instances, pointers and references are deliberately NOT deep-copied --
// they are reference values by nature, and VAR_IN_OUT relies on sharing them.
func (v Value) Clone() Value {
	switch v.Kind {
	case ValArray:
		if v.Array == nil {
			return v
		}
		out := v
		out.Array = make([]Value, len(v.Array))
		for i := range v.Array {
			out.Array[i] = v.Array[i].Clone()
		}
		return out
	case ValStruct:
		if v.Struct == nil {
			return v
		}
		out := v
		out.Struct = make(map[string]Value, len(v.Struct))
		for k, f := range v.Struct {
			out.Struct[k] = f.Clone()
		}
		return out
	default:
		return v
	}
}

// IsAggregate reports whether the Value is an ARRAY or STRUCT, i.e. a value
// type whose backing store must be cloned on assignment.
func (v Value) IsAggregate() bool {
	return v.Kind == ValArray || v.Kind == ValStruct
}

// IsTruthy returns whether the Value represents a truthy value.
func (v Value) IsTruthy() bool {
	switch v.Kind {
	case ValBool:
		return v.Bool
	case ValInt:
		return v.Int != 0
	case ValReal:
		return v.Real != 0
	case ValString:
		return v.Str != ""
	default:
		return false
	}
}

// Zero returns the zero value for a given IEC type kind.
func Zero(kind types.TypeKind) Value {
	switch kind {
	case types.KindBOOL:
		return Value{Kind: ValBool, Bool: false, IECType: kind}
	case types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT:
		return Value{Kind: ValInt, Int: 0, IECType: kind}
	case types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT:
		return Value{Kind: ValInt, Int: 0, IECType: kind}
	case types.KindBYTE, types.KindWORD, types.KindDWORD, types.KindLWORD:
		return Value{Kind: ValInt, Int: 0, IECType: kind}
	case types.KindREAL, types.KindLREAL:
		return Value{Kind: ValReal, Real: 0.0, IECType: kind}
	case types.KindSTRING, types.KindWSTRING:
		return Value{Kind: ValString, Str: "", IECType: kind}
	case types.KindTIME:
		return Value{Kind: ValTime, Time: 0, IECType: kind}
	case types.KindDATE:
		return Value{Kind: ValDate, Time: 0, IECType: kind}
	case types.KindDT:
		return Value{Kind: ValDateTime, Time: 0, IECType: kind}
	case types.KindTOD:
		return Value{Kind: ValTod, Time: 0, IECType: kind}
	default:
		return Value{Kind: ValInt, Int: 0, IECType: kind}
	}
}

// BoolValue is a convenience constructor for a boolean Value.
func BoolValue(b bool) Value {
	return Value{Kind: ValBool, Bool: b, IECType: types.KindBOOL}
}

// IntValue is a convenience constructor for an integer Value.
func IntValue(n int64) Value {
	return Value{Kind: ValInt, Int: n, IECType: types.KindDINT}
}

// RealValue is a convenience constructor for a real Value.
func RealValue(f float64) Value {
	return Value{Kind: ValReal, Real: f, IECType: types.KindLREAL}
}

// StringValue is a convenience constructor for a string Value.
func StringValue(s string) Value {
	return Value{Kind: ValString, Str: s, IECType: types.KindSTRING}
}

// TimeValue is a convenience constructor for a time Value.
func TimeValue(d time.Duration) Value {
	return Value{Kind: ValTime, Time: d, IECType: types.KindTIME}
}

// MarshalJSON encodes a Value as a JSON value appropriate to its kind.
// Bool -> JSON bool, Int -> JSON number, Real -> JSON number,
// String -> JSON string, Time -> JSON string, others -> JSON string.
func (v Value) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case ValBool:
		if v.Bool {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case ValInt:
		return []byte(fmt.Sprintf("%d", v.Int)), nil
	case ValReal:
		return []byte(fmt.Sprintf("%g", v.Real)), nil
	case ValString:
		b, err := json.Marshal(v.Str)
		return b, err
	case ValTime:
		b, err := json.Marshal(v.Time.String())
		return b, err
	default:
		b, err := json.Marshal(v.String())
		return b, err
	}
}
