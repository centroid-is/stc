package interp

import (
	"slices"
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

// stdFBFactory returns the Go factory for an upper-case FB type name: an
// engine-local override (fbOverrides, the Tc2_EtherCAT mocks) first, then
// StdlibFBFactory. A nil interpreter has no overrides.
func (interp *Interpreter) stdFBFactory(upperType string) (func() StandardFB, bool) {
	if interp != nil {
		if f, ok := interp.fbOverrides[upperType]; ok {
			return f, true
		}
	}
	f, ok := StdlibFBFactory[upperType]
	return f, ok
}

// FBInstance wraps either a StandardFB (for stdlib FBs) or an Env+Decl
// pair (for user-defined FBs). It provides a unified interface for FB
// call statements and member access.
type FBInstance struct {
	TypeName string

	// Virtual time at which this instance last executed, and whether it ever
	// has. Timers get the delta since their own previous call rather than a
	// delta shared by every FB in the scan -- calling the same TON twice in
	// one scan must not advance it twice, because no time passed between the
	// two calls on a real PLC.
	lastRun time.Duration
	hasRun  bool

	// For stdlib FBs (non-nil when wrapping a StandardFB implementation)
	FB StandardFB

	// For user-defined FBs
	Env         *Env
	Decl        *ast.FunctionBlockDecl
	ParentDecl  *ast.FunctionBlockDecl // parent FB decl for EXTENDS chain
	inputNames  []string               // VAR_INPUT variable names (uppercase)
	outputNames []string               // VAR_OUTPUT variable names (uppercase)
	inoutNames  []string               // VAR_IN_OUT variable names (uppercase)
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
	env.self = inst

	// EXTENDS chain known to the FBDecls registry, base-most first, so the
	// instance env also holds every inherited variable and ACTION, and a
	// derived FB's declaration of the same name wins.
	chain := fbExtendsChain(decl, interp)
	if len(chain) > 1 {
		inst.ParentDecl = chain[len(chain)-2]
	}
	for _, d := range chain {
		for _, a := range d.Actions {
			env.DefineAction(a)
		}
		// Inline VAR enums: their bare values must resolve before the
		// initialisers below are evaluated.
		if interp != nil && d.Name != nil {
			interp.RegisterInlineEnums(d.Name.Name, d.VarBlocks)
		}
	}

	if interp == nil {
		interp = New()
	}
	// Variables of the whole chain, base first, through the shared
	// instantiation path; nested FB members are one level deeper.
	for _, d := range chain {
		for _, vb := range d.VarBlocks {
			for _, vd := range vb.Declarations {
				interp.instantiateVar(env, env, vd, depth+1)
				for _, n := range vd.Names {
					upper := strings.ToUpper(n.Name)
					switch vb.Section {
					case ast.VarInput:
						inst.inputNames = append(inst.inputNames, upper)
					case ast.VarOutput:
						inst.outputNames = append(inst.outputNames, upper)
					case ast.VarInOut:
						inst.inoutNames = append(inst.inoutNames, upper)
					}
				}
			}
		}
	}

	return inst
}

// RegisterInlineEnums registers every anonymous enumeration declared in
// blocks as <pou>.<var>, so its bare values resolve inside the POU.
func (interp *Interpreter) RegisterInlineEnums(pou string, blocks []*ast.VarBlock) {
	local := interp.blockConsts(blocks)
	for _, vb := range blocks {
		if vb == nil {
			continue
		}
		for _, vd := range vb.Declarations {
			et, ok := vd.Type.(*ast.EnumType)
			if !ok {
				continue
			}
			for _, n := range vd.Names {
				interp.registerEnum(pou+"."+n.Name, et, vd.Attributes, local)
			}
		}
	}
}

// fbExtendsChain returns decl and the FBs it EXTENDS that the interpreter's
// FBDecls registry knows, base-most first and decl last. A cycle or a chain
// deeper than maxFBNestDepth stops the walk.
func fbExtendsChain(decl *ast.FunctionBlockDecl, interp *Interpreter) []*ast.FunctionBlockDecl {
	chain := []*ast.FunctionBlockDecl{decl}
	cur := decl
	for len(chain) < maxFBNestDepth && cur.Extends != nil && interp != nil && interp.FBDecls != nil {
		base := interp.FBDecls[strings.ToUpper(cur.Extends.Name)]
		if base == nil || slices.Contains(chain, base) {
			break
		}
		chain = append([]*ast.FunctionBlockDecl{base}, chain...)
		cur = base
	}
	return chain
}

// deltaFor reports how much virtual time has passed since this instance last
// executed. The first call gets the scan's delta, since there is no previous
// run to measure from; later calls in the same scan get zero, which is what a
// real PLC would give them.
//
// This matters for the standard idiom that reads a free-running TON and
// immediately restarts it to measure a scan:
//
//	ton(IN := TRUE, PT := T#1D);   // read ET
//	ton(IN := FALSE, PT := T#1D);  // IEC: IN FALSE zeroes ET
//	ton(IN := TRUE, PT := T#1D);   // start again from zero
//
// With a scan-wide delta the third call would immediately re-accumulate the
// whole scan and the measurement would grow without bound.
func (inst *FBInstance) deltaFor(clock time.Duration, scanDt time.Duration) time.Duration {
	if !inst.hasRun {
		inst.hasRun = true
		inst.lastRun = clock
		return scanDt
	}
	dt := clock - inst.lastRun
	inst.lastRun = clock
	if dt < 0 {
		return 0
	}
	return dt
}

// Execute runs one execution cycle of the FB instance.
// For stdlib FBs, it delegates to the StandardFB.Execute method.
// For user-defined FBs, it executes the body statements against the persistent
// env. A runtime error in the body is returned to the caller -- swallowing it
// would leave outputs stale and turn the bug into a silent wrong value.
//
// The body runs under EnterCall, so every way of calling a user FB (fb();,
// fb(x := 1);, G.fb();, s.fb();, outer.inner(); and the scan engine) is
// bounded by MaxCallDepth. An FB that calls its own instance through a GVL
// then fails with a RuntimeError instead of overflowing the Go stack.
func (inst *FBInstance) Execute(dt time.Duration, interp *Interpreter) error {
	if inst.FB != nil {
		inst.FB.Execute(dt)
		return nil
	}
	// User-defined FB: execute body statements
	if interp != nil && inst.Decl != nil && inst.Env != nil {
		if inst.Env.self == nil {
			// Hand-built instance: THIS^ and unqualified METHOD calls in
			// its body still need to find it.
			inst.Env.self = inst
		}
		if err := interp.EnterCall(inst.TypeName, ast.Pos{}); err != nil {
			return err
		}
		defer interp.ExitCall()
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
		if cur, ok := inst.Env.Get(name); ok {
			v = storeAs(cur, v)
		}
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

// IsInOut reports whether name is declared in a VAR_IN_OUT section of this FB.
// Stdlib FBs have no VAR_IN_OUT parameters, so this is always false for them.
func (inst *FBInstance) IsInOut(name string) bool {
	upper := strings.ToUpper(name)
	for _, n := range inst.inoutNames {
		if n == upper {
			return true
		}
	}
	return false
}

// GetInOut reads a VAR_IN_OUT value out of the FB env after execution, so the
// caller can copy it back into the argument variable (by-reference semantics).
// The second result is false when name is not a VAR_IN_OUT parameter of this
// instance, or the instance has no env of its own (stdlib FB).
func (inst *FBInstance) GetInOut(name string) (Value, bool) {
	if inst.Env == nil || !inst.IsInOut(name) {
		return Value{}, false
	}
	return inst.Env.Get(name)
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
	return zeroFromType(ts, typeCtx{resolve: resolve}, depth)
}

// zeroFromType builds the zero value of ts. Named user types resolve through
// ctx.resolve; array bounds and member or TYPE defaults use ctx.interp and
// ctx.env when set (see typeCtx).
func zeroFromType(ts ast.TypeSpec, ctx typeCtx, depth int) Value {
	if depth > maxTypeNestDepth {
		return Zero(types.KindDINT)
	}
	switch t := ts.(type) {
	case *ast.NamedType:
		if t.Name != nil {
			name := strings.ToUpper(t.Name.Name)
			if name == "STRING" {
				return Value{Kind: ValString, Str: ""}
			}
			if name == "WSTRING" {
				return Value{Kind: ValString, Str: "", IECType: types.KindWSTRING}
			}
			if typ, found := types.LookupElementaryType(name); found {
				return Zero(typ.Kind())
			}
			// An FB type used as an array element or struct member: a live
			// instance of its own (ARRAY[1..2] OF FB_Drive).
			if fb, ok := ctx.fbInstance(t.Name.Name, depth); ok {
				return fb
			}
			// Not elementary: it may be a user-defined TYPE (struct, array,
			// enum, subrange or alias). Resolve and recurse so that aggregates
			// nested inside other aggregates are built correctly.
			if ctx.resolve != nil {
				if target, found := ctx.resolve(name); found {
					v := zeroFromType(target, ctx, depth+1)
					if _, isEnum := target.(*ast.EnumType); isEnum {
						v.Enum = name
					}
					return ctx.typeDefault(name, target, v)
				}
			}
		}
		// Unknown type name: with an interpreter it is an undeclared FB or
		// TYPE (library drift), run as a zero-output auto-stub with a
		// warning; without one, default to DINT zero.
		if ctx.interp != nil && t.Name != nil {
			return ctx.interp.autoStub(t)
		}
		return Zero(types.KindDINT)
	case *ast.ArrayType:
		return zeroArrayCtx(t, ctx, depth)
	case *ast.StructType:
		return zeroStructCtx(t, ctx, depth)
	case *ast.StringType:
		if t.IsWide {
			return Value{Kind: ValString, Str: "", IECType: types.KindWSTRING}
		}
		return Value{Kind: ValString, Str: ""}
	case *ast.SubrangeType:
		// Use the base type's zero
		return zeroFromType(t.BaseType, ctx, depth+1)
	case *ast.PointerType:
		// Null pointer
		return Value{Kind: ValPointer}
	case *ast.ReferenceType:
		// Null reference
		return Value{Kind: ValReference}
	case *ast.EnumType:
		return zeroEnum(t)
	default:
		return Zero(types.KindDINT)
	}
}

// maxArraySlots caps the slots allocated for one array dimension so a huge
// declared bound cannot exhaust memory.
const maxArraySlots = 10000

// zeroArray creates a zero-filled array Value from an ArrayType AST node.
// The interpreter uses direct indexing (arr[i] maps to slice index i),
// so for ARRAY[1..10] we allocate high+1 elements to support 1-based indexing.
func zeroArray(at *ast.ArrayType) Value {
	return zeroArrayCtx(at, typeCtx{}, 0)
}

func zeroArrayCtx(at *ast.ArrayType, ctx typeCtx, depth int) Value {
	if len(at.Ranges) == 0 {
		return Value{Kind: ValArray, Array: []Value{}}
	}
	// Only the first dimension is modelled.
	low, high := ctx.bounds(at.Ranges[0])
	if low < 0 {
		ctx.fail(at.Ranges[0].Low, "negative array lower bound not supported: %s", exprText(at.Ranges[0].Low))
		low = 0
	}
	if high < low {
		ctx.fail(at.Ranges[0].High, "array upper bound %d is below lower bound %d", high, low)
		high = low
	}
	size := high + 1 // allocate enough for direct indexing
	if size > maxArraySlots {
		ctx.fail(at.Ranges[0].High, "array size %d exceeds the %d element limit", size, maxArraySlots)
		size = maxArraySlots
		if low >= size {
			low = 0
		}
	}
	elemZero := zeroFromType(at.ElementType, ctx, depth+1)
	arr := make([]Value, size)
	if elemZero.Kind == ValFBInstance {
		// FB instances are references, so Clone would share one instance
		// between all elements: build each element separately.
		arr[0] = elemZero
		for i := 1; i < len(arr); i++ {
			arr[i] = zeroFromType(at.ElementType, ctx, depth+1)
		}
		return Value{Kind: ValArray, Array: arr, ArrayLow: int(low)}
	}
	for i := range arr {
		// Clone per element: an aggregate element is backed by a slice or map,
		// so sharing one zero value would make a write to one slot visible in
		// every slot.
		arr[i] = elemZero.Clone()
	}
	return Value{Kind: ValArray, Array: arr, ArrayLow: int(low)}
}

// zeroStruct creates a zero-valued struct Value from a StructType AST node.
// Keys are stored in UPPER case to match the interpreter's member access logic.
func zeroStruct(st *ast.StructType) Value {
	return zeroStructCtx(st, typeCtx{}, 0)
}

// zeroStructCtx builds a struct value with every member at its declared
// default (member initialiser) or zero, and records member names in declared
// case and order in Fields.
func zeroStructCtx(st *ast.StructType, ctx typeCtx, depth int) Value {
	fields := make(map[string]Value, len(st.Members))
	names := make([]string, 0, len(st.Members))
	for _, m := range st.Members {
		if m.Name == nil {
			continue
		}
		v := zeroFromType(m.Type, ctx, depth+1)
		if m.InitValue != nil {
			v = ctx.applyInit(m.Type, m.InitValue, v)
		}
		fields[strings.ToUpper(m.Name.Name)] = v
		names = append(names, m.Name.Name)
	}
	return Value{Kind: ValStruct, Struct: fields, Fields: names}
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
