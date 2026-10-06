package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// gvlState holds the global variable lists (GVLs) registered on an
// Interpreter. It is a minimal global environment layer: each GVL gets its own
// Env, reachable as GVL.x through member access. GVLs without the
// qualified_only attribute are also chained as ancestors of the program or
// test environment, so their variables resolve as bare names.
//
// Per-task instantiation and PERSISTENT/RETAIN semantics are out of scope.
type gvlState struct {
	// envs maps an upper-case GVL name to its environment.
	envs map[string]*Env
	// unqualified is the innermost environment of the chain of non
	// qualified_only GVLs, or nil when there are none. Earlier GVLs are its
	// ancestors, so a later GVL shadows an earlier one on a bare name clash.
	unqualified *Env
}

// RegisterGVL builds an environment for decl, stores it under the GVL name and
// returns it. Variables are created exactly like program variables: stdlib FBs
// and user FBs (from FBDecls) become live instances, other types get their
// zero value or initialiser. Registering a GVL whose name is already
// registered replaces the qualified entry.
//
// The checker enforces qualified_only; the interpreter only uses it to decide
// whether the GVL joins the bare-name chain. Returns nil for a nil or unnamed
// declaration.
func (interp *Interpreter) RegisterGVL(decl *ast.GVLDecl) *Env {
	if decl == nil || decl.Name == nil || decl.Name.Name == "" {
		return nil
	}
	if interp.gvls.envs == nil {
		interp.gvls.envs = make(map[string]*Env)
	}

	qualifiedOnly := ast.HasAttribute(decl.Attributes, "qualified_only")
	for _, vb := range decl.Blocks {
		if vb != nil && ast.HasAttribute(vb.Attributes, "qualified_only") {
			qualifiedOnly = true
		}
	}

	// Initialisers may refer to earlier unqualified GVLs, so give every GVL
	// env the current chain as parent. Member access (GVL.x) only looks at
	// the GVL's own scope, so the parent never leaks into qualified lookups.
	env := NewEnv(interp.gvls.unqualified)
	// FB instances declared in the GVL see the global chain. For a plain GVL
	// that includes its own variables; a qualified_only GVL is left out so
	// an FB body cannot read its variables bare (TwinCAT rejects that, and
	// the checker reports SEMA033).
	fbParent := env
	if qualifiedOnly {
		fbParent = interp.gvls.unqualified
	}
	for _, vb := range decl.Blocks {
		if vb == nil {
			continue
		}
		for _, vd := range vb.Declarations {
			interp.initVarDecl(env, fbParent, vd)
		}
	}

	interp.gvls.envs[strings.ToUpper(decl.Name.Name)] = env
	if !qualifiedOnly {
		interp.gvls.unqualified = env
	}
	interp.retryPendingEnums()
	return env
}

// GlobalParent returns the environment that program and test environments
// should use as their parent so that bare names reach non qualified_only
// GVLs. It is nil when no such GVL is registered.
func (interp *Interpreter) GlobalParent() *Env {
	return interp.gvls.unqualified
}

// lookupGVL returns the environment of the GVL called name
// (case-insensitive), or nil when no such GVL is registered.
func (interp *Interpreter) lookupGVL(name string) *Env {
	if interp.gvls.envs == nil {
		return nil
	}
	return interp.gvls.envs[strings.ToUpper(name)]
}

// gvlRoot reports the GVL environment that obj names, when obj is a bare
// identifier that is not a variable in env but is a registered GVL name. A
// local or FB variable with the same name shadows the GVL.
func (interp *Interpreter) gvlRoot(env *Env, obj ast.Expr) (*Env, *ast.Ident) {
	id, ok := obj.(*ast.Ident)
	if !ok {
		return nil, nil
	}
	if _, found := env.Get(id.Name); found {
		return nil, nil
	}
	if g := interp.lookupGVL(id.Name); g != nil {
		return g, id
	}
	return nil, nil
}

// gvlMemberError reports access to a variable a GVL does not declare.
func gvlMemberError(gvl *ast.Ident, member *ast.Ident) error {
	return &RuntimeError{
		Msg: fmt.Sprintf("GVL '%s' has no variable '%s'", gvl.Name, member.Name),
		Pos: member.Span().Start,
	}
}

// evalGVLMember reads gvl.member from the GVL's own scope.
func evalGVLMember(g *Env, gvl *ast.Ident, member *ast.Ident) (Value, error) {
	v, ok := g.GetLocal(member.Name)
	if !ok {
		return Value{}, gvlMemberError(gvl, member)
	}
	return v, nil
}

// assignGVLMember writes gvl.member in the GVL's own scope. Unknown members
// are an error rather than being created, so a typo cannot add a variable.
func assignGVLMember(g *Env, gvl *ast.Ident, member *ast.Ident, val Value) error {
	cur, ok := g.GetLocal(member.Name)
	if !ok {
		return gvlMemberError(gvl, member)
	}
	g.Define(member.Name, adoptEnumTag(cur, val))
	return nil
}
