// Package analyzer provides the public facade for semantic analysis
// of IEC 61131-3 Structured Text source files. It orchestrates parsing,
// declaration resolution, type checking, usage analysis, and vendor
// compatibility checking into a single Analyze() call.
package analyzer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/checker"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
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

// AnalyzeFiles is a convenience function that reads files from disk,
// parses each with the parser, and then runs Analyze on the parsed ASTs.
// Parse errors are included in the returned diagnostics.
//
// If filenames is empty and cfg has source roots configured, it discovers
// .st files from those source roots automatically.
func AnalyzeFiles(filenames []string, cfg *project.Config) AnalysisResult {
	// If no filenames given, try to discover from source roots
	if len(filenames) == 0 && cfg != nil && len(cfg.Build.SourceRoots) > 0 {
		filenames = discoverSTFiles(cfg.Build.SourceRoots)
	}

	var files []*ast.SourceFile
	var parseDiags []diag.Diagnostic

	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			parseDiags = append(parseDiags, diag.Diagnostic{
				Severity: diag.Error,
				Code:     "IO001",
				Message:  err.Error(),
			})
			continue
		}

		result := parser.Parse(filename, string(content))
		files = append(files, result.File)
		parseDiags = append(parseDiags, result.Diags...)
	}

	// Run semantic analysis on parsed files
	analysisResult := Analyze(files, cfg)

	// Combine parse diagnostics with analysis diagnostics
	allDiags := make([]diag.Diagnostic, 0, len(parseDiags)+len(analysisResult.Diags))
	allDiags = append(allDiags, parseDiags...)
	allDiags = append(allDiags, analysisResult.Diags...)

	return AnalysisResult{
		Symbols: analysisResult.Symbols,
		Diags:   allDiags,
	}
}

// discoverSTFiles walks the given source root directories and returns
// all .st files found.
func discoverSTFiles(roots []string) []string {
	var files []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".st") {
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}
