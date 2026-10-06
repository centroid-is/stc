package checker

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// checkCallArgs binds the arguments of a FUNCTION or METHOD call expression
// to fn's parameters (DIAL-06) and reports SEMA020, SEMA021 and SEMA024
// errors. It returns false when an all-positional call has the wrong number
// of arguments; the call then has no usable result type.
func (c *Checker) checkCallArgs(e *ast.CallExpr, fn *types.FunctionType) bool {
	args := make([]*ast.CallArg, 0, len(e.Args)+len(e.NamedArgs))
	for _, a := range e.Args {
		args = append(args, &ast.CallArg{Value: a})
	}
	args = append(args, e.NamedArgs...)
	return c.bindCallArgs(e, fn, args)
}

// bindCallArgs binds a call's argument list in source order (ruling A1):
//   - a positional argument (no name) binds to the input or in-out at its
//     index in the full list, wherever it appears;
//   - name := v binds to the input or in-out called name (case-insensitive);
//   - name => target binds the VAR_OUTPUT called name to a variable.
//
// An all-positional call must supply every parameter. Once any argument is
// named, omitted inputs take their defaults; a VAR_IN_OUT has no default and
// must still be bound. Unknown names and parameters bound twice are SEMA024,
// too many arguments SEMA020, and type mismatches, a VAR_IN_OUT bound to a
// non-variable and an unbound VAR_IN_OUT SEMA021. Every argument value is
// checked, so its variables count as used.
func (c *Checker) bindCallArgs(call ast.Node, fn *types.FunctionType, args []*ast.CallArg) bool {
	named := false
	for _, a := range args {
		if a.Name != nil {
			named = true
		}
	}
	if !named && len(args) != len(fn.Params) {
		c.diags.Errorf(astPosToSource(call.Span().Start), CodeWrongArgCount,
			"%s expects %d argument(s), got %d", fn.Name, len(fn.Params), len(args))
		for _, a := range args {
			c.checkArgValue(a)
		}
		return false
	}

	bound := make(map[string]bool)
	inOutBound := make(map[string]bool)
	for i, a := range args {
		var p types.Parameter
		var found bool
		switch {
		case a.Name == nil:
			if i >= len(fn.Params) {
				c.diags.Errorf(astPosToSource(a.Value.Span().Start), CodeWrongArgCount,
					"too many arguments for %s (expects at most %d)", fn.Name, len(fn.Params))
				c.checkArgValue(a)
				continue
			}
			p, found = fn.Params[i], true
		case a.IsOutput:
			p, found = findParam(fn.Outputs, a.Name.Name)
		default:
			p, found = findParam(fn.Params, a.Name.Name)
		}
		if !found {
			c.diags.Errorf(astPosToSource(a.Name.Span().Start), CodeNoMember,
				"%s has no %s parameter %q", fn.Name, paramDirStr(a.IsOutput), a.Name.Name)
			c.checkArgValue(a)
			continue
		}

		key := strings.ToUpper(p.Name)
		if bound[key] {
			c.diags.Errorf(astPosToSource(a.Span().Start), CodeNoMember,
				"parameter %q of %s is bound more than once", p.Name, fn.Name)
		}
		bound[key] = true

		switch {
		case a.IsOutput:
			c.checkOutputBinding(p, a.Value)
		case p.Direction == types.DirInOut && a.Value != nil:
			inOutBound[key] = true
			if c.checkInOutArg(fn.Name, p, a.Value) {
				c.checkInputArg(p, a, i)
			}
		default:
			c.checkInputArg(p, a, i)
		}
	}
	for _, p := range fn.Params {
		if p.Direction == types.DirInOut && !inOutBound[strings.ToUpper(p.Name)] {
			c.diags.Errorf(astPosToSource(call.Span().Start), CodeWrongArgType,
				"VAR_IN_OUT %q of %s is not bound", p.Name, fn.Name)
		}
	}
	return true
}

// checkInOutArg reports SEMA021 when a VAR_IN_OUT argument is not a
// variable: the callee writes it back, so a literal or computed value has
// nowhere to go. It returns false after reporting (the value is still
// checked for usage marks).
func (c *Checker) checkInOutArg(callee string, p types.Parameter, v ast.Expr) bool {
	if isLValue(v) {
		return true
	}
	c.checkExpr(v)
	c.diags.Errorf(astPosToSource(v.Span().Start), CodeWrongArgType,
		"VAR_IN_OUT %q of %s must be bound to a variable", p.Name, callee)
	return false
}

// checkArgValue type-checks an argument value only for its side effects
// (usage marks and diagnostics inside the value).
func (c *Checker) checkArgValue(a *ast.CallArg) {
	if a.Value != nil {
		c.checkExpr(a.Value)
	}
}

// checkInputArg checks that an input value widens to the parameter type.
// Integer and real literals fit any integer or real parameter; a REFERENCE
// TO T parameter takes a value of exactly T. An empty
// named argument (name := with no value) is accepted.
func (c *Checker) checkInputArg(p types.Parameter, a *ast.CallArg, index int) {
	if a.Value == nil {
		return
	}
	argType := c.checkExpr(a.Value)
	if argType == types.Invalid || p.Type == nil || p.Type == types.Invalid {
		return
	}
	if involvesEnum(argType, derefRef(p.Type)) {
		c.checkEnumArg(a.Value, argType, derefRef(p.Type), func() { c.reportInputArg(p, a, index, argType) })
		return
	}
	if ref, isRef := p.Type.(*types.ReferenceType); isRef {
		// A REFERENCE TO T input binds a T variable (or another reference
		// to T) implicitly; no widening applies.
		if derefRef(ref).Equal(derefRef(argType)) {
			return
		}
	} else if p.Type.Equal(argType) || types.CanWiden(argType.Kind(), p.Type.Kind()) {
		return
	}
	if isLiteralExpr(a.Value) && isLiteralCompatible(argType.Kind(), p.Type.Kind()) {
		return
	}
	c.reportInputArg(p, a, index, argType)
}

// reportInputArg reports SEMA021 for an input argument of the wrong type.
func (c *Checker) reportInputArg(p types.Parameter, a *ast.CallArg, index int, argType types.Type) {
	pos := astPosToSource(a.Value.Span().Start)
	if a.Name == nil {
		c.diags.Errorf(pos, CodeWrongArgType,
			"argument %d: cannot pass %s as %s", index+1, argType, p.Type)
		return
	}
	c.diags.Errorf(pos, CodeWrongArgType,
		"cannot pass %s as input parameter %q (expected %s)", argType, p.Name, p.Type)
}

// checkOutputBinding checks name => target: the target must be a variable
// whose type the output widens to. An empty binding (name =>) is accepted.
func (c *Checker) checkOutputBinding(p types.Parameter, target ast.Expr) {
	if target == nil {
		return
	}
	if !isLValue(target) {
		c.checkExpr(target)
		c.diags.Errorf(astPosToSource(target.Span().Start), CodeWrongArgType,
			"output %q must be bound to a variable", p.Name)
		return
	}
	t := c.checkExpr(target)
	if t == types.Invalid || p.Type == nil || p.Type == types.Invalid {
		return
	}
	if involvesEnum(p.Type, t) {
		c.checkEnumArg(target, p.Type, t, func() { c.reportOutputBinding(p, target, t) })
		return
	}
	if t.Equal(p.Type) || types.CanWiden(p.Type.Kind(), t.Kind()) {
		return
	}
	c.reportOutputBinding(p, target, t)
}

func (c *Checker) reportOutputBinding(p types.Parameter, target ast.Expr, t types.Type) {
	c.diags.Errorf(astPosToSource(target.Span().Start), CodeWrongArgType,
		"cannot bind output %q (%s) to %s", p.Name, p.Type, t)
}

// isLValue reports whether e names storage: a variable, member, element,
// dereference or bit.
func isLValue(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident, *ast.MemberAccessExpr, *ast.IndexExpr, *ast.DerefExpr, *ast.BitAccessExpr:
		return true
	case *ast.ParenExpr:
		return isLValue(x.Inner)
	}
	return false
}

// findParam looks a parameter up by case-insensitive name.
func findParam(params []types.Parameter, name string) (types.Parameter, bool) {
	for _, p := range params {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return types.Parameter{}, false
}
