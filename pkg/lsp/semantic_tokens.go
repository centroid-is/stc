package lsp

import (
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// semanticTokensLegend defines the token types and modifiers advertised by the server.
// We use "comment" as the token type for inactive preprocessor regions so that
// editors gray them out by default.
var semanticTokensLegend = protocol.SemanticTokensLegend{
	TokenTypes:     []string{"comment"},
	TokenModifiers: []string{},
}

// inactiveRegion represents a range of lines that are inside an inactive
// preprocessor branch (ELSE/ELSIF blocks when assuming the first IF branch
// is the default active one).
type inactiveRegion struct {
	startLine int // 0-based
	endLine   int // 0-based, inclusive
}

// findInactiveRegions scans source content for preprocessor directives and
// identifies inactive regions. Without knowing the actual defines, the heuristic
// assumes the first {IF} branch is active, so {ELSIF} and {ELSE} blocks are
// marked as inactive.
func findInactiveRegions(content string) []inactiveRegion {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var regions []inactiveRegion

	type frame struct {
		// firstBranchDone is true after we pass the first {IF} branch
		// (i.e., we hit {ELSIF} or {ELSE})
		firstBranchDone bool
		// inactiveStart is the 0-based line where the current inactive region starts,
		// or -1 if not currently in an inactive region
		inactiveStart int
		// nestDepth tracks nested {IF} blocks inside an inactive region
		nestDepth int
	}

	var stack []frame

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for preprocessor directives
		if len(trimmed) < 2 || trimmed[0] != '{' {
			continue
		}

		closeBrace := strings.IndexByte(trimmed, '}')
		if closeBrace < 0 {
			continue
		}

		inner := strings.TrimSpace(trimmed[1:closeBrace])
		innerUpper := strings.ToUpper(inner)
		_ = upper // used implicitly through innerUpper

		switch {
		case strings.HasPrefix(innerUpper, "IF ") || strings.HasPrefix(innerUpper, "IF("):
			if len(stack) > 0 && stack[len(stack)-1].inactiveStart >= 0 {
				// We're inside an inactive region; track nesting
				stack[len(stack)-1].nestDepth++
				continue
			}
			stack = append(stack, frame{
				firstBranchDone: false,
				inactiveStart:   -1,
				nestDepth:       0,
			})

		case innerUpper == "ELSIF" || strings.HasPrefix(innerUpper, "ELSIF ") || strings.HasPrefix(innerUpper, "ELSIF("):
			if len(stack) == 0 {
				continue
			}
			top := &stack[len(stack)-1]
			if top.nestDepth > 0 {
				continue // nested IF inside inactive region
			}
			if !top.firstBranchDone {
				// End of the first (assumed-active) branch
				top.firstBranchDone = true
				top.inactiveStart = i // mark start of inactive region (the ELSIF line)
			}
			// If already inactive, the region continues

		case innerUpper == "ELSE":
			if len(stack) == 0 {
				continue
			}
			top := &stack[len(stack)-1]
			if top.nestDepth > 0 {
				continue
			}
			if !top.firstBranchDone {
				top.firstBranchDone = true
				top.inactiveStart = i
			}

		case innerUpper == "END_IF":
			if len(stack) == 0 {
				continue
			}
			top := &stack[len(stack)-1]
			if top.nestDepth > 0 {
				top.nestDepth--
				continue
			}
			if top.inactiveStart >= 0 {
				// Close the inactive region at the line before END_IF
				endLine := i - 1
				if endLine >= top.inactiveStart {
					regions = append(regions, inactiveRegion{
						startLine: top.inactiveStart,
						endLine:   endLine,
					})
				}
			}
			stack = stack[:len(stack)-1]
		}
	}

	return regions
}

// handleSemanticTokensFull returns an LSP handler for textDocument/semanticTokens/full.
// It encodes inactive preprocessor regions as "comment" typed semantic tokens
// so the editor grays them out.
func handleSemanticTokensFull(store *DocumentStore) protocol.TextDocumentSemanticTokensFullFunc {
	return func(ctx *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
		doc := store.Get(params.TextDocument.URI)
		if doc == nil {
			return &protocol.SemanticTokens{Data: []uint32{}}, nil
		}

		regions := findInactiveRegions(doc.Content)
		if len(regions) == 0 {
			return &protocol.SemanticTokens{Data: []uint32{}}, nil
		}

		lines := strings.Split(doc.Content, "\n")
		var data []uint32
		prevLine := 0

		for _, region := range regions {
			for line := region.startLine; line <= region.endLine; line++ {
				if line >= len(lines) {
					break
				}
				lineContent := strings.TrimRight(lines[line], "\r")
				lineLen := len(lineContent)
				if lineLen == 0 {
					continue
				}

				deltaLine := line - prevLine
				deltaStartChar := 0
				if deltaLine == 0 && prevLine == 0 && line == 0 {
					deltaStartChar = 0
				}

				data = append(data,
					uint32(deltaLine),     // deltaLine
					uint32(deltaStartChar), // deltaStartChar
					uint32(lineLen),        // length
					0,                      // tokenType (0 = "comment")
					0,                      // tokenModifiers (none)
				)
				prevLine = line
			}
		}

		return &protocol.SemanticTokens{Data: data}, nil
	}
}
