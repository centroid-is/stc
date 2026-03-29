package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	// Build the stc-mcp binary and verify it compiles without errors.
	tmpDir := t.TempDir()
	binName := "stc-mcp"
	if runtime.GOOS == "windows" {
		binName = "stc-mcp.exe"
	}
	outPath := tmpDir + "/" + binName

	cmd := exec.Command("go", "build", "-o", outPath, "./cmd/stc-mcp")
	cmd.Dir = findProjectRoot(t)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(output))

	// Verify binary was created
	_, err = os.Stat(outPath)
	require.NoError(t, err, "binary not found at %s", outPath)
}

func TestToolRegistration(t *testing.T) {
	// Verify that registerTools doesn't panic and that a server can be created.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "stc-mcp-test",
		Version: "test",
	}, nil)

	// Should not panic
	assert.NotPanics(t, func() {
		registerTools(server)
	})

	// Verify all 6 tools are defined in our metadata.
	tools := allToolDefinitions()
	assert.Len(t, tools, 6)

	expectedNames := []string{"stc_parse", "stc_check", "stc_test", "stc_emit", "stc_lint", "stc_format"}
	for i, tool := range tools {
		assert.Equal(t, expectedNames[i], tool.name)
		assert.NotEmpty(t, tool.description)
	}
}

// findProjectRoot locates the project root by looking for go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	// Walk up to find go.mod
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:max(0, len(dir)-1)]
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}
