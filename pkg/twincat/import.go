package twincat

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
)

// Options configures Import.
type Options struct {
	// Defines are preprocessor symbols; Import keeps them for callers that
	// parse the model (ParseModel takes them explicitly).
	Defines map[string]bool
}

// Import reads a TwinCAT solution (.tsproj) or a PLC project (.plcproj)
// into a Model: user sources in plcproj order, tasks, library references
// resolved in the locked order and the library sources to register before
// user code. Diagnostics are sorted by file, line and code. An error is
// returned for an unsupported extension, an unreadable tsproj/plcproj or a
// broken stc.toml.
func Import(path string, opts Options) (*Model, []diag.Diagnostic, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	m := &Model{}
	var ds []diag.Diagnostic
	var tsTasks []Task
	plcprojPath := abs

	switch strings.ToLower(filepath.Ext(abs)) {
	case ".tsproj":
		ti, tds, err := ReadTsproj(abs)
		ds = append(ds, tds...)
		if err != nil {
			return nil, ds, err
		}
		m.SolutionPath = abs
		m.PlcName, m.AmsPort, tsTasks = ti.PlcName, ti.AmsPort, ti.Tasks
		plcprojPath = ti.PlcprojPath
	case ".plcproj":
		m.PlcName = strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	default:
		return nil, nil, fmt.Errorf("%s: unsupported file type; expected .tsproj or .plcproj", path)
	}

	info, pds, err := ReadPlcproj(plcprojPath)
	ds = append(ds, pds...)
	if err != nil {
		return nil, ds, err
	}
	m.ProjectPath = info.Path
	if m.PlcName == "" {
		m.PlcName = strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
	}

	var ttos []ttoFile
	m.Sources = convertItems(info, "", &ds, func(it Item) {
		t, err := ReadTcTTO(it.AbsPath)
		if err != nil {
			ds = append(ds, readFailure(it.AbsPath, err))
			return
		}
		ttos = append(ttos, ttoFile{Path: it.AbsPath, Task: t})
	})
	tasks, tds := mergeTasks(tsTasks, ttos, info.Path)
	m.Tasks = tasks
	ds = append(ds, tds...)

	var libraryPaths map[string]string
	cfgDir := ""
	if cfgPath, err := project.FindConfig(filepath.Dir(info.Path)); err == nil {
		cfg, err := project.LoadConfig(cfgPath)
		if err != nil {
			return nil, ds, err
		}
		libraryPaths, cfgDir = cfg.Build.LibraryPaths, filepath.Dir(cfgPath)
	}
	ctx := newResolveCtx(libraryPaths, cfgDir)
	m.Libraries, m.LibrarySources = resolveLibraries(info, m.PlcName, ctx)
	ds = append(ds, ctx.diags...)

	sortDiags(ds)
	return m, ds, nil
}

// sortDiags orders diagnostics by file, line, column and code for stable output.
func sortDiags(ds []diag.Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i].Pos, ds[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return ds[i].Code < ds[j].Code
	})
}

// ParseModel parses the model's user and library sources. GVL names come
// from Source.Name (the TcGVL object name), never from the file basename.
func ParseModel(m *Model, defines map[string]bool) (user, libs []*ast.SourceFile, diags []diag.Diagnostic) {
	parse := func(src Source) *ast.SourceFile {
		r := pipeline.Parse(src.Path, src.Text, defines)
		diags = append(diags, r.Diags...)
		if r.File != nil && src.Kind == KindGVL && src.Name != "" {
			ast.SetGVLName(r.File, src.Name)
		}
		return r.File
	}
	for _, src := range m.LibrarySources {
		if f := parse(src); f != nil {
			libs = append(libs, f)
		}
	}
	for _, src := range m.Sources {
		if f := parse(src); f != nil {
			user = append(user, f)
		}
	}
	return user, libs, diags
}

// IsSiblingSource reports whether a library source came from a sibling
// TwinCAT project, that is real converted code rather than an embedded
// Beckhoff stub or a .st file from [build.library_paths].
func IsSiblingSource(s Source) bool {
	return s.Library != "" && !strings.HasPrefix(s.Path, stubDisplayDir+"/") &&
		!strings.EqualFold(filepath.Ext(s.Path), ".st")
}

// ParseForTest parses the model for `stc test --project`: sibling library
// sources (real implementations) come first in project, followed by the user
// sources; embedded stubs and library_paths files go to stubs.
func ParseForTest(m *Model, defines map[string]bool) (project, stubs []*ast.SourceFile, diags []diag.Diagnostic) {
	split := &Model{}
	for _, s := range m.LibrarySources {
		if IsSiblingSource(s) {
			split.Sources = append(split.Sources, s)
		} else {
			split.LibrarySources = append(split.LibrarySources, s)
		}
	}
	split.Sources = append(split.Sources, m.Sources...)
	project, stubs, diags = ParseModel(split, defines)
	return project, stubs, diags
}
