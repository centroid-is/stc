package devices

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// EL9222 models the EL9222-5500 two-channel overcurrent protection terminal.
//
// Channel n is the PDO pair "OCP Inputs Channel n" / "OCP Outputs Channel n".
// A channel is enabled while Control__Switch is on and it is neither tripped,
// cooling down nor in hardware protection. Trip latches until the PLC pulses
// Control__Reset: only a rising edge clears it, so holding Reset high or
// toggling Switch does not. A channel without a mapped Control__Switch
// (trimmed exports) counts as switched on.
//
// Current is written in 0.01 A units (EL922x documentation, object
// 0x60n0:22 "Current [0,01 A]") and reads 0 while the channel is disabled.
type EL9222 struct {
	Base
	chans []*ocpChannel // index ch-1
}

type ocpChannel struct {
	in  map[string]ecat.Field // entry suffix after "Status__" (or "Current")
	out map[string]ecat.Field // entry suffix after "Control__"

	tripped, warning, coolDown, hwProtection bool
	amps                                     float64
	lastReset, enabled                       bool
	counter                                  uint8
}

// Bind stores the layout and resolves the per-channel entries.
func (d *EL9222) Bind(l *ecat.Layout) {
	d.Base.Bind(l)
	d.chans = nil
	get := func(ch int) *ocpChannel {
		for len(d.chans) < ch {
			d.chans = append(d.chans, &ocpChannel{in: map[string]ecat.Field{}, out: map[string]ecat.Field{}})
		}
		return d.chans[ch-1]
	}
	for _, dir := range []ecat.Dir{ecat.DirIn, ecat.DirOut} {
		for _, f := range l.Fields(dir) {
			ch, ok := channelNo(f.Pdo)
			if !ok || ch < 1 {
				continue
			}
			c := get(ch)
			if dir == ecat.DirIn {
				c.in[strings.TrimPrefix(f.Entry, "Status__")] = f
			} else {
				c.out[strings.TrimPrefix(f.Entry, "Control__")] = f
			}
		}
	}
}

// Channels returns the number of channels found in the layout.
func (d *EL9222) Channels() int { return len(d.chans) }

// Step runs each channel's state machine and writes its status.
func (d *EL9222) Step(_ time.Duration, out, in []byte) {
	for _, c := range d.chans {
		reset := false
		if f, ok := c.out["Reset"]; ok {
			reset = ecat.Get(out, f) != 0
		}
		sw := true
		if f, ok := c.out["Switch"]; ok {
			sw = ecat.Get(out, f) != 0
		}
		if reset && !c.lastReset {
			c.tripped = false
		}
		c.lastReset = reset
		c.enabled = sw && !c.tripped && !c.coolDown && !c.hwProtection
		c.counter = (c.counter + 1) & 3

		var current uint64
		if c.enabled {
			current = centiAmps(c.amps, c.in["Current"].BitLen)
		}
		for name, v := range map[string]uint64{
			"Enabled":               b2u(c.enabled),
			"Tripped":               b2u(c.tripped),
			"Hardware Protection":   b2u(c.hwProtection),
			"Current Level Warning": b2u(c.warning),
			"Cool Down Lock":        b2u(c.coolDown),
			"Error":                 b2u(c.tripped || c.hwProtection),
			"Diag":                  b2u(c.tripped || c.hwProtection || c.warning || c.coolDown),
			"TxPDO State":           0,
			"Input cycle counter":   uint64(c.counter),
			"State Reset":           b2u(reset),
			"State Switch":          b2u(sw),
			"Current":               current,
		} {
			if f, ok := c.in[name]; ok {
				ecat.Put(in, f, v)
			}
		}
	}
	d.stepIO(out, in)
}

// centiAmps converts amps to 0.01 A, clamped to the entry's unsigned range.
func centiAmps(amps float64, bitLen int) uint64 {
	if bitLen <= 0 || bitLen > 32 {
		bitLen = 16
	}
	max := uint64(1)<<uint(bitLen) - 1
	v := math.Round(amps * 100)
	switch {
	case v <= 0:
		return 0
	case v >= float64(max):
		return max
	}
	return uint64(v)
}

func (d *EL9222) channel(ch int) (*ocpChannel, error) {
	if ch < 1 || ch > len(d.chans) {
		return nil, fmt.Errorf("%s: no OCP channel %d", d.name(), ch)
	}
	return d.chans[ch-1], nil
}

// Trip latches an overcurrent trip on channel ch until a Reset rising edge.
func (d *EL9222) Trip(ch int) error {
	c, err := d.channel(ch)
	if err == nil {
		c.tripped = true
	}
	return err
}

// SetWarning sets or clears the Current Level Warning of channel ch.
func (d *EL9222) SetWarning(ch int, on bool) error {
	c, err := d.channel(ch)
	if err == nil {
		c.warning = on
	}
	return err
}

// SetCoolDown sets or clears the Cool Down Lock of channel ch; while set the
// channel cannot be enabled.
func (d *EL9222) SetCoolDown(ch int, on bool) error {
	c, err := d.channel(ch)
	if err == nil {
		c.coolDown = on
	}
	return err
}

// SetHardwareProtection sets or clears Hardware Protection of channel ch;
// while set the channel is disabled and reports Error.
func (d *EL9222) SetHardwareProtection(ch int, on bool) error {
	c, err := d.channel(ch)
	if err == nil {
		c.hwProtection = on
	}
	return err
}

// SetLoadCurrent sets the load current in amps reported while ch is enabled.
func (d *EL9222) SetLoadCurrent(ch int, amps float64) error {
	c, err := d.channel(ch)
	if err != nil {
		return err
	}
	if math.IsNaN(amps) {
		return fmt.Errorf("%s: load current of channel %d is NaN", d.name(), ch)
	}
	c.amps = amps
	return nil
}

// Enabled reports whether channel ch was enabled in the last Step.
func (d *EL9222) Enabled(ch int) bool {
	c, err := d.channel(ch)
	return err == nil && c.enabled
}

// Tripped reports whether channel ch is latched tripped.
func (d *EL9222) Tripped(ch int) bool {
	c, err := d.channel(ch)
	return err == nil && c.tripped
}
