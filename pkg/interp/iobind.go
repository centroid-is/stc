package interp

import (
	"fmt"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
)

// IOBinder copies TcLinkTo-bound variables between the interpreter env and
// EtherCAT process images at scan boundaries: inputs before the program
// body, outputs after it. Bindings resolve lazily on first use, once the
// program and GVL envs exist; bindings that do not resolve are dropped and
// reported through Errors.
type IOBinder struct {
	// ioCodec holds the interpreter (set when the binder is attached) and
	// does the declared-type copies.
	ioCodec
	// progEnv returns the env of the PROGRAM called name, or nil.
	progEnv  func(name string) *Env
	net      *ecat.Network
	images   *ecat.Images
	bindings []ecat.Binding
	ins      []boundSlot
	outs     []boundSlot
	errs     []error
	resolved bool
}

// boundSlot is a binding resolved to an env path and an image buffer.
type boundSlot struct {
	b    ecat.Binding
	path *RefPath
	spec ast.TypeSpec // declared type of the leaf, nil when not a user TYPE
	buf  []byte       // the master's In or Out image
}

// NewIOBinder binds bindings to net's process images. A nil net leaves every
// binding unresolved.
func NewIOBinder(bindings []ecat.Binding, net *ecat.Network) *IOBinder {
	b := &IOBinder{net: net, bindings: bindings}
	if net != nil {
		b.images = net.Images()
	}
	return b
}

// SetIOBinder attaches b to the engine; Tick then runs its scan-boundary
// copies. A nil binder restores the plain scan cycle.
func (e *ScanCycleEngine) SetIOBinder(b *IOBinder) {
	if b != nil {
		b.interp = e.interp
		b.progEnv = func(name string) *Env {
			if e.program.Name != nil && strings.EqualFold(e.program.Name.Name, name) {
				return e.env
			}
			return nil
		}
		b.resolved = false
	}
	e.ioBinder = b
}

// Errors returns the resolution errors of dropped bindings.
func (b *IOBinder) Errors() []error {
	return append([]error(nil), b.errs...)
}

func (b *IOBinder) fail(bd ecat.Binding, format string, args ...any) {
	b.errs = append(b.errs, fmt.Errorf("%s: %s", strings.Join(bd.Var.Steps, "."), fmt.Sprintf(format, args...)))
}

// resolve maps every binding to an env path and image buffer.
func (b *IOBinder) resolve() {
	b.resolved = true
	b.ins, b.outs, b.errs = nil, nil, nil
	for _, bd := range b.bindings {
		bs, ok := b.resolveOne(bd)
		if !ok {
			continue
		}
		if bd.Slot.Dir == ecat.DirOut {
			b.outs = append(b.outs, bs)
		} else {
			b.ins = append(b.ins, bs)
		}
	}
}

func (b *IOBinder) resolveOne(bd ecat.Binding) (boundSlot, bool) {
	steps := bd.Var.Steps
	if len(steps) < 2 {
		b.fail(bd, "binding path needs a GVL or program and a variable")
		return boundSlot{}, false
	}
	var img *ecat.Image
	if b.images != nil {
		img = b.images.Get(bd.Slot.Master)
	}
	if img == nil {
		b.fail(bd, "no process image for master %q", bd.Slot.Master)
		return boundSlot{}, false
	}
	env := b.interp.lookupGVL(steps[0])
	if env == nil {
		env = b.progEnv(steps[0])
	}
	if env == nil {
		b.fail(bd, "unknown GVL or program %q", steps[0])
		return boundSlot{}, false
	}
	p := &RefPath{Env: env, Var: strings.ToUpper(steps[1])}
	v, err := readRef(p)
	for _, st := range steps[2:] {
		if err != nil {
			break
		}
		if v.Kind == ValFBInstance && v.FBRef != nil && v.FBRef.Env != nil {
			p = &RefPath{Env: v.FBRef.Env, Var: strings.ToUpper(st)}
		} else {
			p = p.with(RefStep{Member: strings.ToUpper(st)})
		}
		v, err = readRef(p)
	}
	if err != nil {
		b.fail(bd, "%v", err)
		return boundSlot{}, false
	}
	bs := boundSlot{b: bd, path: p, spec: b.lookupType(bd.Var.TypeName, 0), buf: img.In}
	if bd.Slot.Dir == ecat.DirOut {
		bs.buf = img.Out
	}
	if !b.codecSupports(v, bs.spec, 0) {
		b.fail(bd, "type %s cannot be bound to the process image", bd.Var.TypeName)
		return boundSlot{}, false
	}
	return bs, true
}

func (bs *boundSlot) cursor() *bitCursor {
	start := bs.b.Slot.Byte*8 + bs.b.Slot.Bit
	return &bitCursor{buf: bs.buf, pos: start, end: start + bs.b.Slot.BitLen}
}

// decodeSlot returns cur updated from the slot's bits in the image.
func (b *IOBinder) decodeSlot(cur Value, bs *boundSlot) Value {
	return b.ioCodec.decode(cur, bs.spec, bs.cursor(), true)
}

// encodeSlot writes v into the slot's bits in the image.
func (b *IOBinder) encodeSlot(v Value, bs *boundSlot) {
	b.ioCodec.encode(v, bs.spec, bs.cursor(), true)
}

// preScan steps the network by dt and copies input slots into the env.
func (b *IOBinder) preScan(dt time.Duration) {
	if !b.resolved {
		b.resolve()
	}
	if b.net != nil {
		b.net.Step(dt)
	}
	for i := range b.ins {
		bs := &b.ins[i]
		if cur, err := readRef(bs.path); err == nil {
			_ = writeRef(bs.path, b.decodeSlot(cur, bs))
		}
	}
}

// postScan copies output variables from the env into their slots.
func (b *IOBinder) postScan() {
	for i := range b.outs {
		bs := &b.outs[i]
		if v, err := readRef(bs.path); err == nil {
			b.encodeSlot(v, bs)
		}
	}
}
