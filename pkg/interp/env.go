package interp

import (
	"fmt"
	"strings"
)

// SubrangeConstraint stores the bounds for a subrange variable.
type SubrangeConstraint struct {
	Low  int64
	High int64
}

// Env is a scoped environment for variable storage during interpretation.
// Variables are stored with uppercase keys for case-insensitive IEC 61131-3
// identifier lookup. A parent pointer enables scope chain walking.
type Env struct {
	parent     *Env
	vars       map[string]Value
	subranges  map[string]*SubrangeConstraint // optional subrange bounds per variable
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

// DefineSubrange registers a subrange constraint for a variable.
// When Set is called for this variable, the value will be range-checked.
func (e *Env) DefineSubrange(name string, low, high int64) {
	key := strings.ToUpper(name)
	if e.subranges == nil {
		e.subranges = make(map[string]*SubrangeConstraint)
	}
	e.subranges[key] = &SubrangeConstraint{Low: low, High: high}
}

// CheckSubrange checks if a value satisfies the subrange constraint for a variable.
// Returns an error message if out of range, or empty string if OK or no constraint.
func (e *Env) CheckSubrange(name string, v Value) string {
	key := strings.ToUpper(name)
	if e.subranges != nil {
		if sr, ok := e.subranges[key]; ok {
			if v.Kind == ValInt {
				if v.Int < sr.Low || v.Int > sr.High {
					return fmt.Sprintf("value %d out of subrange [%d..%d] for variable '%s'", v.Int, sr.Low, sr.High, name)
				}
			}
		}
	}
	if e.parent != nil {
		return e.parent.CheckSubrange(name, v)
	}
	return ""
}

// FindOwner returns the Env in the scope chain that directly contains the
// given variable (case-insensitive). Returns nil if not found.
func (e *Env) FindOwner(name string) *Env {
	key := strings.ToUpper(name)
	if _, ok := e.vars[key]; ok {
		return e
	}
	if e.parent != nil {
		return e.parent.FindOwner(name)
	}
	return nil
}
