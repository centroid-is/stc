// Package devices holds simulated EtherCAT device models. Importing it
// registers every model into ecat.DefaultRegistry.
package devices

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ecat"
)

// Base is embedded by every model. It keeps the slave, its Layout and the
// views of the latest Step, and applies input overrides set through Set.
type Base struct {
	slave   *ecat.Slave
	layout  *ecat.Layout
	out, in []byte

	overrides []override // applied in Set order each Step
}

type override struct {
	f ecat.Field
	v uint64
}

// Init stores the slave.
func (b *Base) Init(s *ecat.Slave) { b.slave = s }

// Bind stores the entry layout.
func (b *Base) Bind(l *ecat.Layout) { b.layout = l }

// Slave returns the slave this model simulates (nil before Init).
func (b *Base) Slave() *ecat.Slave { return b.slave }

// Layout returns the slave's entry layout (nil before Bind).
func (b *Base) Layout() *ecat.Layout { return b.layout }

func (b *Base) name() string {
	if b.slave == nil {
		return "<unbound slave>"
	}
	return b.slave.Name
}

// stepIO records the views of this Step and applies the input overrides.
func (b *Base) stepIO(out, in []byte) {
	b.out, b.in = out, in
	for _, o := range b.overrides {
		ecat.Put(b.in, o.f, o.v)
	}
}

// setField overrides input field f with v from now on and writes it to the
// current view immediately.
func (b *Base) setField(f ecat.Field, v uint64) {
	for i := range b.overrides {
		if b.overrides[i].f == f {
			b.overrides[i].v = v
			ecat.Put(b.in, f, v)
			return
		}
	}
	b.overrides = append(b.overrides, override{f, v})
	ecat.Put(b.in, f, v)
}

// Set forces input entry (pdo, entry) to v; it is reapplied every Step.
func (b *Base) Set(pdo, entry string, v uint64) error {
	f, ok := b.layout.Field(pdo, entry)
	if !ok {
		return fmt.Errorf("%s: no entry %q/%q", b.name(), pdo, entry)
	}
	if f.Dir != ecat.DirIn {
		return fmt.Errorf("%s: %q/%q is an output; only inputs can be set", b.name(), pdo, entry)
	}
	b.setField(f, v)
	return nil
}

// Get reads entry (pdo, entry) from the latest Step's view: the in-view for
// inputs, the out-view (what the PLC wrote) for outputs.
func (b *Base) Get(pdo, entry string) (uint64, bool) {
	f, ok := b.layout.Field(pdo, entry)
	if !ok {
		return 0, false
	}
	return b.read(f), true
}

func (b *Base) read(f ecat.Field) uint64 {
	if f.Dir == ecat.DirOut {
		return ecat.Get(b.out, f)
	}
	return ecat.Get(b.in, f)
}

// ChannelField returns channel n (1-based) of direction dir: the first field
// in a PDO whose name ends in "Channel n", otherwise the n-th 1-bit field of
// that direction in layout order.
func (b *Base) ChannelField(dir ecat.Dir, n int) (ecat.Field, bool) {
	if n < 1 {
		return ecat.Field{}, false
	}
	fields := b.layout.Fields(dir)
	for _, f := range fields {
		if ch, ok := channelNo(f.Pdo); ok && ch == n {
			return f, true
		}
	}
	k := 0
	for _, f := range fields {
		if f.BitLen == 1 {
			k++
			if k == n {
				return f, true
			}
		}
	}
	return ecat.Field{}, false
}

// channelNo parses the trailing integer of a PDO name ending in "Channel n".
func channelNo(pdo string) (int, bool) {
	i := strings.LastIndex(pdo, "Channel ")
	if i < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(pdo[i+len("Channel "):])
	return n, err == nil
}
