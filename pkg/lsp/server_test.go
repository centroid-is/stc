package lsp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestConvertDiagnostics_Severity(t *testing.T) {
	tests := []struct {
		name     string
		severity diag.Severity
		expected protocol.DiagnosticSeverity
	}{
		{"Error", diag.Error, protocol.DiagnosticSeverityError},
		{"Warning", diag.Warning, protocol.DiagnosticSeverityWarning},
		{"Info", diag.Info, protocol.DiagnosticSeverityInformation},
		{"Hint", diag.Hint, protocol.DiagnosticSeverityHint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := []diag.Diagnostic{
				{
					Severity: tt.severity,
					Pos:      source.Pos{File: "test.st", Line: 1, Col: 1},
					EndPos:   source.Pos{File: "test.st", Line: 1, Col: 5},
					Code:     "TEST001",
					Message:  "test message",
				},
			}

			result := convertDiagnostics(diags)
			if len(result) != 1 {
				t.Fatalf("expected 1 diagnostic, got %d", len(result))
			}
			if *result[0].Severity != tt.expected {
				t.Errorf("expected severity %d, got %d", tt.expected, *result[0].Severity)
			}
		})
	}
}

func TestConvertDiagnostics_PositionMapping(t *testing.T) {
	// stc uses 1-based positions, LSP uses 0-based
	diags := []diag.Diagnostic{
		{
			Severity: diag.Error,
			Pos:      source.Pos{File: "test.st", Line: 10, Col: 5},
			EndPos:   source.Pos{File: "test.st", Line: 10, Col: 15},
			Code:     "ERR001",
			Message:  "test error",
		},
	}

	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result))
	}

	d := result[0]
	if d.Range.Start.Line != 9 {
		t.Errorf("start line: expected 9 (0-based), got %d", d.Range.Start.Line)
	}
	if d.Range.Start.Character != 4 {
		t.Errorf("start character: expected 4 (0-based), got %d", d.Range.Start.Character)
	}
	if d.Range.End.Line != 9 {
		t.Errorf("end line: expected 9 (0-based), got %d", d.Range.End.Line)
	}
	if d.Range.End.Character != 14 {
		t.Errorf("end character: expected 14 (0-based), got %d", d.Range.End.Character)
	}
}

func TestConvertDiagnostics_Source(t *testing.T) {
	diags := []diag.Diagnostic{
		{
			Severity: diag.Warning,
			Pos:      source.Pos{File: "test.st", Line: 1, Col: 1},
			Message:  "unused variable",
		},
	}

	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result))
	}
	if result[0].Source == nil || *result[0].Source != "stc" {
		t.Errorf("expected source 'stc', got %v", result[0].Source)
	}
}

func TestConvertDiagnostics_Code(t *testing.T) {
	diags := []diag.Diagnostic{
		{
			Severity: diag.Error,
			Pos:      source.Pos{File: "test.st", Line: 1, Col: 1},
			Code:     "SEMA042",
			Message:  "type mismatch",
		},
	}

	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result))
	}
	if result[0].Code == nil {
		t.Fatal("expected code to be set")
	}
	if result[0].Code.Value != "SEMA042" {
		t.Errorf("expected code 'SEMA042', got %v", result[0].Code.Value)
	}
}

func TestConvertDiagnostics_ZeroEndPos(t *testing.T) {
	// When EndPos is zero, use StartPos
	diags := []diag.Diagnostic{
		{
			Severity: diag.Error,
			Pos:      source.Pos{File: "test.st", Line: 5, Col: 3},
			EndPos:   source.Pos{}, // zero value
			Message:  "error at point",
		},
	}

	result := convertDiagnostics(diags)
	if len(result) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result))
	}

	d := result[0]
	if d.Range.End.Line != d.Range.Start.Line {
		t.Errorf("end line should equal start line when EndPos is zero")
	}
	if d.Range.End.Character != d.Range.Start.Character {
		t.Errorf("end character should equal start character when EndPos is zero")
	}
}

func TestDocumentStore_Lifecycle(t *testing.T) {
	store := NewDocumentStore()

	// Open
	doc := store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)
	if doc == nil {
		t.Fatal("Open returned nil")
	}

	// Get
	got := store.Get("file:///test.st")
	if got == nil {
		t.Fatal("Get returned nil for open document")
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}

	// Update
	doc2 := store.Update("file:///test.st", "PROGRAM Main\nVAR\n  x : INT;\nEND_VAR\nEND_PROGRAM\n", 2)
	if doc2 == nil {
		t.Fatal("Update returned nil")
	}
	if doc2.Version != 2 {
		t.Errorf("expected version 2, got %d", doc2.Version)
	}

	// Close
	store.Close("file:///test.st")
	if store.Get("file:///test.st") != nil {
		t.Error("expected nil after Close")
	}
}

func TestDocumentStore_GetNonexistent(t *testing.T) {
	store := NewDocumentStore()
	if store.Get("file:///nonexistent.st") != nil {
		t.Error("expected nil for nonexistent document")
	}
}

func TestDocumentStore_OpenTriggersParseAndAnalysis(t *testing.T) {
	store := NewDocumentStore()
	doc := store.Open("file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n", 1)

	if doc.ParseResult == nil {
		t.Fatal("ParseResult should not be nil after Open")
	}
	if doc.ParseResult.File == nil {
		t.Fatal("ParseResult.File should not be nil after Open")
	}
	if doc.AnalysisResult == nil {
		t.Fatal("AnalysisResult should not be nil after Open")
	}
}

func TestDocumentStore_UpdateTriggersReparse(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nEND_PROGRAM\n", 1)
	doc := store.Update("file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\nEND_PROGRAM\n", 2)

	if doc.ParseResult == nil {
		t.Fatal("ParseResult should not be nil after Update")
	}
	if doc.ParseResult.File == nil {
		t.Fatal("ParseResult.File should not be nil after Update")
	}
}

func TestFormatting_ProducesTextEdit(t *testing.T) {
	store := NewDocumentStore()
	store.Open("file:///test.st", "PROGRAM Main\nVAR\nx : INT;\nEND_VAR\nx := 42;\nEND_PROGRAM\n", 1)

	formatter := handleFormatting(store)
	edits, err := formatter(nil, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///test.st",
		},
	})
	if err != nil {
		t.Fatalf("formatting error: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 text edit, got %d", len(edits))
	}

	edit := edits[0]
	// Should start at 0,0
	if edit.Range.Start.Line != 0 || edit.Range.Start.Character != 0 {
		t.Errorf("expected start at 0:0, got %d:%d", edit.Range.Start.Line, edit.Range.Start.Character)
	}
	// Should have formatted content
	if edit.NewText == "" {
		t.Error("expected non-empty formatted text")
	}
}

func TestFormatting_NilDocument(t *testing.T) {
	store := NewDocumentStore()

	formatter := handleFormatting(store)
	edits, err := formatter(nil, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///nonexistent.st",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edits != nil {
		t.Errorf("expected nil edits for nonexistent document, got %v", edits)
	}
}

func TestURIToFilename(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"file:///home/user/test.st", "/home/user/test.st"},
		{"file:///C:/Users/test.st", "/C:/Users/test.st"},
		{"untitled:Untitled-1", "untitled:Untitled-1"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := uriToFilename(tt.uri)
			if got != tt.expected {
				t.Errorf("uriToFilename(%q) = %q, want %q", tt.uri, got, tt.expected)
			}
		})
	}
}
