package checker

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// REF=, THIS and SUPER (DIAL-09). The messages mirror the interpreter's
// runtime errors (pkg/interp, 20-07).

// checkRefAssignStmt checks r REF= x: r must be a REFERENCE TO T and x a
// variable path (identifier, member, element or dereference) of type T,
// or another reference to T. Every violation is SEMA038.
func (c *Checker) checkRefAssignStmt(s *ast.RefAssignStmt) {
	targetType := c.checkExpr(s.Target)
	if !isRefTarget(s.Value) {
		c.checkExpr(s.Value)
		if targetType != types.Invalid {
			c.diags.Errorf(astPosToSource(s.Value.Span().Start), CodeRefThisSuper,
				"REF= requires a variable on the right-hand side")
		}
		return
	}
	valueType := c.checkExpr(s.Value)
	if targetType == types.Invalid {
		return
	}
	ref, ok := targetType.(*types.ReferenceType)
	if !ok {
		c.diags.Errorf(astPosToSource(s.Target.Span().Start), CodeRefThisSuper,
			"REF= target %s is not a REFERENCE TO (type %s)", exprText(s.Target), targetType)
		return
	}
	if valueType == types.Invalid {
		return
	}
	if !derefRef(valueType).Equal(derefRef(ref)) {
		c.diags.Errorf(astPosToSource(s.Value.Span().Start), CodeRefThisSuper,
			"REF= type mismatch: cannot bind %s to %s", ref, derefRef(valueType))
	}
}

// isRefTarget reports whether e names storage a reference can bind to. A
// bit cannot be referenced.
func isRefTarget(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident, *ast.MemberAccessExpr, *ast.IndexExpr, *ast.DerefExpr:
		return true
	case *ast.ParenExpr:
		return isRefTarget(x.Inner)
	}
	return false
}

// exprText names a REF= target for a diagnostic.
func exprText(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return "'" + id.Name + "'"
	}
	return "expression"
}

// checkThisExpr types THIS as POINTER TO the current FUNCTION_BLOCK. It is
// SEMA038 outside an FB body or action (METHOD bodies are not checked yet).
func (c *Checker) checkThisExpr(e *ast.ThisExpr) types.Type {
	if c.currentFB == nil {
		c.diags.Errorf(astPosToSource(e.Span().Start), CodeRefThisSuper,
			"THIS used outside a function block")
		return types.Invalid
	}
	return c.fbPointer(c.currentFB.Name.Name)
}

// checkSuperExpr types SUPER as POINTER TO the base FB of the current
// FUNCTION_BLOCK. It is SEMA038 outside an FB and in an FB without EXTENDS.
// An undeclared base is already reported by the resolver (SEMA037).
func (c *Checker) checkSuperExpr(e *ast.SuperExpr) types.Type {
	pos := astPosToSource(e.Span().Start)
	if c.currentFB == nil {
		c.diags.Errorf(pos, CodeRefThisSuper, "SUPER used outside a function block")
		return types.Invalid
	}
	if c.currentFB.Extends == nil {
		c.diags.Errorf(pos, CodeRefThisSuper,
			"SUPER used in %s, which does not EXTEND another function block", c.currentFB.Name.Name)
		return types.Invalid
	}
	return c.fbPointer(c.currentFB.Extends.Name)
}

// fbPointer returns POINTER TO the FB type called name, or Invalid when the
// name does not resolve to a function block.
func (c *Checker) fbPointer(name string) types.Type {
	sym := c.table.LookupGlobal(name)
	if sym == nil {
		return types.Invalid
	}
	fb, ok := sym.Type.(*types.FunctionBlockType)
	if !ok {
		return types.Invalid
	}
	return &types.PointerType{BaseType: fb}
}

// selfMember resolves THIS^.m and SUPER^.m through the POU scope chain of
// the FB, so local variables, methods and inherited members are visible.
// ok is false when obj is not THIS^ or SUPER^.
func (c *Checker) selfMember(obj ast.Expr, objType types.Type, member *ast.Ident) (types.Type, bool) {
	deref, isDeref := obj.(*ast.DerefExpr)
	if !isDeref {
		return nil, false
	}
	switch deref.Operand.(type) {
	case *ast.ThisExpr, *ast.SuperExpr:
	default:
		return nil, false
	}
	fb := objType.(*types.FunctionBlockType)
	scope := c.table.LookupPOU(fb.Name)
	sym := lookupInPOUChain(scope, member.Name)
	if sym == nil {
		c.diags.Errorf(astPosToSource(member.Span().Start), CodeNoMember,
			"type %s has no member %q", fb, member.Name)
		return types.Invalid, true
	}
	sym.MarkUsed()
	if t, ok := sym.Type.(types.Type); ok {
		return t, true
	}
	return types.Invalid, true
}

// checkMemberCalleeRoot validates THIS^ and SUPER^ at the root of a member
// callee (THIS^.M(), SUPER^.M()), which is otherwise accepted unchecked
// (research Pitfall 11), and marks an identifier root as used.
func (c *Checker) checkMemberCalleeRoot(callee ast.Expr) {
	root := callee
	for {
		m, ok := root.(*ast.MemberAccessExpr)
		if !ok {
			break
		}
		root = m.Object
	}
	if d, ok := root.(*ast.DerefExpr); ok {
		switch d.Operand.(type) {
		case *ast.ThisExpr, *ast.SuperExpr:
			c.checkExpr(d)
			return
		}
	}
	c.markRootUsed(callee)
}

// isReference reports whether t is a REFERENCE TO type.
func isReference(t types.Type) bool {
	_, ok := t.(*types.ReferenceType)
	return ok
}
