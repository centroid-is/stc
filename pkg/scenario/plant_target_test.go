package scenario

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/ecat/devices"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	slaveDI  = "DEMO.A1.01 (EL1008)"
	slaveOCP = "DEMO.A1.03 (EL9222-5500)"
	slaveATV = "DEMO.CN01.FD01 (ATV320 EtherCAT)"
	linkCur  = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Current"
)

const plantMain = `PROGRAM MAIN
VAR
	n : INT;
	r : REAL;
	b : BOOL;
	s : STRING;
	xIn AT %IX0.0 : BOOL;
END_VAR
ECT.CN01();
END_PROGRAM
`

var _ Target = (*Plant)(nil)

func tickN(t *testing.T, p *Plant, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, p.Tick())
	}
}

func readVal(t *testing.T, p *Plant, path string) any {
	t.Helper()
	v, err := p.Read(path)
	require.NoError(t, err, path)
	return v
}

func TestPlantStepper(t *testing.T) {
	p := demoPlant(t, plantMain)
	assert.Equal(t, DefaultBaseTick, p.BaseTick())
	assert.Zero(t, p.Clock())
	tickN(t, p, 3)
	assert.Equal(t, 3*DefaultBaseTick, p.Clock())
}

func TestPlantSetLinkedInput(t *testing.T) {
	p := demoPlant(t, plantMain)
	require.NoError(t, p.Check(Action{Kind: ActSet, Path: "ECT.A1_01.I1", Value: true}))
	_, forced := p.Network().Forced(linkI1)
	assert.False(t, forced, "Check has no side effects")
	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: "ect.a1_01.i1", Value: true}))
	for i := 0; i < 6; i++ {
		tickN(t, p, 1)
		assert.Equal(t, true, readVal(t, p, "ECT.A1_01.I1"), "tick %d", i)
	}
	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: "ECT.A1_01.I1", Value: "FALSE"}))
	tickN(t, p, 1)
	assert.Equal(t, false, readVal(t, p, "ECT.A1_01.I1"))
	assert.Error(t, p.Check(Action{Kind: ActSet, Path: "ECT.A1_01.I1", Value: int64(2)}))
	assert.Error(t, p.Check(Action{Kind: ActSet, Path: "ECT.A1_01.I1", Value: []int{1}}))
}

func TestPlantSetVariable(t *testing.T) {
	p := demoPlant(t, plantMain)
	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: "MAIN.n", Value: int64(7)}))
	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: "MAIN.s", Value: "hi"}))
	tickN(t, p, 1)
	assert.EqualValues(t, 7, readVal(t, p, "MAIN.n"))
	assert.Equal(t, "hi", readVal(t, p, "MAIN.s"))

	err := p.Check(Action{Kind: ActSet, Path: "MAIN.xIn", Value: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownPath))
	assert.Contains(t, err.Error(), "AT %I")

	err = p.Apply(Action{Kind: ActSet, Path: "MAIN.nope", Value: int64(1)})
	assert.True(t, errors.Is(err, ErrUnknownPath))
	err = p.Check(Action{Kind: ActSet, Path: "MAIN.nope", Value: int64(1)})
	assert.True(t, errors.Is(err, ErrUnknownPath))
}

func TestPlantLink(t *testing.T) {
	p := demoPlant(t, plantMain)
	require.NoError(t, p.Apply(Action{Kind: ActLink, Path: linkI1, Value: int64(1)}))
	tickN(t, p, 1)
	assert.Equal(t, true, readVal(t, p, "ECT.A1_01.I1"))
	assert.Equal(t, true, readVal(t, p, linkI1))

	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: linkCur, Value: "16#1F4"}))
	tickN(t, p, 1)
	assert.EqualValues(t, 500, readVal(t, p, linkCur))
	assert.EqualValues(t, 500, readVal(t, p, "ECT.A1_03.p_Current"))

	err := p.Apply(Action{Kind: ActLink, Path: linkO1, Value: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownPath))
	err = p.Check(Action{Kind: ActLink, Path: "ECT.A1_01.I1", Value: true})
	assert.True(t, errors.Is(err, ErrUnknownPath))
	err = p.Check(Action{Kind: ActLink, Path: "TIID^Device 1 (EtherCAT)^Nope", Value: true})
	assert.True(t, errors.Is(err, ErrUnknownPath))
	assert.Error(t, p.Check(Action{Kind: ActLink, Path: linkCur, Value: "16#ZZ"}))

	assert.Equal(t, false, readVal(t, p, linkO1), "output image read")
}

func TestPlantTrip(t *testing.T) {
	p := demoPlant(t, plantMain)
	tickN(t, p, 2)
	require.Equal(t, true, readVal(t, p, "ECT.A1_03.p_stat_Enabled"))
	require.NoError(t, p.Apply(Action{Kind: ActTrip, Slave: slaveOCP, Channel: 1}))
	tickN(t, p, 1)
	assert.Equal(t, false, readVal(t, p, "ECT.A1_03.p_stat_Enabled"))
	assert.Equal(t, true, readVal(t, p, "ECT.A1_03.p_stat_Tripped"))

	err := p.Check(Action{Kind: ActTrip, Slave: slaveDI, Channel: 1})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownSlave))
	assert.Contains(t, err.Error(), "is EL1008, not EL9222")
	assert.Error(t, p.Check(Action{Kind: ActTrip, Slave: slaveOCP, Channel: 3}))
	err = p.Check(Action{Kind: ActTrip, Slave: "nope", Channel: 1})
	assert.True(t, errors.Is(err, ErrUnknownSlave))
}

func TestPlantSlaveState(t *testing.T) {
	p := demoPlant(t, plantMain)
	tickN(t, p, 1)
	assert.EqualValues(t, 8, readVal(t, p, "ECT.A1_01_State"))
	require.NoError(t, p.Apply(Action{Kind: ActSlaveState, Slave: "DEMO.A1.01", State: "not_present"}))
	tickN(t, p, 1)
	assert.EqualValues(t, 0x0011, readVal(t, p, "ECT.A1_01_State"))
	assert.Equal(t, true, readVal(t, p, "ECT.A1_01_WcState"))

	require.NoError(t, p.Apply(Action{Kind: ActSlaveState, Slave: slaveDI, State: "ok"}))
	tickN(t, p, 1)
	assert.EqualValues(t, 8, readVal(t, p, "ECT.A1_01_State"))
	assert.Equal(t, false, readVal(t, p, "ECT.A1_01_WcState"))

	require.NoError(t, p.Apply(Action{Kind: ActSlaveState, Slave: slaveDI, HasStateCode: true, StateCode: 0x12}))
	tickN(t, p, 1)
	assert.EqualValues(t, 0x12, readVal(t, p, "ECT.A1_01_State"))
	assert.Error(t, p.Check(Action{Kind: ActSlaveState, Slave: slaveDI, HasStateCode: true, StateCode: 70000}))
	assert.Error(t, p.Check(Action{Kind: ActSlaveState, Slave: slaveDI, State: "melted"}))
	for _, st := range []string{"init", "preop", "safeop", "op", "link_error"} {
		assert.NoError(t, p.Check(Action{Kind: ActSlaveState, Slave: slaveDI, State: st}))
	}
}

func TestPlantDriveFault(t *testing.T) {
	p := demoPlant(t, plantMain)
	dev, ok := p.Network().DeviceByName(slaveATV)
	require.True(t, ok)
	drv := dev.(*devices.ATV320)
	require.NoError(t, p.Apply(Action{Kind: ActDriveFault, Slave: slaveATV, LFT: 16}))
	tickN(t, p, 2)
	assert.Equal(t, uint16(16), drv.LFT())
	require.NoError(t, p.Apply(Action{Kind: ActDriveFault, Slave: "DEMO.CN01.FD01", LFT: 0}))
	assert.Error(t, p.Check(Action{Kind: ActDriveFault, Slave: slaveATV, LFT: -1}))
	err := p.Check(Action{Kind: ActDriveFault, Slave: slaveDI, LFT: 1})
	assert.True(t, errors.Is(err, ErrUnknownSlave))
}

// mainOnly is a project without links, for single-export networks.
func mainOnly(t *testing.T, io string) *Plant {
	t.Helper()
	src := interp.ProjectSpec{Files: []*ast.SourceFile{parseST(t, "main.st", "PROGRAM MAIN\nVAR n : INT; END_VAR\nEND_PROGRAM\n")}}
	spec, err := BuildPlantSpec(src, ioPaths(io))
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	return p
}

func TestPlantAnalog(t *testing.T) {
	p := mainOnly(t, "Demo Analog.xml")
	const ai = "DEMO.A2.01 (EL3054)"
	val := "TIID^Device 1 (EtherCAT)^DEMO.A2.00 (EK1100)^DEMO.A2.01 (EL3054)^AI Standard Channel 1^Value"
	require.NoError(t, p.Apply(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "mA", Value: 12.0}))
	tickN(t, p, 1)
	assert.EqualValues(t, 16384, readVal(t, p, val))
	require.NoError(t, p.Apply(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "raw", Value: 1234.4}))
	tickN(t, p, 1)
	assert.EqualValues(t, 1234, readVal(t, p, val))

	assert.Error(t, p.Apply(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "raw", Value: 40000.0}))
	assert.Error(t, p.Apply(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "V", Value: 5.0}), "current terminal")
	assert.Error(t, p.Apply(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "furlongs", Value: 5.0}))
	assert.Error(t, p.Check(Action{Kind: ActAnalog, Slave: ai, Channel: 9, Unit: "mA", Value: 5.0}))
	assert.Error(t, p.Check(Action{Kind: ActAnalog, Slave: ai, Channel: 1, Unit: "mA", Value: "x"}))
	err := p.Check(Action{Kind: ActAnalog, Slave: "DEMO.A2.00", Channel: 1, Unit: "mA", Value: 5.0})
	assert.True(t, errors.Is(err, ErrUnknownSlave))
	assert.Contains(t, err.Error(), "is EK1100")

	assert.NoError(t, p.Check(Action{Kind: ActRamp, Slave: ai, Channel: 1, Unit: "mA", From: 4, To: 20}))
	require.NoError(t, p.Apply(Action{Kind: ActAnalog, Slave: "DEMO.A2.02", Channel: 1, Unit: "V", Value: 5.0}))
}

func TestPlantSerialPeer(t *testing.T) {
	p := mainOnly(t, "Demo Serial.xml")
	dev, ok := p.Network().DeviceByName("DEMO.A3.01 (EL6001)")
	require.True(t, ok)
	el := dev.(*devices.EL6001)
	require.NoError(t, p.Apply(Action{Kind: ActSerialPeer, Slave: "DEMO.A3.01", Script: "baader"}))
	_, isScripted := el.Peer().(*devices.ScriptedPeer)
	assert.True(t, isScripted)
	require.NoError(t, p.Apply(Action{Kind: ActSerialPeer, Slave: "DEMO.A3.01", Script: "loopback"}))
	_, isLoop := el.Peer().(*devices.LoopbackPeer)
	assert.True(t, isLoop)
	require.NoError(t, p.Apply(Action{Kind: ActSerialPeer, Slave: "DEMO.A3.01", Script: "none"}))
	assert.Nil(t, el.Peer())
	assert.Error(t, p.Check(Action{Kind: ActSerialPeer, Slave: "DEMO.A3.01", Script: "telnet"}))
	err := p.Check(Action{Kind: ActSerialPeer, Slave: "DEMO.A3.00", Script: "none"})
	assert.True(t, errors.Is(err, ErrUnknownSlave))
}

func TestPlantSetNumber(t *testing.T) {
	p := demoPlant(t, plantMain)
	require.NoError(t, p.SetNumber("MAIN.n", 2.5))
	assert.EqualValues(t, 3, readVal(t, p, "MAIN.n"))
	require.NoError(t, p.SetNumber("MAIN.n", -2.5))
	assert.EqualValues(t, -3, readVal(t, p, "MAIN.n"))
	require.NoError(t, p.SetNumber("MAIN.r", 1.25))
	assert.InDelta(t, 1.25, readVal(t, p, "MAIN.r"), 1e-9)
	require.NoError(t, p.SetNumber("MAIN.b", 1))
	assert.Equal(t, true, readVal(t, p, "MAIN.b"))
	assert.Error(t, p.SetNumber("MAIN.s", 1))
	assert.True(t, errors.Is(p.SetNumber("MAIN.zz", 1), ErrUnknownPath))
	assert.NoError(t, p.Check(Action{Kind: ActRamp, Path: "MAIN.n", From: 0, To: 10}))

	require.NoError(t, p.SetNumber("ECT.A1_03.p_Current", 7.6))
	tickN(t, p, 1)
	assert.EqualValues(t, 8, readVal(t, p, "ECT.A1_03.p_Current"))
	assert.Error(t, p.SetNumber(linkO1, 1))
}

func TestPlantEncodeDecode(t *testing.T) {
	real32 := ecat.Slot{DataType: "REAL", BitLen: 32}
	bits, err := encodeSlot(real32, 1.5)
	require.NoError(t, err)
	assert.Equal(t, uint64(math.Float32bits(1.5)), bits)
	assert.Equal(t, 1.5, decodeSlot(real32, bits))
	bits, err = encodeSlot(real32, int64(2))
	require.NoError(t, err)
	assert.Equal(t, uint64(math.Float32bits(2)), bits)

	lreal := ecat.Slot{DataType: "LREAL", BitLen: 64}
	bits, err = encodeSlot(lreal, 2.25)
	require.NoError(t, err)
	assert.Equal(t, math.Float64bits(2.25), bits)
	assert.Equal(t, 2.25, decodeSlot(lreal, bits))
	bits, _ = encodeSlot(lreal, 3)
	assert.Equal(t, math.Float64bits(3), bits)

	i16 := ecat.Slot{DataType: "INT", BitLen: 16}
	bits, err = encodeSlot(i16, int64(-2))
	require.NoError(t, err)
	assert.EqualValues(t, -2, decodeSlot(i16, bits&0xFFFF))
	bits, _ = encodeSlot(i16, -2.5)
	assert.EqualValues(t, -3, decodeSlot(i16, bits&0xFFFF))
	_, err = encodeSlot(i16, math.NaN())
	assert.Error(t, err)
	bits, _ = encodeSlot(i16, "TRUE")
	assert.Equal(t, uint64(1), bits)
	bits, _ = encodeSlot(i16, "false")
	assert.Equal(t, uint64(0), bits)
	assert.EqualValues(t, int64(-1), decodeSlot(ecat.Slot{DataType: "LINT", BitLen: 64}, ^uint64(0)))
	assert.EqualValues(t, int64(7), decodeSlot(ecat.Slot{DataType: "UINT", BitLen: 16}, 7))
}

func TestParseIECInt(t *testing.T) {
	for in, want := range map[string]int64{
		"42": 42, "-42": -42, "+7": 7, "16#FF": 255, "16#ff_ff": 0xFFFF, "2#1010": 10, "8#17": 15,
		"-16#10": -16, "INT#5": 5, "DINT#16#10": 16, "1_000": 1000,
	} {
		got, err := ParseIECInt(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "x", "3#12", "#5", "16#", "1.5", "16#GG"} {
		_, err := ParseIECInt(bad)
		assert.Error(t, err, bad)
	}
}

func TestPlantReadAndCheckPath(t *testing.T) {
	p := demoPlant(t, plantMain)
	assert.NoError(t, p.CheckPath("MAIN.n"))
	assert.NoError(t, p.CheckPath(linkO1))
	assert.True(t, errors.Is(p.CheckPath("MAIN.zz"), ErrUnknownPath))
	assert.True(t, errors.Is(p.CheckPath("TIID^x"), ErrUnknownPath))
	assert.Error(t, p.Check(Action{Kind: "bogus"}))
}

func TestPlantNoNetwork(t *testing.T) {
	spec, err := BuildPlantSpec(demoSources(t, plantMain), nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	for _, a := range []Action{
		{Kind: ActLink, Path: linkI1, Value: true},
		{Kind: ActSet, Path: linkI1, Value: true},
		{Kind: ActTrip, Slave: slaveOCP, Channel: 1},
		{Kind: ActAnalog, Slave: "x", Channel: 1, Unit: "mA", Value: 1.0},
		{Kind: ActSlaveState, Slave: slaveDI, State: "ok"},
		{Kind: ActDriveFault, Slave: slaveATV, LFT: 1},
		{Kind: ActSerialPeer, Slave: "x", Script: "none"},
		{Kind: ActRamp, Slave: "x", Channel: 1, Unit: "mA"},
	} {
		err := p.Apply(a)
		assert.True(t, errors.Is(err, ErrNoNetwork), "%s: %v", a.Kind, err)
	}
	_, err = p.Read(linkI1)
	assert.True(t, errors.Is(err, ErrNoNetwork))
	// Variables still work without a network.
	require.NoError(t, p.Apply(Action{Kind: ActSet, Path: "MAIN.n", Value: int64(3)}))
	assert.EqualValues(t, 3, readVal(t, p, "MAIN.n"))
	assert.False(t, strings.Contains(ErrNoNetwork.Error(), "%"))
}

func TestPlantTickError(t *testing.T) {
	src := interp.ProjectSpec{Files: []*ast.SourceFile{parseST(t, "main.st",
		"PROGRAM MAIN\nVAR a : ARRAY[1..2] OF INT; i : INT := 5; END_VAR\na[i] := 1;\nEND_PROGRAM\n")}}
	spec, err := BuildPlantSpec(src, nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	assert.Error(t, p.Tick())
	assert.Equal(t, p.BaseTick(), p.Clock(), "Project semantics: the clock advances past a failing task")
}

// Values that do not fit the slot width are errors, not truncated, and
// IEC literals outside int64 are rejected (review 2 ME-02).
func TestPlantEncodeRange(t *testing.T) {
	u8 := ecat.Slot{DataType: "USINT", BitLen: 8}
	s8 := ecat.Slot{DataType: "SINT", BitLen: 8}
	u16 := ecat.Slot{DataType: "UINT", BitLen: 16}
	u64 := ecat.Slot{DataType: "ULINT", BitLen: 64}
	i64 := ecat.Slot{DataType: "LINT", BitLen: 64}
	for _, c := range []struct {
		slot ecat.Slot
		v    any
		want uint64
	}{
		{u8, int64(255), 255}, {u8, 0, 0}, {s8, int64(-128), uint64(0xFFFFFFFFFFFFFF80)}, {s8, int64(127), 127},
		{u16, "16#FFFF", 0xFFFF}, {u16, 300.4, 300}, {u64, "16#FFFFFFFFFFFFFFFF", ^uint64(0)},
		{u64, "18446744073709551615", ^uint64(0)}, {i64, "-9223372036854775808", 1 << 63},
	} {
		got, err := encodeSlot(c.slot, c.v)
		require.NoError(t, err, "%s %v", c.slot.DataType, c.v)
		assert.Equal(t, c.want, got, "%s %v", c.slot.DataType, c.v)
	}
	for _, c := range []struct {
		slot ecat.Slot
		v    any
		msg  string
	}{
		{u8, int64(300), "value 300 does not fit an 8-bit unsigned slot"},
		{u8, -1, "value -1 does not fit an 8-bit unsigned slot"},
		{s8, int64(128), "value 128 does not fit an 8-bit signed slot"},
		{s8, "-129", "does not fit an 8-bit signed slot"},
		{u16, 70000.0, "does not fit a 16-bit unsigned slot"},
		{u64, int64(-1), "does not fit a 64-bit unsigned slot"},
		{i64, "16#FFFFFFFFFFFFFFFF", "out of range"},
		{u64, "-1", "does not fit a 64-bit unsigned slot"},
		{u64, "18446744073709551616", "invalid IEC integer literal"},
	} {
		_, err := encodeSlot(c.slot, c.v)
		require.Error(t, err, "%s %v", c.slot.DataType, c.v)
		assert.Contains(t, err.Error(), c.msg)
	}
	for _, bad := range []string{"-9999999999999999999", "16#FFFFFFFFFFFFFFFF", "9223372036854775808", "-9223372036854775809"} {
		_, err := ParseIECInt(bad)
		assert.Error(t, err, bad)
	}
	n, err := ParseIECInt("-9223372036854775808")
	require.NoError(t, err)
	assert.Equal(t, int64(math.MinInt64), n)
}

// A link value wider than its slot fails Check (SCN007 in Prepare)
// instead of being truncated by ForceInput.
func TestPlantLinkValueOutOfRange(t *testing.T) {
	p := demoPlant(t, plantMain)
	err := p.Check(Action{Kind: ActLink, Path: linkCur, Value: int64(1) << 40})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not fit")
}
