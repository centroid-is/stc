package symbols

import (
	"fmt"
	"strings"
)

// ScopeKind identifies the level of a scope in the hierarchy.
type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota // Top-level global scope
	ScopePOU                     // PROGRAM, FUNCTION_BLOCK, or FUNCTION scope
	ScopeMethod                  // METHOD scope
	ScopeBlock                   // Block scope (FOR, IF, etc.)
)

var scopeKindNames = [...]string{
	ScopeGlobal: "Global",
	ScopePOU:    "POU",
	ScopeMethod: "Method",
	ScopeBlock:  "Block",
}

// String returns the human-readable name of the scope kind.
func (k ScopeKind) String() string {
	if int(k) >= 0 && int(k) < len(scopeKindNames) {
		return scopeKindNames[k]
	}
	return fmt.Sprintf("ScopeKind(%d)", k)
}

// Scope represents a lexical scope in the IEC 61131-3 program.
// Scopes form a tree: Global -> POU -> Method -> Block.
// Name lookup walks up the parent chain until a match is found.
type Scope struct {
	Parent   *Scope     // Enclosing scope (nil for global)
	Kind     ScopeKind  // Level in the hierarchy
	Name     string     // Scope name (POU name, method name, etc.)
	Children []*Scope   // Child scopes

	symbols map[string]*Symbol // Keyed by UPPERCASE name for case-insensitive lookup
}

// NewScope creates a new scope with the given parent, kind, and name.
// If parent is non-nil, the new scope is added to parent's Children.
func NewScope(parent *Scope, kind ScopeKind, name string) *Scope {
	s := &Scope{
		Parent:  parent,
		Kind:    kind,
		Name:    name,
		symbols: make(map[string]*Symbol),
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}

// Insert adds a symbol to this scope. Returns an error if a symbol with
// the same name (case-insensitive) already exists in this scope.
func (s *Scope) Insert(sym *Symbol) error {
	key := strings.ToUpper(sym.Name)
	if existing, ok := s.symbols[key]; ok {
		return fmt.Errorf("redeclaration of %q (previously declared at %s:%d:%d)",
			sym.Name, existing.Pos.File, existing.Pos.Line, existing.Pos.Col)
	}
	s.symbols[key] = sym
	return nil
}

// Lookup searches for a symbol by name (case-insensitive) in this scope
// and all ancestor scopes. Returns nil if not found.
func (s *Scope) Lookup(name string) *Symbol {
	key := strings.ToUpper(name)
	for scope := s; scope != nil; scope = scope.Parent {
		if sym, ok := scope.symbols[key]; ok {
			return sym
		}
	}
	return nil
}

// LookupLocal searches for a symbol by name (case-insensitive) in this
// scope only, without walking the parent chain. Returns nil if not found.
func (s *Scope) LookupLocal(name string) *Symbol {
	return s.symbols[strings.ToUpper(name)]
}

// Symbols returns all symbols defined in this scope.
// Useful for iterating over symbols for unused variable detection.
func (s *Scope) Symbols() []*Symbol {
	result := make([]*Symbol, 0, len(s.symbols))
	for _, sym := range s.symbols {
		result = append(result, sym)
	}
	return result
}
