package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// handleReferences returns a TextDocumentReferencesFunc that finds all
// references to the symbol at the cursor position.
func handleReferences(store *DocumentStore) protocol.TextDocumentReferencesFunc {
	return func(ctx *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
		doc := store.Get(params.TextDocument.URI)
		if doc == nil {
			return nil, nil
		}

		// Convert 0-based LSP position to 1-based stc position
		line := int(params.Position.Line) + 1
		col := int(params.Position.Character) + 1

		ident := findIdentAtPosition(doc.ParseResult.File, line, col)
		if ident == nil {
			return nil, nil
		}

		refs := findAllReferences(doc, ident.Name)
		if len(refs) == 0 {
			return nil, nil
		}

		locations := make([]protocol.Location, 0, len(refs))
		for _, pos := range refs {
			refLine := uint32(0)
			refCol := uint32(0)
			if pos.Line > 0 {
				refLine = uint32(pos.Line - 1)
			}
			if pos.Col > 0 {
				refCol = uint32(pos.Col - 1)
			}
			locations = append(locations, protocol.Location{
				URI: params.TextDocument.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: refLine, Character: refCol},
					End:   protocol.Position{Line: refLine, Character: refCol + uint32(len(ident.Name))},
				},
			})
		}

		return locations, nil
	}
}
