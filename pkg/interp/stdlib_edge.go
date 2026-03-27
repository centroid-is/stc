package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

// RTRIG implements the IEC 61131-3 R_TRIG (rising edge) detector.
// Q is TRUE for exactly one scan when CLK transitions from FALSE to TRUE.
type RTRIG struct {
	clk     bool
	q       bool
	prevCLK bool
}

func (r *RTRIG) Execute(_ time.Duration) {
	r.q = r.clk && !r.prevCLK
	r.prevCLK = r.clk
}

func (r *RTRIG) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "CLK":
		r.clk = v.Bool
	}
}

func (r *RTRIG) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: r.q, IECType: types.KindBOOL}
	}
	return Value{}
}

func (r *RTRIG) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "CLK":
		return Value{Kind: ValBool, Bool: r.clk, IECType: types.KindBOOL}
	}
	return Value{}
}

// FTRIG implements the IEC 61131-3 F_TRIG (falling edge) detector.
// Q is TRUE for exactly one scan when CLK transitions from TRUE to FALSE.
type FTRIG struct {
	clk     bool
	q       bool
	prevCLK bool
}

func (f *FTRIG) Execute(_ time.Duration) {
	f.q = !f.clk && f.prevCLK
	f.prevCLK = f.clk
}

func (f *FTRIG) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "CLK":
		f.clk = v.Bool
	}
}

func (f *FTRIG) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: f.q, IECType: types.KindBOOL}
	}
	return Value{}
}

func (f *FTRIG) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "CLK":
		return Value{Kind: ValBool, Bool: f.clk, IECType: types.KindBOOL}
	}
	return Value{}
}

func init() {
	StdlibFBFactory["R_TRIG"] = func() StandardFB { return &RTRIG{} }
	StdlibFBFactory["F_TRIG"] = func() StandardFB { return &FTRIG{} }
}
