package main

import (
	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/projectload"
)

// loadProjectSpec turns the inputs of `stc sim` and `stc serve` into an
// interp.ProjectSpec; see projectload.Load for the rules.
func loadProjectSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error) {
	spec, _, ds, err := loadProjectAnalysis(paths, defines)
	return spec, ds, err
}

// loadProjectAnalysis is loadProjectSpec that also returns the analysis
// result (symbol table and files), which `stc serve --opcua` needs to build
// the OPC UA address space.
func loadProjectAnalysis(paths []string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	return projectload.Load(paths, defines)
}
