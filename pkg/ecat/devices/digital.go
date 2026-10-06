package devices

import (
	"fmt"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// DigitalIO models digital input/output terminals and boxes (EL1008, EL1018,
// EL2008, EP2338, Festo CTEU).
//
// Channels are numbered 1..N per direction. A channel is the field in a PDO
// named "Channel n" of that direction when one exists, otherwise the n-th
// 1-bit field of that direction in layout order; so EP2338's output PDOs
// "Channel 9".."Channel 16" are Output(1)..Output(8).
//
// When the outputs are multi-bit entries (Festo CTEU valve terminals with one
// byte per coil group), Output(i) is bit i-1 of all output fields concatenated
// in entry order: Output(1..8) is the first entry, Output(9..16) the second.
type DigitalIO struct{ Base }

// Step applies SetInput stimulus to the input view and records the outputs.
func (d *DigitalIO) Step(_ time.Duration, out, in []byte) { d.stepIO(out, in) }

// SetInput forces input channel ch; it persists across Steps.
func (d *DigitalIO) SetInput(ch int, v bool) error {
	f, ok := d.ChannelField(ecat.DirIn, ch)
	if !ok {
		return fmt.Errorf("%s: no digital input channel %d", d.name(), ch)
	}
	var bit uint64
	if v {
		bit = 1
	}
	d.setField(f, bit)
	return nil
}

// Output reports output channel ch as written by the PLC in the last Step.
// Unknown channels read false.
func (d *DigitalIO) Output(ch int) bool {
	if f, ok := d.ChannelField(ecat.DirOut, ch); ok {
		return d.read(f) != 0
	}
	if ch < 1 {
		return false
	}
	pos := ch - 1
	for _, f := range d.layout.Fields(ecat.DirOut) {
		if pos < f.BitLen {
			return ecat.Get(d.out, ecat.Field{Dir: f.Dir, Bit: f.Bit + pos, BitLen: 1}) != 0
		}
		pos -= f.BitLen
	}
	return false
}
