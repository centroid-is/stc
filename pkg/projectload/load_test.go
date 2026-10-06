package projectload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	demoTsproj    = "../twincat/testdata/sln/Demo/Demo solution.tsproj"
	inlinePlcproj = "../twincat/testdata/inline/Inline/Inline.plcproj"
	brokenPlcproj = "../twincat/testdata/broken/Broken.plcproj"
)

func write(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(src), 0o644))
	return p
}

func TestIsProjectPath(t *testing.T) {
	assert.True(t, IsProjectPath("a/B.TSPROJ"))
	assert.True(t, IsProjectPath("x.plcproj"))
	assert.False(t, IsProjectPath("main.st"))
	assert.True(t, HasProjectArg([]string{"a.st", "b.tsproj"}))
	assert.False(t, HasProjectArg([]string{"a.st"}))
	assert.False(t, HasProjectArg(nil))
}

func TestLoadTwinCAT(t *testing.T) {
	spec, res, ds, err := Load([]string{demoTsproj}, nil)
	require.NoError(t, err)
	m, _, err := twincat.Import(demoTsproj, twincat.Options{})
	require.NoError(t, err)
	var want []string
	for _, mt := range m.Tasks {
		if len(mt.Programs) > 0 {
			want = append(want, mt.Name)
		}
	}
	var got []string
	for _, ts := range spec.Tasks {
		got = append(got, ts.Name)
		assert.NotEmpty(t, ts.Programs)
	}
	assert.Equal(t, want, got)
	assert.NotEmpty(t, spec.Files)
	assert.NotEmpty(t, res.Files)
	assert.Zero(t, CountErrors(ds))
	p, err := interp.LoadProject(spec)
	require.NoError(t, err)
	require.NoError(t, p.Tick())

	// A .plcproj without tasks yields none, so LoadProject uses MAIN.
	spec, ds, err = loadSpec(inlinePlcproj)
	require.NoError(t, err)
	assert.Empty(t, spec.Tasks)
	assert.True(t, hasCode(ds, twincat.CodeDefaultCycle))

	_, _, _, err = Load([]string{brokenPlcproj}, nil)
	assert.ErrorContains(t, err, "has errors")
	_, _, _, err = Load([]string{filepath.Join(t.TempDir(), "missing.tsproj")}, nil)
	assert.ErrorContains(t, err, "importing")
}

func loadSpec(path string) (interp.ProjectSpec, []diag.Diagnostic, error) {
	spec, _, ds, err := Load([]string{path}, nil)
	return spec, ds, err
}

func hasCode(ds []diag.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestLoadUsageErrors(t *testing.T) {
	_, _, _, err := Load(nil, nil)
	assert.ErrorContains(t, err, "no project or source files")
	_, _, _, err = Load([]string{demoTsproj, "main.st"}, nil)
	assert.ErrorContains(t, err, "must be the only argument")
	dir := t.TempDir()
	_, _, _, err = Load([]string{filepath.Join(dir, "missing.st")}, nil)
	assert.ErrorContains(t, err, "cannot read")
	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.Mkdir(empty, 0o755))
	_, _, _, err = Load([]string{empty}, nil)
	assert.ErrorContains(t, err, "no .st files in")
}

func TestLoadSTFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "src/b/main.st", "PROGRAM MAIN\nGVL.count := GVL.count + 1;\nEND_PROGRAM\n")
	write(t, dir, "src/a/GVL.ST", "VAR_GLOBAL\n\tcount : INT;\nEND_VAR\n")
	write(t, dir, "src/notes.txt", "not ST")

	files, err := ExpandSTInputs([]string{filepath.Join(dir, "src"), "kept.st"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join(dir, "src", "a", "GVL.ST"),
		filepath.Join(dir, "src", "b", "main.st"),
		"kept.st",
	}, files)

	spec, res, _, err := Load([]string{filepath.Join(dir, "src")}, map[string]bool{"STC_SIM": true})
	require.NoError(t, err)
	assert.Len(t, spec.Files, 2)
	assert.Empty(t, spec.Tasks)
	assert.Len(t, res.Files, 2)
	p, err := interp.LoadProject(spec)
	require.NoError(t, err)
	require.NoError(t, p.Tick())
	v, err := p.Runtime().Get("GVL.count")
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.Int)

	bad := write(t, dir, "bad.st", "PROGRAM MAIN\nVAR x : INT; END_VAR\nx := nope;\nEND_PROGRAM\n")
	_, _, ds, err := Load([]string{bad}, nil)
	assert.ErrorContains(t, err, "bad.st:3:")
	assert.Positive(t, CountErrors(ds))
}

// Undeclared types and members are downgraded to warnings; other errors
// stay errors.
func TestRunTolerant(t *testing.T) {
	ds := RunTolerant([]diag.Diagnostic{
		{Severity: diag.Error, Code: checker.CodeUndeclaredType},
		{Severity: diag.Error, Code: checker.CodeNoMember},
		{Severity: diag.Info, Code: checker.CodeNoMember},
		{Severity: diag.Error, Code: "OTHER"},
	})
	assert.Equal(t, diag.Warning, ds[0].Severity)
	assert.Equal(t, diag.Warning, ds[1].Severity)
	assert.Equal(t, diag.Info, ds[2].Severity)
	assert.Equal(t, diag.Error, ds[3].Severity)
	assert.Equal(t, 1, CountErrors(ds))
	assert.Error(t, FirstError(ds))
	assert.NoError(t, FirstError(ds[:3]))
	assert.Zero(t, CountErrors(nil))

	dir := t.TempDir()
	p := write(t, dir, "main.st", "PROGRAM MAIN\nVAR conv : FB_Missing; END_VAR\nconv();\nEND_PROGRAM\n")
	_, _, ds, err := Load([]string{p}, nil)
	require.NoError(t, err)
	assert.True(t, hasCode(ds, checker.CodeUndeclaredType))
}

// stc.toml [build.library_paths] stubs are loaded with the sources.
func TestLoadLibraryPaths(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "stc.toml", "[project]\nname = \"x\"\n\n[build.library_paths]\nvend = \"lib\"\n")
	write(t, dir, "lib/fb.st", "FUNCTION_BLOCK FB_Vendor\nVAR_OUTPUT\n\tbDone : BOOL;\nEND_VAR\nEND_FUNCTION_BLOCK\n")
	mainSt := write(t, dir, "main.st", "PROGRAM MAIN\nVAR f : FB_Vendor; d : BOOL; END_VAR\nf();\nd := f.bDone;\nEND_PROGRAM\n")
	spec, _, ds, err := Load([]string{mainSt}, nil)
	require.NoError(t, err)
	assert.Len(t, spec.LibraryFiles, 1)
	assert.False(t, hasCode(ds, checker.CodeUndeclaredType))

	write(t, dir, "stc.toml", "[project]\nname = \"x\"\n\n[build.library_paths]\nvend = \"missing\"\n")
	_, _, _, err = Load([]string{mainSt}, nil)
	assert.ErrorContains(t, err, "loading libraries")
}
