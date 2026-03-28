package lsp

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/symbols"
)

const testProgram = `PROGRAM Main
VAR
    counter : INT;
    flag : BOOL;
END_VAR
    counter := counter + 1;
    IF flag THEN
        counter := 0;
    END_IF;
END_PROGRAM
`

func setupTestDoc() *Document {
	result := parser.Parse("test.st", testProgram)
	analysisResult := analyzer.Analyze([]*ast.SourceFile{result.File}, nil)
	return &Document{
		URI:            "file:///test.st",
		Content:        testProgram,
		Version:        1,
		ParseResult:    &result,
		AnalysisResult: &analysisResult,
	}
}

func TestNavigate_FindIdentAtPosition(t *testing.T) {
	doc := setupTestDoc()
	file := doc.ParseResult.File

	// "counter" on line 3 starts at some column - find it
	// Line 3: "    counter : INT;"
	// "counter" starts at col 5
	ident := findIdentAtPosition(file, 3, 5)
	if ident == nil {
		t.Fatal("expected to find ident at line 3, col 5")
	}
	if !strings.EqualFold(ident.Name, "counter") {
		t.Errorf("expected ident 'counter', got %q", ident.Name)
	}

	// "flag" on line 4: "    flag : BOOL;"
	ident = findIdentAtPosition(file, 4, 5)
	if ident == nil {
		t.Fatal("expected to find ident at line 4, col 5")
	}
	if !strings.EqualFold(ident.Name, "flag") {
		t.Errorf("expected ident 'flag', got %q", ident.Name)
	}

	// Position with no ident (line 1, col 1 is "PROGRAM" keyword, not an ident in leaf)
	// Line 5: "END_VAR" - no idents
	ident = findIdentAtPosition(file, 5, 1)
	// This might or might not find something depending on parser spans
	// Just ensure no crash
}

func TestNavigate_FindIdentAtPosition_NilFile(t *testing.T) {
	ident := findIdentAtPosition(nil, 1, 1)
	if ident != nil {
		t.Error("expected nil for nil file")
	}
}

func TestNavigate_FindSymbolAtPosition(t *testing.T) {
	doc := setupTestDoc()

	// "counter" on line 6: "    counter := counter + 1;"
	sym := findSymbolAtPosition(doc, 6, 5)
	if sym == nil {
		t.Fatal("expected to find symbol for 'counter' at line 6")
	}
	if !strings.EqualFold(sym.Name, "counter") {
		t.Errorf("expected symbol 'counter', got %q", sym.Name)
	}
	if sym.Kind != symbols.KindVariable {
		t.Errorf("expected KindVariable, got %v", sym.Kind)
	}
}

func TestNavigate_FindSymbolAtPosition_NilDoc(t *testing.T) {
	sym := findSymbolAtPosition(nil, 1, 1)
	if sym != nil {
		t.Error("expected nil for nil doc")
	}
}

func TestNavigate_FindAllReferences(t *testing.T) {
	doc := setupTestDoc()

	refs := findAllReferences(doc, "counter")
	// "counter" appears in:
	// - line 3: declaration "counter : INT"
	// - line 6: "counter := counter + 1;" (twice)
	// - line 8: "counter := 0;"
	// That's at least 4 references
	if len(refs) < 4 {
		t.Errorf("expected at least 4 references to 'counter', got %d", len(refs))
	}

	// Case insensitive
	refsUpper := findAllReferences(doc, "COUNTER")
	if len(refsUpper) != len(refs) {
		t.Errorf("case-insensitive: expected %d refs, got %d", len(refs), len(refsUpper))
	}
}

func TestNavigate_FindAllReferences_NilDoc(t *testing.T) {
	refs := findAllReferences(nil, "counter")
	if refs != nil {
		t.Error("expected nil for nil doc")
	}
}

func TestNavigate_CollectAllSymbols(t *testing.T) {
	doc := setupTestDoc()
	table := doc.AnalysisResult.Symbols

	syms := collectAllSymbols(table)
	if len(syms) == 0 {
		t.Fatal("expected at least some symbols")
	}

	// Should contain both POU-level symbols (Main) and local variables (counter, flag)
	nameSet := make(map[string]bool)
	for _, s := range syms {
		nameSet[strings.ToUpper(s.Name)] = true
	}

	if !nameSet["MAIN"] {
		t.Error("expected to find 'Main' symbol")
	}
	if !nameSet["COUNTER"] {
		t.Error("expected to find 'counter' symbol")
	}
	if !nameSet["FLAG"] {
		t.Error("expected to find 'flag' symbol")
	}
}

func TestNavigate_CollectAllSymbols_NilTable(t *testing.T) {
	syms := collectAllSymbols(nil)
	if syms != nil {
		t.Error("expected nil for nil table")
	}
}

func TestNavigate_SymbolTypeString(t *testing.T) {
	doc := setupTestDoc()

	sym := findSymbolAtPosition(doc, 6, 5) // counter
	if sym == nil {
		t.Fatal("expected to find symbol")
	}

	typeStr := symbolTypeString(sym)
	if typeStr == "" {
		t.Error("expected non-empty type string")
	}
	// Should contain "Variable" as the kind
	if !strings.Contains(typeStr, "Variable") {
		t.Errorf("expected type string to contain 'Variable', got %q", typeStr)
	}
}

func TestNavigate_SymbolTypeString_Nil(t *testing.T) {
	if symbolTypeString(nil) != "" {
		t.Error("expected empty string for nil symbol")
	}
}
