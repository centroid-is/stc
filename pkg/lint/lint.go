package lint

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

// LintResult holds all diagnostics produced by the linter.
type LintResult struct {
	Diags []diag.Diagnostic
}

// LintFile runs all lint rules against a single source file.
func LintFile(file *ast.SourceFile, opts LintOptions) LintResult {
	var diags []diag.Diagnostic

	// PLCopen rules
	diags = append(diags, checkMagicNumbers(file)...)
	diags = append(diags, checkNestingDepth(file, 3)...)
	diags = append(diags, checkPOULength(file, 200)...)
	diags = append(diags, checkMissingReturnType(file)...)

	// Naming rules
	diags = append(diags, checkNaming(file, opts.NamingConvention)...)

	return LintResult{Diags: diags}
}
