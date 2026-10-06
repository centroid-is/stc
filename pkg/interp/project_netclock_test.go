package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clockDevice sums the time the network steps it by.
type clockDevice struct{ total time.Duration }

func (d *clockDevice) Init(*ecat.Slave)                   {}
func (d *clockDevice) Step(dt time.Duration, _, _ []byte) { d.total += dt }

// TestProjectNetworkTimeNonHarmonic is the ME-01 regression: with task
// cycles of 4 ms and 6 ms the base tick is 2 ms, but only 7 of 10 base
// ticks run a task. The network must still see the full elapsed virtual
// time, not one base tick per scan.
func TestProjectNetworkTimeNonHarmonic(t *testing.T) {
	f := parseRT(t, "p.st", `
PROGRAM A
VAR n : INT; END_VAR
n := n + 1;
END_PROGRAM
PROGRAM B
VAR n : INT; END_VAR
n := n + 1;
END_PROGRAM
`)
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{f}, Tasks: []TaskSpec{
		{Name: "TA", Cycle: 4 * time.Millisecond, Priority: 1, Programs: []string{"A"}},
		{Name: "TB", Cycle: 6 * time.Millisecond, Priority: 2, Programs: []string{"B"}},
	}})
	require.NoError(t, err)
	require.Equal(t, 2*time.Millisecond, p.BaseTick())

	dev := &clockDevice{}
	reg := ecat.NewRegistry()
	reg.Register(1, 2, func() ecat.Device { return dev })
	topo := &ecat.Topology{Masters: []*ecat.Master{{Name: "M", InBytes: 8, OutBytes: 8, Slaves: []*ecat.Slave{
		{Name: "S (X)", Vendor: 1, Product: 2, HasVendor: true, HasProduct: true},
	}}}}
	net := ecat.NewNetwork(topo, reg)
	require.NoError(t, p.Advance(10*time.Millisecond)) // runs before the binder is attached
	p.SetIOBinder(NewIOBinder(nil, net))

	require.NoError(t, p.Advance(120*time.Millisecond))
	// The binder was attached at 10 ms. The last tick that runs a task
	// starts at 128 ms and ends at 130 ms, so the devices have seen 120 ms.
	assert.Equal(t, 120*time.Millisecond, dev.total)
}
