package ecat

import (
	"fmt"
	"time"
)

// Device simulates one slave's process data. Step receives views over the
// byte span of the slave's own PDO entries: out is master-to-slave (read it),
// in is slave-to-master (write it). Either may be empty.
type Device interface {
	Init(s *Slave)
	Step(dt time.Duration, out []byte, in []byte)
}

// Passthrough is the default device: it never touches the image, so inputs
// keep whatever a test or later model wrote (zero otherwise).
type Passthrough struct{}

// Init does nothing.
func (Passthrough) Init(*Slave) {}

// Step does nothing.
func (Passthrough) Step(time.Duration, []byte, []byte) {}

type deviceKey struct{ vendor, product uint32 }

// Registry maps (VendorId, ProductCode) to device model factories.
type Registry struct {
	factories map[deviceKey]func() Device
}

// NewRegistry returns an empty registry; unknown slaves get Passthrough.
func NewRegistry() *Registry {
	return &Registry{factories: map[deviceKey]func() Device{}}
}

// DefaultRegistry is used by NewNetwork when no registry is given. Device
// model packages register into it.
var DefaultRegistry = NewRegistry()

// Register installs a factory for a (vendor, product) identity, replacing any
// previous one.
func (r *Registry) Register(vendor, product uint32, factory func() Device) {
	r.factories[deviceKey{vendor, product}] = factory
}

// New creates the device model for s, falling back to Passthrough.
func (r *Registry) New(s *Slave) Device {
	if f, ok := r.factories[deviceKey{s.Vendor, s.Product}]; ok {
		return f()
	}
	return Passthrough{}
}

// Healthy pseudo-input values (Beckhoff semantics).
const (
	StateOP   uint16 = 0x0008 // InfoData.State: slave in OP
	FirstPort        = 1001   // EtherCAT address of the first slave without PhysAddr
)

type span struct{ lo, hi int } // byte range [lo, hi); lo == hi when empty

type slaveRT struct {
	slave   *Slave
	dev     Device
	in, out span
}

type masterRT struct {
	m      *Master
	img    *Image
	slaves []slaveRT
}

type slaveRef struct {
	master string
	slave  int
}

// Network is the simulated EtherCAT side of a project: one process image per
// master, a device model per slave, Beckhoff pseudo-inputs and a fault API.
type Network struct {
	Topo    *Topology
	images  *Images
	masters []*masterRT

	slaveState map[slaveRef]uint16
	wcBad      map[slaveRef]bool
	devState   map[string]uint16
	svc        services

	// StateDelay is the number of Steps a state request on a plain device
	// takes (DefaultStateDelay unless changed).
	StateDelay int
}

// NewNetwork allocates images for topo and creates a device for every slave
// from reg (DefaultRegistry when nil).
func NewNetwork(topo *Topology, reg *Registry) *Network {
	if reg == nil {
		reg = DefaultRegistry
	}
	n := &Network{Topo: topo, images: NewImages(topo), StateDelay: DefaultStateDelay}
	n.svc.plainState = map[slaveRef]uint16{}
	n.svc.pending = map[slaveRef]pendingState{}
	n.ClearFaults()
	for _, m := range topo.Masters {
		mr := &masterRT{m: m, img: n.images.Get(m.Name)}
		for _, s := range m.Slaves {
			rt := slaveRT{slave: s, dev: reg.New(s)}
			rt.in, rt.out = slaveSpans(m, s)
			rt.dev.Init(s)
			bindLayout(m, rt)
			mr.slaves = append(mr.slaves, rt)
		}
		n.masters = append(n.masters, mr)
	}
	return n
}

// slaveSpans returns the byte spans covering s's PDO entry slots per direction.
func slaveSpans(m *Master, s *Slave) (in, out span) {
	sp := [2]span{{-1, -1}, {-1, -1}}
	for _, p := range s.Pdos {
		for _, e := range p.Entries {
			if e.Padding() {
				continue
			}
			slot, ok := m.Slot(LinkPath(m.Name, s, p, e))
			if !ok {
				continue
			}
			lo, hi := slot.Byte, (slot.Byte*8+slot.Bit+slot.BitLen+7)/8
			cur := &sp[slot.Dir]
			if cur.lo < 0 || lo < cur.lo {
				cur.lo = lo
			}
			if hi > cur.hi {
				cur.hi = hi
			}
		}
	}
	for i := range sp {
		if sp[i].lo < 0 {
			sp[i] = span{}
		}
	}
	return sp[DirIn], sp[DirOut]
}

func view(buf []byte, s span) []byte {
	if s.hi > len(buf) || s.lo >= s.hi {
		return buf[:0]
	}
	return buf[s.lo:s.hi:s.hi]
}

// Images returns the process images shared with the interpreter binder.
func (n *Network) Images() *Images { return n.images }

func (n *Network) master(name string) (*masterRT, error) {
	for _, mr := range n.masters {
		if mr.m.Name == name {
			return mr, nil
		}
	}
	return nil, fmt.Errorf("ecat: unknown master %q", name)
}

func (n *Network) slaveRef(master string, slave int) (slaveRef, error) {
	mr, err := n.master(master)
	if err != nil {
		return slaveRef{}, err
	}
	if slave < 0 || slave >= len(mr.slaves) {
		return slaveRef{}, fmt.Errorf("ecat: master %q has no slave %d", master, slave)
	}
	return slaveRef{master, slave}, nil
}

// SetMasterNetID sets the AmsNetId a master reports; visible at the next Step.
func (n *Network) SetMasterNetID(master string, id [6]byte) error {
	mr, err := n.master(master)
	if err != nil {
		return err
	}
	mr.m.NetID = id
	return nil
}

// SetSlaveState overrides InfoData.State of slave (bus index) until ClearFaults.
func (n *Network) SetSlaveState(master string, slave int, state uint16) error {
	ref, err := n.slaveRef(master, slave)
	if err != nil {
		return err
	}
	n.slaveState[ref] = state
	return nil
}

// SetWcState marks a slave's working counter invalid (WcState = 1) or valid.
func (n *Network) SetWcState(master string, slave int, bad bool) error {
	ref, err := n.slaveRef(master, slave)
	if err != nil {
		return err
	}
	n.wcBad[ref] = bad
	return nil
}

// SetDevState overrides the master's DevState flags until ClearFaults.
func (n *Network) SetDevState(master string, bits uint16) error {
	if _, err := n.master(master); err != nil {
		return err
	}
	n.devState[master] = bits
	return nil
}

// ClearFaults restores healthy pseudo-input values, link state, CRC counters
// and master state. Requested slave states are kept.
func (n *Network) ClearFaults() {
	n.slaveState = map[slaveRef]uint16{}
	n.wcBad = map[slaveRef]bool{}
	n.devState = map[string]uint16{}
	n.svc.clearFaults()
}

func netIDBits(id [6]byte) uint64 {
	var v uint64
	for i, b := range id {
		v |= uint64(b) << (8 * uint(i))
	}
	return v
}

// put writes v to the slot at path in the master's input image, if it exists.
func (mr *masterRT) put(path string, v uint64) {
	if s, ok := mr.m.Slot(path); ok && s.Dir == DirIn {
		WriteBits(mr.img.In, s.Byte, s.Bit, s.BitLen, v)
	}
}

// Step advances every device by dt, then publishes pseudo-inputs.
func (n *Network) Step(dt time.Duration) {
	n.stepPending()
	for _, mr := range n.masters {
		for _, rt := range mr.slaves {
			rt.dev.Step(dt, view(mr.img.Out, rt.out), view(mr.img.In, rt.in))
		}
		netID := netIDBits(mr.m.NetID)
		for i, rt := range mr.slaves {
			ref := slaveRef{mr.m.Name, i}
			base := SlaveBasePath(mr.m.Name, rt.slave)
			state := n.effectiveState(ref, rt.dev)
			wc := uint64(0)
			if n.wcBad[ref] {
				wc = 1
			}
			port := uint64(slaveAddr(rt.slave))
			mr.put(base+"^InfoData^State", uint64(state))
			mr.put(base+"^WcState^WcState", wc)
			mr.put(base+"^InfoData^AdsAddr", netID|port<<48)
		}
		mb := "TIID^" + mr.m.Name + "^"
		mr.put(mb+"Inputs^DevState", uint64(n.devState[mr.m.Name]))
		mr.put(mb+"Inputs^SlaveCount", uint64(len(mr.slaves)))
		mr.put(mb+"Inputs^Frm0State", 0)
		mr.put(mb+"Inputs^Frm0WcState", 0)
		mr.put(mb+"InfoData^AmsNetId", netID)
		mr.put(mb+"InfoData^ChangeCount", 0)
	}
}
