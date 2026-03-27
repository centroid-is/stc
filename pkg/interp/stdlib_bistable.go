package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

// SR implements the IEC 61131-3 set-dominant bistable.
// Q1 = S1 OR (NOT R AND Q1). Set takes priority.
type SR struct {
	s1 bool
	r  bool
	q1 bool
}

func (sr *SR) Execute(_ time.Duration) {
	sr.q1 = sr.s1 || (!sr.r && sr.q1)
}

func (sr *SR) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "S1":
		sr.s1 = v.Bool
	case "R":
		sr.r = v.Bool
	}
}

func (sr *SR) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q1":
		return Value{Kind: ValBool, Bool: sr.q1, IECType: types.KindBOOL}
	}
	return Value{}
}

func (sr *SR) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "S1":
		return Value{Kind: ValBool, Bool: sr.s1, IECType: types.KindBOOL}
	case "R":
		return Value{Kind: ValBool, Bool: sr.r, IECType: types.KindBOOL}
	}
	return Value{}
}

// RS implements the IEC 61131-3 reset-dominant bistable.
// Q1 = NOT R1 AND (S OR Q1). Reset takes priority.
type RS struct {
	s  bool
	r1 bool
	q1 bool
}

func (rs *RS) Execute(_ time.Duration) {
	rs.q1 = !rs.r1 && (rs.s || rs.q1)
}

func (rs *RS) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "S":
		rs.s = v.Bool
	case "R1":
		rs.r1 = v.Bool
	}
}

func (rs *RS) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q1":
		return Value{Kind: ValBool, Bool: rs.q1, IECType: types.KindBOOL}
	}
	return Value{}
}

func (rs *RS) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "S":
		return Value{Kind: ValBool, Bool: rs.s, IECType: types.KindBOOL}
	case "R1":
		return Value{Kind: ValBool, Bool: rs.r1, IECType: types.KindBOOL}
	}
	return Value{}
}

func init() {
	StdlibFBFactory["SR"] = func() StandardFB { return &SR{} }
	StdlibFBFactory["RS"] = func() StandardFB { return &RS{} }
}
