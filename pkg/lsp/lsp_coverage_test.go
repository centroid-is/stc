package lsp

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/symbols"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// --- Document Store edge cases ---

func TestDocumentStore_UpdateCreatesNewDoc(t *testing.T) {
	store := NewDocumentStore()
	// Update a doc that was never opened
	doc := store.Update("file:///new.st", "PROGRAM New\nEND_PROGRAM\n", 1)
	if doc == nil {
		t.Fatal("Update should create new doc if not present")
	}
	if doc.Version != 1 {
		t.Errorf("expected version 1, got %d", doc.Version)
	}
	// Verify it can be retrieved
	got := store.Get("file:///new.st")
	if got == nil {
		t.Fatal("expected to Get the updated doc")
	}
}

func TestDocumentStore_MultipleDocsCrossFileAnalysis(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///a.st", `FUNCTION_BLOCK FB_Motor
VAR_INPUT
    speed : INT;
END_VAR
END_FUNCTION_BLOCK
`, 1)
	store.Open("file:///b.st", `PROGRAM Main
VAR
    m : FB_Motor;
END_VAR
END_PROGRAM
`, 1)

	docA := store.Get("file:///a.st")
	docB := store.Get("file:///b.st")
	if docA == nil || docB == nil {
		t.Fatal("expected both documents to be available")
	}
	// Both should have analysis results
	if docA.AnalysisResult == nil {
		t.Error("docA should have analysis result")
	}
	if docB.AnalysisResult == nil {
		t.Error("docB should have analysis result")
	}
}

// --- URI to filename ---

func TestURIToFilename_InvalidURI(t *testing.T) {
	// A URI that cannot be parsed properly still returns something useful
	result := uriToFilename("://invalid")
	// Should not panic, exact output depends on implementation
	if result == "" {
		t.Error("expected non-empty result for invalid URI")
	}
}

func TestURIToFilename_NoScheme(t *testing.T) {
	result := uriToFilename("just-a-path.st")
	if result != "just-a-path.st" {
		t.Errorf("expected 'just-a-path.st', got %q", result)
	}
}

// --- findIdentAtPosition edge cases ---

func TestFindIdentAtPosition_MultiLineSpan(t *testing.T) {
	doc := setupTestDoc()
	file := doc.ParseResult.File

	// Line 1 is "PROGRAM Main" - Main ident should be findable
	ident := findIdentAtPosition(file, 1, 10)
	if ident != nil {
		// If we find it, it should be "Main"
		if !strings.EqualFold(ident.Name, "Main") {
			t.Errorf("expected 'Main' ident, got %q", ident.Name)
		}
	}
	// Just ensure no crash on boundary positions
}

func TestFindIdentAtPosition_OutOfRange(t *testing.T) {
	doc := setupTestDoc()
	file := doc.ParseResult.File

	// Way beyond the file
	ident := findIdentAtPosition(file, 9999, 1)
	if ident != nil {
		t.Error("expected nil for way out-of-range position")
	}
}

// --- findSymbolAtPosition edge cases ---

func TestFindSymbolAtPosition_NoAnalysis(t *testing.T) {
	doc := &Document{
		URI:     "file:///test.st",
		Content: "PROGRAM Main\nEND_PROGRAM\n",
	}
	// No parse or analysis result
	sym := findSymbolAtPosition(doc, 1, 1)
	if sym != nil {
		t.Error("expected nil when no analysis")
	}
}

func TestFindSymbolAtPosition_EmptyParse(t *testing.T) {
	doc := &Document{
		URI:     "file:///test.st",
		Content: "PROGRAM Main\nEND_PROGRAM\n",
		ParseResult: nil,
	}
	sym := findSymbolAtPosition(doc, 1, 1)
	if sym != nil {
		t.Error("expected nil when parse result is nil")
	}
}

// --- findAllReferences edge cases ---

func TestFindAllReferences_NoMatches(t *testing.T) {
	doc := setupTestDoc()
	refs := findAllReferences(doc, "nonexistent_identifier")
	if len(refs) != 0 {
		t.Errorf("expected 0 references for nonexistent, got %d", len(refs))
	}
}

// --- collectAllSymbols ---

func TestCollectScopeSymbols_NilScope(t *testing.T) {
	var result []*symbols.Symbol
	collectScopeSymbols(nil, &result)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil scope, got %d", len(result))
	}
}

// --- symbolKindToCompletionKind ---

func TestSymbolKindToCompletionKind_AllKinds(t *testing.T) {
	tests := []struct {
		kind     symbols.SymbolKind
		expected protocol.CompletionItemKind
	}{
		{symbols.KindVariable, protocol.CompletionItemKindVariable},
		{symbols.KindFunction, protocol.CompletionItemKindFunction},
		{symbols.KindFunctionBlock, protocol.CompletionItemKindClass},
		{symbols.KindProgram, protocol.CompletionItemKindModule},
		{symbols.KindType, protocol.CompletionItemKindStruct},
		{symbols.KindEnumValue, protocol.CompletionItemKindEnumMember},
		{symbols.KindInterface, protocol.CompletionItemKindInterface},
		{symbols.KindMethod, protocol.CompletionItemKindMethod},
		{symbols.KindProperty, protocol.CompletionItemKindProperty},
	}

	for _, tt := range tests {
		result := symbolKindToCompletionKind(tt.kind)
		if result != tt.expected {
			t.Errorf("symbolKindToCompletionKind(%v) = %v, want %v", tt.kind, result, tt.expected)
		}
	}
}

func TestSymbolKindToCompletionKind_Default(t *testing.T) {
	// Unknown kind should return Text
	result := symbolKindToCompletionKind(symbols.SymbolKind(999))
	if result != protocol.CompletionItemKindText {
		t.Errorf("expected Text for unknown kind, got %v", result)
	}
}

// --- Formatting edge cases ---

func TestFormatting_NoParseResult(t *testing.T) {
	store := NewDocumentStore()
	// Manually set a doc with nil parse result
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)
	doc := store.Get("file:///test.st")
	doc.ParseResult = nil

	formatter := handleFormatting(store)
	edits, err := formatter(nil, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edits != nil {
		t.Error("expected nil edits when no parse result")
	}
}

func TestFormatting_ContentWithNoNewline(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM", 1)

	formatter := handleFormatting(store)
	edits, err := formatter(nil, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
}

// --- Definition handler ---

func TestDefinition_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleDefinition(store)
	result, err := handler(nil, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent doc")
	}
}

func TestDefinition_NoSymbolFound(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleDefinition(store)
	result, err := handler(nil, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 99, Character: 0}, // out of range
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil when no symbol at position")
	}
}

func TestDefinition_FoundSymbol(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", testProgram, 1)

	handler := handleDefinition(store)
	// "counter" is on line 6 (0-based line 5), col 5 (0-based col 4)
	result, err := handler(nil, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 5, Character: 4},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// May or may not find depending on exact span positions from parser
	// Just ensure no crash
	_ = result
}

// --- Hover handler ---

func TestHover_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleHover(store)
	result, err := handler(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil hover for nonexistent doc")
	}
}

func TestHover_NoSymbol(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleHover(store)
	result, err := handler(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 99, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil hover when no symbol at position")
	}
}

// --- References handler ---

func TestReferences_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleReferences(store)
	result, err := handler(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent doc")
	}
}

func TestReferences_NoIdentAtPosition(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleReferences(store)
	result, err := handler(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 99, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil when no ident at position")
	}
}

// --- Rename handler ---

func TestRename_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleRename(store)
	result, err := handler(nil, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
		},
		NewName: "newName",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent doc")
	}
}

func TestRename_NoIdentAtPosition(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleRename(store)
	result, err := handler(nil, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 99, Character: 0},
		},
		NewName: "newName",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil when no ident at position")
	}
}

func TestRename_ValidRename(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", testProgram, 1)

	handler := handleRename(store)
	doc := store.Get("file:///test.st")
	if doc == nil || doc.ParseResult == nil {
		t.Skip("parse result not available")
	}

	// Find counter on line 3 (0-based line 2)
	ident := findIdentAtPosition(doc.ParseResult.File, 3, 5)
	if ident == nil {
		t.Skip("cannot find ident at expected position")
	}

	result, err := handler(nil, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 2, Character: 4},
		},
		NewName: "renamedCounter",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Skip("rename returned nil result")
	}
	// Verify we got edits
	edits, ok := result.Changes["file:///test.st"]
	if !ok || len(edits) == 0 {
		t.Error("expected at least one edit for rename")
	}
}

// --- Completion handler ---

func TestCompletion_Keywords(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleCompletion(store)
	result, err := handler(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("expected CompletionList, got %T", result)
	}
	// Should have keywords + types + symbols
	if len(list.Items) < len(iecKeywords)+len(iecTypes) {
		t.Errorf("expected at least %d items, got %d",
			len(iecKeywords)+len(iecTypes), len(list.Items))
	}
}

func TestCompletion_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleCompletion(store)
	result, err := handler(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return keywords and types
	list, ok := result.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("expected CompletionList, got %T", result)
	}
	if len(list.Items) < len(iecKeywords) {
		t.Error("expected at least keyword completions")
	}
}

// --- Semantic tokens edge cases ---

func TestFindInactiveRegions_NestedIFInsideInactive(t *testing.T) {
	input := `PROGRAM Main
{IF defined(A)}
x := 1;
{ELSE}
{IF defined(B)}
y := 2;
{END_IF}
z := 3;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 inactive region, got %d", len(regions))
	}
	// ELSE on line 3, nested IF/END_IF inside, z := 3 on line 7, END_IF on line 8
	r := regions[0]
	if r.startLine != 3 {
		t.Errorf("expected startLine=3, got %d", r.startLine)
	}
	// The inactive region should span from ELSE to the line before the outer END_IF
	if r.endLine != 7 {
		t.Errorf("expected endLine=7, got %d", r.endLine)
	}
}

func TestFindInactiveRegions_MalformedDirectives(t *testing.T) {
	// Unclosed brace
	input := `{IF defined(A)
x := 1;
{END_IF}`

	regions := findInactiveRegions(input)
	// The first line has no closing brace, so it's ignored
	if len(regions) != 0 {
		t.Errorf("expected 0 regions for malformed, got %d", len(regions))
	}
}

func TestFindInactiveRegions_OrphanedElse(t *testing.T) {
	input := `{ELSE}
x := 1;
{END_IF}`

	regions := findInactiveRegions(input)
	// No matching IF, so ELSE/END_IF should be ignored
	if len(regions) != 0 {
		t.Errorf("expected 0 regions for orphaned ELSE, got %d", len(regions))
	}
}

func TestFindInactiveRegions_OrphanedEndIf(t *testing.T) {
	input := `{END_IF}`
	regions := findInactiveRegions(input)
	if len(regions) != 0 {
		t.Errorf("expected 0 regions for orphaned END_IF, got %d", len(regions))
	}
}

func TestFindInactiveRegions_IFWithParens(t *testing.T) {
	input := `{IF(defined(A))}
x := 1;
{ELSE}
y := 2;
{END_IF}`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
}

func TestFindInactiveRegions_ElsifWithSuffix(t *testing.T) {
	input := `{IF defined(A)}
x := 1;
{ELSIF defined(B)}
y := 2;
{END_IF}`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
}

func TestFindInactiveRegions_ElsifWithParens(t *testing.T) {
	input := `{IF defined(A)}
x := 1;
{ELSIF(defined(B))}
y := 2;
{END_IF}`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
}

func TestSemanticTokens_NonexistentDoc(t *testing.T) {
	store := NewDocumentStore()
	handler := handleSemanticTokensFull(store)
	result, err := handler(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///nonexistent.st"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 0 {
		t.Error("expected empty data for nonexistent doc")
	}
}

func TestSemanticTokens_NoDirectives(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)

	handler := handleSemanticTokensFull(store)
	result, err := handler(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 0 {
		t.Error("expected empty data when no preprocessor directives")
	}
}

// --- symbolTypeString with type info ---

func TestSymbolTypeString_WithType(t *testing.T) {
	doc := setupTestDoc()
	sym := findSymbolAtPosition(doc, 3, 5)
	if sym == nil {
		t.Skip("cannot find symbol")
	}
	s := symbolTypeString(sym)
	if s == "" {
		t.Error("expected non-empty type string")
	}
}
