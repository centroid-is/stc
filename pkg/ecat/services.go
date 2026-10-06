package ecat

import "fmt"

// CoEDevice is implemented by device models with a CANopen over EtherCAT
// object dictionary. The second result is the CoE abort code; 0 means ok.
type CoEDevice interface {
	SDORead(index uint16, sub uint8) ([]byte, uint32)
	SDOWrite(index uint16, sub uint8, data []byte) uint32
}

// StateDevice is implemented by device models that run their own EtherCAT
// state machine (for example a slave whose final state is PreOp).
type StateDevice interface {
	EcState() uint16
	RequestState(s uint16)
}

// CoE SDO abort codes.
const (
	AbortReadOnly    uint32 = 0x06010002 // attempt to write a read-only object
	AbortNoObject    uint32 = 0x06020000 // object does not exist
	AbortLength      uint32 = 0x06070010 // data type length does not match
	AbortNoSubIndex  uint32 = 0x06090011 // subindex does not exist
	AbortDeviceState uint32 = 0x08000022 // not possible in the present device state
)

// EtherCAT slave states (InfoData.State low nibble). StateOP is in network.go.
const (
	StateInit   uint16 = 0x0001
	StatePreOp  uint16 = 0x0002
	StateSafeOp uint16 = 0x0004
)

// Link state bits reported next to the slave state (ST_EcSlaveState.linkState).
const (
	LinkNotPresent  uint8 = 0x01
	LinkWithoutComm uint8 = 0x02
)

// DefaultStateDelay is the number of Steps a state request takes.
const DefaultStateDelay = 2

type pendingState struct {
	state uint16
	left  int
}

// services holds the master-service state: link and CRC overrides, master
// state, and state requests for devices without their own state machine.
type services struct {
	link        map[slaveRef]uint8
	crc         map[slaveRef][4]uint32
	masterState map[string]uint16
	plainState  map[slaveRef]uint16
	pending     map[slaveRef]pendingState
}

func (sv *services) clearFaults() {
	sv.link = map[slaveRef]uint8{}
	sv.crc = map[slaveRef][4]uint32{}
	sv.masterState = map[string]uint16{}
}

// slaveAddr is the EtherCAT address: PhysAddr when present, else FirstPort+Index.
func slaveAddr(s *Slave) uint16 {
	if s.HasPhys {
		return uint16(s.Phys)
	}
	return uint16(FirstPort + s.Index)
}

// effectiveState is the InfoData.State a slave reports: a SetSlaveState
// override, else the device's own state machine, else the requested state
// of a plain device (StateOP by default).
func (n *Network) effectiveState(ref slaveRef, dev Device) uint16 {
	if s, ok := n.slaveState[ref]; ok {
		return s
	}
	if sd, ok := dev.(StateDevice); ok {
		return sd.EcState()
	}
	if s, ok := n.svc.plainState[ref]; ok {
		return s
	}
	return StateOP
}

// stepPending counts down state requests of plain devices.
func (n *Network) stepPending() {
	for ref, p := range n.svc.pending {
		p.left--
		if p.left <= 0 {
			n.svc.plainState[ref] = p.state
			delete(n.svc.pending, ref)
			continue
		}
		n.svc.pending[ref] = p
	}
}

func (n *Network) slaveRT(master string, i int) (*slaveRT, slaveRef, error) {
	ref, err := n.slaveRef(master, i)
	if err != nil {
		return nil, ref, err
	}
	mr, _ := n.master(master)
	return &mr.slaves[i], ref, nil
}

// MasterNames lists the masters in topology order.
func (n *Network) MasterNames() []string {
	names := make([]string, 0, len(n.masters))
	for _, mr := range n.masters {
		names = append(names, mr.m.Name)
	}
	return names
}

// MasterByNetID returns the master whose AmsNetId is id. An all-zero or
// unknown id is an error; callers decide whether to fall back.
func (n *Network) MasterByNetID(id [6]byte) (string, error) {
	if id != ([6]byte{}) {
		for _, mr := range n.masters {
			if mr.m.NetID == id {
				return mr.m.Name, nil
			}
		}
	}
	return "", fmt.Errorf("ecat: no master with AmsNetId %v", id)
}

// SlaveCount returns the number of slaves on master (0 when unknown).
func (n *Network) SlaveCount(master string) int {
	mr, err := n.master(master)
	if err != nil {
		return 0
	}
	return len(mr.slaves)
}

// SlaveAddr returns the EtherCAT address of slave i (0 when unknown).
func (n *Network) SlaveAddr(master string, i int) uint16 {
	rt, _, err := n.slaveRT(master, i)
	if err != nil {
		return 0
	}
	return slaveAddr(rt.slave)
}

// Slave finds a slave by EtherCAT address and returns its bus index and model.
func (n *Network) Slave(master string, addr uint16) (int, Device, error) {
	mr, err := n.master(master)
	if err != nil {
		return -1, nil, err
	}
	for i, rt := range mr.slaves {
		if slaveAddr(rt.slave) == addr {
			return i, rt.dev, nil
		}
	}
	return -1, nil, fmt.Errorf("ecat: master %q has no slave at address %d", master, addr)
}

// SlaveState returns the effective EtherCAT state and link state of slave i
// ((0, 0) when unknown).
func (n *Network) SlaveState(master string, i int) (uint16, uint8) {
	rt, ref, err := n.slaveRT(master, i)
	if err != nil {
		return 0, 0
	}
	return n.effectiveState(ref, rt.dev), n.svc.link[ref]
}

// RequestSlaveState asks slave i to change EtherCAT state. StateDevices
// handle the request themselves; plain devices reach s after StateDelay Steps.
func (n *Network) RequestSlaveState(master string, i int, s uint16) error {
	rt, ref, err := n.slaveRT(master, i)
	if err != nil {
		return err
	}
	if sd, ok := rt.dev.(StateDevice); ok {
		sd.RequestState(s)
		return nil
	}
	if n.StateDelay <= 0 {
		n.svc.plainState[ref] = s
		delete(n.svc.pending, ref)
		return nil
	}
	n.svc.pending[ref] = pendingState{state: s, left: n.StateDelay}
	return nil
}

// SetLinkState overrides the link state of slave i until ClearFaults.
func (n *Network) SetLinkState(master string, i int, link uint8) error {
	_, ref, err := n.slaveRT(master, i)
	if err != nil {
		return err
	}
	n.svc.link[ref] = link
	return nil
}

// CrcErrors returns the CRC error counters of ports A..D of slave i.
func (n *Network) CrcErrors(master string, i int) [4]uint32 {
	_, ref, err := n.slaveRT(master, i)
	if err != nil {
		return [4]uint32{}
	}
	return n.svc.crc[ref]
}

// SetCrcErrors sets the CRC error counters of slave i until cleared.
func (n *Network) SetCrcErrors(master string, i int, c [4]uint32) error {
	_, ref, err := n.slaveRT(master, i)
	if err != nil {
		return err
	}
	n.svc.crc[ref] = c
	return nil
}

// ClearCrc resets the CRC error counters of slave i (physical write to 0x0300).
func (n *Network) ClearCrc(master string, i int) error {
	_, ref, err := n.slaveRT(master, i)
	if err != nil {
		return err
	}
	delete(n.svc.crc, ref)
	return nil
}

// MasterState returns the master's EtherCAT state: StateOP unless overridden,
// 0 for an unknown master.
func (n *Network) MasterState(master string) uint16 {
	if _, err := n.master(master); err != nil {
		return 0
	}
	if s, ok := n.svc.masterState[master]; ok {
		return s
	}
	return StateOP
}

// SetMasterState overrides the master's EtherCAT state until ClearFaults.
func (n *Network) SetMasterState(master string, s uint16) error {
	if _, err := n.master(master); err != nil {
		return err
	}
	n.svc.masterState[master] = s
	return nil
}
