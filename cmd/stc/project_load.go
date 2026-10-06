package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/twincat"
)

// loadProjectSpec turns the inputs of `stc sim` and `stc serve` into an
// interp.ProjectSpec. A single .tsproj or .plcproj is imported and analysed
// like `stc check` does; its tasks become TaskSpecs (tasks without programs,
// such as the VEND024 default PlcTask, are dropped so LoadProject applies its
// own 10 ms MAIN default). Any other paths are parsed and analysed together
// as .st files and yield no tasks. All import, parse and analysis
// diagnostics are returned; error-severity ones also fail the load with the
// first error's file:line:col. Undeclared types and members (library drift
// such as an FB_TwoWayConveyor the project never declares) are downgraded to
// warnings by runTolerant: the runtime runs them as zero-output auto-stubs
// and zero-valued member reads, each reported as a warning.
func loadProjectSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error) {
	if len(paths) == 0 {
		return interp.ProjectSpec{}, nil, errors.New("no project or source files given")
	}
	if hasProjectArg(paths) {
		if len(paths) != 1 {
			return interp.ProjectSpec{}, nil, errors.New("a project path (.tsproj or .plcproj) must be the only argument")
		}
		return loadTwinCATSpec(paths[0], defines)
	}
	return loadSTSpec(paths, defines)
}

func loadTwinCATSpec(path string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error) {
	m, ds, err := twincat.Import(path, twincat.Options{Defines: defines})
	if err != nil {
		return interp.ProjectSpec{}, ds, fmt.Errorf("importing %s: %w", path, err)
	}
	var cfg *project.Config
	if cp, err := project.FindConfig(filepath.Dir(m.ProjectPath)); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	res := analyzer.AnalyzeProject(m, cfg, defines)
	ds = append(ds, runTolerant(res.Diags)...)
	if err := firstError(ds); err != nil {
		return interp.ProjectSpec{}, ds, fmt.Errorf("project %s has errors: %w", path, err)
	}
	spec := interp.ProjectSpec{Files: res.Files, LibraryFiles: res.LibraryFiles}
	for _, t := range m.Tasks {
		if len(t.Programs) == 0 {
			continue
		}
		spec.Tasks = append(spec.Tasks, interp.TaskSpec{
			Name:     t.Name,
			Cycle:    t.CycleTime,
			Priority: t.Priority,
			Programs: append([]string(nil), t.Programs...),
		})
	}
	return spec, ds, nil
}

func loadSTSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error) {
	var ds []diag.Diagnostic
	files := make([]*ast.SourceFile, 0, len(paths))
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return interp.ProjectSpec{}, ds, fmt.Errorf("cannot read %s: %w", p, err)
		}
		res := pipeline.Parse(p, string(src), defines)
		ds = append(ds, res.Diags...)
		files = append(files, res.File)
	}
	var cfg *project.Config
	if cp, err := project.FindConfig(filepath.Dir(paths[0])); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	res := analyzer.Analyze(files, cfg)
	ds = append(ds, runTolerant(res.Diags)...)
	if err := firstError(ds); err != nil {
		return interp.ProjectSpec{}, ds, err
	}
	return interp.ProjectSpec{Files: files}, ds, nil
}

// runTolerant downgrades the analysis errors the runtime tolerates to
// warnings: undeclared types (auto-stubbed) and undeclared members (read as
// zero). The diagnostics are modified in place and returned.
func runTolerant(ds []diag.Diagnostic) []diag.Diagnostic {
	for i := range ds {
		switch ds[i].Code {
		case checker.CodeUndeclaredType, checker.CodeNoMember:
			if ds[i].Severity == diag.Error {
				ds[i].Severity = diag.Warning
			}
		}
	}
	return ds
}

// firstError returns the first error-severity diagnostic as an error, or nil.
func firstError(ds []diag.Diagnostic) error {
	for _, d := range ds {
		if d.Severity == diag.Error {
			return errors.New(d.String())
		}
	}
	return nil
}
