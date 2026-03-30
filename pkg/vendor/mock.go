package vendor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
)

// LoadMocks reads and parses .st mock files from all configured mock paths.
// Each path in cfg.Test.MockPaths is a directory containing .st mock files.
// Relative paths are resolved against projectDir. Returns all parsed
// SourceFiles, or an error if any configured path does not exist.
func LoadMocks(cfg *project.Config, projectDir string) ([]*ast.SourceFile, error) {
	if len(cfg.Test.MockPaths) == 0 {
		return nil, nil
	}

	var result []*ast.SourceFile

	for _, mockPath := range cfg.Test.MockPaths {
		// Resolve relative paths against project directory
		if !filepath.IsAbs(mockPath) {
			mockPath = filepath.Join(projectDir, mockPath)
		}

		// Verify directory exists
		info, err := os.Stat(mockPath)
		if err != nil {
			return nil, fmt.Errorf("mock path %q does not exist: %w", mockPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("mock path %q is not a directory", mockPath)
		}

		// Glob for .st files (non-recursive)
		pattern := filepath.Join(mockPath, "*.st")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("mock path %q: glob error: %w", mockPath, err)
		}

		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil {
				return nil, fmt.Errorf("mock path %q: reading %q: %w", mockPath, match, err)
			}

			parsed := pipeline.Parse(match, string(content), nil)
			if parsed.File != nil {
				result = append(result, parsed.File)
			}
		}
	}

	return result, nil
}
