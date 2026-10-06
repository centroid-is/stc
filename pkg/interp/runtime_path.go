package interp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// MaxRuntimePathLen bounds the length of a path accepted by Get and Set.
// Paths arrive from remote clients, so the cap keeps parsing cheap. It
// equals symtree.MaxPathLen.
const MaxRuntimePathLen = 1024

// pathSeg is one step of a runtime path: a name, an array index or a bit.
type pathSeg struct {
	name    string
	index   int
	isIndex bool
	isBit   bool
	bit     int
}

// parseRuntimePath splits p with the grammar of symtree.ParsePath (copied,
// because interp must not import symtree):
//
//	path := ident ( '.' ident | '[' ['-'] digits ']' | '.' digits )*
//
// `a[1,2]` is rejected and paths are capped at MaxRuntimePathLen bytes. It
// never panics.
func parseRuntimePath(p string) ([]pathSeg, error) {
	if p == "" {
		return nil, errors.New("empty path")
	}
	if len(p) > MaxRuntimePathLen {
		return nil, fmt.Errorf("path longer than %d bytes", MaxRuntimePathLen)
	}
	if !pathIdentStart(p[0]) {
		return nil, fmt.Errorf("path must start with an identifier: %q", p)
	}
	i := pathIdentEnd(p, 0)
	segs := []pathSeg{{name: p[:i]}}
	for i < len(p) {
		switch p[i] {
		case '.':
			i++
			switch {
			case i < len(p) && pathIdentStart(p[i]):
				j := pathIdentEnd(p, i)
				segs = append(segs, pathSeg{name: p[i:j]})
				i = j
			case i < len(p) && pathDigit(p[i]):
				j := i
				for j < len(p) && pathDigit(p[j]) {
					j++
				}
				bit, err := strconv.Atoi(p[i:j])
				if err != nil {
					return nil, fmt.Errorf("invalid bit number %q in %q", p[i:j], p)
				}
				segs = append(segs, pathSeg{isBit: true, bit: bit})
				i = j
			default:
				return nil, fmt.Errorf("expected identifier or bit number after '.' at offset %d in %q", i, p)
			}
		case '[':
			i++
			j := i
			if j < len(p) && p[j] == '-' {
				j++
			}
			for j < len(p) && pathDigit(p[j]) {
				j++
			}
			idx, err := strconv.Atoi(p[i:j])
			if err != nil {
				return nil, fmt.Errorf("invalid array index at offset %d in %q", i, p)
			}
			if j < len(p) && p[j] == ',' {
				return nil, fmt.Errorf("multi-dimensional arrays not supported: %q", p)
			}
			if j >= len(p) || p[j] != ']' {
				return nil, fmt.Errorf("expected ']' at offset %d in %q", j, p)
			}
			segs = append(segs, pathSeg{isIndex: true, index: idx})
			i = j + 1
		default:
			return nil, fmt.Errorf("unexpected character %q at offset %d in %q", p[i], i, p)
		}
	}
	return segs, nil
}

func pathIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func pathDigit(c byte) bool { return c >= '0' && c <= '9' }

func pathIdentEnd(p string, i int) int {
	for i < len(p) && (pathIdentStart(p[i]) || pathDigit(p[i])) {
		i++
	}
	return i
}

// location is a resolved path: its current value and how to write it.
type location struct {
	val Value
	// ref is the write path, rooted at the deepest env (GVL, program or
	// user FB env). Nil for a standard FB member.
	ref *RefPath
	// head is the upper-case ROOT.NAME of the path, for the CONSTANT check.
	head string
	// std is the standard FB owning the leaf member stdName; stdOut is true
	// when that member is an output.
	std     *FBInstance
	stdName string
	stdOut  bool
	// isBit marks a trailing bit step: val is the bit, word the integer that
	// holds it and bit its number.
	isBit bool
	word  Value
	bit   int
}

// rootEnv returns the env of root: a GVL first, then a PROGRAM.
func (r *Runtime) rootEnv(root string) *Env {
	if g := r.interp.lookupGVL(root); g != nil {
		return g
	}
	if p := r.program(root); p != nil {
		return p.engine.env
	}
	return nil
}

// resolve walks path from its root. References met on the way are followed
// to their targets; a reference at the leaf is left for the caller.
func (r *Runtime) resolve(path string) (*location, error) {
	segs, err := parseRuntimePath(path)
	if err != nil {
		return nil, err
	}
	env := r.rootEnv(segs[0].name)
	if env == nil {
		return nil, fmt.Errorf("unknown root %q in %q", segs[0].name, path)
	}
	if len(segs) == 1 {
		return nil, fmt.Errorf("path %q names a root, not a variable", path)
	}
	first := segs[1]
	if first.name == "" {
		return nil, fmt.Errorf("expected a variable name after %q in %q", segs[0].name, path)
	}
	upper := strings.ToUpper(first.name)
	v, ok := env.GetLocal(upper)
	if !ok {
		return nil, fmt.Errorf("unknown variable %q in %s", first.name, segs[0].name)
	}
	loc := &location{
		val:  v,
		ref:  &RefPath{Env: env, Var: upper},
		head: strings.ToUpper(segs[0].name) + "." + upper,
	}
	for _, sg := range segs[2:] {
		if err := r.step(loc, sg, path); err != nil {
			return nil, err
		}
	}
	return loc, nil
}

// deref follows a reference at loc to its target.
func deref(loc *location, path string) error {
	if loc.val.Kind != ValReference {
		return nil
	}
	p, ok := refPathOf(loc.val)
	if !ok {
		return fmt.Errorf("unbound reference at %q", path)
	}
	v, err := readRef(p)
	if err != nil {
		return err
	}
	loc.val, loc.ref = v, p
	return nil
}

// step applies one segment to loc.
func (r *Runtime) step(loc *location, sg pathSeg, path string) error {
	if loc.isBit {
		return fmt.Errorf("bit access must be last in %q", path)
	}
	if loc.std != nil {
		return fmt.Errorf("standard FB member %s has no member or element in %q", loc.stdName, path)
	}
	if err := deref(loc, path); err != nil {
		return err
	}
	v := loc.val
	if v.Kind == ValPointer {
		return fmt.Errorf("cannot traverse pointer in %q", path)
	}
	switch {
	case sg.isBit:
		bit, err := readBit(v, int64(sg.bit), ast.Pos{})
		if err != nil {
			return err
		}
		loc.word, loc.val, loc.bit, loc.isBit = v, bit, sg.bit, true
	case sg.isIndex:
		if v.Kind != ValArray {
			return fmt.Errorf("%s is not an array in %q", v.Kind, path)
		}
		if sg.index < v.ArrayLow || sg.index >= len(v.Array) {
			return fmt.Errorf("index %d out of range [%d..%d] in %q", sg.index, v.ArrayLow, len(v.Array)-1, path)
		}
		loc.val = v.Array[sg.index]
		loc.ref = loc.ref.with(RefStep{IsIndex: true, Index: sg.index})
	default:
		upper := strings.ToUpper(sg.name)
		switch {
		case v.Kind == ValStruct:
			f, ok := v.Struct[upper]
			if !ok {
				return fmt.Errorf("struct has no member %q in %q", sg.name, path)
			}
			loc.val = f
			loc.ref = loc.ref.with(RefStep{Member: upper})
		case v.Kind == ValFBInstance && v.FBRef != nil && v.FBRef.Env != nil:
			m, ok := v.FBRef.Env.GetLocal(upper)
			if !ok {
				return fmt.Errorf("FB %s has no member %q in %q", v.FBRef.TypeName, sg.name, path)
			}
			loc.val = m
			loc.ref = &RefPath{Env: v.FBRef.Env, Var: upper}
		case v.Kind == ValFBInstance && v.FBRef != nil && v.FBRef.FB != nil:
			m, out, ok := stdMember(v.FBRef.FB, upper)
			if !ok {
				return fmt.Errorf("FB %s has no member %q in %q", v.FBRef.TypeName, sg.name, path)
			}
			loc.val, loc.ref = m, nil
			loc.std, loc.stdName, loc.stdOut = v.FBRef, upper, out
		default:
			return fmt.Errorf("%s has no member %q in %q", v.Kind, sg.name, path)
		}
	}
	return nil
}

// present reports whether a standard FB returned a real member value:
// unknown names come back as the zero Value without an IEC type.
func present(v Value) bool {
	return v.Kind != ValBool || v.IECType != 0 || v.Bool
}

// stdMember reads member name of a standard FB, outputs first.
func stdMember(fb StandardFB, name string) (Value, bool, bool) {
	if v := fb.GetOutput(name); present(v) {
		return v, true, true
	}
	if v := fb.GetInput(name); present(v) {
		return v, false, true
	}
	return Value{}, false, false
}

// Get returns the current value at path, following a reference at the leaf
// to its target. Paths are case-insensitive: ROOT.var.member[3].bit, where
// ROOT is a GVL (looked up first) or PROGRAM name.
func (r *Runtime) Get(path string) (Value, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	loc, err := r.resolve(path)
	if err != nil {
		return Value{}, err
	}
	if err := deref(loc, path); err != nil {
		return Value{}, err
	}
	return loc.val, nil
}
