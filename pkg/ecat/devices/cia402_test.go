package devices

import "testing"

// statusWordToState mirrors FB_ATV320.status_word_to_state; the returned
// values are SVNCore's states enum.
func statusWordToState(w uint16) int {
	b := func(n uint) bool { return w&(1<<n) != 0 }
	rtso, so, oe, fault, ve, qs, sod := b(0), b(1), b(2), b(3), b(4), b(5), b(6)
	switch {
	case fault && oe && so && rtso:
		return 7
	case fault:
		return 8
	case !rtso && !so && !oe && !sod:
		return 0
	case sod:
		return 2
	case !qs:
		return 6
	case rtso && qs && !so:
		return 3
	case rtso && so && ve && oe:
		return 5
	case rtso && so && ve:
		return 4
	}
	return 0
}

func machineAt(st CiA402State) *CiA402 { return &CiA402{st: st} }

func TestCiA402Transitions(t *testing.T) {
	cases := []struct {
		name  string
		from  CiA402State
		cmd   uint16
		fault bool
		want  CiA402State
	}{
		{"power-up", StNotReady, 0x0F, false, StSwitchOnDisabled},
		{"SOD shutdown", StSwitchOnDisabled, 0x06, false, StReadyToSwitchOn},
		{"SOD ignores switch on", StSwitchOnDisabled, 0x07, false, StSwitchOnDisabled},
		{"SOD ignores enable", StSwitchOnDisabled, 0x0F, false, StSwitchOnDisabled},
		{"SOD quick stop stays", StSwitchOnDisabled, 0x02, false, StSwitchOnDisabled},
		{"RTSO switch on", StReadyToSwitchOn, 0x07, false, StSwitchedOn},
		{"RTSO enable passes SO", StReadyToSwitchOn, 0x0F, false, StSwitchedOn},
		{"RTSO disable voltage", StReadyToSwitchOn, 0x00, false, StSwitchOnDisabled},
		{"RTSO quick stop", StReadyToSwitchOn, 0x02, false, StSwitchOnDisabled},
		{"RTSO shutdown stays", StReadyToSwitchOn, 0x06, false, StReadyToSwitchOn},
		{"SO enable", StSwitchedOn, 0x0F, false, StOperationEnabled},
		{"SO shutdown", StSwitchedOn, 0x06, false, StReadyToSwitchOn},
		{"SO disable voltage", StSwitchedOn, 0x00, false, StSwitchOnDisabled},
		{"SO quick stop", StSwitchedOn, 0x02, false, StSwitchOnDisabled},
		{"OE disable operation", StOperationEnabled, 0x07, false, StSwitchedOn},
		{"OE shutdown", StOperationEnabled, 0x06, false, StReadyToSwitchOn},
		{"OE disable voltage", StOperationEnabled, 0x00, false, StSwitchOnDisabled},
		{"OE quick stop", StOperationEnabled, 0x02, false, StQuickStopActive},
		{"OE halt stays", StOperationEnabled, 0x10F, false, StOperationEnabled},
		{"OE fault reset bit ignored", StOperationEnabled, 0x80, false, StOperationEnabled},
		{"QSA quick stop stays", StQuickStopActive, 0x02, false, StQuickStopActive},
		{"QSA disable voltage", StQuickStopActive, 0x00, false, StSwitchOnDisabled},
		{"OE fault", StOperationEnabled, 0x0F, true, StFaultReactionActive},
		{"SOD fault", StSwitchOnDisabled, 0x06, true, StFaultReactionActive},
		{"FRA to fault", StFaultReactionActive, 0x80, false, StFault},
		{"fault reset edge", StFault, 0x80, false, StSwitchOnDisabled},
		{"fault reset blocked while active", StFault, 0x80, true, StFault},
		{"fault ignores shutdown", StFault, 0x06, false, StFault},
	}
	for _, c := range cases {
		m := machineAt(c.from)
		m.Step(c.cmd, c.fault)
		if m.State() != c.want {
			t.Errorf("%s: %v --%#x--> %v, want %v", c.name, c.from, c.cmd, m.State(), c.want)
		}
	}
}

func TestCiA402StartupSequence(t *testing.T) {
	var m CiA402
	if m.State() != StNotReady || m.ETA(false) != 0 {
		t.Fatalf("zero value = %v %#x", m.State(), m.ETA(false))
	}
	want := []CiA402State{StSwitchOnDisabled, StReadyToSwitchOn, StSwitchedOn, StOperationEnabled}
	for i, cmd := range []uint16{0x00, 0x06, 0x07, 0x0F} {
		m.Step(cmd, false)
		if m.State() != want[i] {
			t.Fatalf("step %d: %v, want %v", i, m.State(), want[i])
		}
	}
	if !m.Enabled() {
		t.Error("not enabled in OE")
	}
}

func TestCiA402EnableDoubleStep(t *testing.T) {
	m := machineAt(StReadyToSwitchOn)
	m.Step(0x0F, false)
	if m.State() != StSwitchedOn {
		t.Fatalf("first 0x0F: %v", m.State())
	}
	m.Step(0x0F, false)
	if m.State() != StOperationEnabled {
		t.Fatalf("second 0x0F: %v", m.State())
	}
}

func TestCiA402QuickStopPath(t *testing.T) {
	m := machineAt(StOperationEnabled)
	m.QuickStopDone() // no effect outside QSA
	if m.State() != StOperationEnabled {
		t.Fatal("QuickStopDone left OE")
	}
	m.Step(0x02, false)
	m.Step(0x02, false)
	if m.State() != StQuickStopActive {
		t.Fatalf("state %v", m.State())
	}
	m.QuickStopDone()
	if m.State() != StSwitchOnDisabled {
		t.Fatalf("after done: %v", m.State())
	}
}

func TestCiA402FaultReset(t *testing.T) {
	m := machineAt(StSwitchedOn)
	m.Fault()
	if m.State() != StFaultReactionActive {
		t.Fatal("Fault() did not enter FRA")
	}
	m.Step(0x80, true) // FRA -> Fault; the held reset bit is not an edge later
	m.Step(0x80, true)
	if m.State() != StFault {
		t.Fatalf("state %v", m.State())
	}
	m.Step(0x80, false) // condition cleared but no new edge
	if m.State() != StFault {
		t.Fatal("reset without a rising edge")
	}
	m.Step(0x00, false)
	m.Step(0x80, false)
	if m.State() != StSwitchOnDisabled {
		t.Fatalf("after edge: %v", m.State())
	}
}

func TestCiA402DisableAndHalt(t *testing.T) {
	for _, st := range []CiA402State{StNotReady, StReadyToSwitchOn, StSwitchedOn, StOperationEnabled, StQuickStopActive} {
		m := machineAt(st)
		m.Disable()
		if m.State() != StSwitchOnDisabled {
			t.Errorf("Disable from %v: %v", st, m.State())
		}
	}
	for _, st := range []CiA402State{StFault, StFaultReactionActive} {
		m := machineAt(st)
		m.Disable()
		if m.State() != st {
			t.Errorf("Disable left %v", st)
		}
	}
	m := machineAt(StSwitchedOn)
	m.Step(0x107, false)
	if m.Halt() {
		t.Error("halt outside OE")
	}
	m.Step(0x10F, false)
	if !m.Halt() || m.ETA(false) != 0x0337 || m.ETA(true) != 0x0737 {
		t.Errorf("halt=%v eta=%#x/%#x", m.Halt(), m.ETA(false), m.ETA(true))
	}
}

func TestCiA402ETA(t *testing.T) {
	enum := map[CiA402State]int{
		StNotReady: 0, StSwitchOnDisabled: 2, StReadyToSwitchOn: 3, StSwitchedOn: 4,
		StOperationEnabled: 5, StQuickStopActive: 6, StFaultReactionActive: 7, StFault: 8,
	}
	words := map[CiA402State]uint16{
		StNotReady: 0x0000, StSwitchOnDisabled: 0x0250, StReadyToSwitchOn: 0x0231,
		StSwitchedOn: 0x0233, StOperationEnabled: 0x0237, StQuickStopActive: 0x0217,
		StFaultReactionActive: 0x021F, StFault: 0x0218,
	}
	for st, w := range words {
		m := machineAt(st)
		if got := m.ETA(true); st != StOperationEnabled && got != w {
			t.Errorf("%v ETA = %#x, want %#x", st, got, w)
		}
		if got := m.ETA(false); got != w {
			t.Errorf("%v ETA = %#x, want %#x", st, got, w)
		}
		if got := statusWordToState(w); got != enum[st] {
			t.Errorf("%v: %#x decodes to %d, want %d", st, w, got, enum[st])
		}
		if st.String() == "Unknown" {
			t.Errorf("no name for %d", st)
		}
	}
	if got := machineAt(StOperationEnabled).ETA(true); got != 0x0637 || statusWordToState(got) != 5 {
		t.Errorf("OE target reached = %#x", got)
	}
	if CiA402State(42).String() != "Unknown" || machineAt(42).ETA(false) != 0 {
		t.Error("invalid state handling")
	}
	if statusWordToState(0x0023) != 0 { // RTSO+SO without voltage enabled falls through
		t.Error("fallthrough decode")
	}
}
