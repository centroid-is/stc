// Package pipeline provides a unified preprocess-then-parse entry point
// so that all consumers (CLI commands, LSP, MCP, tests) automatically
// evaluate IEC 61131-3 preprocessor directives before parsing.
package pipeline

import (
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/preprocess"
)

// ParseResult combines the parser output with any preprocessor diagnostics
// and the source map for position remapping.
type ParseResult struct {
	parser.ParseResult
	// PPDiags holds preprocessor-only diagnostics (e.g., unterminated IF,
	// {ERROR} hits). These are also merged into Diags for convenience.
	PPDiags []diag.Diagnostic
	// SourceMap maps preprocessed output positions back to original source.
	SourceMap *preprocess.SourceMap
}

// Parse preprocesses the source text (evaluating {IF}/{DEFINE}/{ERROR}
// directives) and then parses the preprocessed output.
//
// Diagnostics from both the preprocessor and the parser are combined.
// The source map is used to remap parser positions back to original source
// lines so that error messages reference the correct locations.
//
// If defines is nil, no external symbols are defined (only {DEFINE}
// directives within the source take effect).
func Parse(filename, src string, defines map[string]bool) ParseResult {
	// Phase 1: preprocess
	ppResult := preprocess.Preprocess(src, preprocess.Options{
		Filename: filename,
		Defines:  defines,
	})

	// Phase 2: parse the preprocessed output
	parseResult := parser.Parse(filename, ppResult.Output)

	// Phase 3: remap parser diagnostic positions through the source map
	// so they reference original source lines, not preprocessed output lines.
	if ppResult.SourceMap != nil && ppResult.SourceMap.Len() > 0 {
		for i := range parseResult.Diags {
			d := &parseResult.Diags[i]
			orig := ppResult.SourceMap.OriginalPos(d.Pos.Line, d.Pos.Col)
			if orig.Line > 0 {
				d.Pos = orig
			}
		}
	}

	// Phase 4: combine diagnostics (preprocessor first, then parser)
	allDiags := make([]diag.Diagnostic, 0, len(ppResult.Diags)+len(parseResult.Diags))
	allDiags = append(allDiags, ppResult.Diags...)
	allDiags = append(allDiags, parseResult.Diags...)

	return ParseResult{
		ParseResult: parser.ParseResult{
			File:  parseResult.File,
			Diags: allDiags,
		},
		PPDiags:   ppResult.Diags,
		SourceMap: ppResult.SourceMap,
	}
}
