package ecat

// Field is one PDO entry of a slave as seen by its device model. Bit is
// relative to the start of the slave's byte span in that direction, i.e. to
// the in or out view passed to Device.Step.
type Field struct {
	Pdo      string // PDO name, e.g. "Channel 3"
	Entry    string // entry name, e.g. "Input" or "Status__Tripped"
	Dir      Dir
	Bit      int // bit offset inside the direction's view
	BitLen   int
	DataType string
}

// Layout is a name-addressed view of a slave's PDO entries in PDO/entry order.
// Padding and entries without a slot are omitted. A nil *Layout is empty.
type Layout struct {
	fields []Field
	byKey  map[string]int
}

// NewLayout builds a Layout from fields in order; for duplicate (pdo, entry)
// pairs the first field wins name lookup.
func NewLayout(fields []Field) *Layout {
	l := &Layout{fields: append([]Field(nil), fields...), byKey: map[string]int{}}
	for i, f := range l.fields {
		k := f.Pdo + "\x00" + f.Entry
		if _, dup := l.byKey[k]; !dup {
			l.byKey[k] = i
		}
	}
	return l
}

// Field returns the entry named entry in PDO pdo.
func (l *Layout) Field(pdo, entry string) (Field, bool) {
	if l == nil {
		return Field{}, false
	}
	i, ok := l.byKey[pdo+"\x00"+entry]
	if !ok {
		return Field{}, false
	}
	return l.fields[i], true
}

// Fields returns every field of direction dir in layout order.
func (l *Layout) Fields(dir Dir) []Field {
	if l == nil {
		return nil
	}
	var out []Field
	for _, f := range l.fields {
		if f.Dir == dir {
			out = append(out, f)
		}
	}
	return out
}

// FindEntry returns every field of direction dir named entry, in PDO order.
func (l *Layout) FindEntry(dir Dir, entry string) []Field {
	if l == nil {
		return nil
	}
	var out []Field
	for _, f := range l.fields {
		if f.Dir == dir && f.Entry == entry {
			out = append(out, f)
		}
	}
	return out
}

// Get reads f from a direction view; fields outside buf read as 0.
func Get(buf []byte, f Field) uint64 {
	if f.Bit < 0 {
		return 0
	}
	return ReadBits(buf, f.Bit/8, f.Bit%8, f.BitLen)
}

// Put writes v to f in a direction view; fields outside buf are ignored.
func Put(buf []byte, f Field, v uint64) {
	if f.Bit < 0 {
		return
	}
	WriteBits(buf, f.Bit/8, f.Bit%8, f.BitLen, v)
}

// Binder is implemented by device models that need their entry positions.
// NewNetwork calls Bind after Init.
type Binder interface {
	Bind(l *Layout)
}

// slaveLayout builds s's Layout relative to its in/out spans.
func slaveLayout(m *Master, s *Slave, in, out span) *Layout {
	var fields []Field
	for _, p := range s.Pdos {
		for _, e := range p.Entries {
			if e.Padding() {
				continue
			}
			slot, ok := m.Slot(LinkPath(m.Name, s, p, e))
			if !ok {
				continue
			}
			lo := in.lo
			if slot.Dir == DirOut {
				lo = out.lo
			}
			fields = append(fields, Field{
				Pdo: p.Name, Entry: e.Name, Dir: slot.Dir,
				Bit: (slot.Byte-lo)*8 + slot.Bit, BitLen: slot.BitLen, DataType: slot.DataType,
			})
		}
	}
	return NewLayout(fields)
}
