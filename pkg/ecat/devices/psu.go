package devices

import (
	"math"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// PSUState is the simulated condition of a PS2001-2410 power supply.
type PSUState struct {
	Warning, Error, DCOK, InputUndervoltage bool
	OutputVoltage, OutputCurrent            float64 // volts, amps
}

// PSU models the PS2001-2410 EtherCAT power supply. Its entries are found by
// name across all input PDOs because the exports split them over "PSU
// Inputs" and "PSU Inputs Device". The default state is healthy: DC OK at
// 24 V and 0 A. While the PLC sets "Disable output", DC OK, voltage and
// current read 0.
//
// Voltage and current are written as IEEE floats when the entry is REAL or
// LREAL (as in the exports), otherwise as an unsigned integer in milli-units
// (mV, mA) clamped to the entry's bit length.
type PSU struct {
	Base
	state PSUState
}

func psu() ecat.Device { return &PSU{state: PSUState{DCOK: true, OutputVoltage: 24}} }

// SetPSU replaces the simulated supply state; it shows up from the next Step.
func (p *PSU) SetPSU(s PSUState) { p.state = s }

// State returns the simulated supply state.
func (p *PSU) State() PSUState { return p.state }

// Step writes the supply state, honouring the PLC's Disable output.
func (p *PSU) Step(_ time.Duration, out, in []byte) {
	s := p.state
	for _, f := range p.layout.FindEntry(ecat.DirOut, "Disable output") {
		if ecat.Get(out, f) != 0 {
			s.DCOK, s.OutputVoltage, s.OutputCurrent = false, 0, 0
		}
	}
	for name, v := range map[string]bool{
		"Warning": s.Warning, "Error": s.Error, "DC OK": s.DCOK, "Input undervoltage": s.InputUndervoltage,
	} {
		for _, f := range p.layout.FindEntry(ecat.DirIn, name) {
			ecat.Put(in, f, b2u(v))
		}
	}
	for name, v := range map[string]float64{"Output voltage": s.OutputVoltage, "Output current": s.OutputCurrent} {
		for _, f := range p.layout.FindEntry(ecat.DirIn, name) {
			ecat.Put(in, f, encodeAnalog(f, v))
		}
	}
	p.stepIO(out, in)
}

// encodeAnalog encodes a physical value for entry f: REAL/LREAL as IEEE bits,
// anything else as milli-units clamped to 0..2^BitLen-1. NaN encodes as 0.
func encodeAnalog(f ecat.Field, v float64) uint64 {
	if math.IsNaN(v) {
		v = 0
	}
	switch f.DataType {
	case "REAL":
		return uint64(math.Float32bits(float32(v)))
	case "LREAL":
		return math.Float64bits(v)
	}
	bits := f.BitLen
	if bits <= 0 || bits > 32 {
		bits = 32
	}
	max := float64(uint64(1)<<uint(bits) - 1)
	m := math.Round(v * 1000)
	switch {
	case m <= 0:
		return 0
	case m >= max:
		return uint64(max)
	}
	return uint64(m)
}
