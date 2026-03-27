package symbols

import (
	"strings"

	"github.com/centroid-is/stc/pkg/source"
)

// Table is the top-level symbol table for semantic analysis.
// It manages the global scope, scope stack, POU registry, and file tracking.
type Table struct {
	global  *Scope            // Root global scope
	current *Scope            // Current position in scope tree
	pous    map[string]*Scope // POU name (uppercased) -> POU scope
	files   []string          // Tracked source files
}

// NewTable creates a new symbol table with an empty global scope.
func NewTable() *Table {
	global := NewScope(nil, ScopeGlobal, "global")
	return &Table{
		global:  global,
		current: global,
		pous:    make(map[string]*Scope),
	}
}

// GlobalScope returns the root global scope.
func (t *Table) GlobalScope() *Scope {
	return t.global
}

// CurrentScope returns the current scope (top of the scope stack).
func (t *Table) CurrentScope() *Scope {
	return t.current
}

// EnterScope creates a new child scope of the current scope and pushes it
// as the current scope. Returns the new scope.
func (t *Table) EnterScope(kind ScopeKind, name string) *Scope {
	child := NewScope(t.current, kind, name)
	t.current = child
	return child
}

// ExitScope pops the current scope and sets current to its parent.
// Panics if called when already at the global scope (programming error).
func (t *Table) ExitScope() {
	if t.current.Parent == nil {
		panic("symbols: ExitScope called at global scope")
	}
	t.current = t.current.Parent
}

// RegisterPOU registers a Program Organization Unit (PROGRAM, FUNCTION_BLOCK,
// or FUNCTION) in the global scope and POU registry. Returns the POU's scope.
func (t *Table) RegisterPOU(name string, kind SymbolKind, pos source.Pos) *Scope {
	// Insert symbol into global scope
	sym := &Symbol{
		Name: name,
		Kind: kind,
		Pos:  pos,
	}
	// Ignore error — duplicate POU detection is the checker's job
	_ = t.global.Insert(sym)

	// Create a scope for the POU
	pouScope := NewScope(t.global, ScopePOU, name)

	// Register in POU lookup map (case-insensitive)
	t.pous[strings.ToUpper(name)] = pouScope

	return pouScope
}

// LookupPOU looks up a POU scope by name (case-insensitive).
// Returns nil if not found.
func (t *Table) LookupPOU(name string) *Scope {
	return t.pous[strings.ToUpper(name)]
}

// LookupGlobal looks up a symbol in the global scope only (case-insensitive).
// Returns nil if not found.
func (t *Table) LookupGlobal(name string) *Symbol {
	return t.global.LookupLocal(name)
}

// Insert adds a symbol to the current scope. Convenience method.
func (t *Table) Insert(sym *Symbol) error {
	return t.current.Insert(sym)
}

// Lookup searches for a symbol starting from the current scope and walking
// up the parent chain. Convenience method.
func (t *Table) Lookup(name string) *Symbol {
	return t.current.Lookup(name)
}

// RegisterFile adds a source file to the tracked files list.
func (t *Table) RegisterFile(filename string) {
	t.files = append(t.files, filename)
}

// Files returns the list of tracked source files.
func (t *Table) Files() []string {
	return t.files
}
