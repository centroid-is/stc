package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const inlinePlcproj = "../../pkg/twincat/testdata/inline/Inline/Inline.plcproj"

func TestLoadProjectSpec(t *testing.T) {
	t.Run("tsproj tasks and libraries", func(t *testing.T) {
		spec, _, err := loadProjectSpec([]string{demoTsproj}, nil)
		require.NoError(t, err)
		m, _, err := twincat.Import(demoTsproj, twincat.Options{})
		require.NoError(t, err)
		require.NotEmpty(t, m.Tasks)
		require.Len(t, spec.Tasks, len(m.Tasks))
		for i, mt := range m.Tasks {
			assert.Equal(t, mt.Name, spec.Tasks[i].Name)
			assert.Equal(t, mt.CycleTime, spec.Tasks[i].Cycle)
			assert.Equal(t, mt.Priority, spec.Tasks[i].Priority)
			assert.Equal(t, mt.Programs, spec.Tasks[i].Programs)
		}
		assert.Len(t, spec.LibraryFiles, len(m.LibrarySources))
		assert.NotEmpty(t, spec.Files)

		p, err := interp.LoadProject(spec)
		require.NoError(t, err)
		for i := 0; i < 10; i++ {
			require.NoError(t, p.Tick())
		}
		assert.Equal(t, 10*p.BaseTick(), p.Clock())
	})

	t.Run("plcproj without tasks uses the MAIN default", func(t *testing.T) {
		spec, ds, err := loadProjectSpec([]string{inlinePlcproj}, nil)
		require.NoError(t, err)
		assert.Empty(t, spec.Tasks)
		var codes []string
		for _, d := range ds {
			codes = append(codes, d.Code)
		}
		assert.Contains(t, codes, twincat.CodeDefaultCycle)
	})

	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(src), 0o644))
		return p
	}
	gvl := write("GVL.st", "VAR_GLOBAL\n\tcount : INT;\nEND_VAR\n")
	mainSt := write("main.st", "PROGRAM MAIN\nGVL.count := GVL.count + 1;\nEND_PROGRAM\n")

	t.Run(".st files", func(t *testing.T) {
		spec, _, err := loadProjectSpec([]string{gvl, mainSt}, nil)
		require.NoError(t, err)
		assert.Empty(t, spec.Tasks)
		assert.Len(t, spec.Files, 2)
		p, err := interp.LoadProject(spec)
		require.NoError(t, err)
		require.NoError(t, p.Advance(30*time.Millisecond))
		v, err := p.Runtime().Get("GVL.count")
		require.NoError(t, err)
		assert.Equal(t, int64(3), v.Int)
	})

	t.Run("analysis error names file:line:col", func(t *testing.T) {
		bad := write("bad.st", "PROGRAM MAIN\nVAR x : INT; END_VAR\nx := nope;\nEND_PROGRAM\n")
		_, _, err := loadProjectSpec([]string{bad}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad.st:3:")
	})

	t.Run("parse error", func(t *testing.T) {
		bad := write("broken.st", "PROGRAM MAIN\nx := ;\n")
		_, _, err := loadProjectSpec([]string{bad}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "broken.st:")
	})

	t.Run("usage errors", func(t *testing.T) {
		_, _, err := loadProjectSpec(nil, nil)
		assert.Error(t, err)
		_, _, err = loadProjectSpec([]string{demoTsproj, gvl}, nil)
		assert.Error(t, err)
		_, _, err = loadProjectSpec([]string{filepath.Join(dir, "missing.st")}, nil)
		assert.Error(t, err)
		_, _, err = loadProjectSpec([]string{filepath.Join(dir, "missing.tsproj")}, nil)
		assert.Error(t, err)
	})

	t.Run("broken project", func(t *testing.T) {
		_, _, err := loadProjectSpec([]string{"../../pkg/twincat/testdata/broken/Broken.plcproj"}, nil)
		assert.Error(t, err)
	})
}

// Undeclared types and members are warnings for sim and serve: the runtime
// auto-stubs them. Other analysis errors still fail the load.
func TestLoadProjectSpecTolerantDrift(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.st")
	src := `TYPE ST_R : STRUCT a : INT; END_STRUCT END_TYPE
PROGRAM MAIN
VAR conv : FB_Missing; r : ST_R; z : INT; END_VAR
conv();
z := r.gone;
END_PROGRAM
`
	require.NoError(t, os.WriteFile(p, []byte(src), 0o644))
	spec, ds, err := loadProjectSpec([]string{p}, nil)
	require.NoError(t, err)
	var codes []string
	for _, d := range ds {
		assert.Equal(t, diag.Warning, d.Severity, d.String())
		codes = append(codes, d.Code)
	}
	assert.Contains(t, codes, checker.CodeUndeclaredType)
	assert.Contains(t, codes, checker.CodeNoMember)
	prj, err := interp.LoadProject(spec)
	require.NoError(t, err)
	require.NoError(t, prj.Tick())

	bad := filepath.Join(dir, "bad.st")
	require.NoError(t, os.WriteFile(bad, []byte("PROGRAM MAIN\nVAR z : INT; END_VAR\nz := nope;\nEND_PROGRAM\n"), 0o644))
	_, _, err = loadProjectSpec([]string{bad}, nil)
	assert.Error(t, err)
}
