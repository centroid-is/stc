package scenario

import (
	"fmt"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/interp"
)

// DefaultBaseTick is the scan period of a Plant whose sources set none
// (the TwinCAT default PlcTask cycle).
const DefaultBaseTick = 10 * time.Millisecond

// ProjectSources is the parsed project a Plant runs: library files
// (registered first), the project files and the scan period. It stands in
// for the Phase 23 interp.ProjectSpec until that lands.
type ProjectSources struct {
	LibraryFiles []*ast.SourceFile
	Files        []*ast.SourceFile
	BaseTick     time.Duration // 0 means DefaultBaseTick
}

// PlantSpec is everything needed to build fresh, independent Plants: the
// sources, the EtherCAT topology (nil without --io) and the resolved
// TcLinkTo bindings with their warnings.
type PlantSpec struct {
	Sources     ProjectSources
	Topology    *ecat.Topology
	Bindings    []ecat.Binding
	Diagnostics []diag.Diagnostic
}

// BuildPlantSpec loads the EtherCAT exports in ioFiles, collects the
// TcLinkTo links of every source file and resolves them. Warnings are kept
// in Diagnostics; any error diagnostic makes BuildPlantSpec fail (the
// spec is still returned with every diagnostic). No ioFiles means no
// network.
func BuildPlantSpec(src ProjectSources, ioFiles []string) (PlantSpec, error) {
	spec := PlantSpec{Sources: src}
	if len(ioFiles) == 0 {
		return spec, nil
	}
	topo, err := ecat.LoadProject(ioFiles...)
	if err != nil {
		return spec, err
	}
	spec.Topology = topo
	all := spec.allFiles()
	vars, cds := ecat.CollectLinks(all)
	bindings, rds := ecat.Resolve(topo, vars)
	spec.Bindings = bindings
	spec.Diagnostics = append(append(spec.Diagnostics, cds...), rds...)
	ecat.SortDiagnostics(spec.Diagnostics)
	var msgs []string
	for _, d := range spec.Diagnostics {
		if d.Severity == diag.Error {
			msgs = append(msgs, fmt.Sprintf("%s: %s", d.Code, d.Message))
		}
	}
	if len(msgs) > 0 {
		return spec, fmt.Errorf("TcLinkTo resolution failed: %s", strings.Join(msgs, "; "))
	}
	return spec, nil
}

func (s PlantSpec) allFiles() []*ast.SourceFile {
	all := make([]*ast.SourceFile, 0, len(s.Sources.LibraryFiles)+len(s.Sources.Files))
	all = append(all, s.Sources.LibraryFiles...)
	return append(all, s.Sources.Files...)
}

// Plant is a simulated plant: a Runtime over the project sources, and when
// a topology is loaded an EtherCAT network with the Tc2_EtherCAT mocks and
// an IOBinder. It implements Target; it is not safe for concurrent use.
type Plant struct {
	rt     *interp.Runtime
	net    *ecat.Network // nil without --io
	binder *interp.IOBinder
	base   time.Duration
	clock  time.Duration
	// atIn holds the upper-case ROOT.VAR paths declared with an explicit
	// AT %I address (not %I*); the AT sync overwrites them (D-13).
	atIn map[string]bool
}

// New builds a fresh Plant. Each call creates its own Runtime, Network and
// IOBinder, so Plants never share state.
func (s PlantSpec) New() (*Plant, error) {
	p := &Plant{base: s.Sources.BaseTick, atIn: atInputs(s.allFiles())}
	if p.base <= 0 {
		p.base = DefaultBaseTick
	}
	opts := interp.RuntimeOpts{LibraryFiles: s.Sources.LibraryFiles}
	if s.Topology != nil {
		p.net = ecat.NewNetwork(s.Topology, nil)
		opts.Network = p.net
	}
	rt, err := interp.NewRuntime(s.Sources.Files, opts)
	if err != nil {
		return nil, err
	}
	p.rt = rt
	if p.net != nil {
		p.binder = interp.NewIOBinder(s.Bindings, p.net)
		rt.SetIOBinder(p.binder)
	}
	return p, nil
}

// atInputs collects GVL and PROGRAM variables with an explicit %I address.
func atInputs(files []*ast.SourceFile) map[string]bool {
	out := map[string]bool{}
	add := func(root string, blocks []*ast.VarBlock) {
		for _, vb := range blocks {
			if vb == nil {
				continue
			}
			for _, vd := range vb.Declarations {
				if vd == nil || vd.AtAddress == nil {
					continue
				}
				a := strings.ToUpper(strings.TrimSpace(vd.AtAddress.Name))
				if !strings.HasPrefix(a, "%I") || strings.HasSuffix(a, "*") {
					continue
				}
				for _, n := range vd.Names {
					out[strings.ToUpper(root+"."+n.Name)] = true
				}
			}
		}
	}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.GVLDecl:
				if d.Name != nil {
					add(d.Name.Name, d.Blocks)
				}
			case *ast.ProgramDecl:
				if d.Name != nil {
					add(d.Name.Name, d.VarBlocks)
				}
			}
		}
	}
	return out
}

// Runtime returns the plant's Runtime (Get, Set, Snapshot, ToJSON).
func (p *Plant) Runtime() *interp.Runtime { return p.rt }

// Network returns the plant's EtherCAT network, nil without --io.
func (p *Plant) Network() *ecat.Network { return p.net }

// IOBinder returns the plant's TcLinkTo binder, nil without --io.
func (p *Plant) IOBinder() *interp.IOBinder { return p.binder }

// BaseTick is the scan period of one Tick.
func (p *Plant) BaseTick() time.Duration { return p.base }

// Clock is the simulated time before the next Tick.
func (p *Plant) Clock() time.Duration { return p.clock }

// Tick runs one scan of every PROGRAM and advances the clock by BaseTick.
func (p *Plant) Tick() error {
	if err := p.rt.Tick(p.base); err != nil {
		return err
	}
	p.clock += p.base
	return nil
}
