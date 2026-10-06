package ecat

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	forceI1   = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input"
	forceO1   = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 1^Output"
	forceBase = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)"
	forceCur  = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Current"
)

// zeroDev clears its input span every Step, like a model driving inputs low.
type zeroDev struct{}

func (zeroDev) Init(*Slave) {}
func (zeroDev) Step(_ time.Duration, _ []byte, in []byte) {
	for i := range in {
		in[i] = 0
	}
}

// zeroNet loads Demo Device 1/2 with DEMO.A1.01 driven by zeroDev.
func zeroNet(t *testing.T) (*Topology, *Network) {
	t.Helper()
	topo, err := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	require.NoError(t, err)
	s := topo.Masters[0].Slaves[1]
	reg := NewRegistry()
	reg.Register(s.Vendor, s.Product, func() Device { return zeroDev{} })
	return topo, NewNetwork(topo, reg)
}

func TestForceInputOverridesModel(t *testing.T) {
	topo, n := zeroNet(t)
	require.NoError(t, n.ForceInput(forceI1, 1))
	for i := 0; i < 3; i++ {
		n.Step(time.Millisecond)
		assert.Equal(t, uint64(1), readPath(t, topo, n, forceI1))
	}
	bits, ok := n.Forced(forceI1)
	assert.True(t, ok)
	assert.Equal(t, uint64(1), bits)

	n.ReleaseInput(forceI1)
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(0), readPath(t, topo, n, forceI1))
	_, ok = n.Forced(forceI1)
	assert.False(t, ok)

	require.NoError(t, n.ForceInput(forceI1, 1))
	require.NoError(t, n.ForceInput(forceCur, 7))
	n.ReleaseAll()
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(0), readPath(t, topo, n, forceI1))
	n.ReleaseInput("TIID^nope") // unknown path is a no-op
	_, ok = n.Forced("TIID^nope")
	assert.False(t, ok)
}

func TestForceInputErrorsAndTruncation(t *testing.T) {
	topo, n := loadNet(t, nil)
	err := n.ForceInput(forceO1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), forceO1)
	err = n.ForceInput("TIID^Device 1 (EtherCAT)^Nope", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nope")

	require.NoError(t, n.ForceInput(forceI1, 0xFF))
	bits, _ := n.Forced(forceI1)
	assert.Equal(t, uint64(1), bits)

	slot, _ := topo.Slot(forceCur)
	require.NoError(t, n.ForceInput(forceCur, 1<<uint(slot.BitLen)|5))
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(5), readPath(t, topo, n, forceCur))
}

func TestForceInputBeatsPseudoInputs(t *testing.T) {
	topo, n := loadNet(t, nil)
	require.NoError(t, n.SetSlaveState("Device 1 (EtherCAT)", 1, 0x0012))
	require.NoError(t, n.ForceInput(forceBase+"^InfoData^State", 0x0008))
	require.NoError(t, n.SetWcState("Device 1 (EtherCAT)", 1, true))
	require.NoError(t, n.ForceInput(forceBase+"^WcState^WcState", 0))
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(0x0008), readPath(t, topo, n, forceBase+"^InfoData^State"))
	assert.Equal(t, uint64(0), readPath(t, topo, n, forceBase+"^WcState^WcState"))

	n.ClearFaults() // also clears forces
	_, ok := n.Forced(forceBase + "^InfoData^State")
	assert.False(t, ok)
}

func TestForceInput64BitSlot(t *testing.T) {
	topo, n := loadNet(t, nil)
	path := "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^InfoData^AdsAddr"
	slot, ok := topo.Slot(path)
	require.True(t, ok)
	require.Equal(t, 64, slot.BitLen)
	require.NoError(t, n.ForceInput(path, ^uint64(0)))
	n.Step(time.Millisecond)
	assert.Equal(t, ^uint64(0), readPath(t, topo, n, path))
}

func TestSlaveByName(t *testing.T) {
	_, n := loadNet(t, nil)
	for _, name := range []string{"DEMO.A1.01 (EL1008)", "demo.a1.01 (el1008)", "DEMO.A1.01", "demo.a1.01", " DEMO.A1.01 "} {
		m, i, err := n.SlaveByName(name)
		require.NoError(t, err, name)
		assert.Equal(t, "Device 1 (EtherCAT)", m)
		assert.Equal(t, 1, i)
	}
	m, i, err := n.SlaveByName("D2.V1")
	require.NoError(t, err)
	assert.Equal(t, "Device 2 (EtherCAT)", m)
	assert.Equal(t, 3, i)

	_, _, err = n.SlaveByName("EL9999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown slave")
	assert.Equal(t, maxCandidates-1, strings.Count(err.Error(), ", "), err.Error())

	_, _, err = n.SlaveByName("A1.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEMO.A1.00 (EK1200)")
}

func TestSlaveByNameAmbiguity(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	require.NoError(t, err)
	m1 := topo.Masters[0]
	m1.Slaves[2].Name = "DEMO.A1.01 (EL2008)"
	topo.Masters[1].Slaves[1].Name = "DEMO.A1.03 (EL9222-5500)"
	n := NewNetwork(topo, nil)

	_, i, err := n.SlaveByName("DEMO.A1.01 (EL2008)")
	require.NoError(t, err)
	assert.Equal(t, 2, i)

	_, _, err = n.SlaveByName("DEMO.A1.01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
	assert.Contains(t, err.Error(), "DEMO.A1.01 (EL1008)")

	_, _, err = n.SlaveByName("DEMO.A1.03 (EL9222-5500)")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Device 1 (EtherCAT)")
	assert.Contains(t, err.Error(), "Device 2 (EtherCAT)")

	// More than five same-master hits are capped.
	for _, s := range m1.Slaves {
		s.Name = "X (" + s.Model + ")"
	}
	_, _, err = n.SlaveByName("x")
	require.Error(t, err)
	assert.Equal(t, maxCandidates-1, strings.Count(err.Error(), ", "), err.Error())

	empty := NewNetwork(&Topology{}, nil)
	_, _, err = empty.SlaveByName("x")
	assert.Contains(t, err.Error(), "no slaves loaded")
}

func TestSlavePreset(t *testing.T) {
	topo, n := loadNet(t, nil)
	const dev = "Device 1 (EtherCAT)"
	state := func() uint64 { return readPath(t, topo, n, forceBase+"^InfoData^State") }
	wc := func() uint64 { return readPath(t, topo, n, forceBase+"^WcState^WcState") }

	require.NoError(t, n.ApplySlavePreset(dev, 1, "not_present"))
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(0x0011), state())
	assert.Equal(t, uint64(1), wc())
	_, link := n.SlaveState(dev, 1)
	assert.Equal(t, LinkNotPresent, link)

	require.NoError(t, n.ApplySlavePreset(dev, 1, "OK"))
	n.Step(time.Millisecond)
	assert.Equal(t, uint64(StateOP), state())
	assert.Equal(t, uint64(0), wc())
	st, link := n.SlaveState(dev, 1)
	assert.Equal(t, StateOP, st)
	assert.Equal(t, uint8(0), link)

	require.NoError(t, n.ApplySlavePreset(dev, 1, "link_error"))
	_, link = n.SlaveState(dev, 1)
	assert.Equal(t, LinkWithoutComm, link)

	for preset, want := range map[string]uint16{"init": StateInit, "preop": StatePreOp, "safeop": StateSafeOp, "op": StateOP} {
		require.NoError(t, n.ApplySlavePreset(dev, 1, preset))
		n.Step(time.Millisecond)
		assert.Equal(t, uint64(want), state(), preset)
	}
	err := n.ApplySlavePreset(dev, 1, "melted")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "melted")
	assert.Error(t, n.ApplySlavePreset(dev, 99, "ok"))
}
