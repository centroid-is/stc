package devices

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

type analogKind int

const (
	analogCurrent analogKind = iota // 4-20 mA (EL305x)
	analogVoltage                   // 0-10 V (EL306x)
)

func (k analogKind) String() string {
	if k == analogVoltage {
		return "0-10 V"
	}
	return "4-20 mA"
}

// Status WORD bit positions of the EL30xx "AI Standard" PDO.
const (
	aiUnderrange    = 0
	aiOverrange     = 1
	aiError         = 6
	aiTxPDOState    = 14
	aiTxPDOToggle   = 15
	aiFullScale     = 32767
	aiOpenWireMilli = 0 // mA at or below this is an open wire
)

// Analog models EL3054 (4-20 mA) and EL3064 (0-10 V) analog input terminals.
//
// Channels are the input PDOs whose name ends in "Channel n". Each has a
// 16-bit "Value" entry and a status, either split into Status__* bit entries
// (as TwinCAT exports it) or one 16-bit "Status" entry with bits 0
// Underrange, 1 Overrange, 6 Error, 14 TxPDO State and 15 TxPDO Toggle.
// The toggle bit flips every Step.
type Analog struct {
	Base
	kind   analogKind
	chans  []*aiChannel // index ch-1; nil when the channel has no Value
	toggle bool
}

type aiChannel struct {
	value  ecat.Field
	status *ecat.Field           // single 16-bit Status entry
	bits   map[string]ecat.Field // Status__ suffix -> field

	raw              uint64
	under, over, err bool
}

// Bind stores the layout and resolves the channel entries.
func (a *Analog) Bind(l *ecat.Layout) {
	a.Base.Bind(l)
	a.chans = nil
	type parts struct {
		value  *ecat.Field
		status *ecat.Field
		bits   map[string]ecat.Field
	}
	found := map[int]*parts{}
	maxCh := 0
	for _, f := range l.Fields(ecat.DirIn) {
		ch, ok := channelNo(f.Pdo)
		if !ok || ch < 1 {
			continue
		}
		p := found[ch]
		if p == nil {
			p = &parts{bits: map[string]ecat.Field{}}
			found[ch] = p
		}
		f := f
		switch {
		case f.Entry == "Value" && p.value == nil:
			p.value = &f
		case f.Entry == "Status" && f.BitLen == 16:
			p.status = &f
		case strings.HasPrefix(f.Entry, "Status__"):
			p.bits[strings.TrimPrefix(f.Entry, "Status__")] = f
		}
		if ch > maxCh {
			maxCh = ch
		}
	}
	a.chans = make([]*aiChannel, maxCh)
	for ch, p := range found {
		if p.value == nil {
			continue
		}
		a.chans[ch-1] = &aiChannel{value: *p.value, status: p.status, bits: p.bits}
	}
}

// Channels returns the number of channels found in the layout.
func (a *Analog) Channels() int { return len(a.chans) }

// Step writes every channel's value and status, then applies Set overrides.
func (a *Analog) Step(_ time.Duration, out, in []byte) {
	a.toggle = !a.toggle
	for _, c := range a.chans {
		if c == nil {
			continue
		}
		ecat.Put(in, c.value, c.raw)
		flags := map[string]bool{
			"Underrange": c.under, "Overrange": c.over, "Error": c.err,
			"TxPDO State": false, "TxPDO Toggle": a.toggle,
		}
		if c.status != nil {
			var w uint64
			for bit, on := range map[int]bool{aiUnderrange: c.under, aiOverrange: c.over, aiError: c.err, aiTxPDOToggle: a.toggle} {
				if on {
					w |= 1 << uint(bit)
				}
			}
			ecat.Put(in, *c.status, w)
		}
		for name, f := range c.bits {
			if on, ok := flags[name]; ok {
				ecat.Put(in, f, b2u(on))
			}
		}
	}
	a.stepIO(out, in)
}

func (a *Analog) channel(ch int) (*aiChannel, error) {
	if ch < 1 || ch > len(a.chans) || a.chans[ch-1] == nil {
		return nil, fmt.Errorf("%s: no analog input channel %d", a.name(), ch)
	}
	return a.chans[ch-1], nil
}

func (a *Analog) wantKind(k analogKind) error {
	if a.kind != k {
		return fmt.Errorf("%s: %s terminal cannot take a %s stimulus", a.name(), a.kind, k)
	}
	return nil
}

// SetCurrent drives channel ch (1-based) of a 4-20 mA terminal with mA.
// 4..20 mA maps to 0..32767; below 4 mA sets Underrange, above 20 mA sets
// Overrange, and 0 mA or less (or NaN) is an open wire: Error and Underrange.
func (a *Analog) SetCurrent(ch int, mA float64) error {
	if err := a.wantKind(analogCurrent); err != nil {
		return err
	}
	c, err := a.channel(ch)
	if err != nil {
		return err
	}
	if math.IsNaN(mA) || mA <= aiOpenWireMilli {
		c.raw, c.under, c.over, c.err = 0, true, false, true
		return nil
	}
	c.raw, c.under, c.over = scale((mA-4)/16), mA < 4, mA > 20
	c.err = false
	return nil
}

// SetVoltage drives channel ch of a 0-10 V terminal with v volts. 0..10 V
// maps to 0..32767; below 0 V sets Underrange, above 10 V Overrange, and
// NaN sets Error.
func (a *Analog) SetVoltage(ch int, v float64) error {
	if err := a.wantKind(analogVoltage); err != nil {
		return err
	}
	c, err := a.channel(ch)
	if err != nil {
		return err
	}
	if math.IsNaN(v) {
		c.raw, c.under, c.over, c.err = 0, false, false, true
		return nil
	}
	c.raw, c.under, c.over = scale(v/10), v < 0, v > 10
	c.err = false
	return nil
}

// SetRaw writes the INT value of channel ch directly with a clean status.
func (a *Analog) SetRaw(ch int, v int16) error {
	c, err := a.channel(ch)
	if err != nil {
		return err
	}
	c.raw, c.under, c.over, c.err = uint64(uint16(v)), false, false, false
	return nil
}

// scale maps a 0..1 fraction to 0..32767, clamping (and handling ±Inf).
func scale(frac float64) uint64 {
	if frac <= 0 {
		return 0
	}
	if frac >= 1 {
		return aiFullScale
	}
	return uint64(math.Round(frac * aiFullScale))
}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
