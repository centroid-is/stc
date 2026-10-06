package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
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
	spec, _, ds, err := loadProjectAnalysis(paths, defines)
	return spec, ds, err
}

// loadProjectAnalysis is loadProjectSpec that also returns the analysis
// result (symbol table and files), which `stc serve --opcua` needs to build
// the OPC UA address space.
func loadProjectAnalysis(paths []string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	if len(paths) == 0 {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, errors.New("no project or source files given")
	}
	if hasProjectArg(paths) {
		if len(paths) != 1 {
			return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, errors.New("a project path (.tsproj or .plcproj) must be the only argument")
		}
		return loadTwinCATSpec(paths[0], defines)
	}
	files, err := expandSTInputs(paths)
	if err != nil {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, err
	}
	return loadSTSpec(files, defines)
}

// expandSTInputs lists the .st files named by inputs, walking directories
// in lexical order. Files are kept as given; a directory without .st files
// is an error.
func expandSTInputs(inputs []string) ([]string, error) {
	var out []string
	for _, in := range inputs {
		fi, err := os.Stat(in)
		if err != nil || !fi.IsDir() {
			out = append(out, in) // a missing file is reported when it is read
			continue
		}
		var found []string
		err = filepath.WalkDir(in, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".st") {
				found = append(found, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("no .st files in %s", in)
		}
		sort.Strings(found)
		out = append(out, found...)
	}
	return out, nil
}

func loadTwinCATSpec(path string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	m, ds, err := twincat.Import(path, twincat.Options{Defines: defines})
	if err != nil {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, ds, fmt.Errorf("importing %s: %w", path, err)
	}
	var cfg *project.Config
	if cp, err := project.FindConfig(filepath.Dir(m.ProjectPath)); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	res := analyzer.AnalyzeProject(m, cfg, defines)
	ds = append(ds, runTolerant(res.Diags)...)
	if err := firstError(ds); err != nil {
		return interp.ProjectSpec{}, res, ds, fmt.Errorf("project %s has errors: %w", path, err)
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
	return spec, res, ds, nil
}

func loadSTSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	var ds []diag.Diagnostic
	files := make([]*ast.SourceFile, 0, len(paths))
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return interp.ProjectSpec{}, analyzer.AnalysisResult{}, ds, fmt.Errorf("cannot read %s: %w", p, err)
		}
		res := pipeline.Parse(p, string(src), defines)
		ds = append(ds, res.Diags...)
		files = append(files, res.File)
	}
	var cfg *project.Config
	var libs []*ast.SourceFile
	if cp, err := project.FindConfig(filepath.Dir(paths[0])); err == nil {
		cfg, _ = project.LoadConfig(cp)
		if cfg != nil {
			// stc.toml [build.library_paths] stubs, as stc test loads them.
			if libs, err = vendor.LoadLibraries(cfg, filepath.Dir(cp)); err != nil {
				return interp.ProjectSpec{}, analyzer.AnalysisResult{}, ds, fmt.Errorf("loading libraries: %w", err)
			}
		}
	}
	res := analyzer.Analyze(files, cfg, analyzer.AnalyzeOpts{LibraryFiles: libs})
	ds = append(ds, runTolerant(res.Diags)...)
	if err := firstError(ds); err != nil {
		return interp.ProjectSpec{}, res, ds, err
	}
	return interp.ProjectSpec{Files: files, LibraryFiles: libs}, res, ds, nil
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
