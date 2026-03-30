package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
