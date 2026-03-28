package lint

import (
	"sort"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

// LintResult holds all diagnostics produced by the linter.
type LintResult struct {
	Diags []diag.Diagnostic
}

// Lint runs all lint rules against the given source files.
func Lint(files []*ast.SourceFile, opts LintOptions) LintResult {
	var allDiags []diag.Diagnostic
	for _, f := range files {
		result := LintFile(f, opts)
		allDiags = append(allDiags, result.Diags...)
	}
	sort.Slice(allDiags, func(i, j int) bool {
		if allDiags[i].Pos.File != allDiags[j].Pos.File {
			return allDiags[i].Pos.File < allDiags[j].Pos.File
		}
		if allDiags[i].Pos.Line != allDiags[j].Pos.Line {
			return allDiags[i].Pos.Line < allDiags[j].Pos.Line
		}
		return allDiags[i].Pos.Col < allDiags[j].Pos.Col
	})
	return LintResult{Diags: allDiags}
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
