package checker

import (
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// Enum rules (DIAL-07, CODESYS semantics):
//   - a value of a qualified_only enum must be written E.v; bare v is SEMA036;
//   - a strict enum takes no arithmetic and converts to or from an integer
//     only through an explicit conversion function (SEMA036);
//   - a non-strict enum converts implicitly to and from its base integer
//     type and takes part in arithmetic and comparison as that type
//     (ruling A4);
//   - two different enum types never assign to each other; comparing them
//     is SEMA036 when either is strict and allowed otherwise, as before.

// asEnum returns t as an enum type, or nil.
func asEnum(t types.Type) *types.EnumType {
	et, _ := t.(*types.EnumType)
	return et
}

// involvesEnum reports whether a or b is an enum type.
func involvesEnum(a, b types.Type) bool {
	return asEnum(a) != nil || asEnum(b) != nil
}

// enumBase returns the base integer type of et (INT when unset).
func enumBase(et *types.EnumType) types.Type {
	if et.BaseType == types.KindInvalid {
		return types.TypeINT
	}
	return &types.PrimitiveType{Kind_: et.BaseType}
}

// asBase replaces a non-strict enum type by its base integer type; every
// other type is returned unchanged.
func asBase(t types.Type) types.Type {
	if et := asEnum(t); et != nil && !et.Strict {
		return enumBase(et)
	}
	return t
}

// strictOf returns the first strict enum among ts, or nil.
func strictOf(ts ...types.Type) *types.EnumType {
	for _, t := range ts {
		if et := asEnum(t); et != nil && et.Strict {
			return et
		}
	}
	return nil
}

// reportQualifiedEnumValue reports SEMA036 when name is a value of a
// qualified_only enum, written bare. It returns false (and reports nothing)
// when no such enum declares name, so the caller reports SEMA010.
func (c *Checker) reportQualifiedEnumValue(pos source.Pos, name string) bool {
	var hits []*types.EnumType
	for _, sym := range c.table.GlobalScope().Symbols() {
		if sym.Kind != symbols.KindType {
			continue
		}
		et, ok := sym.Type.(*types.EnumType)
		if !ok || !et.Qualified {
			continue
		}
		for _, v := range et.Values {
			if strings.EqualFold(v, name) {
				hits = append(hits, et)
				break
			}
		}
	}
	if len(hits) == 0 {
		return false
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Name < hits[j].Name })
	et := hits[0]
	value := name
	for _, v := range et.Values {
		if strings.EqualFold(v, name) {
			value = v
		}
	}
	c.diags.Errorf(pos, CodeEnumRule,
		"enum value '%s' of qualified_only enum %s must be written %s.%s", value, et.Name, et.Name, value)
	return true
}

// checkEnumBinary applies the enum rules to a binary operation. It returns
// the operand types to use for the ordinary operator rules (non-strict
// enums replaced by their base type), or ok == false after reporting a
// strict-enum violation. done is true when the result type is decided
// here (result then holds it).
func (c *Checker) checkEnumBinary(e *ast.BinaryExpr, op string, left, right types.Type) (l, r, result types.Type, done, ok bool) {
	if !involvesEnum(left, right) || isBooleanOp(op) {
		return left, right, nil, false, true
	}
	pos := astPosToSource(e.Op.Span.Start)
	switch {
	case isArithmeticOp(op):
		if et := strictOf(left, right); et != nil {
			c.diags.Errorf(pos, CodeEnumRule,
				"operator %s not allowed on strict enum %s", e.Op.Text, et.Name)
			return nil, nil, nil, true, false
		}
	case isComparisonOp(op):
		le, re := asEnum(left), asEnum(right)
		if le != nil && re != nil && le.Equal(re) {
			return left, right, types.TypeBOOL, true, true
		}
		if et := strictOf(left, right); et != nil {
			other := right
			if asEnum(left) != et {
				other = left
			}
			c.diags.Errorf(pos, CodeEnumRule,
				"cannot compare strict enum %s with %s", et.Name, other)
			return nil, nil, nil, true, false
		}
		if le != nil && re != nil {
			// Two different non-strict enums compare as before.
			return left, right, types.TypeBOOL, true, true
		}
	}
	return asBase(left), asBase(right), nil, false, true
}

// enumAssignable decides whether a value of type from may be stored in a
// variable or parameter of type to, when either is an enum. value is the
// stored expression (nil when unknown); an untyped constant value adopts a
// non-strict enum's base type. strictViolation is set when the
// conversion fails only because a strict enum is involved (SEMA036).
func enumAssignable(from, to types.Type, value ast.Expr) (ok bool, strictViolation bool) {
	fe, te := asEnum(from), asEnum(to)
	if fe != nil && te != nil {
		if fe.Equal(te) {
			return true, false
		}
		return false, fe.Strict || te.Strict
	}
	if strictOf(from, to) != nil {
		return false, true
	}
	from, to = asBase(from), asBase(to)
	if from.Equal(to) || types.CanWiden(from.Kind(), to.Kind()) {
		return true, false
	}
	if ok, _ := untypedAssignable(value, to); ok {
		return true, false
	}
	return false, false
}

// reportStrictConversion reports an implicit conversion that involves a
// strict enum; enumAssignable sets strictViolation only when one does.
func (c *Checker) reportStrictConversion(pos source.Pos, from, to types.Type) {
	et := strictOf(from, to)
	c.diags.Errorf(pos, CodeEnumRule,
		"no implicit conversion from %s to %s: %s is a strict enum, use an explicit conversion", from, to, et.Name)
}

// isConversionBuiltin reports whether name is a built-in conversion
// function: <FROM>_TO_<TO> or the overloaded TO_<TO>. These take a strict
// enum argument explicitly (CONTEXT DIAL-07).
func isConversionBuiltin(name string) bool {
	upper := strings.ToUpper(name)
	if _, ok := types.BuiltinFunctions[upper]; !ok {
		return false
	}
	return strings.HasPrefix(upper, "TO_") || strings.Contains(upper, "_TO_")
}

// caseLabelCompatible checks one CASE label value against the selector
// type, applying the enum rules. An untyped constant label adopts the
// selector type (range-checked). It reports at most one diagnostic.
func (c *Checker) caseLabelCompatible(at ast.Node, selector types.Type, value ast.Expr, label types.Type) {
	if selector == types.Invalid || label == types.Invalid {
		return
	}
	pos := astPosToSource(at.Span().Start)
	se, le := asEnum(selector), asEnum(label)
	if se != nil && le != nil {
		if !se.Equal(le) {
			c.diags.Errorf(pos, CodeTypeMismatch,
				"case label type %s incompatible with selector type %s", label, selector)
		}
		return
	}
	if et := strictOf(selector, label); et != nil {
		c.diags.Errorf(pos, CodeEnumRule,
			"case label type %s incompatible with strict enum selector type %s", label, selector)
		return
	}
	if c.untypedStore(value, asBase(selector), CodeTypeMismatch) {
		return
	}
	if _, ok := types.CommonType(asBase(selector).Kind(), asBase(label).Kind()); !ok {
		c.diags.Errorf(pos, CodeTypeMismatch,
			"case label type %s incompatible with selector type %s", label, selector)
	}
}

// checkEnumAssign checks target := value when either side is an enum.
func (c *Checker) checkEnumAssign(s *ast.AssignStmt, from, to types.Type) {
	c.checkEnumArg(s.Value, from, to, func() {
		c.diags.Errorf(astPosToSource(s.Span().Start), CodeTypeMismatch,
			"cannot assign %s to %s", from, to)
	})
}

// checkEnumArg checks that value (of type from) may be stored as type to,
// when either is an enum. A strict-enum violation is SEMA036 at value; any
// other mismatch is reported by mismatch.
func (c *Checker) checkEnumArg(value ast.Expr, from, to types.Type, mismatch func()) {
	ok, strict := enumAssignable(from, to, value)
	switch {
	case ok:
	case strict:
		c.reportStrictConversion(astPosToSource(value.Span().Start), from, to)
	default:
		mismatch()
	}
}
