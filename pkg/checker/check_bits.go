package checker

import (
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// bitWidth returns the number of addressable bits of an integer or
// bit-string kind, and 0 for every other kind (BOOL included). The table
// matches the interpreter's (pkg/interp/bits.go).
func bitWidth(k types.TypeKind) int {
	switch k {
	case types.KindBYTE, types.KindSINT, types.KindUSINT:
		return 8
	case types.KindWORD, types.KindINT, types.KindUINT:
		return 16
	case types.KindDWORD, types.KindDINT, types.KindUDINT:
		return 32
	case types.KindLWORD, types.KindLINT, types.KindULINT:
		return 64
	}
	return 0
}

// derefRef unwraps REFERENCE TO T to T. References dereference implicitly
// wherever a value of T is expected; other types are returned unchanged.
func derefRef(t types.Type) types.Type {
	for {
		ref, ok := t.(*types.ReferenceType)
		if !ok || ref.BaseType == nil {
			return t
		}
		t = ref.BaseType
	}
}

// checkBitAccessExpr types target.N (DIAL-04). The target must be an
// integer or bit-string type (after reference deref), and the literal index
// must lie in 0..width-1; both errors are SEMA035. The result is BOOL.
func (c *Checker) checkBitAccessExpr(e *ast.BitAccessExpr) types.Type {
	targetType := c.checkExpr(e.Target)
	if targetType == types.Invalid {
		return types.Invalid
	}
	targetType = derefRef(targetType)
	width := bitWidth(targetType.Kind())
	if width == 0 {
		c.diags.Errorf(astPosToSource(e.Span().Start), CodeBitAccess,
			"bit access on non-integer type %s", targetType)
		return types.Invalid
	}
	if lit, ok := e.Index.(*ast.Literal); ok {
		// An index that does not fit uint64 (a 20-digit literal) is out of
		// range like any other large index.
		idx, err := strconv.ParseUint(strings.ReplaceAll(lit.Value, "_", ""), 10, 64)
		if err != nil || idx >= uint64(width) {
			c.reportBitRange(e.Index, lit.Value, targetType, width)
		}
	}
	return types.TypeBOOL
}

// constBitIndex recognises v.c, where v is an integer or bit-string value
// and c names a CONSTANT integer variable, as bit access with a constant
// index (ruling A3: the parser keeps it a MemberAccessExpr). ok is false
// for every other member access, so struct, FB and enum members keep their
// meaning. known reports whether the constant's value is statically known.
func (c *Checker) constBitIndex(e *ast.MemberAccessExpr, objType types.Type) (value int64, known, ok bool) {
	if bitWidth(derefRef(objType).Kind()) == 0 {
		return 0, false, false
	}
	sym := c.constIndexSymbol(e.Member)
	if sym == nil {
		return 0, false, false
	}
	sym.MarkUsed()
	return sym.ConstInt, sym.HasConstInt, true
}

// constIndexSymbol returns the variable that member names when it is a
// CONSTANT (VAR CONSTANT with an integer literal value, or VAR_GLOBAL
// CONSTANT) of integer or bit-string type, else nil.
func (c *Checker) constIndexSymbol(member *ast.Ident) *symbols.Symbol {
	if member == nil || c.currentScope == nil {
		return nil
	}
	sym := c.currentScope.Lookup(member.Name)
	if sym == nil || sym.Kind != symbols.KindVariable || !(sym.IsConstant || sym.HasConstInt) {
		return nil
	}
	if t, ok := sym.Type.(types.Type); !ok || bitWidth(t.Kind()) == 0 {
		return nil
	}
	return sym
}

// checkConstBitIndex types v.c as BOOL when constBitIndex accepts it and
// range-checks a known constant against the width of v's type.
func (c *Checker) checkConstBitIndex(e *ast.MemberAccessExpr, objType types.Type) (types.Type, bool) {
	value, known, ok := c.constBitIndex(e, objType)
	if !ok {
		return nil, false
	}
	objType = derefRef(objType)
	width := bitWidth(objType.Kind())
	if known && (value < 0 || value >= int64(width)) {
		c.reportBitRange(e.Member, strconv.FormatInt(value, 10), objType, width)
	}
	return types.TypeBOOL, true
}

func (c *Checker) reportBitRange(at ast.Node, index string, t types.Type, width int) {
	c.diags.Errorf(astPosToSource(at.Span().Start), CodeBitAccess,
		"bit index %s out of range for %s (0..%d)", index, t, width-1)
}

// assignRoot strips bit access from an assignment target, so the constant
// check sees the variable whose bit is written: w.3, w.cBit and G.w.cBit
// all write w (or G.w). A member of a GVL symbol is never stripped, since
// G.cBit names the GVL variable itself.
func (c *Checker) assignRoot(target ast.Expr) ast.Expr {
	for {
		switch t := target.(type) {
		case *ast.BitAccessExpr:
			target = t.Target
			continue
		case *ast.MemberAccessExpr:
			if !c.isGVLIdent(t.Object) && c.constIndexSymbol(t.Member) != nil {
				target = t.Object
				continue
			}
		}
		return target
	}
}

func (c *Checker) isGVLIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok || c.currentScope == nil {
		return false
	}
	sym := c.currentScope.Lookup(id.Name)
	return sym != nil && sym.Kind == symbols.KindGVL
}
