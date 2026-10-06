// Package types defines the IEC 61131-3 type system for the STC compiler.
// It provides type representations, a widening lattice encoding implicit
// conversion rules, and built-in type/function constants.
package types

import "fmt"

// TypeKind identifies the kind of a type in the IEC 61131-3 type system.
type TypeKind int

const (
	KindInvalid TypeKind = iota // Error/unknown type for propagation

	// ANY_BIT
	KindBOOL
	KindBYTE
	KindWORD
	KindDWORD
	KindLWORD

	// ANY_SIGNED
	KindSINT
	KindINT
	KindDINT
	KindLINT

	// ANY_UNSIGNED
	KindUSINT
	KindUINT
	KindUDINT
	KindULINT

	// ANY_REAL
	KindREAL
	KindLREAL

	// ANY_STRING
	KindSTRING
	KindWSTRING

	// ANY_DATE
	KindTIME
	KindDATE
	KindDT
	KindTOD

	// ANY_CHAR
	KindCHAR
	KindWCHAR

	// Pseudo-kinds for non-elementary types
	KindVoid          // Procedures with no return value
	KindArray
	KindStruct
	KindEnum
	KindFunctionBlock
	KindFunction
	KindPointer
	KindReference
)

var kindNames = [...]string{
	KindInvalid:       "Invalid",
	KindBOOL:          "BOOL",
	KindBYTE:          "BYTE",
	KindWORD:          "WORD",
	KindDWORD:         "DWORD",
	KindLWORD:         "LWORD",
	KindSINT:          "SINT",
	KindINT:           "INT",
	KindDINT:          "DINT",
	KindLINT:          "LINT",
	KindUSINT:         "USINT",
	KindUINT:          "UINT",
	KindUDINT:         "UDINT",
	KindULINT:         "ULINT",
	KindREAL:          "REAL",
	KindLREAL:         "LREAL",
	KindSTRING:        "STRING",
	KindWSTRING:       "WSTRING",
	KindTIME:          "TIME",
	KindDATE:          "DATE",
	KindDT:            "DT",
	KindTOD:           "TOD",
	KindCHAR:          "CHAR",
	KindWCHAR:         "WCHAR",
	KindVoid:          "VOID",
	KindArray:         "ARRAY",
	KindStruct:        "STRUCT",
	KindEnum:          "ENUM",
	KindFunctionBlock: "FUNCTION_BLOCK",
	KindFunction:      "FUNCTION",
	KindPointer:       "POINTER",
	KindReference:     "REFERENCE",
}

// String returns the IEC name for a TypeKind.
func (k TypeKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("TypeKind(%d)", k)
}

// Type is the interface implemented by all type representations.
type Type interface {
	Kind() TypeKind
	String() string
	Equal(Type) bool
}

// ParamDirection indicates the direction of a function/FB parameter.
type ParamDirection int

const (
	DirInput  ParamDirection = iota // VAR_INPUT
	DirOutput                       // VAR_OUTPUT
	DirInOut                        // VAR_IN_OUT
)

// Parameter represents a named parameter of a function or function block.
type Parameter struct {
	Name              string
	Type              Type
	Direction         ParamDirection
	GenericConstraint func(TypeKind) bool // For ANY_* generic parameters
}

// ArrayDimension represents a single array dimension with low and high bounds.
// Known is set when both bounds are integer literals; a bound such as
// GVL.MAX is not evaluated yet and leaves Known false (Low and High 0).
type ArrayDimension struct {
	Low   int
	High  int
	Known bool
	// Text is the source form of the bounds ("1..GVL.N") when they are
	// not Known, so two unknown dimensions compare by their expressions.
	Text string
}

// sameDimension reports whether two array dimensions denote the same
// range: equal bounds, both Known or both unknown, and for unknown bounds
// the same bound expressions. A Known dimension never equals an unknown one.
func sameDimension(a, b ArrayDimension) bool {
	return a.Known == b.Known && a.Low == b.Low && a.High == b.High && a.Text == b.Text
}

// StructMember represents a named member of a struct type.
type StructMember struct {
	Name string
	Type Type
}

// Invalid is the sentinel type used for error propagation.
var Invalid Type = &PrimitiveType{Kind_: KindInvalid}

// --- Concrete type implementations ---

// PrimitiveType represents an IEC elementary type (BOOL, INT, REAL, etc.).
type PrimitiveType struct {
	Kind_ TypeKind
}

func (t *PrimitiveType) Kind() TypeKind   { return t.Kind_ }
func (t *PrimitiveType) String() string   { return t.Kind_.String() }
func (t *PrimitiveType) Equal(o Type) bool {
	if p, ok := o.(*PrimitiveType); ok {
		return t.Kind_ == p.Kind_
	}
	return false
}

// ArrayType represents an ARRAY[dims] OF element_type.
type ArrayType struct {
	ElementType Type
	Dimensions  []ArrayDimension
}

func (t *ArrayType) Kind() TypeKind { return KindArray }
func (t *ArrayType) String() string {
	return fmt.Sprintf("ARRAY OF %s", t.ElementType.String())
}
func (t *ArrayType) Equal(o Type) bool {
	a, ok := o.(*ArrayType)
	if !ok {
		return false
	}
	if !t.ElementType.Equal(a.ElementType) {
		return false
	}
	if len(t.Dimensions) != len(a.Dimensions) {
		return false
	}
	for i, d := range t.Dimensions {
		if !sameDimension(d, a.Dimensions[i]) {
			return false
		}
	}
	return true
}

// StructType represents a STRUCT...END_STRUCT type.
type StructType struct {
	Name    string
	Members []StructMember
}

func (t *StructType) Kind() TypeKind { return KindStruct }
func (t *StructType) String() string { return t.Name }
func (t *StructType) Equal(o Type) bool {
	s, ok := o.(*StructType)
	if !ok {
		return false
	}
	return t.Name == s.Name
}

// EnumType represents an enumeration type.
type EnumType struct {
	Name     string
	BaseType TypeKind // integer or bit-string kind; INT when not declared
	Values   []string
	// Ordinals holds the numeric value of each entry of Values (same
	// index), numbered with the IEC previous+1 rule.
	Ordinals []int64
	// Qualified, Strict and ToString record the {attribute 'qualified_only'},
	// {attribute 'strict'} and {attribute 'to_string'} pragmas on the TYPE.
	Qualified bool
	Strict    bool
	ToString  bool
}

func (t *EnumType) Kind() TypeKind { return KindEnum }
func (t *EnumType) String() string { return t.Name }
func (t *EnumType) Equal(o Type) bool {
	e, ok := o.(*EnumType)
	if !ok {
		return false
	}
	return t.Name == e.Name
}

// FunctionBlockType represents a FUNCTION_BLOCK type.
type FunctionBlockType struct {
	Name    string
	Inputs  []Parameter
	Outputs []Parameter
	InOuts  []Parameter
	// Properties are the FB's PROPERTY declarations, readable and writable
	// as members (fb.Prop) from outside the FB.
	Properties []Parameter
	// ParamAliases maps an upper-cased alternative input name to the
	// canonical input it sets (the IEC R of CTU sets RESET). Only the
	// standard FBs have aliases.
	ParamAliases map[string]string
}

func (t *FunctionBlockType) Kind() TypeKind { return KindFunctionBlock }
func (t *FunctionBlockType) String() string { return t.Name }
func (t *FunctionBlockType) Equal(o Type) bool {
	fb, ok := o.(*FunctionBlockType)
	if !ok {
		return false
	}
	return t.Name == fb.Name
}

// FunctionType represents a FUNCTION type signature.
type FunctionType struct {
	Name       string
	ReturnType Type
	// Params lists the VAR_INPUT and VAR_IN_OUT parameters in declaration
	// order; positional arguments bind to them by index.
	Params []Parameter
	// Outputs lists the VAR_OUTPUT parameters, bound with name => target.
	Outputs []Parameter
}

func (t *FunctionType) Kind() TypeKind { return KindFunction }
func (t *FunctionType) String() string { return t.Name }
func (t *FunctionType) Equal(o Type) bool {
	f, ok := o.(*FunctionType)
	if !ok {
		return false
	}
	return t.Name == f.Name
}

// PointerType represents a POINTER TO base_type.
type PointerType struct {
	BaseType Type
}

func (t *PointerType) Kind() TypeKind { return KindPointer }
func (t *PointerType) String() string {
	return fmt.Sprintf("POINTER TO %s", t.BaseType.String())
}
func (t *PointerType) Equal(o Type) bool {
	p, ok := o.(*PointerType)
	if !ok {
		return false
	}
	return t.BaseType.Equal(p.BaseType)
}

// ReferenceType represents a REFERENCE TO base_type.
type ReferenceType struct {
	BaseType Type
}

func (t *ReferenceType) Kind() TypeKind { return KindReference }
func (t *ReferenceType) String() string {
	return fmt.Sprintf("REFERENCE TO %s", t.BaseType.String())
}
func (t *ReferenceType) Equal(o Type) bool {
	r, ok := o.(*ReferenceType)
	if !ok {
		return false
	}
	return t.BaseType.Equal(r.BaseType)
}
