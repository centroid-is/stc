// Package symbols provides the symbol table and scope chain for
// IEC 61131-3 semantic analysis. It supports hierarchical scoping
// (Global -> POU -> Method -> Block), case-insensitive name lookup
// with original casing preserved, and usage tracking for unused
// variable detection.
package symbols

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/source"
)

// SymbolKind identifies what a symbol represents.
type SymbolKind int

const (
	KindVariable      SymbolKind = iota // Local/global variable
	KindFunction                        // FUNCTION
	KindFunctionBlock                   // FUNCTION_BLOCK
	KindProgram                         // PROGRAM
	KindType                            // TYPE declaration
	KindEnumValue                       // Enum member value
	KindInterface                       // INTERFACE
	KindMethod                          // METHOD
	KindProperty                        // PROPERTY
)

var symbolKindNames = [...]string{
	KindVariable:      "Variable",
	KindFunction:      "Function",
	KindFunctionBlock: "FunctionBlock",
	KindProgram:       "Program",
	KindType:          "Type",
	KindEnumValue:     "EnumValue",
	KindInterface:     "Interface",
	KindMethod:        "Method",
	KindProperty:      "Property",
}

// String returns the human-readable name of the symbol kind.
func (k SymbolKind) String() string {
	if int(k) >= 0 && int(k) < len(symbolKindNames) {
		return symbolKindNames[k]
	}
	return fmt.Sprintf("SymbolKind(%d)", k)
}

// Symbol represents a named entity in the program: a variable, function,
// type, etc. The Name field preserves original casing for diagnostics,
// while lookups are case-insensitive per IEC 61131-3.
type Symbol struct {
	Name     string         // Original casing of the identifier
	Kind     SymbolKind     // What this symbol represents
	Pos      source.Pos     // Declaration site position
	Span     source.Span    // Full declaration span
	Used      bool           // Whether this symbol has been referenced
	IsLibrary bool           // Whether this symbol was loaded from a vendor library stub
	ParamDir  ast.VarSection // Parameter direction (VAR_INPUT, VAR_OUTPUT, etc.)
	Type     any            // Type info — will be type-asserted by checker
}

// MarkUsed marks this symbol as having been referenced.
func (s *Symbol) MarkUsed() {
	s.Used = true
}

// String returns a human-readable representation of the symbol.
func (s *Symbol) String() string {
	return fmt.Sprintf("%s %s at %s", s.Kind, s.Name, s.Pos)
}
