package interp

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/types"
)

// IOBinder copies TcLinkTo-bound variables between the interpreter env and
// EtherCAT process images at scan boundaries: inputs before the program
// body, outputs after it. Bindings resolve lazily on first use, once the
// program and GVL envs exist; bindings that do not resolve are dropped and
// reported through Errors.
type IOBinder struct {
	interp *Interpreter
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

// lookupType resolves a declared type name to its user TYPE spec, following
// aliases; nil for elementary or unknown names.
func (b *IOBinder) lookupType(name string, depth int) ast.TypeSpec {
	ts, ok := b.interp.TypeDecls[strings.ToUpper(name)]
	if !ok || depth > maxTypeNestDepth {
		return nil
	}
	if nt, isNamed := ts.(*ast.NamedType); isNamed && nt.Name != nil {
		if inner := b.lookupType(nt.Name.Name, depth+1); inner != nil {
			return inner
		}
	}
	return ts
}

// resolveSpec turns a member or element spec into a struct or array spec
// when it names a user TYPE.
func (b *IOBinder) resolveSpec(spec ast.TypeSpec) ast.TypeSpec {
	if nt, ok := spec.(*ast.NamedType); ok && nt.Name != nil {
		return b.lookupType(nt.Name.Name, 0)
	}
	return spec
}

// codecSupports reports whether v's shape can be copied bit-wise.
// Values are finite (zero values stop at maxTypeNestDepth), so the
// recursion terminates.
func (b *IOBinder) codecSupports(v Value, spec ast.TypeSpec, depth int) bool {
	switch v.Kind {
	case ValBool, ValInt, ValReal:
		return true
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := b.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		for _, el := range v.Array {
			if !b.codecSupports(el, elem, depth+1) {
				return false
			}
		}
		return true
	case ValStruct:
		for _, m := range b.structOrder(v, spec) {
			if !b.codecSupports(v.Struct[m.name], m.spec, depth+1) {
				return false
			}
		}
		return true
	}
	return false
}

type memberSpec struct {
	name string
	spec ast.TypeSpec
}

// structOrder lists v's members in declaration order when spec names a
// STRUCT, else sorted by name.
func (b *IOBinder) structOrder(v Value, spec ast.TypeSpec) []memberSpec {
	var out []memberSpec
	if st, ok := b.resolveSpec(spec).(*ast.StructType); ok {
		for _, m := range st.Members {
			if m.Name == nil {
				continue
			}
			name := strings.ToUpper(m.Name.Name)
			if _, present := v.Struct[name]; present {
				out = append(out, memberSpec{name, m.Type})
			}
		}
		if len(out) == len(v.Struct) {
			return out
		}
		out = nil
	}
	for name := range v.Struct {
		out = append(out, memberSpec{name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// bitCursor walks a slot's bits in order, never past the slot end.
type bitCursor struct {
	buf      []byte
	pos, end int
}

func (c *bitCursor) take(n int) (byteOff, bit, width int) {
	if rest := c.end - c.pos; n > rest {
		n = rest
	}
	byteOff, bit = c.pos/8, c.pos%8
	c.pos += n
	return byteOff, bit, n
}

func (c *bitCursor) read(n int) uint64 {
	off, bit, w := c.take(n)
	if w == 0 {
		return 0
	}
	return ecat.ReadBits(c.buf, off, bit, w)
}

func (c *bitCursor) write(n int, v uint64) {
	if off, bit, w := c.take(n); w > 0 {
		ecat.WriteBits(c.buf, off, bit, w, v)
	}
}

// intWidth is the bit width and signedness of an integer or real IEC type;
// 0 for unknown kinds (enums without an explicit base).
func intWidth(k types.TypeKind) (int, bool) {
	switch k {
	case types.KindBYTE, types.KindUSINT, types.KindCHAR:
		return 8, false
	case types.KindSINT:
		return 8, true
	case types.KindWORD, types.KindUINT, types.KindWCHAR:
		return 16, false
	case types.KindINT:
		return 16, true
	case types.KindDWORD, types.KindUDINT, types.KindREAL:
		return 32, false
	case types.KindDINT:
		return 32, true
	case types.KindLWORD, types.KindULINT, types.KindLREAL:
		return 64, false
	case types.KindLINT:
		return 64, true
	}
	return 0, true
}

// scalarWidth is the width of a scalar; unknown widths take the rest of the
// slot at the top level and 16 bits (the default enum base INT) inside an
// aggregate.
func scalarWidth(v Value, c *bitCursor, top bool) (int, bool) {
	if v.Kind == ValBool {
		return 1, false
	}
	w, signed := intWidth(v.IECType)
	if w == 0 {
		w = 16
		if top {
			w = c.end - c.pos
		}
	}
	if v.Kind == ValReal && w != 32 {
		w = 64
	}
	return w, signed
}

func (b *IOBinder) decode(cur Value, spec ast.TypeSpec, c *bitCursor, top bool) Value {
	switch cur.Kind {
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := b.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		out := cur
		out.Array = make([]Value, len(cur.Array))
		for i, el := range cur.Array {
			out.Array[i] = b.decode(el, elem, c, false)
		}
		return out
	case ValStruct:
		out := cur
		out.Struct = make(map[string]Value, len(cur.Struct))
		for k, v := range cur.Struct {
			out.Struct[k] = v
		}
		for _, m := range b.structOrder(cur, spec) {
			out.Struct[m.name] = b.decode(cur.Struct[m.name], m.spec, c, false)
		}
		return out
	}
	w, signed := scalarWidth(cur, c, top)
	raw := c.read(w)
	out := cur
	switch cur.Kind {
	case ValBool:
		out.Bool = raw != 0
	case ValReal:
		if w == 32 {
			out.Real = float64(math.Float32frombits(uint32(raw)))
		} else {
			out.Real = math.Float64frombits(raw)
		}
	default:
		if signed && w > 0 && w < 64 {
			sh := uint(64 - w)
			out.Int = int64(raw<<sh) >> sh
		} else {
			out.Int = int64(raw)
		}
	}
	return out
}

func (b *IOBinder) encode(v Value, spec ast.TypeSpec, c *bitCursor, top bool) {
	switch v.Kind {
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := b.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		for _, el := range v.Array {
			b.encode(el, elem, c, false)
		}
		return
	case ValStruct:
		for _, m := range b.structOrder(v, spec) {
			b.encode(v.Struct[m.name], m.spec, c, false)
		}
		return
	}
	w, _ := scalarWidth(v, c, top)
	var raw uint64
	switch v.Kind {
	case ValBool:
		if v.Bool {
			raw = 1
		}
	case ValReal:
		if w == 32 {
			raw = uint64(math.Float32bits(float32(v.Real)))
		} else {
			raw = math.Float64bits(v.Real)
		}
	default:
		raw = uint64(v.Int)
	}
	c.write(w, raw)
}

func (bs *boundSlot) cursor() *bitCursor {
	start := bs.b.Slot.Byte*8 + bs.b.Slot.Bit
	return &bitCursor{buf: bs.buf, pos: start, end: start + bs.b.Slot.BitLen}
}

// decodeSlot returns cur updated from the slot's bits in the image.
func (b *IOBinder) decodeSlot(cur Value, bs *boundSlot) Value {
	return b.decode(cur, bs.spec, bs.cursor(), true)
}

// encodeSlot writes v into the slot's bits in the image.
func (b *IOBinder) encodeSlot(v Value, bs *boundSlot) {
	b.encode(v, bs.spec, bs.cursor(), true)
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
