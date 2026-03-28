package lsp

import (
	"fmt"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// handleHover returns a TextDocumentHoverFunc that shows type information
// for the symbol at the cursor position.
func handleHover(store *DocumentStore) protocol.TextDocumentHoverFunc {
	return func(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
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

		typeStr := symbolTypeString(sym)
		content := fmt.Sprintf("**%s** `%s` : `%s`", sym.Kind.String(), sym.Name, typeStr)

		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: content,
			},
		}, nil
	}
}
