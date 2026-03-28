package lsp

import (
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// IEC 61131-3 keywords for completion
var iecKeywords = []string{
	"IF", "THEN", "ELSE", "ELSIF", "END_IF",
	"FOR", "TO", "BY", "DO", "END_FOR",
	"WHILE", "END_WHILE",
	"REPEAT", "UNTIL", "END_REPEAT",
	"CASE", "OF", "END_CASE",
	"RETURN", "EXIT", "CONTINUE",
	"VAR", "VAR_INPUT", "VAR_OUTPUT", "VAR_IN_OUT", "VAR_TEMP", "VAR_GLOBAL", "END_VAR",
	"PROGRAM", "END_PROGRAM",
	"FUNCTION_BLOCK", "END_FUNCTION_BLOCK",
	"FUNCTION", "END_FUNCTION",
	"METHOD", "END_METHOD",
	"PROPERTY", "END_PROPERTY",
	"INTERFACE", "END_INTERFACE",
	"TYPE", "END_TYPE",
	"STRUCT", "END_STRUCT",
	"TRUE", "FALSE",
	"NOT", "AND", "OR", "XOR", "MOD",
}

// IEC 61131-3 primitive types for completion
var iecTypes = []string{
	"BOOL", "BYTE", "WORD", "DWORD", "LWORD",
	"SINT", "INT", "DINT", "LINT",
	"USINT", "UINT", "UDINT", "ULINT",
	"REAL", "LREAL",
	"STRING", "WSTRING",
	"TIME", "DATE", "DT", "TOD",
}

// symbolKindToCompletionKind maps stc SymbolKind to LSP CompletionItemKind.
func symbolKindToCompletionKind(kind symbols.SymbolKind) protocol.CompletionItemKind {
	switch kind {
	case symbols.KindVariable:
		return protocol.CompletionItemKindVariable
	case symbols.KindFunction:
		return protocol.CompletionItemKindFunction
	case symbols.KindFunctionBlock:
		return protocol.CompletionItemKindClass
	case symbols.KindProgram:
		return protocol.CompletionItemKindModule
	case symbols.KindType:
		return protocol.CompletionItemKindStruct
	case symbols.KindEnumValue:
		return protocol.CompletionItemKindEnumMember
	case symbols.KindInterface:
		return protocol.CompletionItemKindInterface
	case symbols.KindMethod:
		return protocol.CompletionItemKindMethod
	case symbols.KindProperty:
		return protocol.CompletionItemKindProperty
	default:
		return protocol.CompletionItemKindText
	}
}

// handleCompletion returns a TextDocumentCompletionFunc that provides
// keyword, type, and symbol completions.
func handleCompletion(store *DocumentStore) protocol.TextDocumentCompletionFunc {
	return func(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
		var items []protocol.CompletionItem

		// 1. IEC keywords
		keywordKind := protocol.CompletionItemKindKeyword
		for _, kw := range iecKeywords {
			items = append(items, protocol.CompletionItem{
				Label: kw,
				Kind:  &keywordKind,
			})
		}

		// 2. IEC primitive types
		typeKind := protocol.CompletionItemKindTypeParameter
		for _, tp := range iecTypes {
			items = append(items, protocol.CompletionItem{
				Label: tp,
				Kind:  &typeKind,
			})
		}

		// 3. Declared symbols from analysis
		doc := store.Get(params.TextDocument.URI)
		if doc != nil && doc.AnalysisResult != nil && doc.AnalysisResult.Symbols != nil {
			syms := collectAllSymbols(doc.AnalysisResult.Symbols)
			for _, sym := range syms {
				kind := symbolKindToCompletionKind(sym.Kind)
				detail := symbolTypeString(sym)
				items = append(items, protocol.CompletionItem{
					Label:  sym.Name,
					Kind:   &kind,
					Detail: &detail,
				})
			}
		}

		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        items,
		}, nil
	}
}
