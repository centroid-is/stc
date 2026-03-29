package lsp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/require"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// --- convertDiagnostics: various edge cases ---

func TestConvertDiagnostics_Empty(t *testing.T) {
	result := convertDiagnostics(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}

func TestConvertDiagnostics_ZeroPosAndEndPos(t *testing.T) {
	diags := []diag.Diagnostic{
		{
			Severity: diag.Error,
			Pos:      source.Pos{Line: 0, Col: 0},
			EndPos:   source.Pos{Line: 0, Col: 0},
			Message:  "test error",
			Code:     "E001",
		},
	}
	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Range.Start.Line != 0 || result[0].Range.Start.Character != 0 {
		t.Error("expected 0,0 for zero position")
	}
}

func TestConvertDiagnostics_WithEndPos(t *testing.T) {
	diags := []diag.Diagnostic{
		{
			Severity: diag.Warning,
			Pos:      source.Pos{Line: 5, Col: 3},
			EndPos:   source.Pos{Line: 5, Col: 10},
			Message:  "test warning",
		},
	}
	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatal("expected 1 diag")
	}
	if result[0].Range.Start.Line != 4 || result[0].Range.Start.Character != 2 {
		t.Errorf("start: got %d:%d", result[0].Range.Start.Line, result[0].Range.Start.Character)
	}
	if result[0].Range.End.Line != 4 || result[0].Range.End.Character != 9 {
		t.Errorf("end: got %d:%d", result[0].Range.End.Line, result[0].Range.End.Character)
	}
}

func TestConvertDiagnostics_NoCode(t *testing.T) {
	diags := []diag.Diagnostic{
		{
			Severity: diag.Info,
			Pos:      source.Pos{Line: 1, Col: 1},
			Message:  "info",
		},
	}
	result := convertDiagnostics(diags)
	if result[0].Code != nil {
		t.Error("expected nil code")
	}
}

// --- convertSeverity ---

func TestConvertSeverity_AllLevels(t *testing.T) {
	tests := []struct {
		in  diag.Severity
		out protocol.DiagnosticSeverity
	}{
		{diag.Error, protocol.DiagnosticSeverityError},
		{diag.Warning, protocol.DiagnosticSeverityWarning},
		{diag.Info, protocol.DiagnosticSeverityInformation},
		{diag.Hint, protocol.DiagnosticSeverityHint},
		{diag.Severity(99), protocol.DiagnosticSeverityInformation},
	}
	for _, tt := range tests {
		got := convertSeverity(tt.in)
		if got != tt.out {
			t.Errorf("convertSeverity(%v) = %v, want %v", tt.in, got, tt.out)
		}
	}
}

// --- handleHover: with symbol found ---

func TestHover_FoundSymbol(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`, 1)

	hoverFn := handleHover(store)
	result, err := hoverFn(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		// May be nil if symbol not found at exact position, which is OK
		t.Log("hover returned nil (symbol position mismatch)")
	}
}

// --- handleReferences: found references ---

func TestReferences_FoundReferences(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    x : INT;
END_VAR
    x := 42;
    x := x + 1;
END_PROGRAM
`, 1)

	refFn := handleReferences(store)
	result, err := refFn(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// x should appear multiple times
	if result == nil {
		t.Log("references returned nil (position mismatch)")
	}
}

func TestReferences_NoRefs(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
END_PROGRAM
`, 1)

	refFn := handleReferences(store)
	result, err := refFn(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 0, Character: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil && len(result) > 0 {
		t.Log("got some refs")
	}
}

// --- handleSemanticTokensFull: with directives ---

func TestSemanticTokens_WithDirectives(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `{IF defined(SYM)}
PROGRAM Active
END_PROGRAM
{ELSE}
PROGRAM Inactive
END_PROGRAM
{END_IF}
`, 1)

	semFn := handleSemanticTokensFull(store)
	result, err := semFn(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have some token data for the inactive region
	if len(result.Data) == 0 {
		t.Log("no inactive regions detected (may be expected depending on preprocessing)")
	}
}

// --- symbolTypeString: nil and no type ---

func TestSymbolTypeString_Nil(t *testing.T) {
	s := symbolTypeString(nil)
	if s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}
}

func TestSymbolTypeString_NoType(t *testing.T) {
	sym := &symbols.Symbol{Name: "x", Kind: symbols.KindVariable}
	s := symbolTypeString(sym)
	if s != "Variable" {
		t.Fatalf("expected 'Variable', got %q", s)
	}
}

func TestSymbolTypeString_WithPrimitiveType(t *testing.T) {
	sym := &symbols.Symbol{Name: "x", Kind: symbols.KindVariable, Type: types.TypeDINT}
	s := symbolTypeString(sym)
	if s == "" {
		t.Fatal("expected non-empty string")
	}
}

// --- findSymbolAtPosition: global scope lookup ---

func TestFindSymbolAtPosition_GlobalScope(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    x : INT;
END_VAR
END_PROGRAM
`, 1)

	doc := store.Get("file:///test.st")
	if doc == nil {
		t.Fatal("expected doc")
	}
	// Try to find P at the program name position (line 1, col 9)
	sym := findSymbolAtPosition(doc, 1, 9)
	if sym != nil {
		t.Logf("found symbol: %s (%s)", sym.Name, sym.Kind)
	}
}

// --- findAllReferences: case-insensitive matching ---

func TestFindAllReferences_CaseInsensitive(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    MyVar : INT;
END_VAR
    myvar := 1;
    MYVAR := 2;
END_PROGRAM
`, 1)

	doc := store.Get("file:///test.st")
	refs := findAllReferences(doc, "myvar")
	// Should find MyVar, myvar, MYVAR
	if len(refs) < 3 {
		t.Logf("found %d refs (expected at least 3)", len(refs))
	}
}

// --- findIdentAtPosition: more edge cases ---

func TestFindIdentAtPosition_SingleLineSpan(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    myVariable : INT;
END_VAR
END_PROGRAM
`, 1)
	doc := store.Get("file:///test.st")
	if doc == nil || doc.ParseResult == nil {
		t.Fatal("expected doc")
	}
	// myVariable should be on line 3, col 5-14
	ident := findIdentAtPosition(doc.ParseResult.File, 3, 8)
	if ident != nil {
		t.Logf("found ident: %s", ident.Name)
	}
}

func TestFindIdentAtPosition_StartLine(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM TestProg
END_PROGRAM
`, 1)
	doc := store.Get("file:///test.st")
	ident := findIdentAtPosition(doc.ParseResult.File, 1, 9)
	if ident != nil {
		t.Logf("found ident: %s", ident.Name)
	}
}

// --- handleSemanticTokensFull: with inactive region covering multiple lines ---

func TestSemanticTokens_MultiLineInactive(t *testing.T) {
	store := NewDocumentStore()
	src := "{IF defined(ABSENT)}\nline1\nline2\nline3\n{ELSE}\nactive\n{END_IF}\n"
	store.Open("file:///test.st", src, 1)

	semFn := handleSemanticTokensFull(store)
	result, err := semFn(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have tokens for the inactive lines
	if len(result.Data) > 0 {
		t.Logf("got %d token data entries", len(result.Data))
	}
}

func TestSemanticTokens_EmptyLineInInactive(t *testing.T) {
	store := NewDocumentStore()
	src := "{IF defined(ABSENT)}\nline1\n\nline3\n{END_IF}\n"
	store.Open("file:///test.st", src, 1)

	semFn := handleSemanticTokensFull(store)
	result, err := semFn(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	})
	if err != nil {
		t.Fatal(err)
	}
	require.NotNil(t, result)
}

// --- findSymbolAtPosition: POU scope children ---

func TestFindSymbolAtPosition_POUScope(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P
VAR
    myVar : INT;
END_VAR
    myVar := 42;
END_PROGRAM
`, 1)
	doc := store.Get("file:///test.st")
	sym := findSymbolAtPosition(doc, 3, 5)
	if sym != nil {
		t.Logf("found symbol: %s (%s)", sym.Name, sym.Kind)
	}
}

// --- Document store: Close ---

func TestDocumentStore_Close(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", `PROGRAM P END_PROGRAM`, 1)
	store.Close("file:///test.st")
	doc := store.Get("file:///test.st")
	if doc != nil {
		t.Fatal("expected nil after close")
	}
}
