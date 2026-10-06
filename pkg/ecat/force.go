package ecat

import (
	"fmt"
	"sort"
	"strings"
)

// maxCandidates caps the names listed in a slave lookup error (T-27-05).
const maxCandidates = 5

// forceEntry is one forced input slot.
type forceEntry struct {
	slot Slot
	bits uint64
}

// ForceInput pins the input slot at path to bits until ReleaseInput,
// ReleaseAll or ClearFaults. Forces are written at the end of every Step,
// after device models and pseudo-inputs, so they win over both. bits wider
// than the slot are truncated to its BitLen. Output slots and unknown paths
// are errors.
func (n *Network) ForceInput(path string, bits uint64) error {
	slot, ok := n.Topo.Slot(path)
	if !ok {
		return fmt.Errorf("ecat: unknown link path %q", path)
	}
	if slot.Dir != DirIn {
		return fmt.Errorf("ecat: %q is an output slot; only inputs can be forced", path)
	}
	if slot.BitLen < 64 {
		bits &= (uint64(1) << uint(slot.BitLen)) - 1
	}
	if n.forces == nil {
		n.forces = map[string]forceEntry{}
	}
	n.forces[slot.Path] = forceEntry{slot: slot, bits: bits}
	return nil
}

// ReleaseInput returns the slot at path to model control on the next Step.
func (n *Network) ReleaseInput(path string) {
	if slot, ok := n.Topo.Slot(path); ok {
		delete(n.forces, slot.Path)
	}
}

// ReleaseAll removes every input force.
func (n *Network) ReleaseAll() {
	n.forces = nil
}

// Forced reports the forced bits of the input slot at path.
func (n *Network) Forced(path string) (uint64, bool) {
	slot, ok := n.Topo.Slot(path)
	if !ok {
		return 0, false
	}
	f, ok := n.forces[slot.Path]
	return f.bits, ok
}

// applyForces writes every force into its master's input image in path
// order, so overlapping forces resolve deterministically.
func (n *Network) applyForces() {
	if len(n.forces) == 0 {
		return
	}
	paths := make([]string, 0, len(n.forces))
	for p := range n.forces {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		f := n.forces[p]
		if img := n.images.Get(f.slot.Master); img != nil {
			WriteBits(img.In, f.slot.Byte, f.slot.Bit, f.slot.BitLen, f.bits)
		}
	}
}

// slaveHit is one slave matched by name.
type slaveHit struct {
	master string
	index  int
	name   string
}

// SlaveByName resolves a slave name to its master and bus index (D-06):
// exact Slave.Name first, then case-insensitive, then the unique prefix
// before " (" (case-insensitive). Unknown and ambiguous names are errors
// listing up to 5 candidates; a name present on several masters is an
// error naming them.
func (n *Network) SlaveByName(name string) (string, int, error) {
	match := func(eq func(s *Slave) bool) []slaveHit {
		var hits []slaveHit
		for _, mr := range n.masters {
			for i, rt := range mr.slaves {
				if eq(rt.slave) {
					hits = append(hits, slaveHit{mr.m.Name, i, rt.slave.Name})
				}
			}
		}
		return hits
	}
	want := strings.TrimSpace(name)
	lower := strings.ToLower(want)
	tiers := []func(s *Slave) bool{
		func(s *Slave) bool { return s.Name == want },
		func(s *Slave) bool { return strings.ToLower(s.Name) == lower },
		func(s *Slave) bool { return strings.ToLower(shortName(s.Name)) == lower },
	}
	for _, eq := range tiers {
		hits := match(eq)
		switch {
		case len(hits) == 1:
			return hits[0].master, hits[0].index, nil
		case len(hits) > 1:
			return "", 0, ambiguous(name, hits)
		}
	}
	return "", 0, fmt.Errorf("ecat: unknown slave %q%s", name, n.suggest(lower))
}

// shortName is the slave name before " (", e.g. "DEMO.A1.01".
func shortName(s string) string {
	if i := strings.Index(s, " ("); i >= 0 {
		return s[:i]
	}
	return s
}

func ambiguous(name string, hits []slaveHit) error {
	masters := map[string]bool{}
	var ms []string
	for _, h := range hits {
		if !masters[h.master] {
			masters[h.master] = true
			ms = append(ms, h.master)
		}
	}
	if len(ms) > 1 {
		return fmt.Errorf("ecat: slave %q is on several masters: %s", name, strings.Join(ms, ", "))
	}
	names := make([]string, 0, maxCandidates)
	for _, h := range hits {
		if len(names) == maxCandidates {
			break
		}
		names = append(names, h.name)
	}
	return fmt.Errorf("ecat: slave name %q is ambiguous; candidates: %s", name, strings.Join(names, ", "))
}

// suggest lists up to 5 slave names containing lower, or the first 5 names.
func (n *Network) suggest(lower string) string {
	var all, near []string
	for _, mr := range n.masters {
		for _, rt := range mr.slaves {
			all = append(all, rt.slave.Name)
			if lower != "" && strings.Contains(strings.ToLower(rt.slave.Name), lower) {
				near = append(near, rt.slave.Name)
			}
		}
	}
	list := near
	if len(list) == 0 {
		list = all
	}
	if len(list) == 0 {
		return " (no slaves loaded)"
	}
	if len(list) > maxCandidates {
		list = list[:maxCandidates]
	}
	return "; candidates: " + strings.Join(list, ", ")
}

// Slave state presets accepted by ApplySlavePreset (CONTEXT D-14).
const (
	PresetNotPresent = "not_present"
	PresetLinkError  = "link_error"
	PresetInit       = "init"
	PresetPreOp      = "preop"
	PresetSafeOp     = "safeop"
	PresetOP         = "op"
	PresetOK         = "ok"
)

// stateErr is InfoData.State of a slave that dropped off: Init plus the
// 16#10 error bit.
const stateErr uint16 = 0x0011

// ApplySlavePreset applies a named slave condition (D-14):
// not_present and link_error set the link state, InfoData.State 16#0011
// and a bad working counter; init/preop/safeop/op set the state nibble;
// ok restores OP, link 0 and a good working counter.
func (n *Network) ApplySlavePreset(master string, index int, preset string) error {
	if _, err := n.slaveRef(master, index); err != nil {
		return err
	}
	var link uint8
	switch strings.ToLower(preset) {
	case PresetNotPresent:
		link = LinkNotPresent
	case PresetLinkError:
		link = LinkWithoutComm
	case PresetInit:
		return n.SetSlaveState(master, index, StateInit)
	case PresetPreOp:
		return n.SetSlaveState(master, index, StatePreOp)
	case PresetSafeOp:
		return n.SetSlaveState(master, index, StateSafeOp)
	case PresetOP:
		return n.SetSlaveState(master, index, StateOP)
	case PresetOK:
		_ = n.SetSlaveState(master, index, StateOP)
		_ = n.SetWcState(master, index, false)
		return n.SetLinkState(master, index, 0)
	default:
		return fmt.Errorf("ecat: unknown slave state %q (want not_present, link_error, init, preop, safeop, op or ok)", preset)
	}
	_ = n.SetSlaveState(master, index, stateErr)
	_ = n.SetWcState(master, index, true)
	return n.SetLinkState(master, index, link)
}
