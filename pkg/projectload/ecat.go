package projectload

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/interp"
)

// ECat is an EtherCAT network attached to a project.
type ECat struct {
	Binder   *interp.IOBinder
	Network  *ecat.Network
	Bindings []ecat.Binding // resolved TcLinkTo links, sorted by variable path
}

// AttachECat loads the EtherCAT exports, resolves the TcLinkTo links of the
// project files against them and attaches the network to p. Resolve errors
// (unresolved links, size or direction mismatches) fail with every
// diagnostic returned; warnings are returned with a nil error.
func AttachECat(p *interp.Project, spec interp.ProjectSpec, ioFiles []string) (*ECat, []diag.Diagnostic, error) {
	topo, err := ecat.LoadProject(ioFiles...)
	if err != nil {
		return nil, nil, fmt.Errorf("--io: %w", err)
	}
	vars, ds := ecat.CollectLinksWithLibraries(spec.Files, spec.LibraryFiles)
	bindings, rds := ecat.Resolve(topo, vars)
	ds = append(ds, rds...)
	ecat.SortDiagnostics(ds)
	if n := CountErrors(ds); n > 0 {
		return nil, ds, fmt.Errorf("EtherCAT links do not resolve: %d error(s)", n)
	}
	net := ecat.NewNetwork(topo, nil)
	b := interp.NewIOBinder(bindings, net)
	p.SetIOBinder(b)
	return &ECat{Binder: b, Network: net, Bindings: bindings}, ds, nil
}

// CountErrors counts the error-severity diagnostics of ds.
func CountErrors(ds []diag.Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == diag.Error {
			n++
		}
	}
	return n
}
