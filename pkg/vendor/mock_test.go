package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseST(src string) *ast.SourceFile {
	result := parser.Parse("test.st", src)
	return result.File
}

func TestLoadMocks_EmptyMockPaths(t *testing.T) {
	cfg := &project.Config{}
	files, err := LoadMocks(cfg, t.TempDir())
	assert.NoError(t, err)
	assert.Nil(t, files)
}

func TestLoadMocks_SingleMockFile(t *testing.T) {
	dir := t.TempDir()
	mockDir := filepath.Join(dir, "mocks")
	require.NoError(t, os.MkdirAll(mockDir, 0o755))

	mockContent := `FUNCTION_BLOCK MC_MoveAbsolute
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
    Done := Execute;
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(mockDir, "motion_mock.st"), []byte(mockContent), 0o644))

	cfg := &project.Config{
		Test: project.TestConfig{
			MockPaths: []string{"mocks"},
		},
	}

	files, err := LoadMocks(cfg, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.NotNil(t, files[0])
	assert.Greater(t, len(files[0].Declarations), 0)
}

func TestLoadMocks_NonexistentPath(t *testing.T) {
	dir := t.TempDir()
	cfg := &project.Config{
		Test: project.TestConfig{
			MockPaths: []string{"/nonexistent/mock/path"},
		},
	}

	files, err := LoadMocks(cfg, dir)
	assert.Error(t, err)
	assert.Nil(t, files)
}

func TestLoadMocks_ParsesFunctionBlockWithBody(t *testing.T) {
	dir := t.TempDir()
	mockDir := filepath.Join(dir, "mocks")
	require.NoError(t, os.MkdirAll(mockDir, 0o755))

	mockContent := `FUNCTION_BLOCK FB_Sensor
VAR_INPUT
    Enable : BOOL;
END_VAR
VAR_OUTPUT
    Value : REAL;
END_VAR
    IF Enable THEN
        Value := 42.0;
    END_IF;
END_FUNCTION_BLOCK
`
	require.NoError(t, os.WriteFile(filepath.Join(mockDir, "sensor_mock.st"), []byte(mockContent), 0o644))

	cfg := &project.Config{
		Test: project.TestConfig{
			MockPaths: []string{"mocks"},
		},
	}

	files, err := LoadMocks(cfg, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Greater(t, len(files[0].Declarations), 0)
}

func TestValidateMockSignatures_MatchingSignature(t *testing.T) {
	// Stub FB in library
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	// Mock with same signature
	mockSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
    Done := Execute;
END_FUNCTION_BLOCK
`
	libFile := parseST(libSrc)
	mockFile := parseST(mockSrc)

	// Register library symbols in the table
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := checker.NewResolver(table, diags)
	resolver.CollectDeclarations(nil, checker.ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	errs := ValidateMockSignatures([]*ast.SourceFile{mockFile}, table)
	assert.Empty(t, errs)
}

func TestValidateMockSignatures_DifferentParamCount(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
    Execute : BOOL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
END_FUNCTION_BLOCK
`
	// Mock with different input count (missing Execute)
	mockSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
END_VAR
VAR_OUTPUT
    Done : BOOL;
END_VAR
    Done := TRUE;
END_FUNCTION_BLOCK
`
	libFile := parseST(libSrc)
	mockFile := parseST(mockSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := checker.NewResolver(table, diags)
	resolver.CollectDeclarations(nil, checker.ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	errs := ValidateMockSignatures([]*ast.SourceFile{mockFile}, table)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "input")
}

func TestValidateMockSignatures_DifferentParamTypes(t *testing.T) {
	libSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : INT;
END_VAR
END_FUNCTION_BLOCK
`
	// Mock with REAL instead of INT
	mockSrc := `FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis : REAL;
END_VAR
    ;
END_FUNCTION_BLOCK
`
	libFile := parseST(libSrc)
	mockFile := parseST(mockSrc)

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := checker.NewResolver(table, diags)
	resolver.CollectDeclarations(nil, checker.ResolveOpts{
		LibraryFiles: []*ast.SourceFile{libFile},
	})

	errs := ValidateMockSignatures([]*ast.SourceFile{mockFile}, table)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "type")
}

func TestValidateMockSignatures_FBNotInLibrary(t *testing.T) {
	// Mock for an FB that's not in the library -- should skip validation
	mockSrc := `FUNCTION_BLOCK FB_Custom
VAR_INPUT
    x : INT;
END_VAR
    ;
END_FUNCTION_BLOCK
`
	mockFile := parseST(mockSrc)

	table := symbols.NewTable()
	// Empty table -- no library symbols

	errs := ValidateMockSignatures([]*ast.SourceFile{mockFile}, table)
	assert.Empty(t, errs)
}
