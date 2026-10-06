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
// returns it. It is RegisterGVLs for a single GVL. Returns nil for a nil or
// unnamed declaration.
func (interp *Interpreter) RegisterGVL(decl *ast.GVLDecl) *Env {
	if decl == nil || decl.Name == nil || decl.Name.Name == "" {
		return nil
	}
	interp.RegisterGVLs([]*ast.GVLDecl{decl})
	return interp.lookupGVL(decl.Name.Name)
}

// gvlReg is one GVL being registered by RegisterGVLs.
type gvlReg struct {
	decl     *ast.GVLDecl
	env      *Env
	fbParent *Env
}

// RegisterGVLs registers decls in the given order (library GVLs first). Each
// GVL gets its own Env, stored under its name; variables are created exactly
// like program variables through instantiateVar. Registering a GVL whose name
// is already registered replaces the qualified entry.
//
// Registration has two passes so that array bounds and initialisers can use
// constants of any GVL in the set, whatever the source order: pass 1 creates
// every GVL env and defines only VAR_GLOBAL CONSTANT blocks, then retries
// enums waiting for constants; pass 2 defines the remaining blocks.
//
// The checker enforces qualified_only; the interpreter only uses it to decide
// whether the GVL joins the bare-name chain. Nil and unnamed declarations are
// skipped.
func (interp *Interpreter) RegisterGVLs(decls []*ast.GVLDecl) {
	if interp.gvls.envs == nil {
		interp.gvls.envs = make(map[string]*Env)
	}
	var regs []gvlReg
	for _, decl := range decls {
		if decl == nil || decl.Name == nil || decl.Name.Name == "" {
			continue
		}
		regs = append(regs, interp.newGVLEnv(decl))
	}
	for _, r := range regs {
		interp.defineGVLBlocks(r, true)
	}
	interp.retryPendingEnums()
	for _, r := range regs {
		interp.defineGVLBlocks(r, false)
	}
	interp.retryPendingEnums()
}

// newGVLEnv creates decl's environment, stores it under the GVL name and,
// for a GVL without qualified_only, makes it the innermost bare-name scope.
func (interp *Interpreter) newGVLEnv(decl *ast.GVLDecl) gvlReg {
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
	interp.gvls.envs[strings.ToUpper(decl.Name.Name)] = env
	if !qualifiedOnly {
		interp.gvls.unqualified = env
	}
	return gvlReg{decl: decl, env: env, fbParent: fbParent}
}

// defineGVLBlocks defines the variables of r's CONSTANT blocks (constants
// true) or of its other blocks (constants false).
func (interp *Interpreter) defineGVLBlocks(r gvlReg, constants bool) {
	for _, vb := range r.decl.Blocks {
		if vb == nil || vb.IsConstant != constants {
			continue
		}
		for _, vd := range vb.Declarations {
			interp.initVarDecl(r.env, r.fbParent, vd)
		}
	}
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
	g.Define(member.Name, storeAs(cur, val))
	return nil
}
