package interp

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
)

// RuntimeOpts configures NewRuntime.
type RuntimeOpts struct {
	// LibraryFiles are registered before the project files, so library
	// GVLs come first in RegisterGVLs.
	LibraryFiles []*ast.SourceFile
}

// programRun is one PROGRAM of a Runtime.
type programRun struct {
	name   string // declared case
	engine *ScanCycleEngine
}

// Runtime is a live PLC image: every TYPE, enum, FUNCTION, FUNCTION_BLOCK,
// GVL and PROGRAM of a set of files on one interpreter, instantiated eagerly
// so initial values are readable before the first Tick.
//
// Tick, Get and Set are serialised by one mutex, so server goroutines (MCP,
// OPC UA, HMI) can read and write while a scan loop ticks. Values written
// with Set are seen by the program bodies on the next Tick. The engines
// returned by Engine bypass the mutex and are for single-goroutine callers
// such as `stc sim`.
type Runtime struct {
	mu       sync.Mutex
	interp   *Interpreter
	programs []*programRun
	// consts holds the upper-case ROOT.NAME of every member of a CONSTANT
	// block in a GVL or PROGRAM; Set rejects writes below them.
	consts map[string]bool
	// roots lists the GVLs (registration order) and then the PROGRAMs
	// (source order) with their variable names, for Snapshot.
	roots []rootInfo
}

// rootInfo is one GVL or PROGRAM root of a Runtime.
type rootInfo struct {
	name string // declared case
	vars []string
}

// NewRuntime registers opts.LibraryFiles and then files on a fresh
// interpreter (RegisterFiles), creates a scan engine for every PROGRAM in
// source order and initialises their variables. Initialiser and array bound
// errors are returned joined.
func NewRuntime(files []*ast.SourceFile, opts ...RuntimeOpts) (*Runtime, error) {
	var o RuntimeOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	all := make([]*ast.SourceFile, 0, len(o.LibraryFiles)+len(files))
	all = append(all, o.LibraryFiles...)
	all = append(all, files...)

	r := &Runtime{interp: New(), consts: make(map[string]bool)}
	var progs []rootInfo
	if err := r.interp.RegisterFiles(all); err != nil {
		return nil, err
	}
	for _, f := range all {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.GVLDecl:
				if d.Name != nil {
					r.addConsts(d.Name.Name, d.Blocks)
					r.roots = append(r.roots, rootInfo{name: d.Name.Name, vars: varNames(d.Blocks)})
				}
			case *ast.ProgramDecl:
				if d.Name == nil {
					continue
				}
				e := NewScanCycleEngineWith(r.interp, d)
				e.Initialize()
				r.programs = append(r.programs, &programRun{name: d.Name.Name, engine: e})
				r.addConsts(d.Name.Name, d.VarBlocks)
				progs = append(progs, rootInfo{name: d.Name.Name, vars: varNames(d.VarBlocks)})
			}
		}
	}
	if err := errors.Join(r.interp.InitErrors()...); err != nil {
		return nil, err
	}
	r.roots = append(r.roots, progs...)
	return r, nil
}

// addConsts records the members of root's CONSTANT blocks.
func (r *Runtime) addConsts(root string, blocks []*ast.VarBlock) {
	for _, vb := range blocks {
		if vb == nil || !vb.IsConstant {
			continue
		}
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				r.consts[strings.ToUpper(root+"."+n.Name)] = true
			}
		}
	}
}

// Tick runs one scan cycle: the interpreter clock advances by dt once, then
// every PROGRAM body runs once in source order, so a GVL written by one
// program is seen by the next in the same Tick. The first program error
// stops the cycle and is returned with the program name.
func (r *Runtime) Tick(dt time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interp.SetDt(dt)
	for _, p := range r.programs {
		if err := p.engine.tick(dt, false); err != nil {
			return fmt.Errorf("PROGRAM %s: %w", p.name, err)
		}
	}
	return nil
}

// Engine returns the scan engine of the PROGRAM called name
// (case-insensitive), or nil when there is none.
func (r *Runtime) Engine(name string) *ScanCycleEngine {
	if p := r.program(name); p != nil {
		return p.engine
	}
	return nil
}

func (r *Runtime) program(name string) *programRun {
	for _, p := range r.programs {
		if strings.EqualFold(p.name, name) {
			return p
		}
	}
	return nil
}

// Interpreter returns the shared interpreter. Callers that use it while
// other goroutines call Tick, Get or Set must provide their own locking.
func (r *Runtime) Interpreter() *Interpreter {
	return r.interp
}

// Snapshot returns the whole live image as a JSON-ready object: one member
// per GVL (registration order) and then per PROGRAM (source order), each an
// object of its variables in declaration order rendered with ToJSON.
func (r *Runtime) Snapshot() any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := &orderedObject{}
	for _, root := range r.roots {
		env := r.rootEnv(root.name)
		obj := &orderedObject{}
		for _, n := range root.vars {
			if v, ok := env.GetLocal(n); ok {
				obj.add(n, r.ToJSON(v))
			}
		}
		out.add(root.name, obj)
	}
	return out
}
