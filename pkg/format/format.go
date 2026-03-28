package format

import (
	"github.com/centroid-is/stc/pkg/ast"
)

// Format produces consistently formatted Structured Text from a parsed AST.
// It returns an empty string for nil input.
func Format(file *ast.SourceFile, opts FormatOptions) string {
	// Stub - tests should fail
	return ""
}
