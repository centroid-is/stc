package preprocess

import (
	"strings"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// Result holds the output of preprocessing a source file.
type Result struct {
	// Output is the preprocessed source text with directive lines removed
	// and only active branches included.
	Output string

	// SourceMap maps positions in Output back to original source positions.
	SourceMap *SourceMap

	// Diags contains any diagnostics generated during preprocessing,
	// such as {ERROR} directive hits or malformed directive errors.
	Diags []diag.Diagnostic
}

// Options configures the preprocessor.
type Options struct {
	// Filename is the name of the source file, used in diagnostic positions.
	Filename string

	// Defines is the set of externally defined symbols (e.g., from CLI flags).
	// The preprocessor may add to this map via {DEFINE} directives.
	Defines map[string]bool
}

// ifFrame tracks the state of a single level of IF/ELSIF/ELSE nesting.
type ifFrame struct {
	active      bool // whether the current branch is active
	branchTaken bool // whether any branch in this IF block was taken
	origLine    int  // original line of the opening {IF} (for diagnostics)
}

// Preprocess evaluates IEC 61131-3 preprocessor directives in the source text
// and returns the preprocessed output, a source map, and any diagnostics.
//
// Supported directives: {IF}, {ELSIF}, {ELSE}, {END_IF}, {DEFINE}, {ERROR}.
// Non-preprocessor pragmas (e.g., {attribute '...'}) pass through unchanged.
func Preprocess(src string, opts Options) Result {
	lines := splitLines(src)
	defines := copyDefines(opts.Defines)

	var (
		stack     []ifFrame
		outLines  []string
		sm        = &SourceMap{}
		diags     []diag.Diagnostic
		outLineNo int
	)

	for lineIdx, line := range lines {
		origLine := lineIdx + 1 // 1-based

		// Check if this line contains a pragma directive
		trimmed := strings.TrimSpace(line)
		d := parsePragmaLine(trimmed)

		if d == nil {
			// Not a preprocessor directive — emit if active
			if isActive(stack) {
				outLineNo++
				outLines = append(outLines, line)
				sm.AddMapping(outLineNo, opts.Filename, origLine)
			}
			continue
		}

		// Handle preprocessor directive
		switch d.kind {
		case dirIF:
			condResult := evalCondition(d.condition, defines)
			active := isActive(stack) && condResult
			stack = append(stack, ifFrame{
				active:      active,
				branchTaken: condResult,
				origLine:    origLine,
			})

		case dirELSIF:
			if len(stack) == 0 {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Pos:      source.Pos{File: opts.Filename, Line: origLine, Col: 1},
					Code:     "PP002",
					Message:  "ELSIF without matching IF",
				})
				continue
			}
			frame := &stack[len(stack)-1]
			if frame.branchTaken {
				frame.active = false
			} else {
				condResult := evalCondition(d.condition, defines)
				parentActive := isActive(stack[:len(stack)-1])
				frame.active = parentActive && condResult
				if condResult {
					frame.branchTaken = true
				}
			}

		case dirELSE:
			if len(stack) == 0 {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Pos:      source.Pos{File: opts.Filename, Line: origLine, Col: 1},
					Code:     "PP002",
					Message:  "ELSE without matching IF",
				})
				continue
			}
			frame := &stack[len(stack)-1]
			if frame.branchTaken {
				frame.active = false
			} else {
				parentActive := isActive(stack[:len(stack)-1])
				frame.active = parentActive
				frame.branchTaken = true
			}

		case dirENDIF:
			if len(stack) == 0 {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Pos:      source.Pos{File: opts.Filename, Line: origLine, Col: 1},
					Code:     "PP002",
					Message:  "END_IF without matching IF",
				})
				continue
			}
			stack = stack[:len(stack)-1]

		case dirDEFINE:
			if isActive(stack) {
				defines[d.name] = true
			}

		case dirERROR:
			if isActive(stack) {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Pos:      source.Pos{File: opts.Filename, Line: origLine, Col: 1},
					Code:     "PP001",
					Message:  d.message,
				})
			}
		}
	}

	// Check for unclosed IF blocks
	if len(stack) > 0 {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Pos:      source.Pos{File: opts.Filename, Line: stack[0].origLine, Col: 1},
			Code:     "PP003",
			Message:  "unterminated IF block (missing END_IF)",
		})
	}

	return Result{
		Output:    strings.Join(outLines, "\n"),
		SourceMap: sm,
		Diags:     diags,
	}
}

// isActive returns true if all frames in the stack are active (i.e.,
// we are inside an active branch at all nesting levels).
func isActive(stack []ifFrame) bool {
	for _, f := range stack {
		if !f.active {
			return false
		}
	}
	return true
}

// parsePragmaLine checks if a trimmed line is a preprocessor directive.
// Returns nil if it's not a preprocessor pragma (including regular pragmas
// like {attribute '...'}).
func parsePragmaLine(trimmed string) *directive {
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return nil
	}
	// Find closing brace
	end := strings.IndexByte(trimmed, '}')
	if end < 0 {
		return nil
	}
	pragma := trimmed[:end+1]
	return parseDirective(pragma)
}

// splitLines splits text into lines without including line terminators.
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	lines := strings.Split(text, "\n")
	// Remove trailing \r from each line (handle \r\n)
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r")
	}
	return lines
}

// copyDefines creates a copy of the defines map so that {DEFINE} directives
// do not mutate the caller's map.
func copyDefines(m map[string]bool) map[string]bool {
	result := make(map[string]bool, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
