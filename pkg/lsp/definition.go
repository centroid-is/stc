package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// handleDefinition returns a TextDocumentDefinitionFunc that resolves
// go-to-definition by looking up the symbol at the cursor position.
func handleDefinition(store *DocumentStore) protocol.TextDocumentDefinitionFunc {
	return func(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
		doc := store.Get(params.TextDocument.URI)
		if doc == nil {
			return nil, nil
		}

		// Convert 0-based LSP position to 1-based stc position
		line := int(params.Position.Line) + 1
		col := int(params.Position.Character) + 1

		sym := findSymbolAtPosition(doc, line, col)
		if sym == nil {
			return nil, nil
		}

		// Build the target URI from the symbol's declaration position
		uri := params.TextDocument.URI
		if sym.Pos.File != "" {
			uri = "file://" + sym.Pos.File
		}

		// Convert 1-based stc position back to 0-based LSP position
		defLine := uint32(0)
		defCol := uint32(0)
		if sym.Pos.Line > 0 {
			defLine = uint32(sym.Pos.Line - 1)
		}
		if sym.Pos.Col > 0 {
			defCol = uint32(sym.Pos.Col - 1)
		}

		return []protocol.Location{
			{
				URI: uri,
				Range: protocol.Range{
					Start: protocol.Position{Line: defLine, Character: defCol},
					End:   protocol.Position{Line: defLine, Character: defCol + uint32(len(sym.Name))},
				},
			},
		}, nil
	}
}
