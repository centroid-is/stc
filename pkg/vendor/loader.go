// Package vendor provides loading of vendor library stub files (.st)
// from configured library paths for type-checking against vendor-specific
// function blocks like MC_MoveAbsolute, ADSREAD, etc.
package vendor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
)

// LoadLibraries reads and parses .st stub files from all configured library
// paths. Each path in cfg.Build.LibraryPaths maps a library name to a
// directory containing .st stub files. Relative paths are resolved against
// projectDir. Returns all parsed SourceFiles, or an error if any configured
// path does not exist.
func LoadLibraries(cfg *project.Config, projectDir string) ([]*ast.SourceFile, error) {
	if len(cfg.Build.LibraryPaths) == 0 {
		return nil, nil
	}

	var result []*ast.SourceFile

	for libName, libPath := range cfg.Build.LibraryPaths {
		// Resolve relative paths against project directory
		if !filepath.IsAbs(libPath) {
			libPath = filepath.Join(projectDir, libPath)
		}

		// Verify directory exists
		info, err := os.Stat(libPath)
		if err != nil {
			return nil, fmt.Errorf("library %q: path %q does not exist: %w", libName, libPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("library %q: path %q is not a directory", libName, libPath)
		}

		// Glob for .st files (non-recursive)
		pattern := filepath.Join(libPath, "*.st")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("library %q: glob error: %w", libName, err)
		}

		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil {
				return nil, fmt.Errorf("library %q: reading %q: %w", libName, match, err)
			}

			parsed := pipeline.Parse(match, string(content), nil)
			if parsed.File != nil {
				result = append(result, parsed.File)
			}
		}
	}

	return result, nil
}
