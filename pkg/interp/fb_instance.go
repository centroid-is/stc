package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// StandardFB is the interface that all standard library function blocks
// (TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, SR, RS, etc.) must implement.
// Plan 04 will populate StdlibFBFactory with constructors for each.
type StandardFB interface {
	Execute(dt time.Duration)
	SetInput(name string, v Value)
	GetOutput(name string) Value
	GetInput(name string) Value
}

// StdlibFBFactory maps FB type names (uppercase) to constructor functions.
// Plan 04 will register standard library FBs here.
var StdlibFBFactory = map[string]func() StandardFB{}

// FBInstance wraps either a StandardFB (for stdlib FBs) or an Env+Decl
// pair (for user-defined FBs). It provides a unified interface for FB
// call statements and member access.
type FBInstance struct {
	TypeName string

	// For stdlib FBs (non-nil when wrapping a StandardFB implementation)
	FB StandardFB

	// For user-defined FBs
	Env         *Env
	Decl        *ast.FunctionBlockDecl
	ParentDecl  *ast.FunctionBlockDecl // parent FB decl for EXTENDS chain
	inputNames  []string               // VAR_INPUT variable names (uppercase)
	outputNames []string               // VAR_OUTPUT variable names (uppercase)
}

// NewUserFBInstance creates an FBInstance for a user-defined function block.
// It initializes a new Env with all variables from the declaration's VarBlocks,
// using zero values based on type names. The env persists across Execute calls.
//
// A VAR whose type is itself a function block -- stdlib (TON, R_TRIG, ...) or
// user-defined via the interpreter's FBDecls registry -- is instantiated
// recursively, so FB composition works at any depth.
func NewUserFBInstance(name string, decl *ast.FunctionBlockDecl, interp *Interpreter, parentEnv *Env) *FBInstance {
	return newUserFBInstanceDepth(name, decl, interp, parentEnv, 0)
}

// maxFBNestDepth bounds recursive FB instantiation. IEC 61131-3 forbids an FB
// containing itself, but a registry cycle must not hang the interpreter.
const maxFBNestDepth = 32

func newUserFBInstanceDepth(name string, decl *ast.FunctionBlockDecl, interp *Interpreter, parentEnv *Env, depth int) *FBInstance {
	env := NewEnv(parentEnv)
	inst := &FBInstance{
		TypeName: name,
		Decl:     decl,
		Env:      env,
	}

	// Walk VarBlocks, initialize variables, and track input/output names
	resolve := interp.TypeResolverFunc()
	for _, vb := range decl.VarBlocks {
		for _, vd := range vb.Declarations {
			// FB-typed member: instantiate rather than zero-fill. One shared
			// value must never be defined for several names, so instantiate
			// per name below.
			typeName := typeNameFromSpec(vd.Type)
			upperType := strings.ToUpper(typeName)
			isStdlibFB := false
			var nestedDecl *ast.FunctionBlockDecl
			if typeName != "" && depth < maxFBNestDepth {
				if _, ok := StdlibFBFactory[upperType]; ok {
					isStdlibFB = true
				} else if interp != nil && interp.FBDecls != nil {
					nestedDecl = interp.FBDecls[upperType]
				}
			}

			for _, n := range vd.Names {
				var val Value
				switch {
				case isStdlibFB:
					val = MakeFBInstanceValue(typeName, StdlibFBFactory[upperType]())
				case nestedDecl != nil:
					nested := newUserFBInstanceDepth(typeName, nestedDecl, interp, env, depth+1)
					val = Value{Kind: ValFBInstance, FBRef: nested}
				default:
					val = zeroFromTypeSpecWith(vd.Type, resolve, 0)
					// If there is an init value, try to evaluate it
					if vd.InitValue != nil && interp != nil {
						if iv, err := interp.evalExpr(env, vd.InitValue); err == nil {
							val = iv
						}
					}
				}

				env.Define(n.Name, val)
				upper := strings.ToUpper(n.Name)
				switch vb.Section {
				case ast.VarInput:
					inst.inputNames = append(inst.inputNames, upper)
				case ast.VarOutput:
					inst.outputNames = append(inst.outputNames, upper)
				}
			}
		}
	}

	return inst
}

// Execute runs one execution cycle of the FB instance.
// For stdlib FBs, it delegates to the StandardFB.Execute method.
// For user-defined FBs, it executes the body statements against the persistent
// env. A runtime error in the body is returned to the caller -- swallowing it
// would leave outputs stale and turn the bug into a silent wrong value.
func (inst *FBInstance) Execute(dt time.Duration, interp *Interpreter) error {
	if inst.FB != nil {
		inst.FB.Execute(dt)
		return nil
	}
	// User-defined FB: execute body statements
	if interp != nil && inst.Decl != nil && inst.Env != nil {
		err := interp.execStatements(inst.Env, inst.Decl.Body)
		if err != nil {
			// ErrReturn is normal FB termination
			if _, ok := err.(*ErrReturn); ok {
				return nil
			}
			return err
		}
	}
	return nil
}

// SetInput sets an input value on the FB instance.
// For stdlib FBs, delegates to StandardFB.SetInput.
// For user-defined FBs, sets the variable in the persistent env.
func (inst *FBInstance) SetInput(name string, v Value) {
	if inst.FB != nil {
		inst.FB.SetInput(name, v)
		return
	}
	if inst.Env != nil {
		if !inst.Env.Set(name, v) {
			inst.Env.Define(name, v)
		}
	}
}

// GetOutput reads an output value from the FB instance.
// For stdlib FBs, delegates to StandardFB.GetOutput.
// For user-defined FBs, reads the variable from the persistent env.
func (inst *FBInstance) GetOutput(name string) Value {
	if inst.FB != nil {
		return inst.FB.GetOutput(name)
	}
	if inst.Env != nil {
		upper := strings.ToUpper(name)
		for _, oName := range inst.outputNames {
			if oName == upper {
				if v, ok := inst.Env.Get(name); ok {
					return v
				}
			}
		}
	}
	return Value{}
}

// GetInput reads an input value from the FB instance.
// For stdlib FBs, delegates to StandardFB.GetInput.
// For user-defined FBs, reads the variable from the persistent env.
func (inst *FBInstance) GetInput(name string) Value {
	if inst.FB != nil {
		return inst.FB.GetInput(name)
	}
	if inst.Env != nil {
		upper := strings.ToUpper(name)
		for _, iName := range inst.inputNames {
			if iName == upper {
				if v, ok := inst.Env.Get(name); ok {
					return v
				}
			}
		}
	}
	return Value{}
}

// GetMember resolves a member access on an FB instance.
// It checks outputs first (most common for fb.Q, fb.ET), then inputs,
// then falls back to any variable in the env for user-defined FBs.
func (inst *FBInstance) GetMember(name string) Value {
	// Try output first
	if v := inst.GetOutput(name); v.Kind != 0 || v.Bool || v.Int != 0 || v.Real != 0 || v.Str != "" || v.Time != 0 {
		return v
	}
	// Try input
	if v := inst.GetInput(name); v.Kind != 0 || v.Bool || v.Int != 0 || v.Real != 0 || v.Str != "" || v.Time != 0 {
		return v
	}
	// For stdlib, that's all we have
	if inst.FB != nil {
		// Check outputs and inputs with the actual interface
		v := inst.FB.GetOutput(name)
		if v.Kind != 0 || v.Bool || v.Int != 0 || v.Real != 0 || v.Str != "" || v.Time != 0 {
			return v
		}
		return inst.FB.GetInput(name)
	}
	// For user-defined: fall back to any env variable
	if inst.Env != nil {
		if v, ok := inst.Env.Get(name); ok {
			return v
		}
	}
	return Value{}
}

// ZeroFromTypeSpec is the exported wrapper around zeroFromTypeSpec
// for use by the test runner package.
func ZeroFromTypeSpec(ts ast.TypeSpec) Value {
	return zeroFromTypeSpec(ts)
}

// TypeResolver maps an upper-case user-defined type name to its TypeSpec.
// It lets zero-value construction resolve named types that appear nested
// inside an aggregate -- as an array element type or a struct member type --
// not just at the top level of a declaration.
type TypeResolver func(upperName string) (ast.TypeSpec, bool)

// maxTypeNestDepth bounds recursion while building a zero value, so a type
// that refers to itself cannot spin forever.
const maxTypeNestDepth = 32

// ZeroFromTypeSpecWith resolves a TypeSpec to its zero Value, using resolve to
// look up user-defined named types at any nesting depth. A nil resolver behaves
// exactly like ZeroFromTypeSpec.
func ZeroFromTypeSpecWith(ts ast.TypeSpec, resolve TypeResolver) Value {
	return zeroFromTypeSpecWith(ts, resolve, 0)
}

// MakeFBInstanceValue creates a Value wrapping a StandardFB as an FBInstance.
// Used by the test runner to initialize FB variables in test environments.
func MakeFBInstanceValue(typeName string, fb StandardFB) Value {
	inst := &FBInstance{
		TypeName: typeName,
		FB:       fb,
	}
	return Value{Kind: ValFBInstance, FBRef: inst}
}

// typeNameFromSpec extracts the type name from a TypeSpec.
// Returns empty string if not a NamedType.
func typeNameFromSpec(ts ast.TypeSpec) string {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		return nt.Name.Name
	}
	return ""
}

// zeroFromTypeSpec resolves a TypeSpec to its zero Value.
// For NamedType, it looks up the elementary type by name.
// For ArrayType, it creates a zero-filled array of the appropriate size.
// For StructType, it creates a struct with zero-valued fields.
func zeroFromTypeSpec(ts ast.TypeSpec) Value {
	return zeroFromTypeSpecWith(ts, nil, 0)
}

func zeroFromTypeSpecWith(ts ast.TypeSpec, resolve TypeResolver, depth int) Value {
	if depth > maxTypeNestDepth {
		return Zero(types.KindDINT)
	}
	switch t := ts.(type) {
	case *ast.NamedType:
		if t.Name != nil {
			name := strings.ToUpper(t.Name.Name)
			if name == "STRING" || name == "WSTRING" {
				return Value{Kind: ValString, Str: ""}
			}
			if typ, found := types.LookupElementaryType(name); found {
				return Zero(typ.Kind())
			}
			// Not elementary: it may be a user-defined TYPE (struct, array,
			// enum, subrange or alias). Resolve and recurse so that aggregates
			// nested inside other aggregates are built correctly.
			if resolve != nil {
				if target, found := resolve(name); found {
					return zeroFromTypeSpecWith(target, resolve, depth+1)
				}
			}
		}
		// Unknown type name; default to INT zero
		return Zero(types.KindDINT)
	case *ast.ArrayType:
		return zeroArrayWith(t, resolve, depth)
	case *ast.StructType:
		return zeroStructWith(t, resolve, depth)
	case *ast.StringType:
		return Value{Kind: ValString, Str: ""}
	case *ast.SubrangeType:
		// Use the base type's zero
		return zeroFromTypeSpecWith(t.BaseType, resolve, depth+1)
	case *ast.PointerType:
		// Null pointer
		return Value{Kind: ValPointer}
	case *ast.ReferenceType:
		// Null reference
		return Value{Kind: ValReference}
	default:
		return Zero(types.KindDINT)
	}
}

// zeroArray creates a zero-filled array Value from an ArrayType AST node.
// The interpreter uses direct indexing (arr[i] maps to slice index i),
// so for ARRAY[1..10] we allocate high+1 elements to support 1-based indexing.
func zeroArray(at *ast.ArrayType) Value {
	return zeroArrayWith(at, nil, 0)
}

func zeroArrayWith(at *ast.ArrayType, resolve TypeResolver, depth int) Value {
	if len(at.Ranges) == 0 {
		return Value{Kind: ValArray, Array: []Value{}}
	}
	// Evaluate the first dimension range
	_, high := evalSubrangeConst(at.Ranges[0])
	size := high + 1 // allocate enough for direct indexing
	if size <= 0 {
		size = 1
	}
	if size > 10000 {
		size = 10000 // safety cap
	}
	elemZero := zeroFromTypeSpecWith(at.ElementType, resolve, depth+1)
	arr := make([]Value, size)
	for i := range arr {
		// Clone per element: an aggregate element is backed by a slice or map,
		// so sharing one zero value would make a write to one slot visible in
		// every slot.
		arr[i] = elemZero.Clone()
	}
	return Value{Kind: ValArray, Array: arr}
}

// zeroStruct creates a zero-valued struct Value from a StructType AST node.
// Keys are stored in UPPER case to match the interpreter's member access logic.
func zeroStruct(st *ast.StructType) Value {
	return zeroStructWith(st, nil, 0)
}

func zeroStructWith(st *ast.StructType, resolve TypeResolver, depth int) Value {
	fields := make(map[string]Value, len(st.Members))
	for _, m := range st.Members {
		if m.Name != nil {
			fields[strings.ToUpper(m.Name.Name)] = zeroFromTypeSpecWith(m.Type, resolve, depth+1)
		}
	}
	return Value{Kind: ValStruct, Struct: fields}
}

// evalSubrangeConst extracts integer bounds from a SubrangeSpec.
// Only handles literal integer expressions; defaults to 0..0 otherwise.
func evalSubrangeConst(sr *ast.SubrangeSpec) (int, int) {
	low := evalConstInt(sr.Low)
	high := evalConstInt(sr.High)
	return low, high
}

// evalConstInt extracts an integer value from a constant expression.
func evalConstInt(expr ast.Expr) int {
	if lit, ok := expr.(*ast.Literal); ok {
		switch lit.LitKind {
		case ast.LitInt:
			n := 0
			for _, ch := range lit.Value {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}
			if len(lit.Value) > 0 && lit.Value[0] == '-' {
				n = -n
			}
			return n
		}
	}
	// Handle unary minus on a literal
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if unary.Op.Text == "-" {
			return -evalConstInt(unary.Operand)
		}
	}
	return 0
}
