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
	// fbParent is the parent env of user FB instances built as array
	// elements or struct members; env when nil.
	fbParent *Env
}

// fbInstance builds a fresh instance when name is a standard or registered
// user FB type and ctx can build one (ctx.interp is set).
func (c typeCtx) fbInstance(name string, depth int) (Value, bool) {
	if c.interp == nil || depth > maxFBNestDepth {
		return Value{}, false
	}
	upper := strings.ToUpper(name)
	if factory := StdlibFBFactory[upper]; factory != nil {
		return MakeFBInstanceValue(name, factory()), true
	}
	decl := c.interp.FBDecls[upper]
	if decl == nil {
		return Value{}, false
	}
	parent := c.fbParent
	if parent == nil {
		parent = c.env
	}
	return Value{Kind: ValFBInstance, FBRef: newUserFBInstanceDepth(name, decl, c.interp, parent, depth)}, true
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

// evalInit applies initialiser init to zero, the current (default) value of
// a variable of type ts, and returns the result:
//
//   - an ArrayInit fills elements from the lower bound on, expanding N(v)
//     repetitions; elements it does not reach keep their default;
//   - a StructInit sets the named members and leaves the others at their
//     TYPE defaults;
//   - anything else is evaluated and stored with storeAs.
//
// Nested initialisers recurse with the element or member type. On error the
// value built so far is returned with the error.
func (interp *Interpreter) evalInit(env *Env, ts ast.TypeSpec, init ast.Expr, zero Value) (Value, error) {
	switch in := init.(type) {
	case *ast.ArrayInit:
		at, ok := interp.resolveSpec(ts).(*ast.ArrayType)
		if !ok || zero.Kind != ValArray {
			return zero, initError(in, "array initialiser for non-array type")
		}
		return interp.evalArrayInit(env, at, in, zero.Clone())
	case *ast.StructInit:
		st, ok := interp.resolveSpec(ts).(*ast.StructType)
		if !ok || zero.Kind != ValStruct {
			return zero, initError(in, "structure initialiser for non-structure type")
		}
		return interp.evalStructInit(env, st, in, zero.Clone())
	}
	v, err := interp.evalExpr(env, init)
	if err != nil {
		return zero, err
	}
	if v.IsAggregate() {
		v = v.Clone()
	}
	return storeAs(zero, v), nil
}

// evalArrayInit fills out from in. The number of elements, repetitions
// included, is checked against the array length before anything is
// expanded, so a huge repetition count cannot allocate or loop.
func (interp *Interpreter) evalArrayInit(env *Env, at *ast.ArrayType, in *ast.ArrayInit, out Value) (Value, error) {
	length := int64(len(out.Array) - out.ArrayLow)
	j := int64(0)
	for _, el := range in.Elements {
		count := int64(1)
		if el.Count != nil {
			n, ok := ast.ConstIntValue(el.Count, interp.constLookup(env))
			if !ok || n < 0 {
				return out, initError(el, "cannot evaluate repetition count "+exprText(el.Count))
			}
			count = n
		}
		if count > length-j {
			return out, initError(el, fmt.Sprintf("array initialiser has more elements than the array length %d", length))
		}
		for range count {
			slot := out.ArrayLow + int(j)
			j++
			if el.Value == nil {
				continue // N() keeps the default
			}
			v, err := interp.evalInit(env, at.ElementType, el.Value, out.Array[slot])
			out.Array[slot] = v
			if err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

// evalStructInit sets the members named in in on out.
func (interp *Interpreter) evalStructInit(env *Env, st *ast.StructType, in *ast.StructInit, out Value) (Value, error) {
	for _, f := range in.Fields {
		key := strings.ToUpper(f.Name.Name)
		cur, ok := out.Struct[key]
		var member *ast.StructMember
		for _, m := range st.Members {
			if m.Name != nil && strings.EqualFold(m.Name.Name, f.Name.Name) {
				member = m
			}
		}
		if !ok || member == nil {
			return out, initError(f, fmt.Sprintf("structure has no member '%s'", f.Name.Name))
		}
		v, err := interp.evalInit(env, member.Type, f.Value, cur)
		out.Struct[key] = v
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// initFB applies a FB variable's initialiser (fb : FB_X := (a := 1)) to the
// fresh instance inst. Members are inputs or variables of the FB (and its
// EXTENDS bases); values evaluate in env, the declaring scope.
func (interp *Interpreter) initFB(env *Env, inst *FBInstance, init ast.Expr) error {
	si, ok := init.(*ast.StructInit)
	if !ok {
		return initError(init, "structure initialiser expected for function block "+inst.TypeName)
	}
	for _, f := range si.Fields {
		if inst.FB != nil {
			v, err := interp.evalExpr(env, f.Value)
			if err != nil {
				return err
			}
			inst.FB.SetInput(f.Name.Name, v)
			continue
		}
		cur, found := inst.Env.GetLocal(f.Name.Name)
		vd := fbVarDecl(inst.Decl, interp, f.Name.Name)
		if !found || vd == nil {
			return initError(f, fmt.Sprintf("function block %s has no member '%s'", inst.TypeName, f.Name.Name))
		}
		v, err := interp.evalInit(env, vd.Type, f.Value, cur)
		if err != nil {
			return err
		}
		inst.Env.Define(f.Name.Name, v)
	}
	return nil
}

// fbVarDecl finds the declaration of variable name in decl or its EXTENDS
// bases; the most derived declaration wins.
func fbVarDecl(decl *ast.FunctionBlockDecl, interp *Interpreter, name string) *ast.VarDecl {
	var found *ast.VarDecl
	for _, d := range fbExtendsChain(decl, interp) {
		for _, vb := range d.VarBlocks {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					if strings.EqualFold(n.Name, name) {
						found = vd
					}
				}
			}
		}
	}
	return found
}

// InstantiateVar defines every name of vd in env through the shared
// instantiation path; user FB instances get env as parent. The test runner
// uses it for TEST_CASE variables.
func (interp *Interpreter) InstantiateVar(env *Env, vd *ast.VarDecl) {
	interp.instantiateVar(env, env, vd, 0)
}

// instantiateVar defines every name of vd in env. It is the one
// instantiation path for program, GVL and FB variables (and so for the
// variables inherited through EXTENDS):
//
//   - stdlib FB types become fresh stdlib instances;
//   - user FB types registered in FBDecls become live instances at nesting
//     depth depth, whose env has fbParent as parent;
//   - anything else gets its type's default (constant bounds, TYPE and
//     member defaults) with the initialiser applied by evalInit.
//
// Each name gets its own value, so aggregates are never shared. Initialiser
// failures are recorded in InitErrors; the variable keeps the value built so
// far.
func (interp *Interpreter) instantiateVar(env, fbParent *Env, vd *ast.VarDecl, depth int) {
	typeName := typeNameFromSpec(vd.Type)
	upperType := strings.ToUpper(typeName)
	var factory func() StandardFB
	var fbDecl *ast.FunctionBlockDecl
	if typeName != "" && depth <= maxFBNestDepth {
		factory = StdlibFBFactory[upperType]
		if factory == nil && interp.FBDecls != nil {
			fbDecl = interp.FBDecls[upperType]
		}
	}

	for _, n := range vd.Names {
		var val Value
		switch {
		case factory != nil:
			val = MakeFBInstanceValue(typeName, factory())
		case fbDecl != nil:
			val = Value{Kind: ValFBInstance, FBRef: newUserFBInstanceDepth(typeName, fbDecl, interp, fbParent, depth)}
		default:
			ctx := interp.typeCtxFor(env)
			ctx.fbParent = fbParent
			val = zeroFromType(vd.Type, ctx, depth)
			if vd.InitValue != nil {
				v, err := interp.evalInit(env, vd.Type, vd.InitValue, val)
				if err != nil {
					interp.recordInitErr(err)
				}
				val = v
			}
			if val.IsAggregate() {
				val = val.Clone()
			}
			env.Define(n.Name, val)
			continue
		}
		if vd.InitValue != nil {
			if err := interp.initFB(env, val.FBRef, vd.InitValue); err != nil {
				interp.recordInitErr(err)
			}
		}
		env.Define(n.Name, val)
	}
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
