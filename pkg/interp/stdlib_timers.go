package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

// TON implements the IEC 61131-3 ON-delay timer.
// Q becomes TRUE after IN is held TRUE for at least PT duration.
// ET tracks elapsed time and is capped at PT.
type TON struct {
	in      bool
	pt      time.Duration
	q       bool
	et      time.Duration
	running bool
}

func (t *TON) Execute(dt time.Duration) {
	if !t.in {
		// IN is FALSE: reset everything
		t.q = false
		t.et = 0
		t.running = false
		return
	}
	// IN is TRUE
	if !t.running {
		// Rising edge of IN: start timing
		t.running = true
		t.et = 0
	}
	// Accumulate time
	t.et += dt
	if t.et >= t.pt {
		t.et = t.pt // Cap ET at PT
		t.q = true
	}
}

func (t *TON) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "IN":
		t.in = v.Bool
	case "PT":
		t.pt = v.Time
	}
}

func (t *TON) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: t.q, IECType: types.KindBOOL}
	case "ET":
		return Value{Kind: ValTime, Time: t.et, IECType: types.KindTIME}
	}
	return Value{}
}

func (t *TON) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "IN":
		return Value{Kind: ValBool, Bool: t.in, IECType: types.KindBOOL}
	case "PT":
		return Value{Kind: ValTime, Time: t.pt, IECType: types.KindTIME}
	}
	return Value{}
}

// TOF implements the IEC 61131-3 OFF-delay timer.
// Q becomes TRUE immediately when IN rises, stays TRUE for PT after IN falls.
type TOF struct {
	in     bool
	pt     time.Duration
	q      bool
	et     time.Duration
	prevIN bool
	timing bool
}

func (t *TOF) Execute(dt time.Duration) {
	// Rising edge of IN
	if t.in && !t.prevIN {
		t.q = true
		t.timing = false
		t.et = 0
	}

	if t.in {
		// IN is TRUE: Q stays TRUE, no timing
		t.q = true
		t.timing = false
		t.et = 0
	} else if t.prevIN && !t.in {
		// Falling edge: start timing
		t.timing = true
		t.et = 0
	}

	if t.timing && !t.in {
		t.et += dt
		if t.et >= t.pt {
			t.et = t.pt
			t.q = false
			t.timing = false
		}
	}

	t.prevIN = t.in
}

func (t *TOF) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "IN":
		t.in = v.Bool
	case "PT":
		t.pt = v.Time
	}
}

func (t *TOF) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: t.q, IECType: types.KindBOOL}
	case "ET":
		return Value{Kind: ValTime, Time: t.et, IECType: types.KindTIME}
	}
	return Value{}
}

func (t *TOF) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "IN":
		return Value{Kind: ValBool, Bool: t.in, IECType: types.KindBOOL}
	case "PT":
		return Value{Kind: ValTime, Time: t.pt, IECType: types.KindTIME}
	}
	return Value{}
}

// TP implements the IEC 61131-3 Pulse timer.
// On rising edge of IN, outputs Q=TRUE for exactly PT duration.
// Changes to IN during active pulse are ignored.
type TP struct {
	in     bool
	pt     time.Duration
	q      bool
	et     time.Duration
	active bool
	prevIN bool
}

func (t *TP) Execute(dt time.Duration) {
	// Rising edge of IN while not active: start pulse
	if t.in && !t.prevIN && !t.active {
		t.active = true
		t.q = true
		t.et = 0
	}

	if t.active {
		t.et += dt
		if t.et >= t.pt {
			t.et = t.pt
			t.q = false
			// Pulse complete; active ends only when IN is also false
			if !t.in {
				t.active = false
			}
		}
	}

	// If pulse completed (Q=false) and IN went false, deactivate
	if t.active && !t.q && !t.in {
		t.active = false
		t.et = 0
	}

	t.prevIN = t.in
}

func (t *TP) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "IN":
		t.in = v.Bool
	case "PT":
		t.pt = v.Time
	}
}

func (t *TP) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: t.q, IECType: types.KindBOOL}
	case "ET":
		return Value{Kind: ValTime, Time: t.et, IECType: types.KindTIME}
	}
	return Value{}
}

func (t *TP) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "IN":
		return Value{Kind: ValBool, Bool: t.in, IECType: types.KindBOOL}
	case "PT":
		return Value{Kind: ValTime, Time: t.pt, IECType: types.KindTIME}
	}
	return Value{}
}

func init() {
	StdlibFBFactory["TON"] = func() StandardFB { return &TON{} }
	StdlibFBFactory["TOF"] = func() StandardFB { return &TOF{} }
	StdlibFBFactory["TP"] = func() StandardFB { return &TP{} }
}
