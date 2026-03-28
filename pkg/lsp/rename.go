package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// handleRename returns a TextDocumentRenameFunc that renames a symbol
// and all its references within the document.
func handleRename(store *DocumentStore) protocol.TextDocumentRenameFunc {
	return func(ctx *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
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

		oldName := ident.Name
		refs := findAllReferences(doc, oldName)
		if len(refs) == 0 {
			return nil, nil
		}

		edits := make([]protocol.TextEdit, 0, len(refs))
		for _, pos := range refs {
			refLine := uint32(0)
			refCol := uint32(0)
			if pos.Line > 0 {
				refLine = uint32(pos.Line - 1)
			}
			if pos.Col > 0 {
				refCol = uint32(pos.Col - 1)
			}
			edits = append(edits, protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: refLine, Character: refCol},
					End:   protocol.Position{Line: refLine, Character: refCol + uint32(len(oldName))},
				},
				NewText: params.NewName,
			})
		}

		return &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentUri][]protocol.TextEdit{
				params.TextDocument.URI: edits,
			},
		}, nil
	}
}
