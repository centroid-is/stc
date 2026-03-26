// Package project provides project configuration loading and management
// for the STC compiler toolchain.
package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the top-level stc.toml project configuration.
type Config struct {
	Project ProjectConfig `toml:"project"`
	Build   BuildConfig   `toml:"build"`
	Lint    LintConfig    `toml:"lint"`
}

// ProjectConfig holds project metadata.
type ProjectConfig struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

// BuildConfig holds build-related configuration.
type BuildConfig struct {
	SourceRoots  []string          `toml:"source_roots"`
	VendorTarget string            `toml:"vendor_target"`
	LibraryPaths map[string]string `toml:"library_paths"`
}

// LintConfig holds linting configuration.
type LintConfig struct {
	NamingConvention string `toml:"naming_convention"`
}

// LoadConfig reads and parses an stc.toml configuration file from the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return &cfg, nil
}

// FindConfig walks up from dir looking for an stc.toml file.
// Returns the path to the config file, or an error if none is found.
func FindConfig(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, "stc.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding stc.toml.
			return "", fmt.Errorf("stc.toml not found (searched up from %s)", dir)
		}
		dir = parent
	}
}
