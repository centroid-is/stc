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
	inputNames  []string // VAR_INPUT variable names (uppercase)
	outputNames []string // VAR_OUTPUT variable names (uppercase)
}

// NewUserFBInstance creates an FBInstance for a user-defined function block.
// It initializes a new Env with all variables from the declaration's VarBlocks,
// using zero values based on type names. The env persists across Execute calls.
func NewUserFBInstance(name string, decl *ast.FunctionBlockDecl, interp *Interpreter, parentEnv *Env) *FBInstance {
	env := NewEnv(parentEnv)
	inst := &FBInstance{
		TypeName: name,
		Decl:     decl,
		Env:      env,
	}

	// Walk VarBlocks, initialize variables, and track input/output names
	for _, vb := range decl.VarBlocks {
		for _, vd := range vb.Declarations {
			// Resolve zero value from the type name
			val := zeroFromTypeSpec(vd.Type)

			// If there is an init value, try to evaluate it
			if vd.InitValue != nil && interp != nil {
				if iv, err := interp.evalExpr(env, vd.InitValue); err == nil {
					val = iv
				}
			}

			for _, n := range vd.Names {
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
// For user-defined FBs, it executes the body statements against the persistent env.
func (inst *FBInstance) Execute(dt time.Duration, interp *Interpreter) {
	if inst.FB != nil {
		inst.FB.Execute(dt)
		return
	}
	// User-defined FB: execute body statements
	if interp != nil && inst.Decl != nil && inst.Env != nil {
		err := interp.execStatements(inst.Env, inst.Decl.Body)
		// Swallow ErrReturn (normal FB termination)
		if err != nil {
			if _, ok := err.(*ErrReturn); !ok {
				// In the future we could propagate this error, but for now
				// FB execution errors are silently swallowed to match PLC behavior
			}
		}
	}
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
func zeroFromTypeSpec(ts ast.TypeSpec) Value {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		name := strings.ToUpper(nt.Name.Name)
		if typ, found := types.LookupElementaryType(name); found {
			return Zero(typ.Kind())
		}
		// Unknown type name; default to INT zero
		return Zero(types.KindDINT)
	}
	// Default fallback
	return Zero(types.KindDINT)
}
