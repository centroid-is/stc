package types

import "strings"

// Package-level Type constants for all IEC 61131-3 elementary types.
// Use these instead of constructing PrimitiveType values directly.
var (
	TypeBOOL    Type = &PrimitiveType{Kind_: KindBOOL}
	TypeBYTE    Type = &PrimitiveType{Kind_: KindBYTE}
	TypeWORD    Type = &PrimitiveType{Kind_: KindWORD}
	TypeDWORD   Type = &PrimitiveType{Kind_: KindDWORD}
	TypeLWORD   Type = &PrimitiveType{Kind_: KindLWORD}
	TypeSINT    Type = &PrimitiveType{Kind_: KindSINT}
	TypeINT     Type = &PrimitiveType{Kind_: KindINT}
	TypeDINT    Type = &PrimitiveType{Kind_: KindDINT}
	TypeLINT    Type = &PrimitiveType{Kind_: KindLINT}
	TypeUSINT   Type = &PrimitiveType{Kind_: KindUSINT}
	TypeUINT    Type = &PrimitiveType{Kind_: KindUINT}
	TypeUDINT   Type = &PrimitiveType{Kind_: KindUDINT}
	TypeULINT   Type = &PrimitiveType{Kind_: KindULINT}
	TypeREAL    Type = &PrimitiveType{Kind_: KindREAL}
	TypeLREAL   Type = &PrimitiveType{Kind_: KindLREAL}
	TypeSTRING  Type = &PrimitiveType{Kind_: KindSTRING}
	TypeWSTRING Type = &PrimitiveType{Kind_: KindWSTRING}
	TypeTIME    Type = &PrimitiveType{Kind_: KindTIME}
	TypeDATE    Type = &PrimitiveType{Kind_: KindDATE}
	TypeDT      Type = &PrimitiveType{Kind_: KindDT}
	TypeTOD     Type = &PrimitiveType{Kind_: KindTOD}
	TypeCHAR    Type = &PrimitiveType{Kind_: KindCHAR}
	TypeWCHAR   Type = &PrimitiveType{Kind_: KindWCHAR}
	TypeVOID    Type = &PrimitiveType{Kind_: KindVoid}
)

// elementaryTypes maps uppercase type names to their Type constants.
var elementaryTypes = map[string]Type{
	"BOOL":          TypeBOOL,
	"BYTE":          TypeBYTE,
	"WORD":          TypeWORD,
	"DWORD":         TypeDWORD,
	"LWORD":         TypeLWORD,
	"SINT":          TypeSINT,
	"INT":           TypeINT,
	"DINT":          TypeDINT,
	"LINT":          TypeLINT,
	"USINT":         TypeUSINT,
	"UINT":          TypeUINT,
	"UDINT":         TypeUDINT,
	"ULINT":         TypeULINT,
	"REAL":          TypeREAL,
	"LREAL":         TypeLREAL,
	"STRING":        TypeSTRING,
	"WSTRING":       TypeWSTRING,
	"TIME":          TypeTIME,
	"DATE":          TypeDATE,
	"DATE_AND_TIME": TypeDT,
	"DT":            TypeDT,
	"TIME_OF_DAY":   TypeTOD,
	"TOD":           TypeTOD,
	"CHAR":          TypeCHAR,
	"WCHAR":         TypeWCHAR,
}

// LookupElementaryType resolves a type name (case-insensitive) to its Type constant.
// Returns (type, true) if found, or (nil, false) if not an elementary type.
func LookupElementaryType(name string) (Type, bool) {
	t, ok := elementaryTypes[strings.ToUpper(name)]
	return t, ok
}

// BuiltinFunctions is the registry of standard IEC 61131-3 function signatures.
// These are used by the type checker to validate function call arguments.
// Function bodies are implemented in Phase 4 (Standard Library).
var BuiltinFunctions map[string]*FunctionType

func init() {
	BuiltinFunctions = make(map[string]*FunctionType)

	// Helper to create parameters with generic constraints
	numParam := func(name string) Parameter {
		return Parameter{Name: name, Type: TypeREAL, Direction: DirInput, GenericConstraint: IsAnyNum}
	}
	intParam := func(name string) Parameter {
		return Parameter{Name: name, Type: TypeINT, Direction: DirInput, GenericConstraint: IsAnyInt}
	}
	realParam := func(name string) Parameter {
		return Parameter{Name: name, Type: TypeREAL, Direction: DirInput, GenericConstraint: IsAnyReal}
	}
	strParam := func(name string) Parameter {
		return Parameter{Name: name, Type: TypeSTRING, Direction: DirInput, GenericConstraint: IsAnyString}
	}
	boolParam := func(name string) Parameter {
		return Parameter{Name: name, Type: TypeBOOL, Direction: DirInput}
	}
	anyParam := func(name string) Parameter {
		return Parameter{Name: name, Type: nil, Direction: DirInput} // nil Type = ANY
	}

	// Arithmetic: ANY_NUM -> ANY_NUM
	for _, name := range []string{"ADD", "SUB", "MUL", "DIV"} {
		BuiltinFunctions[name] = &FunctionType{
			Name:       name,
			ReturnType: TypeREAL, // Generic: returns same type as args
			Params:     []Parameter{numParam("IN1"), numParam("IN2")},
		}
	}
	BuiltinFunctions["MOD"] = &FunctionType{
		Name:       "MOD",
		ReturnType: TypeINT,
		Params:     []Parameter{intParam("IN1"), intParam("IN2")},
	}

	// Math: ANY_REAL -> ANY_REAL
	for _, name := range []string{"ABS", "SQRT", "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP"} {
		BuiltinFunctions[name] = &FunctionType{
			Name:       name,
			ReturnType: TypeREAL,
			Params:     []Parameter{realParam("IN")},
		}
	}

	// MIN, MAX: ANY_NUM, ANY_NUM -> ANY_NUM
	for _, name := range []string{"MIN", "MAX"} {
		BuiltinFunctions[name] = &FunctionType{
			Name:       name,
			ReturnType: TypeREAL,
			Params:     []Parameter{numParam("IN1"), numParam("IN2")},
		}
	}

	// LIMIT: ANY_NUM, ANY_NUM, ANY_NUM -> ANY_NUM
	BuiltinFunctions["LIMIT"] = &FunctionType{
		Name:       "LIMIT",
		ReturnType: TypeREAL,
		Params:     []Parameter{numParam("MN"), numParam("IN"), numParam("MX")},
	}

	// SEL: BOOL, ANY, ANY -> ANY
	BuiltinFunctions["SEL"] = &FunctionType{
		Name:       "SEL",
		ReturnType: nil, // Generic: returns same type as IN0/IN1
		Params:     []Parameter{boolParam("G"), anyParam("IN0"), anyParam("IN1")},
	}

	// MUX: ANY_INT, ANY... -> ANY
	BuiltinFunctions["MUX"] = &FunctionType{
		Name:       "MUX",
		ReturnType: nil,
		Params:     []Parameter{intParam("K"), anyParam("IN0"), anyParam("IN1")},
	}

	// MOVE: ANY -> ANY
	BuiltinFunctions["MOVE"] = &FunctionType{
		Name:       "MOVE",
		ReturnType: nil,
		Params:     []Parameter{anyParam("IN")},
	}

	// Type conversion functions: *_TO_*
	conversions := []struct {
		name string
		from Type
		to   Type
	}{
		{"INT_TO_REAL", TypeINT, TypeREAL},
		{"REAL_TO_INT", TypeREAL, TypeINT},
		{"DINT_TO_REAL", TypeDINT, TypeREAL},
		{"REAL_TO_DINT", TypeREAL, TypeDINT},
		{"INT_TO_DINT", TypeINT, TypeDINT},
		{"DINT_TO_INT", TypeDINT, TypeINT},
		{"INT_TO_STRING", TypeINT, TypeSTRING},
		{"STRING_TO_INT", TypeSTRING, TypeINT},
		{"REAL_TO_STRING", TypeREAL, TypeSTRING},
		{"STRING_TO_REAL", TypeSTRING, TypeREAL},
		{"BOOL_TO_INT", TypeBOOL, TypeINT},
		{"INT_TO_BOOL", TypeINT, TypeBOOL},
		{"BOOL_TO_STRING", TypeBOOL, TypeSTRING},
		{"BYTE_TO_INT", TypeBYTE, TypeINT},
		{"INT_TO_BYTE", TypeINT, TypeBYTE},
		{"DINT_TO_LREAL", TypeDINT, TypeLREAL},
		{"LREAL_TO_DINT", TypeLREAL, TypeDINT},
	}
	for _, c := range conversions {
		BuiltinFunctions[c.name] = &FunctionType{
			Name:       c.name,
			ReturnType: c.to,
			Params:     []Parameter{{Name: "IN", Type: c.from, Direction: DirInput}},
		}
	}

	// String functions
	BuiltinFunctions["LEN"] = &FunctionType{
		Name:       "LEN",
		ReturnType: TypeINT,
		Params:     []Parameter{strParam("IN")},
	}
	BuiltinFunctions["CONCAT"] = &FunctionType{
		Name:       "CONCAT",
		ReturnType: TypeSTRING,
		Params:     []Parameter{strParam("IN1"), strParam("IN2")},
	}
	BuiltinFunctions["LEFT"] = &FunctionType{
		Name:       "LEFT",
		ReturnType: TypeSTRING,
		Params:     []Parameter{strParam("IN"), intParam("L")},
	}
	BuiltinFunctions["RIGHT"] = &FunctionType{
		Name:       "RIGHT",
		ReturnType: TypeSTRING,
		Params:     []Parameter{strParam("IN"), intParam("L")},
	}
	BuiltinFunctions["MID"] = &FunctionType{
		Name:       "MID",
		ReturnType: TypeSTRING,
		Params:     []Parameter{strParam("IN"), intParam("L"), intParam("P")},
	}
	BuiltinFunctions["FIND"] = &FunctionType{
		Name:       "FIND",
		ReturnType: TypeINT,
		Params:     []Parameter{strParam("IN1"), strParam("IN2")},
	}
}
