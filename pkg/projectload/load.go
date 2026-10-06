// Package projectload loads a TwinCAT project or ST sources into an
// interp.ProjectSpec and attaches the EtherCAT network of --io exports. It
// is shared by cmd/stc (`stc sim`, `stc serve`) and cmd/stc-mcp.
package projectload

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
)

// IsProjectPath reports whether p names a TwinCAT project (.tsproj or
// .plcproj).
func IsProjectPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".tsproj", ".plcproj":
		return true
	}
	return false
}

// HasProjectArg reports whether any path is a TwinCAT project path.
func HasProjectArg(paths []string) bool {
	for _, a := range paths {
		if IsProjectPath(a) {
			return true
		}
	}
	return false
}

// Load turns the inputs of `stc sim`, `stc serve` and `stc-mcp --project`
// into an interp.ProjectSpec plus the analysis result (symbol table and
// files) the OPC UA address space is built from. A single .tsproj or
// .plcproj is imported and analysed like `stc check` does; its tasks become
// TaskSpecs (tasks without programs, such as the VEND024 default PlcTask,
// are dropped so LoadProject applies its own 10 ms MAIN default). Any other
// paths are parsed and analysed together as .st files (directories are
// walked in lexical order) and yield no tasks. All import, parse and
// analysis diagnostics are returned; error-severity ones also fail the load
// with the first error's file:line:col. Undeclared types and members are
// downgraded to warnings (RunTolerant): the runtime runs them as
// zero-output auto-stubs and zero-valued member reads.
func Load(paths []string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	if len(paths) == 0 {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, errors.New("no project or source files given")
	}
	if HasProjectArg(paths) {
		if len(paths) != 1 {
			return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, errors.New("a project path (.tsproj or .plcproj) must be the only argument")
		}
		return loadTwinCAT(paths[0], defines)
	}
	files, err := ExpandSTInputs(paths)
	if err != nil {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, nil, err
	}
	return loadST(files, defines)
}

// ExpandSTInputs lists the .st files named by inputs, walking directories
// in lexical order. Files are kept as given; a directory without .st files
// is an error.
func ExpandSTInputs(inputs []string) ([]string, error) {
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

func loadTwinCAT(path string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
	m, ds, err := twincat.Import(path, twincat.Options{Defines: defines})
	if err != nil {
		return interp.ProjectSpec{}, analyzer.AnalysisResult{}, ds, fmt.Errorf("importing %s: %w", path, err)
	}
	var cfg *project.Config
	if cp, err := project.FindConfig(filepath.Dir(m.ProjectPath)); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	res := analyzer.AnalyzeProject(m, cfg, defines)
	ds = append(ds, RunTolerant(res.Diags)...)
	if err := FirstError(ds); err != nil {
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

func loadST(paths []string, defines map[string]bool) (interp.ProjectSpec, analyzer.AnalysisResult, []diag.Diagnostic, error) {
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
	if cp, err := project.FindConfig(filepath.Dir(paths[0])); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	res := analyzer.Analyze(files, cfg)
	ds = append(ds, RunTolerant(res.Diags)...)
	if err := FirstError(ds); err != nil {
		return interp.ProjectSpec{}, res, ds, err
	}
	return interp.ProjectSpec{Files: files}, res, ds, nil
}

// RunTolerant downgrades the analysis errors the runtime tolerates to
// warnings: undeclared types (auto-stubbed) and undeclared members (read as
// zero). The diagnostics are modified in place and returned.
func RunTolerant(ds []diag.Diagnostic) []diag.Diagnostic {
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

// FirstError returns the first error-severity diagnostic as an error, or nil.
func FirstError(ds []diag.Diagnostic) error {
	for _, d := range ds {
		if d.Severity == diag.Error {
			return errors.New(d.String())
		}
	}
	return nil
}
