package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Project.Name)
	assert.Equal(t, "", cfg.Project.Version)
	assert.Nil(t, cfg.Build.SourceRoots)
	assert.Equal(t, "", cfg.Build.VendorTarget)
}

func TestLoadConfig_MinimalProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	content := `[project]
name = "minimal"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "minimal", cfg.Project.Name)
	assert.Equal(t, "", cfg.Project.Version)
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	require.NoError(t, os.WriteFile(path, []byte("this is not valid [toml\n"), 0644))

	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config")
}

func TestLoadConfig_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	content := `[project]
name = "full"
version = "2.0.0"

[build]
source_roots = ["src/", "lib/", "test/"]
vendor_target = "schneider"

[build.library_paths]
oscat = "vendor/oscat/"
util = "vendor/util/"

[lint]
naming_convention = "camelCase"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "full", cfg.Project.Name)
	assert.Equal(t, "2.0.0", cfg.Project.Version)
	assert.Equal(t, []string{"src/", "lib/", "test/"}, cfg.Build.SourceRoots)
	assert.Equal(t, "schneider", cfg.Build.VendorTarget)
	assert.Equal(t, "vendor/oscat/", cfg.Build.LibraryPaths["oscat"])
	assert.Equal(t, "vendor/util/", cfg.Build.LibraryPaths["util"])
	assert.Equal(t, "camelCase", cfg.Lint.NamingConvention)
}

func TestLoadConfig_MissingBuildSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	content := `[project]
name = "no-build"
version = "1.0.0"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.Build.SourceRoots)
	assert.Equal(t, "", cfg.Build.VendorTarget)
}

func TestLoadConfig_MissingLintSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	content := `[project]
name = "no-lint"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Lint.NamingConvention)
}

func TestFindConfig_Found(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "stc.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[project]\nname = \"found\"\n"), 0644))

	// Search from the directory containing stc.toml
	found, err := FindConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, configPath, found)
}

func TestFindConfig_FoundInParent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "stc.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[project]\nname = \"parent\"\n"), 0644))

	// Create a subdirectory
	subdir := filepath.Join(dir, "src", "deep")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	found, err := FindConfig(subdir)
	require.NoError(t, err)
	assert.Equal(t, configPath, found)
}

func TestFindConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	// No stc.toml in temp dir or its parents (temp dirs are at the root)
	_, err := FindConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stc.toml not found")
}

func TestLoadConfig_VendorTargets(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"beckhoff", "beckhoff"},
		{"schneider", "schneider"},
		{"portable", "portable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "stc.toml")
			content := `[build]
vendor_target = "` + tt.target + `"
`
			require.NoError(t, os.WriteFile(path, []byte(content), 0644))

			cfg, err := LoadConfig(path)
			require.NoError(t, err)
			assert.Equal(t, tt.target, cfg.Build.VendorTarget)
		})
	}
}

func TestLoadConfig_EmptySourceRoots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stc.toml")
	content := `[build]
source_roots = []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, []string{}, cfg.Build.SourceRoots)
}
