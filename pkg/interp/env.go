package interp

import "strings"

// Env is a scoped environment for variable storage during interpretation.
// Variables are stored with uppercase keys for case-insensitive IEC 61131-3
// identifier lookup. A parent pointer enables scope chain walking.
type Env struct {
	parent *Env
	vars   map[string]Value
}

// NewEnv creates a new environment with an optional parent scope.
func NewEnv(parent *Env) *Env {
	return &Env{
		parent: parent,
		vars:   make(map[string]Value),
	}
}

// Get looks up a variable by name (case-insensitive), walking the parent
// chain if not found in the current scope.
func (e *Env) Get(name string) (Value, bool) {
	key := strings.ToUpper(name)
	if v, ok := e.vars[key]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return Value{}, false
}

// Set updates a variable in the scope where it was originally defined.
// Returns false if the variable is not found in any scope.
func (e *Env) Set(name string, v Value) bool {
	key := strings.ToUpper(name)
	if _, ok := e.vars[key]; ok {
		e.vars[key] = v
		return true
	}
	if e.parent != nil {
		return e.parent.Set(name, v)
	}
	return false
}

// Define adds or overwrites a variable in the current scope.
func (e *Env) Define(name string, v Value) {
	key := strings.ToUpper(name)
	e.vars[key] = v
}

// AllVars returns a copy of the current scope's variables (for debugging).
func (e *Env) AllVars() map[string]Value {
	result := make(map[string]Value, len(e.vars))
	for k, v := range e.vars {
		result[k] = v
	}
	return result
}

// Parent returns the parent environment, or nil if this is the root scope.
func (e *Env) Parent() *Env {
	return e.parent
}
