package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/symbols"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// readProbe loads a committed TwinCAT probe fixture.
func readProbe(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "twincat_probes", name))
	if err != nil {
		t.Fatalf("read probe %s: %v", name, err)
	}
	return string(data)
}

// posOf returns the 1-based line and column of the first occurrence of needle
// on the first line that contains marker.
func posOf(t *testing.T, src, marker, needle string) (int, int) {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, marker) {
			c := strings.Index(l, needle)
			if c < 0 {
				t.Fatalf("needle %q not on line %q", needle, l)
			}
			return i + 1, c + 1
		}
	}
	t.Fatalf("marker %q not found", marker)
	return 0, 0
}

const ectMainSrc = `PROGRAM MAIN
VAR
	b : BOOL;
END_VAR
b := ECT.X.I1;
END_PROGRAM
`

// TestTwinCAT_GVLHoverAndNavigation opens the ECT GVL fixture (attributes,
// PERSISTENT RETAIN, TcLinkTo) next to a PROGRAM that reads ECT.X.I1, then
// exercises ident lookup, hover, definition and references.
func TestTwinCAT_GVLHoverAndNavigation(t *testing.T) {
	store := NewDocumentStore()
	ect := readProbe(t, "ECT.st")
	store.Open("file:///ECT.st", ect, 1)
	mainDoc := store.Open("file:///main.st", ectMainSrc, 1)
	if mainDoc.ParseResult == nil || len(mainDoc.ParseResult.Diags) != 0 {
		t.Fatalf("main.st parse diagnostics: %v", mainDoc.ParseResult.Diags)
	}

	line, col := posOf(t, ectMainSrc, "ECT.X.I1", "ECT")
	ident := findIdentAtPosition(mainDoc.ParseResult.File, line, col)
	if ident == nil || !strings.EqualFold(ident.Name, "ECT") {
		t.Fatalf("findIdentAtPosition on ECT = %v, want ECT", ident)
	}

	// Hover on the GVL member declaration X in ECT.st must mention its struct type.
	ectDoc := store.Get("file:///ECT.st")
	xLine, xCol := posOf(t, ect, "X : ST_EL1008", "X")
	hover, err := handleHover(store)(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///ECT.st"},
			Position:     protocol.Position{Line: uint32(xLine - 1), Character: uint32(xCol - 1)},
		},
	})
	if err != nil {
		t.Fatalf("hover error: %v", err)
	}
	if hover == nil {
		t.Fatal("expected hover on GVL member X")
	}
	mc, ok := hover.Contents.(protocol.MarkupContent)
	if !ok || !strings.Contains(strings.ToUpper(mc.Value), "ST_EL1008") {
		t.Errorf("hover on X = %#v, want text containing ST_EL1008", hover.Contents)
	}

	// Hovering on every attribute line must not panic, whatever it returns.
	for i, l := range strings.Split(ect, "\n") {
		if !strings.Contains(l, "{attribute") {
			continue
		}
		for c := 0; c < len(l); c += 7 {
			_, _ = handleHover(store)(nil, &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: "file:///ECT.st"},
					Position:     protocol.Position{Line: uint32(i), Character: uint32(c)},
				},
			})
		}
	}

	// Definition and references on ECT in main.st complete without error.
	defParams := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///main.st"},
			Position:     protocol.Position{Line: uint32(line - 1), Character: uint32(col - 1)},
		},
	}
	if _, err := handleDefinition(store)(nil, defParams); err != nil {
		t.Errorf("definition error: %v", err)
	}
	refs, err := handleReferences(store)(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: defParams.TextDocumentPositionParams,
	})
	if err != nil {
		t.Errorf("references error: %v", err)
	}
	_ = refs

	// nLost appears only at its declaration in ECT.st.
	if got := findAllReferences(ectDoc, "nLost"); len(got) != 1 {
		t.Errorf("references to nLost in ECT.st = %d, want 1", len(got))
	}
}

// TestTwinCAT_ActionReferences opens action_inside.st (ACTIONs after METHODs,
// pragmas inside actions) and checks references and hover on action names.
func TestTwinCAT_ActionReferences(t *testing.T) {
	store := NewDocumentStore()
	src := readProbe(t, "action_inside.st")
	doc := store.Open("file:///action_inside.st", src, 1)
	if doc.ParseResult == nil || len(doc.ParseResult.Diags) != 0 {
		t.Fatalf("action_inside.st parse diagnostics: %v", doc.ParseResult.Diags)
	}

	callLine, _ := posOf(t, src, "A100_input();", "A100_input")
	declLine, _ := posOf(t, src, "ACTION A100_input", "A100_input")
	refs := findAllReferences(doc, "A100_input")
	gotLines := map[int]bool{}
	for _, p := range refs {
		gotLines[p.Line] = true
	}
	if !gotLines[callLine] || !gotLines[declLine] {
		t.Errorf("references to A100_input on lines %v, want call line %d and ACTION line %d", refs, callLine, declLine)
	}

	// Action declared inside an FUNCTION_BLOCK after a METHOD: the call site
	// in the FB body and the ACTION name both resolve.
	coeRefs := findAllReferences(doc, "coe")
	if len(coeRefs) != 2 {
		t.Errorf("references to coe = %d, want 2", len(coeRefs))
	}

	// Ident lookup and hover across every position of every ACTION line must not panic.
	for i, l := range strings.Split(src, "\n") {
		if !strings.Contains(l, "ACTION") {
			continue
		}
		for c := 0; c < len(l); c++ {
			_ = findIdentAtPosition(doc.ParseResult.File, i+1, c+1)
			_ = findSymbolAtPosition(doc, i+1, c+1)
		}
	}

	// References through the handler at the ACTION name position.
	dl, dc := posOf(t, src, "ACTION A100_input", "A100_input")
	locs, err := handleReferences(store)(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///action_inside.st"},
			Position:     protocol.Position{Line: uint32(dl - 1), Character: uint32(dc - 1)},
		},
	})
	if err != nil {
		t.Fatalf("references error: %v", err)
	}
	_ = locs
}

// TestTwinCAT_SemanticTokensWithAttributes runs semantic tokens over every
// fixture carrying {attribute ...} or {warning ...} pragmas.
func TestTwinCAT_SemanticTokensWithAttributes(t *testing.T) {
	for _, name := range []string{"ECT.st", "enum_attr.st", "gvl1.st", "gvl2.st", "structat.st", "structpragma.st", "action_inside.st"} {
		t.Run(name, func(t *testing.T) {
			store := NewDocumentStore()
			uri := "file:///" + name
			store.Open(uri, readProbe(t, name), 1)
			toks, err := handleSemanticTokensFull(store)(nil, &protocol.SemanticTokensParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			})
			if err != nil {
				t.Fatalf("semantic tokens error: %v", err)
			}
			if toks == nil {
				t.Fatal("semantic tokens returned nil")
			}
			if len(toks.Data)%5 != 0 {
				t.Errorf("semantic token data length %d not a multiple of 5", len(toks.Data))
			}
		})
	}
}

// TestTwinCAT_QualifiedOnlyGVLMemberLookup checks that a member shared by two
// qualified_only GVLs resolves to the GVL in the cursor's own file, and that a
// name no GVL declares does not resolve.
func TestTwinCAT_QualifiedOnlyGVLMemberLookup(t *testing.T) {
	const gvlA = "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tv : INT;\nEND_VAR\n"
	const gvlB = "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tv : BOOL;\nEND_VAR\n"
	store := NewDocumentStore()
	store.Open("file:///GA.st", gvlA, 1)
	store.Open("file:///GB.st", gvlB, 1)
	store.Open("file:///GC.st", gvlB, 1) // third GVL forces the name-order tie-break

	for uri, want := range map[string]string{"file:///GA.st": "INT", "file:///GB.st": "BOOL"} {
		doc := store.Get(uri)
		sym := findSymbolAtPosition(doc, 3, 2)
		if sym == nil {
			t.Fatalf("%s: no symbol for v", uri)
		}
		if got := symbolTypeString(sym); !strings.HasSuffix(got, want) {
			t.Errorf("%s: v type = %q, want suffix %q", uri, got, want)
		}
	}

	doc := store.Get("file:///GA.st")
	if sym := findGVLMember(doc.AnalysisResult.Symbols, &ast.Ident{Name: "nope"}); sym != nil {
		t.Errorf("findGVLMember(nope) = %v, want nil", sym)
	}
}

// TestTwinCAT_GVLMemberNonStructType covers a GVL symbol whose type is not a
// struct: it is skipped rather than type-asserted unsafely.
func TestTwinCAT_GVLMemberNonStructType(t *testing.T) {
	table := symbols.NewTable()
	_ = table.GlobalScope().Insert(&symbols.Symbol{
		Name: "G",
		Kind: symbols.KindGVL,
		GVL:  &symbols.GVLInfo{Vars: map[string]bool{"V": true}},
	})
	if sym := findGVLMember(table, &ast.Ident{Name: "v"}); sym != nil {
		t.Errorf("findGVLMember on non-struct GVL = %v, want nil", sym)
	}
}
