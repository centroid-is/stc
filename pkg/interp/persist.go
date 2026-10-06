package interp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// StateFileVersion is the "version" of the state files SaveState writes and
// LoadState accepts.
const StateFileVersion = 1

// stateFile is the on-disk form of the PERSISTENT/RETAIN variables:
// {"version": 1, "values": {"<path>": <value>}}. Keys are sorted and there
// is no timestamp, so equal state gives a byte-identical file.
type stateFile struct {
	Version int                        `json:"version"`
	Values  map[string]json.RawMessage `json:"values"`
}

// PersistPaths lists, sorted, the runtime path of every variable declared in
// a PERSISTENT or RETAIN block: GVL and PROGRAM variables directly, and the
// members of such blocks in the FB instances reachable from GVL and PROGRAM
// variables (nested FBs, EXTENDS bases, struct fields and array elements by
// index). A PERSISTENT array or struct is one path. The list is computed once
// by LoadProject; the returned slice is a copy.
func (p *Project) PersistPaths() []string {
	return append([]string(nil), p.persist...)
}

// collectPersist computes PersistPaths from the declarations and the
// instantiated values.
func (p *Project) collectPersist() []string {
	w := persistWalker{rt: p.rt, seen: map[string]bool{}, visited: map[*FBInstance]bool{}}
	for _, f := range p.files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.GVLDecl:
				if d.Name != nil {
					w.blocks(d.Name.Name, d.Blocks, nil, 0)
				}
			case *ast.ProgramDecl:
				if d.Name != nil {
					w.blocks(d.Name.Name, d.VarBlocks, nil, 0)
				}
			}
		}
	}
	sort.Strings(w.out)
	return w.out
}

type persistWalker struct {
	rt      *Runtime
	seen    map[string]bool // upper-case paths already listed
	visited map[*FBInstance]bool
	out     []string
}

func (w *persistWalker) add(path string) {
	if k := strings.ToUpper(path); !w.seen[k] {
		w.seen[k] = true
		w.out = append(w.out, path)
	}
}

// blocks lists prefix.name for the variables of persistent blocks and walks
// the values of the others. inst is the FB owning the blocks (nil for a GVL
// or PROGRAM, whose values are read through the Runtime).
func (w *persistWalker) blocks(prefix string, blocks []*ast.VarBlock, inst *FBInstance, depth int) {
	for _, vb := range blocks {
		if vb == nil || vb.IsConstant {
			continue
		}
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				path := prefix + "." + n.Name
				if vb.IsPersistent || vb.IsRetain {
					w.add(path)
					continue
				}
				var v Value
				var ok bool
				if inst != nil {
					v, ok = inst.Env.GetLocal(n.Name)
				} else {
					var err error
					v, err = w.rt.Get(path)
					ok = err == nil
				}
				if ok {
					w.value(path, v, depth+1)
				}
			}
		}
	}
}

// value walks into user FB instances, structs and arrays of them.
func (w *persistWalker) value(path string, v Value, depth int) {
	if depth > maxJSONDepth {
		return
	}
	switch v.Kind {
	case ValFBInstance:
		inst := v.FBRef
		if inst == nil || inst.Decl == nil || inst.Env == nil || w.visited[inst] {
			return
		}
		w.visited[inst] = true
		for _, d := range w.rt.fbChain(inst) {
			w.blocks(path, d.VarBlocks, inst, depth)
		}
	case ValStruct:
		for _, name := range v.Fields {
			if f, ok := v.Struct[strings.ToUpper(name)]; ok {
				w.value(path+"."+name, f, depth+1)
			}
		}
	case ValArray:
		if v.ArrayLow >= len(v.Array) || !walkable(v.Array[v.ArrayLow].Kind) {
			return
		}
		for i := v.ArrayLow; i < len(v.Array); i++ {
			w.value(fmt.Sprintf("%s[%d]", path, i), v.Array[i], depth+1)
		}
	}
}

// walkable reports whether values of kind k can contain FB instances.
func walkable(k ValueKind) bool {
	return k == ValFBInstance || k == ValStruct || k == ValArray
}

// fbChain returns the declarations of inst's type, EXTENDS bases first.
func (r *Runtime) fbChain(inst *FBInstance) []*ast.FunctionBlockDecl {
	var chain []*ast.FunctionBlockDecl
	seen := map[string]bool{}
	for d := inst.Decl; d != nil && d.Name != nil && !seen[strings.ToUpper(d.Name.Name)]; {
		seen[strings.ToUpper(d.Name.Name)] = true
		chain = append([]*ast.FunctionBlockDecl{d}, chain...)
		if d.Extends == nil {
			break
		}
		d = r.interp.FBDecls[strings.ToUpper(d.Extends.Name)]
	}
	return chain
}

// SaveState writes every PersistPaths value to path as a version 1 state
// file. It writes path+".tmp" in the same directory and renames it over
// path, so a crash never leaves a truncated state file. TIME values are
// stored as decimal milliseconds with nanosecond precision; everything else
// uses the ToJSON form, which Runtime.Set reads back.
func (p *Project) SaveState(path string) error {
	values := make(map[string]any, len(p.persist))
	for _, k := range p.persist {
		v, err := p.rt.Get(k)
		if err != nil {
			return fmt.Errorf("saving %s: %w", k, err)
		}
		values[k] = p.rt.stateJSON(v, 0)
	}
	data, err := json.MarshalIndent(struct {
		Version int            `json:"version"`
		Values  map[string]any `json:"values"`
	}{StateFileVersion, values}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}
	data = append(data, '\n')
	if err := writeAtomic(path, data); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}
	return nil
}

// writeAtomic replaces path with data: it writes and syncs a temp file
// unique to this call in the same directory (so concurrent savers never
// share one), renames it over path, then syncs the directory so the rename
// survives a power loss. The temp file is removed on failure.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(dir)
}

// syncDir flushes a directory entry change (a rename) to stable storage.
// Windows cannot open a directory for syncing; NTFS journals the rename.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	return err
}

// LoadState applies a state file written by SaveState. A missing file is a
// first run: no error and no warnings. An unreadable or malformed file, or
// another version, is an error and nothing is applied. Otherwise every entry
// goes through Runtime.Set (type coercion, range checks, CONSTANT
// rejection) in sorted path order; entries that are not PersistPaths
// ("unknown path") or that Set rejects become warnings and the rest still
// apply. Call it before the first Tick.
func (p *Project) LoadState(path string) (warnings []string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	var sf stateFile
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&sf); err != nil {
		return nil, fmt.Errorf("malformed state file %s: %w", path, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("malformed state file %s: trailing data", path)
	}
	if sf.Version != StateFileVersion {
		return nil, fmt.Errorf("unsupported state file version %d in %s", sf.Version, path)
	}
	known := make(map[string]bool, len(p.persist))
	for _, k := range p.persist {
		known[strings.ToUpper(k)] = true
	}
	keys := make([]string, 0, len(sf.Values))
	for k := range sf.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !known[strings.ToUpper(k)] {
			warnings = append(warnings, k+": unknown path")
			continue
		}
		vd := json.NewDecoder(bytes.NewReader(sf.Values[k]))
		vd.UseNumber()
		var v any
		_ = vd.Decode(&v) // already valid JSON
		if err := p.rt.Set(k, v); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	return warnings, nil
}

// stateJSON is ToJSON with TIME as json.Number milliseconds (which Set
// accepts) at every depth.
func (r *Runtime) stateJSON(v Value, depth int) any {
	if depth > maxJSONDepth {
		return nil
	}
	switch v.Kind {
	case ValTime:
		ns := int64(v.Time)
		sign := ""
		if ns < 0 {
			sign, ns = "-", -ns
		}
		return json.Number(fmt.Sprintf("%s%d.%06d", sign, ns/1e6, ns%1e6))
	case ValArray:
		low := v.ArrayLow
		if low > len(v.Array) {
			low = len(v.Array)
		}
		out := make([]any, 0, len(v.Array)-low)
		for _, e := range v.Array[low:] {
			out = append(out, r.stateJSON(e, depth+1))
		}
		return out
	case ValStruct:
		o := &orderedObject{}
		names := v.Fields
		if len(names) == 0 {
			for k := range v.Struct {
				names = append(names, k)
			}
			sort.Strings(names)
		}
		for _, n := range names {
			if f, ok := v.Struct[strings.ToUpper(n)]; ok {
				o.add(n, r.stateJSON(f, depth+1))
			}
		}
		return o
	case ValFBInstance:
		inst := v.FBRef
		if inst == nil || inst.FB != nil || inst.Env == nil {
			return r.toJSON(v, depth)
		}
		o := &orderedObject{}
		for _, n := range r.fbVarNames(inst) {
			if m, ok := inst.Env.GetLocal(n); ok {
				o.add(n, r.stateJSON(m, depth+1))
			}
		}
		return o
	}
	return r.toJSON(v, depth)
}
