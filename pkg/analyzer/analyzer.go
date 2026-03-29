// Package analyzer provides the public facade for semantic analysis
// of IEC 61131-3 Structured Text source files. It orchestrates parsing,
// declaration resolution, type checking, usage analysis, and vendor
// compatibility checking into a single Analyze() call.
package analyzer

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/symbols"
)

// AnalysisResult holds the output of semantic analysis.
type AnalysisResult struct {
	// Symbols is the populated symbol table after analysis.
	Symbols *symbols.Table
	// Diags contains all diagnostics from parsing and semantic analysis.
	Diags []diag.Diagnostic
}

// Analyze performs full semantic analysis on pre-parsed source files.
// It orchestrates:
//  1. Pass 1: Collect all declarations into the symbol table
//  2. Pass 2: Type-check all POU bodies
//  3. Usage analysis: detect unused variables and unreachable code
//  4. Vendor compatibility: if cfg specifies a vendor target, check for unsupported constructs
//
// Pass nil for cfg to skip vendor checks.
func Analyze(files []*ast.SourceFile, cfg *project.Config) AnalysisResult {
	table := symbols.NewTable()
	diags := diag.NewCollector()

	// Pass 1: Collect declarations
	resolver := checker.NewResolver(table, diags)
	resolver.CollectDeclarations(files)

	// Pass 2: Type-check bodies
	chk := checker.NewChecker(table, diags)
	chk.CheckBodies(files)

	// Usage analysis
	checker.CheckUsage(files, table, diags)

	// Vendor compatibility (optional)
	if cfg != nil && cfg.Build.VendorTarget != "" {
		profile := checker.LookupVendor(cfg.Build.VendorTarget)
		if profile != nil {
			checker.CheckVendorCompat(files, table, profile, diags)
		}
	}

	return AnalysisResult{
		Symbols: table,
		Diags:   diags.All(),
	}
}

