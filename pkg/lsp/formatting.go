package lsp

import (
	"strings"

	"github.com/centroid-is/stc/pkg/format"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// handleFormatting returns a TextDocumentFormattingFunc that formats
// the document using pkg/format and returns a full-document TextEdit.
func handleFormatting(store *DocumentStore) protocol.TextDocumentFormattingFunc {
	return func(ctx *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
		doc := store.Get(params.TextDocument.URI)
		if doc == nil {
			return nil, nil
		}

		if doc.ParseResult == nil || doc.ParseResult.File == nil {
			return nil, nil
		}

		formatted := format.Format(doc.ParseResult.File, format.DefaultFormatOptions())

		// Compute the end position of the original content
		lines := strings.Count(doc.Content, "\n")
		lastLineLen := len(doc.Content)
		if idx := strings.LastIndex(doc.Content, "\n"); idx >= 0 {
			lastLineLen = len(doc.Content) - idx - 1
		}

		return []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End: protocol.Position{
						Line:      uint32(lines),
						Character: uint32(lastLineLen),
					},
				},
				NewText: formatted,
			},
		}, nil
	}
}
