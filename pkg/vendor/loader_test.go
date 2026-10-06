package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadLibraries_EmptyConfig(t *testing.T) {
	cfg := &project.Config{}
	files, err := LoadLibraries(cfg, t.TempDir())
	assert.NoError(t, err)
	assert.Nil(t, files)
}

func TestLoadLibraries_NilLibraryPaths(t *testing.T) {
	cfg := &project.Config{
		Build: project.BuildConfig{
			LibraryPaths: nil,
		},
	}
	files, err := LoadLibraries(cfg, t.TempDir())
	assert.NoError(t, err)
	assert.Nil(t, files)
}

func TestLoadLibraries_SingleStubFile(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "libs", "beckhoff")
	require.NoError(t, os.MkdirAll(libDir, 0o755))

	stubContent := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Position : REAL;
    Velocity : REAL;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
    Busy : BOOL;
    Error : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "motion.st"), []byte(stubContent), 0o644))

	cfg := &project.Config{
		Build: project.BuildConfig{
			LibraryPaths: map[string]string{
				"beckhoff": "libs/beckhoff",
			},
		},
	}

	files, err := LoadLibraries(cfg, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.NotNil(t, files[0])
	// Should have parsed the MC_MoveAbsolute FB declaration
	assert.Greater(t, len(files[0].Declarations), 0)
}

func TestLoadLibraries_NonexistentDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := &project.Config{
		Build: project.BuildConfig{
			LibraryPaths: map[string]string{
				"missing": "/nonexistent/path/to/libs",
			},
		},
	}

	files, err := LoadLibraries(cfg, dir)
	assert.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), "missing")
}

func TestLoadLibraries_StubFBWithNoBody(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "libs")
	require.NoError(t, os.MkdirAll(libDir, 0o755))

	stubContent := `FUNCTION_BLOCK ADSREAD
VAR_INPUT
    NETID : STRING;
    PORT : INT;
    IDXGRP : INT;
    IDXOFFS : INT;
    LEN : INT;
    DESTADDR : INT;
    READ : BOOL;
END_VAR
VAR_OUTPUT
    BUSY : BOOL;
    ERR : BOOL;
    ERRID : INT;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "ads.st"), []byte(stubContent), 0o644))

	cfg := &project.Config{
		Build: project.BuildConfig{
			LibraryPaths: map[string]string{
				"tc3": libDir,
			},
		},
	}

	files, err := LoadLibraries(cfg, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Greater(t, len(files[0].Declarations), 0)
}

func TestLoadLibraries_MultipleLibraries(t *testing.T) {
	dir := t.TempDir()
	lib1 := filepath.Join(dir, "lib1")
	lib2 := filepath.Join(dir, "lib2")
	require.NoError(t, os.MkdirAll(lib1, 0o755))
	require.NoError(t, os.MkdirAll(lib2, 0o755))

	stub1 := `FUNCTION_BLOCK FB_One
VAR_INPUT
    x : INT;
END_VAR
END_FUNCTION_BLOCK
`
	stub2 := `FUNCTION_BLOCK FB_Two
VAR_INPUT
    y : REAL;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(lib1, "one.st"), []byte(stub1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(lib2, "two.st"), []byte(stub2), 0o644))

	cfg := &project.Config{
		Build: project.BuildConfig{
			LibraryPaths: map[string]string{
				"lib1": lib1,
				"lib2": lib2,
			},
		},
	}

	files, err := LoadLibraries(cfg, dir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestSymbol_IsLibrary_DefaultsFalse(t *testing.T) {
	sym := &symbols.Symbol{
		Name: "test",
		Kind: symbols.KindVariable,
	}
	assert.False(t, sym.IsLibrary)
}

func TestLoadLibraries_SortedKeys(t *testing.T) {
	dir := t.TempDir()
	keys := []string{"zeta", "alpha", "mu", "beta", "omega", "delta", "kappa", "gamma"}
	paths := map[string]string{}
	for _, k := range keys {
		d := filepath.Join(dir, k)
		require.NoError(t, os.MkdirAll(d, 0o755))
		src := "FUNCTION_BLOCK FB_" + k + "\nEND_FUNCTION_BLOCK\n"
		require.NoError(t, os.WriteFile(filepath.Join(d, k+".st"), []byte(src), 0o644))
		paths[k] = k
	}
	cfg := &project.Config{Build: project.BuildConfig{LibraryPaths: paths}}
	want := []string{"alpha", "beta", "delta", "gamma", "kappa", "mu", "omega", "zeta"}
	for run := 0; run < 5; run++ {
		files, err := LoadLibraries(cfg, dir)
		require.NoError(t, err)
		require.Len(t, files, len(want))
		for i, f := range files {
			assert.Equal(t, want[i]+".st", filepath.Base(f.Span().Start.File))
		}
	}
}
