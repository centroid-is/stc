package interp

import (
	"math"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/types"
)

// ioCodec copies values between the interpreter and a process image by
// their declared IEC type: BOOL as one bit, integers and bit strings at
// their width with sign extension for signed kinds, REAL and LREAL as IEEE
// bits, enums by their base type, arrays element-wise and structs
// member-wise in declaration order. It is shared by the EtherCAT IOBinder
// and the explicit-address AT sync of the scan cycle, so both bind a
// variable the same way. Only the interpreter's TypeDecls are consulted.
type ioCodec struct {
	interp *Interpreter
}

// decodeBytes returns cur updated from buf starting at bit bitOff. cur
// supplies the declared shape and IEC type; every decoded scalar passes
// through storeAs, so it never exceeds the declared width.
func (c ioCodec) decodeBytes(cur Value, spec ast.TypeSpec, buf []byte, bitOff int) Value {
	return c.decode(cur, spec, &bitCursor{buf: buf, pos: bitOff, end: len(buf) * 8}, true)
}

// encodeBytes writes v into buf starting at bit bitOff, by declared type.
func (c ioCodec) encodeBytes(v Value, spec ast.TypeSpec, buf []byte, bitOff int) {
	c.encode(v, spec, &bitCursor{buf: buf, pos: bitOff, end: len(buf) * 8}, true)
}

// bitSize is the number of image bits v occupies: BOOL 1, scalars their
// IEC width (16 for an enum without a known base), aggregates the sum of
// their elements or members.
func (c ioCodec) bitSize(v Value, spec ast.TypeSpec) int {
	switch v.Kind {
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := c.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		n := 0
		for _, el := range v.Array {
			n += c.bitSize(el, elem)
		}
		return n
	case ValStruct:
		n := 0
		for _, m := range c.structOrder(v, spec) {
			n += c.bitSize(v.Struct[m.name], m.spec)
		}
		return n
	case ValBool, ValInt, ValReal:
		w, _ := scalarWidth(v, &bitCursor{}, false)
		return w
	}
	return 0
}

// lookupType resolves a declared type name to its user TYPE spec, following
// aliases; nil for elementary or unknown names.
func (c ioCodec) lookupType(name string, depth int) ast.TypeSpec {
	ts, ok := c.interp.TypeDecls[strings.ToUpper(name)]
	if !ok || depth > maxTypeNestDepth {
		return nil
	}
	if nt, isNamed := ts.(*ast.NamedType); isNamed && nt.Name != nil {
		if inner := c.lookupType(nt.Name.Name, depth+1); inner != nil {
			return inner
		}
	}
	return ts
}

// resolveSpec turns a member or element spec into a struct or array spec
// when it names a user TYPE.
func (c ioCodec) resolveSpec(spec ast.TypeSpec) ast.TypeSpec {
	if nt, ok := spec.(*ast.NamedType); ok && nt.Name != nil {
		return c.lookupType(nt.Name.Name, 0)
	}
	return spec
}

// codecSupports reports whether v's shape can be copied bit-wise.
// Values are finite (zero values stop at maxTypeNestDepth), so the
// recursion terminates.
func (c ioCodec) codecSupports(v Value, spec ast.TypeSpec, depth int) bool {
	switch v.Kind {
	case ValBool, ValInt, ValReal:
		return true
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := c.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		for _, el := range v.Array {
			if !c.codecSupports(el, elem, depth+1) {
				return false
			}
		}
		return true
	case ValStruct:
		for _, m := range c.structOrder(v, spec) {
			if !c.codecSupports(v.Struct[m.name], m.spec, depth+1) {
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
func (c ioCodec) structOrder(v Value, spec ast.TypeSpec) []memberSpec {
	var out []memberSpec
	if st, ok := c.resolveSpec(spec).(*ast.StructType); ok {
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

func (bc *bitCursor) take(n int) (byteOff, bit, width int) {
	if rest := bc.end - bc.pos; n > rest {
		n = rest
	}
	byteOff, bit = bc.pos/8, bc.pos%8
	bc.pos += n
	return byteOff, bit, n
}

func (bc *bitCursor) read(n int) uint64 {
	off, bit, w := bc.take(n)
	if w == 0 {
		return 0
	}
	return ecat.ReadBits(bc.buf, off, bit, w)
}

func (bc *bitCursor) write(n int, v uint64) {
	if off, bit, w := bc.take(n); w > 0 {
		ecat.WriteBits(bc.buf, off, bit, w, v)
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
func scalarWidth(v Value, bc *bitCursor, top bool) (int, bool) {
	if v.Kind == ValBool {
		return 1, false
	}
	w, signed := intWidth(v.IECType)
	if w == 0 {
		w = 16
		if top {
			w = bc.end - bc.pos
		}
	}
	if v.Kind == ValReal && w != 32 {
		w = 64
	}
	return w, signed
}

func (c ioCodec) decode(cur Value, spec ast.TypeSpec, bc *bitCursor, top bool) Value {
	switch cur.Kind {
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := c.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		out := cur
		out.Array = make([]Value, len(cur.Array))
		for i, el := range cur.Array {
			out.Array[i] = c.decode(el, elem, bc, false)
		}
		return out
	case ValStruct:
		out := cur
		out.Struct = make(map[string]Value, len(cur.Struct))
		for k, v := range cur.Struct {
			out.Struct[k] = v
		}
		for _, m := range c.structOrder(cur, spec) {
			out.Struct[m.name] = c.decode(cur.Struct[m.name], m.spec, bc, false)
		}
		return out
	}
	w, signed := scalarWidth(cur, bc, top)
	raw := bc.read(w)
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
	return storeAs(cur, out)
}

func (c ioCodec) encode(v Value, spec ast.TypeSpec, bc *bitCursor, top bool) {
	switch v.Kind {
	case ValArray:
		var elem ast.TypeSpec
		if at, ok := c.resolveSpec(spec).(*ast.ArrayType); ok {
			elem = at.ElementType
		}
		for _, el := range v.Array {
			c.encode(el, elem, bc, false)
		}
		return
	case ValStruct:
		for _, m := range c.structOrder(v, spec) {
			c.encode(v.Struct[m.name], m.spec, bc, false)
		}
		return
	}
	w, _ := scalarWidth(v, bc, top)
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
	bc.write(w, raw)
}
