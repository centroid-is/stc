package symbols

import (
	"testing"

	"github.com/centroid-is/stc/pkg/source"
)

func TestTableNewHasGlobalScope(t *testing.T) {
	table := NewTable()
	if table.GlobalScope() == nil {
		t.Fatal("NewTable().GlobalScope() should not be nil")
	}
	if table.GlobalScope().Kind != ScopeGlobal {
		t.Fatalf("global scope kind = %v, want ScopeGlobal", table.GlobalScope().Kind)
	}
	if table.CurrentScope() != table.GlobalScope() {
		t.Fatal("initial CurrentScope should be GlobalScope")
	}
}

func TestTableRegisterPOU(t *testing.T) {
	table := NewTable()
	scope := table.RegisterPOU("MyProgram", KindProgram, source.Pos{File: "test.st", Line: 1, Col: 1})
	if scope == nil {
		t.Fatal("RegisterPOU returned nil")
	}

	// Case-insensitive lookup
	got := table.LookupPOU("MYPROGRAM")
	if got != scope {
		t.Fatalf("LookupPOU(MYPROGRAM) = %v, want %v", got, scope)
	}

	got = table.LookupPOU("myprogram")
	if got != scope {
		t.Fatalf("LookupPOU(myprogram) = %v, want %v", got, scope)
	}

	// Not found
	got = table.LookupPOU("nonexistent")
	if got != nil {
		t.Fatalf("LookupPOU(nonexistent) = %v, want nil", got)
	}
}

func TestTableEnterExitScope(t *testing.T) {
	table := NewTable()

	pou := table.EnterScope(ScopePOU, "FB_Motor")
	if pou == nil {
		t.Fatal("EnterScope returned nil")
	}
	if table.CurrentScope() != pou {
		t.Fatal("CurrentScope should be POU scope after EnterScope")
	}
	if pou.Kind != ScopePOU {
		t.Fatalf("scope kind = %v, want ScopePOU", pou.Kind)
	}

	table.ExitScope()
	if table.CurrentScope() != table.GlobalScope() {
		t.Fatal("CurrentScope should be global after ExitScope")
	}
}

func TestTableNestedScopes(t *testing.T) {
	table := NewTable()

	// Insert in global
	globalSym := &Symbol{
		Name: "globalVar",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := table.Insert(globalSym); err != nil {
		t.Fatalf("Insert global failed: %v", err)
	}

	// Enter POU
	table.EnterScope(ScopePOU, "MyFB")
	pouSym := &Symbol{
		Name: "pouVar",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 5, Col: 1},
	}
	if err := table.Insert(pouSym); err != nil {
		t.Fatalf("Insert POU failed: %v", err)
	}

	// Enter method
	table.EnterScope(ScopeMethod, "DoWork")

	// Lookup should walk method -> POU -> global
	if got := table.Lookup("pouVar"); got != pouSym {
		t.Fatalf("Lookup(pouVar) from method = %v, want %v", got, pouSym)
	}
	if got := table.Lookup("globalVar"); got != globalSym {
		t.Fatalf("Lookup(globalVar) from method = %v, want %v", got, globalSym)
	}

	table.ExitScope() // back to POU
	table.ExitScope() // back to global
}

func TestTableGlobalLookup(t *testing.T) {
	table := NewTable()

	sym := &Symbol{
		Name: "gVar",
		Kind: KindVariable,
		Pos:  source.Pos{File: "test.st", Line: 1, Col: 1},
	}
	if err := table.GlobalScope().Insert(sym); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	got := table.LookupGlobal("gVar")
	if got != sym {
		t.Fatalf("LookupGlobal(gVar) = %v, want %v", got, sym)
	}

	got = table.LookupGlobal("GVAR")
	if got != sym {
		t.Fatalf("LookupGlobal(GVAR) = %v, want %v", got, sym)
	}

	got = table.LookupGlobal("nonexistent")
	if got != nil {
		t.Fatalf("LookupGlobal(nonexistent) = %v, want nil", got)
	}
}

func TestTableFileTracking(t *testing.T) {
	table := NewTable()

	table.RegisterFile("main.st")
	table.RegisterFile("utils.st")

	files := table.Files()
	if len(files) != 2 {
		t.Fatalf("len(Files()) = %d, want 2", len(files))
	}
	if files[0] != "main.st" {
		t.Fatalf("Files()[0] = %q, want %q", files[0], "main.st")
	}
	if files[1] != "utils.st" {
		t.Fatalf("Files()[1] = %q, want %q", files[1], "utils.st")
	}
}
