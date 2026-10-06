package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/types"
)

// IOBinding associates a variable name with a parsed I/O address for
// scan-cycle synchronization between the interpreter env and the IOTable.
type IOBinding struct {
	VarName string // uppercase variable name in env
	Address iomap.IOAddress
}

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

	ioTable    *iomap.IOTable // I/O process image table
	ioBindings []IOBinding    // AT-addressed variable bindings
	ioBinder   *IOBinder      // EtherCAT TcLinkTo bindings, nil when unused
	ecat       *ecatServices  // Tc2_EtherCAT mock backend, nil when unused

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
		ioTable: iomap.NewIOTable(),
	}
}

// NewScanCycleEngineWith creates a scan cycle engine for program that runs on
// interp instead of a fresh interpreter, so several programs share its
// TYPEs, FBs, FUNCTIONs and GVLs (see Runtime). Register GVLs on interp
// before the engine initialises: the program env takes the GVL chain as its
// parent when it is created.
func NewScanCycleEngineWith(interp *Interpreter, program *ast.ProgramDecl) *ScanCycleEngine {
	e := NewScanCycleEngine(program)
	e.interp = interp
	return e
}

// IOTable returns the engine's I/O process image table for external access.
// External code can call SetBit/SetWord etc. to inject test inputs before Tick.
func (e *ScanCycleEngine) IOTable() *iomap.IOTable {
	return e.ioTable
}

// Tick executes one scan cycle with the given time delta:
//  0. Copy I/O table values into env for AT-bound input/memory variables,
//     then step the EtherCAT network and copy linked inputs (IOBinder)
//  1. Copy staged inputs into the program environment
//  2. Set dt on the interpreter for FB Execute calls
//  3. Execute the program body
//  4. Copy VAR_OUTPUT variables from env into the outputs map
//  5. Copy AT-bound outputs to the I/O table, then linked outputs (IOBinder)
//  6. Advance the virtual clock by dt
func (e *ScanCycleEngine) Tick(dt time.Duration) error {
	return e.tick(dt, true)
}

// tick runs one scan cycle. advance is false when the caller (Runtime.Tick)
// has already advanced the shared interpreter clock for this cycle, so that
// several programs on one interpreter do not advance it once each.
func (e *ScanCycleEngine) tick(dt time.Duration, advance bool) error {
	if !e.initialized {
		e.initializeEnv()
	}

	// 0. Copy I/O table values into env for AT-bound variables (inputs + memory)
	for _, b := range e.ioBindings {
		if b.Address.Area == iomap.AreaInput || b.Address.Area == iomap.AreaMemory {
			e.env.Set(b.VarName, e.readIOValue(b.Address))
		}
	}
	if e.ioBinder != nil {
		e.ioBinder.preScan(dt)
	}
	if e.ecat != nil {
		e.ecat.preScan(dt, e.ioBinder)
	}

	// 1. Copy inputs into env
	for _, name := range e.inputNames {
		if v, ok := e.inputs[name]; ok {
			e.env.Set(name, v)
		}
	}

	// 2. Advance the interpreter's virtual clock by this scan's delta
	if advance {
		e.interp.SetDt(dt)
	}

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

	// 5. Copy AT-bound output/memory variables to I/O table
	for _, b := range e.ioBindings {
		if b.Address.Area == iomap.AreaOutput || b.Address.Area == iomap.AreaMemory {
			if v, ok := e.env.Get(b.VarName); ok {
				e.writeIOValue(b.Address, v)
			}
		}
	}
	if e.ioBinder != nil {
		e.ioBinder.postScan()
	}

	// 6. Advance clock
	e.clock += dt

	return nil
}

// readIOValue reads a value from the IOTable based on the address size.
func (e *ScanCycleEngine) readIOValue(addr iomap.IOAddress) Value {
	switch addr.Size {
	case iomap.SizeBit:
		return BoolValue(e.ioTable.GetBit(addr.Area, addr.ByteOffset, addr.BitOffset))
	case iomap.SizeByte:
		return Value{Kind: ValInt, Int: int64(e.ioTable.GetByte(addr.Area, addr.ByteOffset)), IECType: types.KindBYTE}
	case iomap.SizeWord:
		return Value{Kind: ValInt, Int: int64(e.ioTable.GetWord(addr.Area, addr.ByteOffset)), IECType: types.KindINT}
	case iomap.SizeDWord:
		return Value{Kind: ValInt, Int: int64(e.ioTable.GetDWord(addr.Area, addr.ByteOffset)), IECType: types.KindDINT}
	}
	return Value{}
}

// writeIOValue writes a value to the IOTable based on the address size.
func (e *ScanCycleEngine) writeIOValue(addr iomap.IOAddress, v Value) {
	switch addr.Size {
	case iomap.SizeBit:
		e.ioTable.SetBit(addr.Area, addr.ByteOffset, addr.BitOffset, v.Bool)
	case iomap.SizeByte:
		e.ioTable.SetByte(addr.Area, addr.ByteOffset, byte(v.Int))
	case iomap.SizeWord:
		e.ioTable.SetWord(addr.Area, addr.ByteOffset, uint16(v.Int))
	case iomap.SizeDWord:
		e.ioTable.SetDWord(addr.Area, addr.ByteOffset, uint32(v.Int))
	}
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

// SetGlobals registers gvls on the engine's interpreter with RegisterGVLs,
// in declaration order, so the program can use GVL.x and, for GVLs without
// qualified_only, bare x. Call it before the first Tick or Initialize: the program env picks
// up the GVL chain as its parent when it is created.
func (e *ScanCycleEngine) SetGlobals(gvls []*ast.GVLDecl) {
	e.interp.RegisterGVLs(gvls)
}

// initVarDecl defines every name of vd in env through instantiateVar.
// Shared by program and GVL environments.
func (interp *Interpreter) initVarDecl(env, fbParent *Env, vd *ast.VarDecl) {
	interp.instantiateVar(env, fbParent, vd, 0)
}

// initializeEnv creates and populates the program environment from VarBlocks.
// Called once on the first Tick (lazy init). Variables persist across scan cycles.
// The env's parent is the chain of non qualified_only GVLs, if any.
func (e *ScanCycleEngine) initializeEnv() {
	e.env = NewEnv(e.interp.GlobalParent())
	e.initialized = true

	// ACTIONs run against the program's own variables.
	for _, a := range e.program.Actions {
		e.env.DefineAction(a)
	}
	if e.program.Name != nil {
		e.interp.RegisterInlineEnums(e.program.Name.Name, e.program.VarBlocks)
	}

	for _, vb := range e.program.VarBlocks {
		for _, vd := range vb.Declarations {
			e.interp.initVarDecl(e.env, e.env, vd)

			for _, n := range vd.Names {
				upper := strings.ToUpper(n.Name)
				switch vb.Section {
				case ast.VarInput:
					e.inputNames = append(e.inputNames, upper)
				case ast.VarOutput:
					e.outputNames = append(e.outputNames, upper)
				}

				// Register AT address binding if present. FB instances
				// never carry an address binding.
				if vd.AtAddress != nil {
					addr, err := iomap.ParseAddress(vd.AtAddress.Name)
					if err == nil && !addr.IsWildcard {
						e.ioBindings = append(e.ioBindings, IOBinding{
							VarName: upper,
							Address: addr,
						})
					}
				}
			}
		}
	}
}
