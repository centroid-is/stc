package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
)

// ScanCycleEngine implements the PLC scan cycle model:
// read inputs -> execute program body -> write outputs -> advance clock.
// Time is deterministic with no wall-clock dependency.
type ScanCycleEngine struct {
	interp  *Interpreter
	program *ast.ProgramDecl
	env     *Env

	inputs  map[string]Value // Staged input values (uppercase keys)
	outputs map[string]Value // Captured output values (uppercase keys)
	clock   time.Duration    // Accumulated virtual time

	inputNames  []string // VAR_INPUT variable names (uppercase)
	outputNames []string // VAR_OUTPUT variable names (uppercase)

	initialized bool
}

// NewScanCycleEngine creates a new scan cycle engine for the given program.
// The engine lazily initializes the environment on the first Tick call.
func NewScanCycleEngine(program *ast.ProgramDecl) *ScanCycleEngine {
	return &ScanCycleEngine{
		interp:  New(),
		program: program,
		inputs:  make(map[string]Value),
		outputs: make(map[string]Value),
	}
}

// Tick executes one scan cycle with the given time delta:
//  1. Copy staged inputs into the program environment
//  2. Set dt on the interpreter for FB Execute calls
//  3. Execute the program body
//  4. Copy VAR_OUTPUT variables from env into the outputs map
//  5. Advance the virtual clock by dt
func (e *ScanCycleEngine) Tick(dt time.Duration) error {
	if !e.initialized {
		e.initializeEnv()
	}

	// 1. Copy inputs into env
	for _, name := range e.inputNames {
		if v, ok := e.inputs[name]; ok {
			e.env.Set(name, v)
		}
	}

	// 2. Set dt on interpreter
	e.interp.dt = dt

	// 3. Execute program body
	err := e.interp.execStatements(e.env, e.program.Body)
	if err != nil {
		// Swallow ErrReturn (normal program termination)
		if _, ok := err.(*ErrReturn); !ok {
			return err
		}
	}

	// 4. Copy outputs from env
	for _, name := range e.outputNames {
		if v, ok := e.env.Get(name); ok {
			e.outputs[name] = v
		}
	}

	// 5. Advance clock
	e.clock += dt

	return nil
}

// SetInput stages an input value to be copied into the program env on the
// next Tick. Keys are case-insensitive. Unknown input names are silently ignored.
func (e *ScanCycleEngine) SetInput(name string, v Value) {
	key := strings.ToUpper(name)
	e.inputs[key] = v
}

// GetOutput returns the current output value after the last Tick.
// Keys are case-insensitive. Returns zero Value if not found.
func (e *ScanCycleEngine) GetOutput(name string) Value {
	key := strings.ToUpper(name)
	if v, ok := e.outputs[key]; ok {
		return v
	}
	return Value{}
}

// Clock returns the accumulated virtual time (deterministic, no wall-clock).
func (e *ScanCycleEngine) Clock() time.Duration {
	return e.clock
}

// OutputNames returns the list of VAR_OUTPUT variable names (uppercase).
// The engine must be initialized (at least one Tick) for this to return values.
func (e *ScanCycleEngine) OutputNames() []string {
	return e.outputNames
}

// InputNames returns the list of VAR_INPUT variable names (uppercase).
// The engine must be initialized (at least one Tick) for this to return values.
func (e *ScanCycleEngine) InputNames() []string {
	return e.inputNames
}

// Initialize forces environment initialization without running a Tick.
// Useful when callers need to query InputNames/OutputNames before the first cycle.
func (e *ScanCycleEngine) Initialize() {
	if !e.initialized {
		e.initializeEnv()
	}
}

// initializeEnv creates and populates the program environment from VarBlocks.
// Called once on the first Tick (lazy init). Variables persist across scan cycles.
func (e *ScanCycleEngine) initializeEnv() {
	e.env = NewEnv(nil)
	e.initialized = true

	for _, vb := range e.program.VarBlocks {
		for _, vd := range vb.Declarations {
			// Check if the type is a stdlib FB
			var val Value
			typeName := typeNameFromSpec(vd.Type)
			if factory, ok := StdlibFBFactory[strings.ToUpper(typeName)]; ok {
				// Create an FB instance for each variable of this type
				for _, n := range vd.Names {
					fb := factory()
					inst := &FBInstance{
						TypeName: typeName,
						FB:       fb,
					}
					val = Value{Kind: ValFBInstance, FBRef: inst}
					e.env.Define(n.Name, val)
					upper := strings.ToUpper(n.Name)
					switch vb.Section {
					case ast.VarInput:
						e.inputNames = append(e.inputNames, upper)
					case ast.VarOutput:
						e.outputNames = append(e.outputNames, upper)
					}
				}
				continue
			}

			// Resolve zero value from the type spec
			val = zeroFromTypeSpec(vd.Type)

			// If there is an init value, try to evaluate it
			if vd.InitValue != nil {
				if iv, err := e.interp.evalExpr(e.env, vd.InitValue); err == nil {
					val = iv
				}
			}

			for _, n := range vd.Names {
				e.env.Define(n.Name, val)
				upper := strings.ToUpper(n.Name)
				switch vb.Section {
				case ast.VarInput:
					e.inputNames = append(e.inputNames, upper)
				case ast.VarOutput:
					e.outputNames = append(e.outputNames, upper)
				}
			}
		}
	}
}
