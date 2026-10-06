package checker

import (
	"math"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// Initialiser checks (DIAL-10 residual gaps). Type-checking only: applying
// initialisers at runtime is Phase 22 (RUNT-06). Values are checked
// against the target type only when they are literals; a non-literal value
// such as A + 1 is walked for undeclared names but not type-checked, since
// untyped literal arithmetic is typed in Phase 22 (RUNT-05).

// checkVarInitializers checks the initialisers of every declaration in
// blocks. scope resolves both the declared variables and the names used in
// initialiser values.
func (c *Checker) checkVarInitializers(blocks []*ast.VarBlock, scope *symbols.Scope) {
	if scope == nil {
		return
	}
	saved := c.currentScope
	c.currentScope = scope
	for _, vb := range blocks {
		for _, vd := range vb.Declarations {
			if vd.InitValue == nil || len(vd.Names) == 0 {
				continue
			}
			sym := scope.LookupLocal(vd.Names[0].Name)
			if sym == nil {
				continue
			}
			c.checkInitializer(vd.InitValue, symbolType(sym))
		}
	}
	c.currentScope = saved
}

// checkGVLInitializers checks a GVL's initialisers. The variable types come
// from the GVL's qualified symbol, so qualified_only GVLs are covered too.
func (c *Checker) checkGVLInitializers(d *ast.GVLDecl) {
	if d.Name == nil {
		return
	}
	gvl := c.table.LookupGlobal(d.Name.Name)
	st, ok := gvl.Type.(*types.StructType)
	if !ok || gvl.Kind != symbols.KindGVL || gvl.IsLibrary {
		return
	}
	saved := c.currentScope
	c.currentScope = c.table.GlobalScope()
	for _, vb := range d.Blocks {
		for _, vd := range vb.Declarations {
			if vd.InitValue == nil || len(vd.Names) == 0 {
				continue
			}
			c.checkInitializer(vd.InitValue, structMemberType(st, vd.Names[0].Name))
		}
	}
	c.currentScope = saved
}

// checkStructMemberInitializers checks the member initialisers of a STRUCT
// TYPE declaration.
func (c *Checker) checkStructMemberInitializers(d *ast.TypeDecl, spec *ast.StructType) {
	sym := c.table.LookupGlobal(d.Name.Name)
	st, ok := sym.Type.(*types.StructType)
	if !ok || sym.IsLibrary {
		return
	}
	saved := c.currentScope
	c.currentScope = c.table.GlobalScope()
	for _, m := range spec.Members {
		if m.InitValue == nil || m.Name == nil {
			continue
		}
		c.checkInitializer(m.InitValue, structMemberType(st, m.Name.Name))
	}
	c.currentScope = saved
}

func symbolType(sym *symbols.Symbol) types.Type {
	if t, ok := sym.Type.(types.Type); ok {
		return t
	}
	return types.Invalid
}

func structMemberType(st *types.StructType, name string) types.Type {
	for _, m := range st.Members {
		if strings.EqualFold(m.Name, name) {
			return m.Type
		}
	}
	return types.Invalid
}

// checkInitializer checks init against target. Struct initialisers check
// field names against struct members or FB inputs and outputs (SEMA024);
// array initialisers count their elements, repetitions included, without
// expanding them, and report too many when the bounds are known.
func (c *Checker) checkInitializer(init ast.Expr, target types.Type) {
	if target == nil || target == types.Invalid {
		return
	}
	switch x := init.(type) {
	case *ast.StructInit:
		c.checkStructInit(x, target)
	case *ast.ArrayInit:
		c.checkArrayInit(x, target)
	default:
		c.checkPlainInit(init, target)
	}
}

func (c *Checker) checkStructInit(x *ast.StructInit, target types.Type) {
	var lookup func(name string) (types.Type, bool)
	var missing string
	switch t := target.(type) {
	case *types.StructType:
		lookup = func(name string) (types.Type, bool) {
			for _, m := range t.Members {
				if strings.EqualFold(m.Name, name) {
					return m.Type, true
				}
			}
			return nil, false
		}
		missing = "type " + t.String() + " has no member %q"
	case *types.FunctionBlockType:
		lookup = func(name string) (types.Type, bool) {
			if p, ok := findParam(t.Inputs, name); ok {
				return p.Type, true
			}
			if p, ok := findParam(t.Outputs, name); ok {
				return p.Type, true
			}
			return nil, false
		}
		missing = t.Name + " has no input parameter %q"
	default:
		c.diags.Errorf(astPosToSource(x.Span().Start), CodeTypeMismatch,
			"structure initialiser for non-structure type %s", target)
		return
	}
	seen := make(map[string]bool)
	for _, f := range x.Fields {
		if f == nil || f.Name == nil {
			continue
		}
		pos := astPosToSource(f.Name.Span().Start)
		ft, ok := lookup(f.Name.Name)
		if !ok {
			c.diags.Errorf(pos, CodeNoMember, missing, f.Name.Name)
			continue
		}
		key := strings.ToUpper(f.Name.Name)
		if seen[key] {
			c.diags.Errorf(pos, CodeNoMember, "member %q initialised more than once", f.Name.Name)
			continue
		}
		seen[key] = true
		if f.Value != nil {
			c.checkInitializer(f.Value, ft)
		}
	}
}

func (c *Checker) checkArrayInit(x *ast.ArrayInit, target types.Type) {
	arr, ok := target.(*types.ArrayType)
	if !ok {
		c.diags.Errorf(astPosToSource(x.Span().Start), CodeTypeMismatch,
			"array initialiser for non-array type %s", target)
		return
	}
	// [[1, 2], [3, 4]] on ARRAY[a..b, c..d] initialises the first
	// dimension with rows; a flat list fills every dimension in order.
	dims := arr.Dimensions
	elemType := arr.ElementType
	if len(dims) > 1 && hasNestedArrayInit(x) {
		elemType = &types.ArrayType{ElementType: arr.ElementType, Dimensions: dims[1:]}
		dims = dims[:1]
	}

	var total int64
	countKnown := true
	for _, el := range x.Elements {
		if el == nil {
			continue
		}
		n := int64(1)
		if el.Count != nil {
			v, ok := ast.IntLiteralValue(el.Count)
			if !ok || v < 0 {
				countKnown = false
				c.checkExpr(el.Count)
			}
			n = v
		}
		total = satAdd(total, n)
		if el.Value != nil {
			c.checkInitializer(el.Value, elemType)
		}
	}
	if capacity, known := arrayCapacity(dims); known && countKnown && total > capacity {
		c.diags.Errorf(astPosToSource(x.Span().Start), CodeTypeMismatch,
			"too many initialisers (%d > %d)", total, capacity)
	}
}

func hasNestedArrayInit(x *ast.ArrayInit) bool {
	for _, el := range x.Elements {
		if el != nil {
			if _, ok := el.Value.(*ast.ArrayInit); ok {
				return true
			}
		}
	}
	return false
}

// arrayCapacity returns the element count of dims, saturating at
// math.MaxInt64. known is false when a bound is not statically known or a
// dimension is empty.
func arrayCapacity(dims []types.ArrayDimension) (int64, bool) {
	capacity := int64(1)
	for _, d := range dims {
		if !d.Known || d.High < d.Low {
			return 0, false
		}
		capacity = satMul(capacity, int64(d.High)-int64(d.Low)+1)
	}
	return capacity, true
}

// satAdd adds two non-negative counts, saturating at math.MaxInt64.
func satAdd(a, b int64) int64 {
	if b > math.MaxInt64-a {
		return math.MaxInt64
	}
	return a + b
}

// satMul multiplies two positive counts, saturating at math.MaxInt64.
func satMul(a, b int64) int64 {
	if b != 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

// checkPlainInit checks a value initialiser. Only literals are type-checked
// (checker blocker 2); every value is walked for undeclared names.
func (c *Checker) checkPlainInit(value ast.Expr, target types.Type) {
	valueType := c.checkExpr(value)
	if valueType == types.Invalid {
		return
	}
	if k, _, _ := untypedConst(value); k == untypedNone && !isLiteralExpr(value) {
		return
	}
	if asEnum(target) != nil {
		c.checkEnumArg(value, valueType, target, func() { c.reportInitMismatch(value, valueType, target) })
		return
	}
	if _, primitive := target.(*types.PrimitiveType); !primitive {
		// Pointers, references, arrays, structs and FBs given a plain
		// literal (p : POINTER TO INT := 0) are left to Phase 22.
		return
	}
	// Initialisers and assignments share one untyped-constant rule.
	if c.untypedStore(value, target, CodeTypeMismatch) {
		return
	}
	if !initLiteralCompatible(valueType, target) {
		c.reportInitMismatch(value, valueType, target)
	}
}

func (c *Checker) reportInitMismatch(value ast.Expr, valueType, target types.Type) {
	c.diags.Errorf(astPosToSource(value.Span().Start), CodeTypeMismatch,
		"cannot initialise %s with %s", target, valueType)
}

// initLiteralCompatible reports whether a typed literal of type valueType
// may initialise a variable of the elementary type target (untyped
// constants are handled by untypedAssignable first). It allows widening,
// an integer literal initialising a bit-string or real variable and a
// string literal a CHAR or WCHAR, as CODESYS does.
func initLiteralCompatible(valueType, target types.Type) bool {
	vk, tk := valueType.Kind(), target.Kind()
	switch {
	case valueType.Equal(target), types.CanWiden(vk, tk):
		return true
	case types.IsAnyInt(vk) && (types.IsAnyReal(tk) || (types.IsAnyBit(tk) && tk != types.KindBOOL)):
		return true
	case types.IsAnyString(vk) && types.IsAnyChar(tk):
		return true
	}
	return false
}
