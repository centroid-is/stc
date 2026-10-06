package devices

// CiA402State is a CiA 402 (IEC 61800-7-201) drive state.
type CiA402State uint8

// CiA402 drive states.
const (
	StNotReady CiA402State = iota
	StSwitchOnDisabled
	StReadyToSwitchOn
	StSwitchedOn
	StOperationEnabled
	StQuickStopActive
	StFaultReactionActive
	StFault
)

var cia402Names = [...]string{"NotReady", "SwitchOnDisabled", "ReadyToSwitchOn", "SwitchedOn",
	"OperationEnabled", "QuickStopActive", "FaultReactionActive", "Fault"}

// String returns the state name.
func (s CiA402State) String() string {
	if int(s) < len(cia402Names) {
		return cia402Names[s]
	}
	return "Unknown"
}

// Status words per state (ATV320: bit4 voltage enabled, bit9 remote).
var cia402ETA = [...]uint16{
	StNotReady:            0x0000,
	StSwitchOnDisabled:    0x0250,
	StReadyToSwitchOn:     0x0231,
	StSwitchedOn:          0x0233,
	StOperationEnabled:    0x0237,
	StQuickStopActive:     0x0217,
	StFaultReactionActive: 0x021F,
	StFault:               0x0218,
}

// Control word bits.
const (
	cmdFaultReset uint16 = 0x0080
	cmdHalt       uint16 = 0x0100
	etaHalt       uint16 = 0x0100
	etaTarget     uint16 = 0x0400
)

// CiA402 is a pure CiA 402 device-control state machine driven by the
// control word (CMD, 0x6040). It never touches a process image.
type CiA402 struct {
	st        CiA402State
	cmd       uint16
	prevReset bool
}

// Step applies one control word. faultActive reports a present fault
// condition: it moves any healthy state to FaultReactionActive and blocks
// fault reset. NotReady advances to SwitchOnDisabled and FaultReactionActive
// to Fault without looking at cmd.
func (c *CiA402) Step(cmd uint16, faultActive bool) {
	reset := cmd&cmdFaultReset != 0
	edge := reset && !c.prevReset
	c.prevReset = reset
	c.cmd = cmd
	switch c.st {
	case StNotReady:
		c.st = StSwitchOnDisabled
		return
	case StFaultReactionActive:
		c.st = StFault
		return
	case StFault:
		if edge && !faultActive {
			c.st = StSwitchOnDisabled
		}
		return
	}
	if faultActive {
		c.st = StFaultReactionActive
		return
	}
	switch {
	case cmd&0x82 == 0x00: // disable voltage
		c.st = StSwitchOnDisabled
	case cmd&0x86 == 0x02: // quick stop
		switch c.st {
		case StOperationEnabled:
			c.st = StQuickStopActive
		case StReadyToSwitchOn, StSwitchedOn:
			c.st = StSwitchOnDisabled
		}
	case cmd&0x87 == 0x06: // shutdown
		switch c.st {
		case StSwitchOnDisabled, StSwitchedOn, StOperationEnabled:
			c.st = StReadyToSwitchOn
		}
	case cmd&0x8F == 0x07: // switch on / disable operation
		switch c.st {
		case StReadyToSwitchOn, StOperationEnabled:
			c.st = StSwitchedOn
		}
	case cmd&0x8F == 0x0F: // enable operation (RTSO passes through SO)
		switch c.st {
		case StReadyToSwitchOn:
			c.st = StSwitchedOn
		case StSwitchedOn:
			c.st = StOperationEnabled
		}
	}
}

// Fault enters FaultReactionActive immediately.
func (c *CiA402) Fault() { c.st = StFaultReactionActive }

// QuickStopDone ends a quick stop once the drive reports zero speed.
func (c *CiA402) QuickStopDone() {
	if c.st == StQuickStopActive {
		c.st = StSwitchOnDisabled
	}
}

// Disable forces SwitchOnDisabled (e.g. STO) unless the drive is faulted.
func (c *CiA402) Disable() {
	if c.st != StFault && c.st != StFaultReactionActive {
		c.st = StSwitchOnDisabled
	}
}

// State returns the current state.
func (c *CiA402) State() CiA402State { return c.st }

// Enabled reports Operation enabled.
func (c *CiA402) Enabled() bool { return c.st == StOperationEnabled }

// Halt reports CMD bit 8 (halt) while operation is enabled.
func (c *CiA402) Halt() bool { return c.Enabled() && c.cmd&cmdHalt != 0 }

// ETA returns the status word; targetReached sets bit 10 in Operation enabled.
func (c *CiA402) ETA(targetReached bool) uint16 {
	if int(c.st) >= len(cia402ETA) {
		return 0
	}
	w := cia402ETA[c.st]
	if c.Enabled() {
		if targetReached {
			w |= etaTarget
		}
		if c.Halt() {
			w |= etaHalt
		}
	}
	return w
}
