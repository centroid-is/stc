package scenario

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/ecat/devices"
	"github.com/centroid-is/stc/pkg/interp"
)

// isLink reports whether path is a TcLinkTo link path (D-03).
func isLink(path string) bool { return strings.Contains(path, "^") }

// Check validates a without changing the plant.
func (p *Plant) Check(a Action) error { return p.do(a, false) }

// Apply performs a on the plant; it takes effect on the next Tick (D-12..D-15).
func (p *Plant) Apply(a Action) error { return p.do(a, true) }

func (p *Plant) do(a Action, apply bool) error {
	switch a.Kind {
	case ActSet:
		return p.set(a.Path, a.Value, apply)
	case ActLink:
		if !isLink(a.Path) {
			return fmt.Errorf("%w: %q is not a link path (TIID^...)", ErrUnknownPath, a.Path)
		}
		return p.set(a.Path, a.Value, apply)
	case ActRamp:
		if a.Path != "" {
			return p.number(a.Path, a.From, false)
		}
		return p.analog(Action{Slave: a.Slave, Channel: a.Channel, Unit: a.Unit, Value: a.From}, false)
	case ActAnalog:
		return p.analog(a, apply)
	case ActTrip:
		d, err := slaveModel[*devices.EL9222](p, a.Slave, "EL9222")
		if err != nil {
			return err
		}
		if err := channelOK(a.Channel, d.Channels()); err != nil {
			return err
		}
		if apply {
			return d.Trip(a.Channel)
		}
		return nil
	case ActSlaveState:
		return p.slaveState(a, apply)
	case ActDriveFault:
		d, err := slaveModel[*devices.ATV320](p, a.Slave, "ATV320")
		if err != nil {
			return err
		}
		if a.LFT < 0 || a.LFT > math.MaxUint16 {
			return fmt.Errorf("lft %d out of range 0..65535", a.LFT)
		}
		if apply {
			if a.LFT == 0 {
				d.ClearFault()
			} else {
				d.InjectFault(uint16(a.LFT))
			}
		}
		return nil
	case ActSerialPeer:
		d, err := slaveModel[*devices.EL6001](p, a.Slave, "EL6001")
		if err != nil {
			return err
		}
		var peer devices.SerialPeer
		switch strings.ToLower(a.Script) {
		case "baader":
			peer = devices.BaaderPeer()
		case "loopback":
			peer = &devices.LoopbackPeer{}
		case "none":
		default:
			return fmt.Errorf("unknown serial script %q (want baader, loopback or none)", a.Script)
		}
		if apply {
			d.SetPeer(peer)
		}
		return nil
	}
	return fmt.Errorf("unsupported action %q", a.Kind)
}

// SetNumber writes v to path for ramps: integer variables round half away
// from zero, BOOL takes v != 0, linked inputs and link paths are forced.
func (p *Plant) SetNumber(path string, v float64) error { return p.number(path, v, true) }

func (p *Plant) number(path string, v float64, apply bool) error {
	if slot, ok, err := p.inputSlot(path); err != nil {
		return err
	} else if ok {
		return p.force(slot, v, apply)
	}
	cur, err := p.rt.Get(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnknownPath, err)
	}
	var val any
	switch cur.Kind {
	case interp.ValInt:
		val = int64(math.Round(v))
	case interp.ValReal:
		val = v
	case interp.ValBool:
		val = v != 0
	default:
		return fmt.Errorf("%w: %s is not numeric", ErrUnknownPath, path)
	}
	if !apply {
		return p.rt.CheckSet(path, val)
	}
	return p.rt.Set(path, val)
}

// set routes a set/link value (D-13): link paths and TcLinkTo-bound input
// variables are forced at the process image, explicit AT %I variables are
// rejected, everything else goes to Runtime.Set.
func (p *Plant) set(path string, v any, apply bool) error {
	slot, ok, err := p.inputSlot(path)
	if err != nil {
		return err
	}
	if ok {
		return p.force(slot, v, apply)
	}
	if p.atIn[strings.ToUpper(strings.TrimSpace(path))] {
		return fmt.Errorf("%w: %s has an explicit AT %%I address and no TcLinkTo; the I/O sync would overwrite it", ErrUnknownPath, path)
	}
	if !apply {
		err = p.rt.CheckSet(path, v)
	} else {
		err = p.rt.Set(path, v)
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnknownPath, err)
	}
	return nil
}

// inputSlot resolves a link path, or a variable path bound to an input
// slot, to that slot. ok is false for unbound variable paths.
func (p *Plant) inputSlot(path string) (ecat.Slot, bool, error) {
	if isLink(path) {
		slot, err := p.linkSlot(path)
		if err != nil {
			return slot, false, err
		}
		if slot.Dir != ecat.DirIn {
			return slot, false, fmt.Errorf("%w: %s is an output slot; only inputs can be set", ErrUnknownPath, path)
		}
		return slot, true, nil
	}
	if p.binder == nil {
		return ecat.Slot{}, false, nil
	}
	bd, ok := p.binder.InputBinding(path)
	return bd.Slot, ok, nil
}

func (p *Plant) linkSlot(path string) (ecat.Slot, error) {
	if p.net == nil {
		return ecat.Slot{}, ErrNoNetwork
	}
	slot, ok := p.net.Topo.Slot(strings.TrimSpace(path))
	if !ok {
		return slot, fmt.Errorf("%w: no process image slot %q", ErrUnknownPath, path)
	}
	return slot, nil
}

func (p *Plant) force(slot ecat.Slot, v any, apply bool) error {
	bits, err := encodeSlot(slot, v)
	if err != nil {
		return fmt.Errorf("%s: %w", slot.Path, err)
	}
	if !apply {
		return nil
	}
	return p.net.ForceInput(slot.Path, bits)
}

// floatBits is 32 or 64 for REAL/LREAL slots, else 0.
func floatBits(slot ecat.Slot) int {
	switch strings.ToUpper(slot.DataType) {
	case "REAL", "FLOAT", "REAL32":
		return 32
	case "LREAL", "DOUBLE", "REAL64":
		return 64
	}
	return 0
}

// signedSlot reports whether the slot holds a signed integer.
func signedSlot(slot ecat.Slot) bool {
	switch strings.ToUpper(slot.DataType) {
	case "SINT", "INT", "DINT", "LINT", "INT8", "INT16", "INT32", "INT64":
		return true
	}
	return false
}

// encodeSlot turns a scenario value into slot bits: BOOL/BIT from bool or
// 0/1, integers two's-complement range-checked against the slot's width
// and signedness, REAL and LREAL as IEEE bits, strings as IEC integer
// literals or TRUE/FALSE.
func encodeSlot(slot ecat.Slot, v any) (uint64, error) {
	switch x := v.(type) {
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case int:
		return encodeInt(slot, int64(x))
	case int64:
		return encodeInt(slot, x)
	case float64:
		switch floatBits(slot) {
		case 32:
			return uint64(math.Float32bits(float32(x))), nil
		case 64:
			return math.Float64bits(x), nil
		}
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, fmt.Errorf("value %v is not finite", x)
		}
		return encodeInt(slot, int64(math.Round(x)))
	case string:
		s := strings.TrimSpace(x)
		switch strings.ToUpper(s) {
		case "TRUE":
			return 1, nil
		case "FALSE":
			return 0, nil
		}
		u, neg, err := parseIECUint(s)
		if err != nil {
			return 0, err
		}
		if !neg && u > math.MaxInt64 && floatBits(slot) == 0 && slot.BitLen >= 64 && !signedSlot(slot) {
			return u, nil // ULINT/LWORD above 2^63-1
		}
		n, err := iecToInt64(s, u, neg)
		if err != nil {
			return 0, err
		}
		return encodeInt(slot, n)
	}
	return 0, fmt.Errorf("unsupported value %v (%T)", v, v)
}

func encodeInt(slot ecat.Slot, n int64) (uint64, error) {
	switch floatBits(slot) {
	case 32:
		return uint64(math.Float32bits(float32(n))), nil
	case 64:
		return math.Float64bits(float64(n)), nil
	}
	if slot.BitLen == 1 && n != 0 && n != 1 {
		return 0, fmt.Errorf("value %d does not fit a BOOL slot", n)
	}
	if slot.BitLen > 1 {
		if signedSlot(slot) {
			if slot.BitLen < 64 {
				lim := int64(1) << (slot.BitLen - 1)
				if n < -lim || n >= lim {
					return 0, fmt.Errorf("value %d does not fit %s %d-bit signed slot", n, article(slot.BitLen), slot.BitLen)
				}
			}
		} else if n < 0 || (slot.BitLen < 64 && uint64(n) >= uint64(1)<<slot.BitLen) {
			return 0, fmt.Errorf("value %d does not fit %s %d-bit unsigned slot", n, article(slot.BitLen), slot.BitLen)
		}
	}
	return uint64(n), nil
}

// article is "an" before 8, 11 and 18 (spoken "eight", "eleven",
// "eighteen") and similar, else "a".
func article(bits int) string {
	switch s := strconv.Itoa(bits); {
	case s[0] == '8', s == "11", s == "18":
		return "an"
	}
	return "a"
}

// ParseIECInt parses an IEC integer literal: decimal, 2#, 8# or 16# with
// optional sign and underscores, and an optional TYPE# prefix (INT#5).
// Values outside the int64 range are errors.
func ParseIECInt(s string) (int64, error) {
	u, neg, err := parseIECUint(s)
	if err != nil {
		return 0, err
	}
	return iecToInt64(s, u, neg)
}

// iecToInt64 applies the sign of a parsed literal, rejecting magnitudes
// outside int64.
func iecToInt64(s string, u uint64, neg bool) (int64, error) {
	switch {
	case neg && u > 1<<63, !neg && u > math.MaxInt64:
		return 0, fmt.Errorf("IEC integer literal %q out of range", s)
	case neg:
		return int64(-u), nil // -(1<<63) wraps to MinInt64
	}
	return int64(u), nil
}

// parseIECUint parses an IEC integer literal into its magnitude and sign.
func parseIECUint(s string) (u uint64, neg bool, err error) {
	lit := strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	if strings.HasPrefix(lit, "-") || strings.HasPrefix(lit, "+") {
		neg = lit[0] == '-'
		lit = lit[1:]
	}
	base := 10
	if i := strings.Index(lit, "#"); i >= 0 {
		head, rest := lit[:i], lit[i+1:]
		switch head {
		case "2", "8", "16":
			base, _ = strconv.Atoi(head)
			lit = rest
		default:
			if _, err := strconv.Atoi(head); err == nil || head == "" {
				return 0, false, fmt.Errorf("invalid IEC integer literal %q", s)
			}
			u, n2, err := parseIECUint(rest) // TYPE#value
			return u, neg != n2, err
		}
	}
	u, err = strconv.ParseUint(lit, base, 64)
	if err != nil || lit == "" {
		return 0, false, fmt.Errorf("invalid IEC integer literal %q", s)
	}
	return u, neg, nil
}

// Read returns a variable path as ToJSON renders it, or a link path's slot
// (input or output image) as bool, int64 or float64.
func (p *Plant) Read(path string) (any, error) {
	if isLink(path) {
		slot, err := p.linkSlot(path)
		if err != nil {
			return nil, err
		}
		img := p.net.Images().Get(slot.Master)
		buf := img.In
		if slot.Dir == ecat.DirOut {
			buf = img.Out
		}
		return decodeSlot(slot, ecat.ReadBits(buf, slot.Byte, slot.Bit, slot.BitLen)), nil
	}
	v, err := p.rt.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnknownPath, err)
	}
	if v.Enum != "" {
		return EnumName(fmt.Sprint(p.rt.ToJSON(v))), nil
	}
	return p.rt.ToJSON(v), nil
}

func decodeSlot(slot ecat.Slot, raw uint64) any {
	switch {
	case slot.BitLen == 1:
		return raw != 0
	case floatBits(slot) == 32:
		return float64(math.Float32frombits(uint32(raw)))
	case floatBits(slot) == 64:
		return math.Float64frombits(raw)
	case signedSlot(slot) && slot.BitLen < 64:
		sh := uint(64 - slot.BitLen)
		return int64(raw<<sh) >> sh
	}
	return int64(raw)
}

// CheckPath validates an expect path without side effects.
func (p *Plant) CheckPath(path string) error {
	_, err := p.Read(path)
	return err
}

// slave resolves a slave name to master, bus index and its Slave record.
func (p *Plant) slave(name string) (string, int, *ecat.Slave, error) {
	if p.net == nil {
		return "", 0, nil, ErrNoNetwork
	}
	m, i, err := p.net.SlaveByName(name)
	if err != nil {
		return "", 0, nil, fmt.Errorf("%w: %v", ErrUnknownSlave, err)
	}
	return m, i, p.net.Topo.Master(m).Slaves[i], nil
}

// slaveModel returns the device model of the named slave as T, or an
// ErrUnknownSlave error naming the actual model (D-15).
func slaveModel[T ecat.Device](p *Plant, name, want string) (T, error) {
	var zero T
	m, i, s, err := p.slave(name)
	if err != nil {
		return zero, err
	}
	dev, _ := p.net.Device(m, i)
	d, ok := dev.(T)
	if !ok {
		model := s.Model
		if model == "" {
			model = "a slave without a model name"
		}
		return zero, fmt.Errorf("%w: %s is %s, not %s", ErrUnknownSlave, s.Name, model, want)
	}
	return d, nil
}

func channelOK(ch, n int) error {
	if ch < 1 || ch > n {
		return fmt.Errorf("channel %d out of range 1..%d", ch, n)
	}
	return nil
}

func (p *Plant) analog(a Action, apply bool) error {
	d, err := slaveModel[*devices.Analog](p, a.Slave, "an analog input terminal")
	if err != nil {
		return err
	}
	if err := channelOK(a.Channel, d.Channels()); err != nil {
		return err
	}
	v, ok := a.Value.(float64)
	if !ok {
		return fmt.Errorf("analog value %v is not a number", a.Value)
	}
	if !apply {
		return nil
	}
	switch a.Unit {
	case "mA":
		return d.SetCurrent(a.Channel, v)
	case "V":
		return d.SetVoltage(a.Channel, v)
	case "raw":
		r := math.Round(v)
		if r < math.MinInt16 || r > math.MaxInt16 {
			return fmt.Errorf("raw value %v out of INT range", v)
		}
		return d.SetRaw(a.Channel, int16(r))
	}
	return fmt.Errorf("unknown analog unit %q", a.Unit)
}

func (p *Plant) slaveState(a Action, apply bool) error {
	m, i, _, err := p.slave(a.Slave)
	if err != nil {
		return err
	}
	if a.HasStateCode {
		if a.StateCode < 0 || a.StateCode > math.MaxUint16 {
			return fmt.Errorf("slave state %d out of range 0..65535", a.StateCode)
		}
		if apply {
			return p.net.SetSlaveState(m, i, uint16(a.StateCode))
		}
		return nil
	}
	switch strings.ToLower(a.State) {
	case ecat.PresetNotPresent, ecat.PresetLinkError, ecat.PresetInit, ecat.PresetPreOp,
		ecat.PresetSafeOp, ecat.PresetOP, ecat.PresetOK:
	default:
		return fmt.Errorf("unknown slave state %q", a.State)
	}
	if apply {
		return p.net.ApplySlavePreset(m, i, a.State)
	}
	return nil
}
