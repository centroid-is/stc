package interp

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

// CTU implements the IEC 61131-3 up counter.
// Counts rising edges on CU. R resets CV to 0. Q=TRUE when CV>=PV.
type CTU struct {
	cu     bool
	r      bool
	pv     int64
	q      bool
	cv     int64
	prevCU bool
}

func (c *CTU) Execute(_ time.Duration) {
	if c.r {
		c.cv = 0
	} else if c.cu && !c.prevCU {
		// Rising edge on CU
		c.cv++
	}
	c.q = c.cv >= c.pv
	c.prevCU = c.cu
}

func (c *CTU) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "CU":
		c.cu = v.Bool
	case "R":
		c.r = v.Bool
	case "PV":
		c.pv = v.Int
	}
}

func (c *CTU) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: c.q, IECType: types.KindBOOL}
	case "CV":
		return Value{Kind: ValInt, Int: c.cv, IECType: types.KindDINT}
	}
	return Value{}
}

func (c *CTU) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "CU":
		return Value{Kind: ValBool, Bool: c.cu, IECType: types.KindBOOL}
	case "R":
		return Value{Kind: ValBool, Bool: c.r, IECType: types.KindBOOL}
	case "PV":
		return Value{Kind: ValInt, Int: c.pv, IECType: types.KindDINT}
	}
	return Value{}
}

// CTD implements the IEC 61131-3 down counter.
// Counts rising edges on CD (decrement). LD loads CV=PV. Q=TRUE when CV<=0.
type CTD struct {
	cd     bool
	ld     bool
	pv     int64
	q      bool
	cv     int64
	prevCD bool
}

func (c *CTD) Execute(_ time.Duration) {
	if c.ld {
		c.cv = c.pv
	} else if c.cd && !c.prevCD {
		// Rising edge on CD
		c.cv--
	}
	c.q = c.cv <= 0
	c.prevCD = c.cd
}

func (c *CTD) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "CD":
		c.cd = v.Bool
	case "LD":
		c.ld = v.Bool
	case "PV":
		c.pv = v.Int
	}
}

func (c *CTD) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "Q":
		return Value{Kind: ValBool, Bool: c.q, IECType: types.KindBOOL}
	case "CV":
		return Value{Kind: ValInt, Int: c.cv, IECType: types.KindDINT}
	}
	return Value{}
}

func (c *CTD) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "CD":
		return Value{Kind: ValBool, Bool: c.cd, IECType: types.KindBOOL}
	case "LD":
		return Value{Kind: ValBool, Bool: c.ld, IECType: types.KindBOOL}
	case "PV":
		return Value{Kind: ValInt, Int: c.pv, IECType: types.KindDINT}
	}
	return Value{}
}

// CTUD implements the IEC 61131-3 up/down counter.
// Priority: R > LD > CU/CD. QU=(CV>=PV), QD=(CV<=0).
type CTUD struct {
	cu     bool
	cd     bool
	r      bool
	ld     bool
	pv     int64
	qu     bool
	qd     bool
	cv     int64
	prevCU bool
	prevCD bool
}

func (c *CTUD) Execute(_ time.Duration) {
	if c.r {
		c.cv = 0
	} else if c.ld {
		c.cv = c.pv
	} else {
		if c.cu && !c.prevCU {
			c.cv++
		}
		if c.cd && !c.prevCD {
			c.cv--
		}
	}
	c.qu = c.cv >= c.pv
	c.qd = c.cv <= 0
	c.prevCU = c.cu
	c.prevCD = c.cd
}

func (c *CTUD) SetInput(name string, v Value) {
	switch strings.ToUpper(name) {
	case "CU":
		c.cu = v.Bool
	case "CD":
		c.cd = v.Bool
	case "R":
		c.r = v.Bool
	case "LD":
		c.ld = v.Bool
	case "PV":
		c.pv = v.Int
	}
}

func (c *CTUD) GetOutput(name string) Value {
	switch strings.ToUpper(name) {
	case "QU":
		return Value{Kind: ValBool, Bool: c.qu, IECType: types.KindBOOL}
	case "QD":
		return Value{Kind: ValBool, Bool: c.qd, IECType: types.KindBOOL}
	case "CV":
		return Value{Kind: ValInt, Int: c.cv, IECType: types.KindDINT}
	}
	return Value{}
}

func (c *CTUD) GetInput(name string) Value {
	switch strings.ToUpper(name) {
	case "CU":
		return Value{Kind: ValBool, Bool: c.cu, IECType: types.KindBOOL}
	case "CD":
		return Value{Kind: ValBool, Bool: c.cd, IECType: types.KindBOOL}
	case "R":
		return Value{Kind: ValBool, Bool: c.r, IECType: types.KindBOOL}
	case "LD":
		return Value{Kind: ValBool, Bool: c.ld, IECType: types.KindBOOL}
	case "PV":
		return Value{Kind: ValInt, Int: c.pv, IECType: types.KindDINT}
	}
	return Value{}
}

func init() {
	StdlibFBFactory["CTU"] = func() StandardFB { return &CTU{} }
	StdlibFBFactory["CTD"] = func() StandardFB { return &CTD{} }
	StdlibFBFactory["CTUD"] = func() StandardFB { return &CTUD{} }
}
