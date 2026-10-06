package main

import (
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/projectload"
)

// loadProjectSpec turns the inputs of `stc sim` and `stc serve` into an
// interp.ProjectSpec; see projectload.Load for the rules.
func loadProjectSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error) {
	spec, _, ds, err := projectload.Load(paths, defines)
	return spec, ds, err
}
