package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/iomap"
)

// ioSlot is an AT-bound variable the Project syncs with its IOTable: GVL
// variables with explicit or wildcard addresses, wildcard PROGRAM variables
// and struct members declared with AT. Explicit PROGRAM variables are
// synced by their scan engine, which shares the Project's table.
type ioSlot struct {
	path string // declared case, ROOT.VAR[.MEMBER]
	ref  *RefPath
	addr iomap.IOAddress // auto-assigned for wildcards
	size int             // bytes
	spec ast.TypeSpec
}

// allocIO collects the project's AT bindings and assigns every wildcard
// (%I*, %Q*, %M*) not claimed by the IOBinder a slot sized by its declared
// type: sequential per area, aligned to its size for 2, 4 and 8 byte
// values, starting after the highest explicit address in that area. The
// caller holds rt.mu. Output and memory slots are seeded from their
// variables' current values.
func (p *Project) allocIO() {
	c := ioCodec{interp: p.rt.interp}
	claimed := make(map[string]bool)
	if p.binder != nil {
		for _, bd := range p.binder.bindings {
			claimed[strings.ToUpper(strings.Join(bd.Var.Steps, "."))] = true
		}
	}
	next := make(map[iomap.Area]int)
	var slots []ioSlot
	add := func(path string, ref *RefPath, spec ast.TypeSpec, at string, engineSynced bool) {
		addr, err := iomap.ParseAddress(at)
		if err != nil {
			return
		}
		cur, err := readRef(ref)
		if err != nil || !c.codecSupports(cur, spec, 0) {
			return
		}
		n := ioByteLen(c, cur, spec, addr.BitOffset)
		if !addr.IsWildcard {
			if end := addr.ByteOffset + n; end > next[addr.Area] {
				next[addr.Area] = end
			}
			if engineSynced {
				return
			}
		} else if claimed[strings.ToUpper(path)] {
			return
		}
		slots = append(slots, ioSlot{path: path, ref: ref, addr: addr, size: n, spec: spec})
	}
	walk := func(root string, env *Env, blocks []*ast.VarBlock, isProgram bool) {
		if env == nil {
			return
		}
		for _, vb := range blocks {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					path := root + "." + n.Name
					ref := &RefPath{Env: env, Var: strings.ToUpper(n.Name)}
					if vd.AtAddress != nil {
						add(path, ref, vd.Type, vd.AtAddress.Name, isProgram)
					}
					st, ok := c.resolveSpec(vd.Type).(*ast.StructType)
					if !ok {
						continue
					}
					for _, m := range st.Members {
						if m.Name != nil && m.AtAddress != nil {
							add(path+"."+m.Name.Name, ref.with(RefStep{Member: strings.ToUpper(m.Name.Name)}), m.Type, m.AtAddress.Name, false)
						}
					}
				}
			}
		}
	}
	for _, f := range p.files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.GVLDecl:
				if d.Name != nil {
					walk(d.Name.Name, p.rt.interp.lookupGVL(d.Name.Name), d.Blocks, false)
				}
			case *ast.ProgramDecl:
				if d.Name == nil {
					continue
				}
				if pr := p.rt.program(d.Name.Name); pr != nil {
					walk(d.Name.Name, pr.engine.env, d.VarBlocks, true)
				}
			}
		}
	}
	for i := range slots {
		s := &slots[i]
		if s.addr.IsWildcard {
			off := next[s.addr.Area]
			if a := s.size; a == 2 || a == 4 || a == 8 {
				off = (off + a - 1) / a * a
			}
			next[s.addr.Area] = off + s.size
			s.addr = iomap.IOAddress{Area: s.addr.Area, Size: wildcardSize(c.bitSize(mustRead(s.ref), s.spec)), ByteOffset: off}
		}
		ioWindow(p.io, s.addr.Area, s.addr.ByteOffset, s.size) // grow the area
		// Output and memory images start from the declared initial values,
		// so a %M variable is not zeroed by its first input copy.
		if s.addr.Area != iomap.AreaInput {
			ioEncodeAt(c, p.io, s.addr, mustRead(s.ref), s.spec)
		}
	}
	p.slots = slots
}

// mustRead returns the value at ref, the zero Value when it cannot be read.
func mustRead(ref *RefPath) Value {
	v, _ := readRef(ref)
	return v
}

// wildcardSize is the address size letter reported for an auto-assigned
// slot of the given bit width.
func wildcardSize(bits int) iomap.Size {
	switch bits {
	case 1:
		return iomap.SizeBit
	case 16:
		return iomap.SizeWord
	case 32:
		return iomap.SizeDWord
	}
	return iomap.SizeByte
}

// syncIOIn copies input and memory slots from the table into their
// variables. The caller holds rt.mu.
func (p *Project) syncIOIn() {
	c := ioCodec{interp: p.rt.interp}
	for i := range p.slots {
		s := &p.slots[i]
		if s.addr.Area == iomap.AreaOutput {
			continue
		}
		if cur, err := readRef(s.ref); err == nil {
			_ = writeRef(s.ref, ioDecodeAt(c, p.io, s.addr, cur, s.spec))
		}
	}
}

// syncIOOut copies output and memory slot variables into the table. The
// caller holds rt.mu.
func (p *Project) syncIOOut() {
	c := ioCodec{interp: p.rt.interp}
	for i := range p.slots {
		s := &p.slots[i]
		if s.addr.Area == iomap.AreaInput {
			continue
		}
		if v, err := readRef(s.ref); err == nil {
			ioEncodeAt(c, p.io, s.addr, v, s.spec)
		}
	}
}

// IOSlot reports the address and byte size of an AT-bound GVL variable,
// wildcard PROGRAM variable or AT struct member synced by the Project
// (path ROOT.VAR or ROOT.VAR.MEMBER, case-insensitive). Wildcards report
// their auto-assigned address. Explicit PROGRAM variables are not listed.
func (p *Project) IOSlot(path string) (iomap.IOAddress, int, bool) {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	for _, s := range p.slots {
		if strings.EqualFold(s.path, path) {
			return s.addr, s.size, true
		}
	}
	return iomap.IOAddress{}, 0, false
}

// ioArea returns the table area named by 'I', 'Q' or 'M'.
func (p *Project) ioArea(area byte) ([]byte, error) {
	switch iomap.Area(area) {
	case iomap.AreaInput:
		return p.io.I, nil
	case iomap.AreaOutput:
		return p.io.Q, nil
	case iomap.AreaMemory:
		return p.io.M, nil
	}
	return nil, fmt.Errorf("unknown I/O area %q (want I, Q or M)", area)
}

// SetIOBytes copies b into the process image area ('I', 'Q' or 'M') at
// byte off. Input changes reach variables on the next Tick. Writes outside
// the area are rejected.
func (p *Project) SetIOBytes(area byte, off int, b []byte) error {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	s, err := p.ioArea(area)
	if err != nil {
		return err
	}
	if off < 0 || off+len(b) > len(s) {
		return fmt.Errorf("I/O write %c[%d:%d] is outside the %d byte area", area, off, off+len(b), len(s))
	}
	copy(s[off:], b)
	return nil
}

// IOBytes returns a copy of n bytes of the process image area ('I', 'Q' or
// 'M') from byte off.
func (p *Project) IOBytes(area byte, off, n int) ([]byte, error) {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	s, err := p.ioArea(area)
	if err != nil {
		return nil, err
	}
	if off < 0 || n < 0 || off+n > len(s) {
		return nil, fmt.Errorf("I/O read %c[%d:%d] is outside the %d byte area", area, off, off+n, len(s))
	}
	return append([]byte(nil), s[off:off+n]...), nil
}
