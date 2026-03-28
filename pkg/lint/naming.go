package lint

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

// checkNaming checks naming conventions based on the specified convention.
func checkNaming(file *ast.SourceFile, convention string) []diag.Diagnostic {
	return nil // stub
}
