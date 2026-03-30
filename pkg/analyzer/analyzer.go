// Package analyzer provides the public facade for semantic analysis
// of IEC 61131-3 Structured Text source files. It orchestrates parsing,
// declaration resolution, type checking, usage analysis, and vendor
// compatibility checking into a single Analyze() call.
package analyzer

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
)

// AnalysisResult holds the output of semantic analysis.
type AnalysisResult struct {
	// Symbols is the populated symbol table after analysis.
	Symbols *symbols.Table
	// Diags contains all diagnostics from parsing and semantic analysis.
	Diags []diag.Diagnostic
}

// AnalyzeOpts provides optional configuration for Analyze.
type AnalyzeOpts struct {
	// LibraryFiles are parsed vendor library stub files that should be
	// registered before user code. Symbols from library files are marked
	// with IsLibrary=true and can be overridden by user code.
	LibraryFiles []*ast.SourceFile
}

// Analyze performs full semantic analysis on pre-parsed source files.
// It orchestrates:
//  1. Pass 1: Collect all declarations into the symbol table
//  2. Pass 2: Type-check all POU bodies
//  3. Usage analysis: detect unused variables and unreachable code
//  4. Vendor compatibility: if cfg specifies a vendor target, check for unsupported constructs
//  5. Cross-vendor enforcement: warn if library path keys suggest a different vendor than target
//
// Pass nil for cfg to skip vendor checks. The variadic opts parameter
// preserves backward compatibility -- existing callers pass no opts.
func Analyze(files []*ast.SourceFile, cfg *project.Config, opts ...AnalyzeOpts) AnalysisResult {
	table := symbols.NewTable()
	diags := diag.NewCollector()

	// Extract library files from opts
	var resolveOpts checker.ResolveOpts
	if len(opts) > 0 && opts[0].LibraryFiles != nil {
		resolveOpts.LibraryFiles = opts[0].LibraryFiles
	}

	// Pass 1: Collect declarations (library files registered first if provided)
	resolver := checker.NewResolver(table, diags)
	resolver.CollectDeclarations(files, resolveOpts)

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

	// Cross-vendor enforcement: warn if library path keys suggest a
	// different vendor than the project's target vendor
	if cfg != nil && cfg.Build.VendorTarget != "" && len(cfg.Build.LibraryPaths) > 0 {
		checkCrossVendorLibraries(cfg, diags)
	}

	return AnalysisResult{
		Symbols: table,
		Diags:   diags.All(),
	}
}

// knownVendorAliases maps vendor identifiers that may appear in library
// path keys to their canonical vendor name.
var knownVendorAliases = map[string]string{
	"beckhoff":      "beckhoff",
	"schneider":     "schneider",
	"allen_bradley": "allen_bradley",
	"ab":            "allen_bradley",
}

// checkCrossVendorLibraries emits a warning for each library path key
// that contains a known vendor name different from the project target.
func checkCrossVendorLibraries(cfg *project.Config, diags *diag.Collector) {
	target := strings.ToLower(cfg.Build.VendorTarget)

	for libKey := range cfg.Build.LibraryPaths {
		keyLower := strings.ToLower(libKey)
		for alias, canonical := range knownVendorAliases {
			if strings.Contains(keyLower, alias) {
				// Found a vendor name in the key -- check if it matches target
				targetCanonical, ok := knownVendorAliases[target]
				if !ok {
					targetCanonical = target
				}
				if canonical != targetCanonical {
					diags.Warnf(
						source.Pos{},
						checker.CodeCrossVendorLib,
						"library %q appears to be for vendor %q but project targets %q",
						libKey, canonical, target,
					)
				}
				break // Only check first vendor match per key
			}
		}
	}
}
