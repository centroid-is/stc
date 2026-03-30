package vendor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countErrors(diags []diag.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if d.Severity == diag.Error {
			count++
		}
	}
	return count
}

func hasDiagCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// TestIntegrationStubLoadingValid exercises the full pipeline:
// config -> LoadLibraries -> parse user code -> Analyze with LibraryFiles.
// User code references MC_MoveAbsolute from a stub and should produce zero errors.
func TestIntegrationStubLoadingValid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create stc.toml
	configContent := `[project]
name = "test"

[build]
vendor_target = "beckhoff"
source_roots = ["."]

[build.library_paths]
tc2_mc2 = "libs/tc2_mc2"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "stc.toml"), []byte(configContent), 0644))

	// Create library stub directory and file
	libDir := filepath.Join(tmpDir, "libs", "tc2_mc2")
	require.NoError(t, os.MkdirAll(libDir, 0755))

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
    ErrorID : INT;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "mc2.st"), []byte(stubContent), 0644))

	// Create user code
	userCode := `PROGRAM Main
VAR
    mover : MC_MoveAbsolute;
    startMove : BOOL;
END_VAR
    mover(Execute := startMove, Position := 100.0, Velocity := 50.0);
    IF mover.Done THEN
        startMove := FALSE;
    END_IF
END_PROGRAM
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.st"), []byte(userCode), 0644))

	// Load config
	cfg, err := project.LoadConfig(filepath.Join(tmpDir, "stc.toml"))
	require.NoError(t, err)

	// Load libraries
	libFiles, err := vendor.LoadLibraries(cfg, tmpDir)
	require.NoError(t, err)
	require.NotEmpty(t, libFiles, "should load at least one library file")

	// Parse user code
	userContent, err := os.ReadFile(filepath.Join(tmpDir, "main.st"))
	require.NoError(t, err)
	pipeResult := pipeline.Parse(filepath.Join(tmpDir, "main.st"), string(userContent), nil)
	require.NotNil(t, pipeResult.File)

	// Analyze with library files
	result := analyzer.Analyze(
		[]*ast.SourceFile{pipeResult.File},
		cfg,
		analyzer.AnalyzeOpts{LibraryFiles: libFiles},
	)

	errors := countErrors(result.Diags)
	assert.Equal(t, 0, errors,
		"valid user code referencing library FB should have 0 errors, got diags: %v", result.Diags)
}

// TestIntegrationStubLoadingWrongParam verifies that using a wrong parameter
// name on a library FB produces a diagnostic error.
func TestIntegrationStubLoadingWrongParam(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[project]
name = "test"

[build]
vendor_target = "beckhoff"
source_roots = ["."]

[build.library_paths]
tc2_mc2 = "libs/tc2_mc2"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "stc.toml"), []byte(configContent), 0644))

	libDir := filepath.Join(tmpDir, "libs", "tc2_mc2")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	stubContent := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Execute : BOOL;
    Position : REAL;
    Velocity : REAL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "mc2.st"), []byte(stubContent), 0644))

	// User code with typo: "Execut" instead of "Execute"
	userCode := `PROGRAM Main
VAR
    mover : MC_MoveAbsolute;
END_VAR
    mover(Execut := TRUE);
END_PROGRAM
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.st"), []byte(userCode), 0644))

	cfg, err := project.LoadConfig(filepath.Join(tmpDir, "stc.toml"))
	require.NoError(t, err)

	libFiles, err := vendor.LoadLibraries(cfg, tmpDir)
	require.NoError(t, err)

	userContent, err := os.ReadFile(filepath.Join(tmpDir, "main.st"))
	require.NoError(t, err)
	pipeResult := pipeline.Parse(filepath.Join(tmpDir, "main.st"), string(userContent), nil)
	require.NotNil(t, pipeResult.File)

	result := analyzer.Analyze(
		[]*ast.SourceFile{pipeResult.File},
		cfg,
		analyzer.AnalyzeOpts{LibraryFiles: libFiles},
	)

	errors := countErrors(result.Diags)
	assert.Greater(t, errors, 0,
		"wrong parameter name should produce at least 1 error, got diags: %v", result.Diags)
}

// TestIntegrationCrossVendorWarning verifies that loading libraries from a
// vendor that doesn't match the project target emits a cross-vendor warning.
func TestIntegrationCrossVendorWarning(t *testing.T) {
	tmpDir := t.TempDir()

	// Library key "schneider_motion" with vendor_target "beckhoff"
	configContent := `[project]
name = "test"

[build]
vendor_target = "beckhoff"
source_roots = ["."]

[build.library_paths]
schneider_motion = "libs/schneider_motion"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "stc.toml"), []byte(configContent), 0644))

	libDir := filepath.Join(tmpDir, "libs", "schneider_motion")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	stubContent := `FUNCTION_BLOCK MC_Power
VAR_INPUT
    Enable : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "motion.st"), []byte(stubContent), 0644))

	userCode := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.st"), []byte(userCode), 0644))

	cfg, err := project.LoadConfig(filepath.Join(tmpDir, "stc.toml"))
	require.NoError(t, err)

	libFiles, err := vendor.LoadLibraries(cfg, tmpDir)
	require.NoError(t, err)

	userContent, err := os.ReadFile(filepath.Join(tmpDir, "main.st"))
	require.NoError(t, err)
	pipeResult := pipeline.Parse(filepath.Join(tmpDir, "main.st"), string(userContent), nil)
	require.NotNil(t, pipeResult.File)

	result := analyzer.Analyze(
		[]*ast.SourceFile{pipeResult.File},
		cfg,
		analyzer.AnalyzeOpts{LibraryFiles: libFiles},
	)

	assert.True(t, hasDiagCode(result.Diags, "VEND010"),
		"cross-vendor library should produce VEND010 warning, got diags: %v", result.Diags)
}
