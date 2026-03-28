package lsp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// findIdentAtPosition walks the AST to find the ast.Ident node whose span
// contains the given 1-based line:col position. Returns nil if no ident found.
func findIdentAtPosition(file *ast.SourceFile, line, col int) *ast.Ident {
	if file == nil {
		return nil
	}
	var found *ast.Ident
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		span := node.Span()

		// Quick rejection: if the node's span doesn't contain the line, skip
		if span.Start.Line > line || span.End.Line < line {
			return false
		}
		// On a single-line span, check column bounds
		if span.Start.Line == line && span.End.Line == line {
			if col < span.Start.Col || col > span.End.Col {
				return false
			}
		}
		// On the start line, check col >= start
		if span.Start.Line == line && col < span.Start.Col {
			return false
		}
		// On the end line, check col <= end
		if span.End.Line == line && col > span.End.Col {
			return false
		}

		if ident, ok := node.(*ast.Ident); ok {
			found = ident
			return false // stop descending
		}
		return true // descend into children
	})
	return found
}

// findSymbolAtPosition locates the symbol at the given 1-based line:col
// position in the document. It finds the ident at position, then looks up
// the corresponding symbol in the symbol table.
func findSymbolAtPosition(doc *Document, line, col int) *symbols.Symbol {
	if doc == nil || doc.ParseResult == nil || doc.ParseResult.File == nil {
		return nil
	}
	if doc.AnalysisResult == nil || doc.AnalysisResult.Symbols == nil {
		return nil
	}

	ident := findIdentAtPosition(doc.ParseResult.File, line, col)
	if ident == nil {
		return nil
	}

	table := doc.AnalysisResult.Symbols

	// Try global scope first
	if sym := table.GlobalScope().Lookup(ident.Name); sym != nil {
		return sym
	}

	// Try each POU scope's children
	for _, child := range table.GlobalScope().Children {
		if sym := child.Lookup(ident.Name); sym != nil {
			return sym
		}
	}

	return nil
}

// findAllReferences walks the AST collecting all ast.Ident nodes whose name
// matches (case-insensitive) and returns their start positions.
func findAllReferences(doc *Document, name string) []ast.Pos {
	if doc == nil || doc.ParseResult == nil || doc.ParseResult.File == nil {
		return nil
	}

	var positions []ast.Pos
	ast.Inspect(doc.ParseResult.File, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		if ident, ok := node.(*ast.Ident); ok {
			if strings.EqualFold(ident.Name, name) {
				positions = append(positions, ident.Span().Start)
			}
		}
		return true
	})
	return positions
}

// collectAllSymbols recursively gathers symbols from the global scope
// and all child scopes. Used by the completion handler.
func collectAllSymbols(table *symbols.Table) []*symbols.Symbol {
	if table == nil {
		return nil
	}
	var result []*symbols.Symbol
	collectScopeSymbols(table.GlobalScope(), &result)
	return result
}

// collectScopeSymbols recursively collects symbols from a scope and its children.
func collectScopeSymbols(scope *symbols.Scope, result *[]*symbols.Symbol) {
	if scope == nil {
		return
	}
	*result = append(*result, scope.Symbols()...)
	for _, child := range scope.Children {
		collectScopeSymbols(child, result)
	}
}

// symbolTypeString returns a human-readable description of a symbol's kind and type.
// Format: "Kind: TypeString" or just "Kind" if no type info.
func symbolTypeString(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	kindStr := sym.Kind.String()
	if sym.Type != nil {
		if t, ok := sym.Type.(types.Type); ok {
			return fmt.Sprintf("%s: %s", kindStr, t.String())
		}
	}
	return kindStr
}
