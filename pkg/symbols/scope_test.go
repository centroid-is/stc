package symbols

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/source"
)

func TestScopeInsertAndLookup(t *testing.T) {
	scope := NewScope(nil, ScopeGlobal, "global")
	sym := &Symbol{
		Name: "counter",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := scope.Insert(sym); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Exact case
	got := scope.Lookup("counter")
	if got != sym {
		t.Fatalf("Lookup(counter) = %v, want %v", got, sym)
	}

	// All uppercase
	got = scope.Lookup("COUNTER")
	if got != sym {
		t.Fatalf("Lookup(COUNTER) = %v, want %v", got, sym)
	}

	// Mixed case
	got = scope.Lookup("Counter")
	if got != sym {
		t.Fatalf("Lookup(Counter) = %v, want %v", got, sym)
	}
}

func TestScopeParentLookup(t *testing.T) {
	parent := NewScope(nil, ScopeGlobal, "global")
	sym := &Symbol{
		Name: "x",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := parent.Insert(sym); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	child := NewScope(parent, ScopePOU, "MyProgram")
	got := child.Lookup("x")
	if got != sym {
		t.Fatalf("child.Lookup(x) = %v, want %v", got, sym)
	}
}

func TestScopeShadowing(t *testing.T) {
	parent := NewScope(nil, ScopeGlobal, "global")
	parentSym := &Symbol{
		Name: "x",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := parent.Insert(parentSym); err != nil {
		t.Fatalf("Insert parent failed: %v", err)
	}

	child := NewScope(parent, ScopePOU, "MyProgram")
	childSym := &Symbol{
		Name: "x",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 5, Col: 1},
	}
	if err := child.Insert(childSym); err != nil {
		t.Fatalf("Insert child failed: %v", err)
	}

	got := child.Lookup("x")
	if got != childSym {
		t.Fatalf("child.Lookup(x) = %v, want child's symbol %v", got, childSym)
	}

	// Parent still sees its own
	got = parent.Lookup("x")
	if got != parentSym {
		t.Fatalf("parent.Lookup(x) = %v, want parent's symbol %v", got, parentSym)
	}
}

func TestScopeRedeclaration(t *testing.T) {
	scope := NewScope(nil, ScopeGlobal, "global")
	sym1 := &Symbol{
		Name: "x",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := scope.Insert(sym1); err != nil {
		t.Fatalf("first Insert failed: %v", err)
	}

	sym2 := &Symbol{
		Name: "X",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 5, Col: 1},
	}
	err := scope.Insert(sym2)
	if err == nil {
		t.Fatal("expected redeclaration error, got nil")
	}

	// Error should mention original position
	if !strings.Contains(err.Error(), "1:1") {
		t.Fatalf("error should contain original position, got: %v", err)
	}
}

func TestScopeNotFound(t *testing.T) {
	parent := NewScope(nil, ScopeGlobal, "global")
	child := NewScope(parent, ScopePOU, "MyProgram")

	got := child.Lookup("nonexistent")
	if got != nil {
		t.Fatalf("Lookup(nonexistent) = %v, want nil", got)
	}
}

func TestSymbolKinds(t *testing.T) {
	kinds := []struct {
		kind SymbolKind
		name string
	}{
		{KindVariable, "Variable"},
		{KindFunction, "Function"},
		{KindFunctionBlock, "FunctionBlock"},
		{KindProgram, "Program"},
		{KindType, "Type"},
		{KindEnumValue, "EnumValue"},
		{KindInterface, "Interface"},
	}

	seen := make(map[SymbolKind]bool)
	for _, tc := range kinds {
		if seen[tc.kind] {
			t.Errorf("duplicate SymbolKind value: %d", tc.kind)
		}
		seen[tc.kind] = true

		if got := tc.kind.String(); got != tc.name {
			t.Errorf("%d.String() = %q, want %q", tc.kind, got, tc.name)
		}
	}
}

func TestScopeKinds(t *testing.T) {
	kinds := []ScopeKind{ScopeGlobal, ScopePOU, ScopeMethod, ScopeBlock}

	seen := make(map[ScopeKind]bool)
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate ScopeKind value: %d", k)
		}
		seen[k] = true
	}
}

func TestSymbolUsageTracking(t *testing.T) {
	sym := &Symbol{
		Name: "x",
		Kind: KindVariable,
	}
	if sym.Used {
		t.Fatal("new symbol should not be marked as used")
	}
	sym.MarkUsed()
	if !sym.Used {
		t.Fatal("after MarkUsed(), symbol should be marked as used")
	}
}

func TestScopeChildren(t *testing.T) {
	parent := NewScope(nil, ScopeGlobal, "global")
	if len(parent.Children) != 0 {
		t.Fatalf("new scope should have 0 children, got %d", len(parent.Children))
	}

	child1 := NewScope(parent, ScopePOU, "POU1")
	child2 := NewScope(parent, ScopePOU, "POU2")

	if len(parent.Children) != 2 {
		t.Fatalf("parent should have 2 children, got %d", len(parent.Children))
	}
	if parent.Children[0] != child1 {
		t.Fatal("first child mismatch")
	}
	if parent.Children[1] != child2 {
		t.Fatal("second child mismatch")
	}
}

func TestCasePreservation(t *testing.T) {
	scope := NewScope(nil, ScopeGlobal, "global")
	sym := &Symbol{
		Name: "myVar",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := scope.Insert(sym); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	got := scope.Lookup("MYVAR")
	if got == nil {
		t.Fatal("Lookup(MYVAR) returned nil")
	}
	if got.Name != "myVar" {
		t.Fatalf("Lookup(MYVAR).Name = %q, want %q", got.Name, "myVar")
	}
}
