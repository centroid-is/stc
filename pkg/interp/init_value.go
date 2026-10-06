package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// typeCtx is what zero-value construction needs beyond the TypeSpec:
// resolve maps user TYPE names to their specs, consts resolves names in
// constant array bounds, and interp/env evaluate member and TYPE defaults
// and collect failures. The zero value of typeCtx builds plain zero values
// with literal-only bounds.
type typeCtx struct {
	resolve TypeResolver
	consts  func(qual, name string) (int64, bool)
	interp  *Interpreter
	env     *Env
}

// typeCtxFor returns the full construction context for variables
// instantiated in env.
func (interp *Interpreter) typeCtxFor(env *Env) typeCtx {
	return typeCtx{
		resolve: interp.TypeResolverFunc(),
		consts:  interp.constLookup(env),
		interp:  interp,
		env:     env,
	}
}

// zeroOf is the zero (default) value of ts for a variable instantiated in
// env: constant bounds resolve and TYPE and member defaults apply.
func (interp *Interpreter) zeroOf(ts ast.TypeSpec, env *Env) Value {
	return zeroFromType(ts, interp.typeCtxFor(env), 0)
}

// bounds evaluates a subrange's bounds. Without a constant lookup only
// literal bounds are understood; with one, any integer constant expression
// over literals and constants is, and failures are recorded.
func (c typeCtx) bounds(sr *ast.SubrangeSpec) (int64, int64) {
	if c.consts == nil {
		low, high := evalSubrangeConst(sr)
		return int64(low), int64(high)
	}
	low, ok := ast.ConstIntValue(sr.Low, c.consts)
	if !ok {
		c.fail(sr.Low, "cannot evaluate array bound %s", exprText(sr.Low))
		low = 0
	}
	high, ok := ast.ConstIntValue(sr.High, c.consts)
	if !ok {
		c.fail(sr.High, "cannot evaluate array bound %s", exprText(sr.High))
		high = max(low, 0)
	}
	return low, high
}

// fail records an instantiation error at node's position. Without an
// interpreter there is nowhere to record it and it is dropped.
func (c typeCtx) fail(node ast.Node, format string, args ...any) {
	if c.interp == nil {
		return
	}
	c.interp.recordInitErr(initError(node, fmt.Sprintf(format, args...)))
}

// applyInit applies init to zero for type ts. Failures are recorded and the
// best value produced so far is kept. Without an interpreter a scratch one
// evaluates the initialiser, so literal defaults still apply.
func (c typeCtx) applyInit(ts ast.TypeSpec, init ast.Expr, zero Value) Value {
	in, env := c.interp, c.env
	if in == nil {
		in = New()
	}
	if env == nil {
		env = NewEnv(in.GlobalParent())
	}
	v, err := in.evalInit(env, ts, init, zero)
	if err != nil && c.interp != nil {
		c.interp.recordInitErr(err)
	}
	return v
}

// typeDefault applies the TYPE-level default of the user type name, if any,
// to its zero value v.
func (c typeCtx) typeDefault(name string, target ast.TypeSpec, v Value) Value {
	if c.interp == nil || c.interp.TypeInits == nil {
		return v
	}
	init, ok := c.interp.TypeInits[name]
	if !ok || init == nil {
		return v
	}
	return c.applyInit(target, init, v)
}

// constLookup resolves constant names for ast.ConstIntValue: G.C reads
// variable C of GVL G, a bare name walks env's scope chain (or the global
// chain when env is nil). Only integer values count.
func (interp *Interpreter) constLookup(env *Env) func(qual, name string) (int64, bool) {
	return func(qual, name string) (int64, bool) {
		var v Value
		var ok bool
		switch {
		case qual != "":
			g := interp.lookupGVL(qual)
			if g == nil {
				return 0, false
			}
			v, ok = g.GetLocal(name)
		case env != nil:
			v, ok = env.Get(name)
		default:
			if g := interp.GlobalParent(); g != nil {
				v, ok = g.Get(name)
			}
		}
		if !ok || v.Kind != ValInt {
			return 0, false
		}
		return v.Int, true
	}
}

// InitErrors returns the array bound and initialiser failures recorded while
// instantiating program, GVL and FB variables, in the order found.
func (interp *Interpreter) InitErrors() []error {
	return interp.initErrs
}

func (interp *Interpreter) recordInitErr(err error) {
	interp.initErrs = append(interp.initErrs, err)
}

// initError is a RuntimeError positioned at node.
func initError(node ast.Node, msg string) error {
	e := &RuntimeError{Msg: msg}
	if node != nil {
		e.Pos = node.Span().Start
	}
	return e
}

// resolveSpec follows named user types (and subranges) to the TypeSpec that
// decides how an initialiser applies.
func (interp *Interpreter) resolveSpec(ts ast.TypeSpec) ast.TypeSpec {
	for range maxTypeNestDepth {
		nt, ok := ts.(*ast.NamedType)
		if !ok || nt.Name == nil || interp.TypeDecls == nil {
			return ts
		}
		target, found := interp.TypeDecls[strings.ToUpper(nt.Name.Name)]
		if !found {
			return ts
		}
		ts = target
	}
	return ts
}

// evalInit applies initialiser init to zero, the current value of a
// variable of type ts.
func (interp *Interpreter) evalInit(env *Env, ts ast.TypeSpec, init ast.Expr, zero Value) (Value, error) {
	v, err := interp.evalExpr(env, init)
	if err != nil {
		return zero, err
	}
	if v.IsAggregate() {
		v = v.Clone()
	}
	return storeAs(zero, v), nil
}

// exprText renders a constant expression for error messages.
func exprText(x ast.Expr) string {
	switch e := x.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.Literal:
		return e.Value
	case *ast.MemberAccessExpr:
		if e.Member != nil {
			return exprText(e.Object) + "." + e.Member.Name
		}
	case *ast.ParenExpr:
		return "(" + exprText(e.Inner) + ")"
	case *ast.UnaryExpr:
		return e.Op.Text + exprText(e.Operand)
	case *ast.BinaryExpr:
		return exprText(e.Left) + " " + e.Op.Text + " " + exprText(e.Right)
	}
	return "<expression>"
}
