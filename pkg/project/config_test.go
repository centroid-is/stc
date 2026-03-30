package project

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("testdata", "stc.toml"))
	require.NoError(t, err)

	assert.Equal(t, "test-project", cfg.Project.Name)
	assert.Equal(t, "1.0.0", cfg.Project.Version)
	assert.Equal(t, []string{"src/", "lib/"}, cfg.Build.SourceRoots)
	assert.Equal(t, "beckhoff", cfg.Build.VendorTarget)
	assert.Equal(t, "vendor/oscat/", cfg.Build.LibraryPaths["oscat"])
	assert.Equal(t, "PascalCase", cfg.Lint.NamingConvention)
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := LoadConfig("nonexistent/stc.toml")
	assert.Error(t, err)
}

func TestLoadConfig_TestSection_MockPaths(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("testdata", "stc_with_test.toml"))
	require.NoError(t, err)

	assert.Equal(t, []string{"mocks/", "test/mocks/"}, cfg.Test.MockPaths)
}

func TestLoadConfig_TestSection_Missing(t *testing.T) {
	// The standard stc.toml has no [test] section; MockPaths should be nil
	cfg, err := LoadConfig(filepath.Join("testdata", "stc.toml"))
	require.NoError(t, err)

	assert.Nil(t, cfg.Test.MockPaths)
}
