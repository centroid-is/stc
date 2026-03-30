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

// RemovePOU removes a POU from the global scope and POU registry.
// Used when user code overrides a library symbol.
func (t *Table) RemovePOU(name string) {
	key := strings.ToUpper(name)
	t.global.Delete(name)
	delete(t.pous, key)
	// Remove child scope
	filtered := t.global.Children[:0]
	for _, child := range t.global.Children {
		if strings.ToUpper(child.Name) != key {
			filtered = append(filtered, child)
		}
	}
	t.global.Children = filtered
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

// PurgeFile removes all symbols declared in the given file from the global
// scope and POU registry. It also removes matching child scopes from the
// global scope and the filename from the tracked files list.
// This supports incremental re-analysis by clearing a file's contributions.
func (t *Table) PurgeFile(filename string) {
	// Collect global symbol keys to delete (symbols declared in this file)
	var keysToDelete []string
	for key, sym := range t.global.symbols {
		if sym.Pos.File == filename {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Track POU names being purged (for scope cleanup)
	purgedPOUs := make(map[string]bool)
	for _, key := range keysToDelete {
		sym := t.global.symbols[key]
		// If this symbol is a POU, mark for POU registry cleanup
		switch sym.Kind {
		case KindProgram, KindFunctionBlock, KindFunction, KindInterface:
			purgedPOUs[strings.ToUpper(sym.Name)] = true
		}
		delete(t.global.symbols, key)
	}

	// Remove from POU registry
	for pouKey := range purgedPOUs {
		delete(t.pous, pouKey)
	}

	// Remove child scopes for purged POUs
	filtered := t.global.Children[:0]
	for _, child := range t.global.Children {
		if !purgedPOUs[strings.ToUpper(child.Name)] {
			filtered = append(filtered, child)
		}
	}
	t.global.Children = filtered

	// Remove filename from tracked files
	newFiles := t.files[:0]
	for _, f := range t.files {
		if f != filename {
			newFiles = append(newFiles, f)
		}
	}
	t.files = newFiles
}

// SymbolsByFile returns all symbols in the global scope that were declared
// in the given file.
func (t *Table) SymbolsByFile(filename string) []*Symbol {
	var result []*Symbol
	for _, sym := range t.global.symbols {
		if sym.Pos.File == filename {
			result = append(result, sym)
		}
	}
	return result
}
