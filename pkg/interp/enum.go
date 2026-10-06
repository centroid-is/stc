package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// EnumDef is the runtime description of an enumeration type.
type EnumDef struct {
	// Name is the upper-cased type name; inline VAR enums use <POU>.<VAR>.
	Name string
	// Base is the IEC type of the values (INT unless declared otherwise).
	Base types.TypeKind
	// Values maps an upper-cased value name to its ordinal.
	Values map[string]int64
	// Names maps an ordinal to the value name in declared casing. When two
	// values share an ordinal the first declared one is kept.
	Names map[int64]string
	// ToString is set by {attribute 'to_string'}: TO_STRING then returns the
	// value name instead of the number.
	ToString bool
}

// value returns the enum value called member (case-insensitive).
func (d *EnumDef) value(member string) (Value, bool) {
	n, ok := d.Values[strings.ToUpper(member)]
	if !ok {
		return Value{}, false
	}
	return Value{Kind: ValInt, Int: n, IECType: d.Base, Enum: d.Name}, true
}

// pendingEnum is an enumeration whose numbering needs a constant that was
// not available when it was registered.
type pendingEnum struct {
	name  string
	et    *ast.EnumType
	attrs []*ast.Attribute
	local func(string) (int64, bool)
}

// RegisterEnumDecl registers the enumeration et under name. Values are
// numbered with ast.EnumOrdinalsWith (the previous+1 rule shared with the
// checker); a value that names a constant (ka := C_BASE, GVL.C, C + 1) is
// evaluated against the registered GVLs, and numbered again when a later
// RegisterGVL supplies it. The base type comes from et.BaseType and
// defaults to INT; attrs supplies the to_string attribute. The legacy
// EnumTypes map is filled as well. A nil et is ignored.
func (interp *Interpreter) RegisterEnumDecl(name string, et *ast.EnumType, attrs []*ast.Attribute) {
	interp.registerEnum(name, et, attrs, nil)
}

// registerEnum implements RegisterEnumDecl. local resolves a bare constant
// name before the GVLs are consulted (an inline enum's POU constants).
func (interp *Interpreter) registerEnum(name string, et *ast.EnumType, attrs []*ast.Attribute, local func(string) (int64, bool)) {
	if et == nil {
		return
	}
	def := &EnumDef{
		Name:     strings.ToUpper(name),
		Base:     enumBase(et),
		Values:   make(map[string]int64),
		Names:    make(map[int64]string),
		ToString: ast.HasAttribute(attrs, "to_string"),
	}
	complete := true
	for _, ord := range ast.EnumOrdinalsWith(et, interp.enumConst(local)) {
		def.Values[strings.ToUpper(ord.Name)] = ord.Value
		if _, dup := def.Names[ord.Value]; !dup {
			def.Names[ord.Value] = ord.Name
		}
		complete = complete && ord.Known
	}
	interp.addEnumDef(def)
	if !complete {
		interp.pendingEnums = append(interp.pendingEnums, pendingEnum{name: name, et: et, attrs: attrs, local: local})
	}
}

// enumConst returns the evaluator for enum values that are not literals:
// constant expressions over local, then bare GVL variables, then GVL.C.
func (interp *Interpreter) enumConst(local func(string) (int64, bool)) func(ast.Expr) (int64, bool) {
	return func(x ast.Expr) (int64, bool) {
		return ast.ConstIntValue(x, func(qual, name string) (int64, bool) {
			var v Value
			var ok bool
			switch {
			case qual != "":
				if g := interp.lookupGVL(qual); g != nil {
					v, ok = g.GetLocal(name)
				}
			default:
				if local != nil {
					if n, found := local(name); found {
						return n, true
					}
				}
				if g := interp.GlobalParent(); g != nil {
					v, ok = g.Get(name)
				}
			}
			return v.Int, ok && v.Kind == ValInt
		})
	}
}

// retryPendingEnums numbers again every enum that waited for a constant.
// Enums still unresolved stay pending.
func (interp *Interpreter) retryPendingEnums() {
	pending := interp.pendingEnums
	interp.pendingEnums = nil
	for _, p := range pending {
		interp.registerEnum(p.name, p.et, p.attrs, p.local)
	}
}

// blockConsts returns a lookup of the integer VAR CONSTANT entries of
// blocks, each evaluated with ast.ConstIntValue against the others and
// the GVLs. A constant that refers back to itself is unresolved.
func (interp *Interpreter) blockConsts(blocks []*ast.VarBlock) func(string) (int64, bool) {
	inits := make(map[string]ast.Expr)
	for _, vb := range blocks {
		if vb == nil || !vb.IsConstant {
			continue
		}
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				if vd.InitValue != nil {
					inits[strings.ToUpper(n.Name)] = vd.InitValue
				}
			}
		}
	}
	if len(inits) == 0 {
		return nil
	}
	visiting := make(map[string]bool)
	var lookup func(string) (int64, bool)
	lookup = func(name string) (int64, bool) {
		key := strings.ToUpper(name)
		x, ok := inits[key]
		if !ok || visiting[key] {
			return 0, false
		}
		visiting[key] = true
		defer delete(visiting, key)
		return interp.enumConst(lookup)(x)
	}
	return lookup
}

// addEnumDef stores def in EnumDefs and mirrors its values into EnumTypes.
func (interp *Interpreter) addEnumDef(def *EnumDef) {
	if interp.EnumDefs == nil {
		interp.EnumDefs = make(map[string]*EnumDef)
	}
	interp.EnumDefs[def.Name] = def
	if interp.EnumTypes == nil {
		interp.EnumTypes = make(map[string]map[string]int64)
	}
	interp.EnumTypes[def.Name] = def.Values
}

// lookupBareEnum resolves a bare enum value name against every registered
// enumeration. Duplicate bare names across enums are a checker error.
func (interp *Interpreter) lookupBareEnum(name string) (Value, bool) {
	for _, def := range interp.EnumDefs {
		if v, ok := def.value(name); ok {
			return v, true
		}
	}
	return Value{}, false
}

// qualifiedEnum resolves E.v when E is not a variable in env (and, since
// gvlRoot runs first, not a GVL) but a registered enumeration. handled is
// false when E is a variable or not an enum, so the caller falls through to
// ordinary member access (T-20-14).
func (interp *Interpreter) qualifiedEnum(env *Env, e *ast.MemberAccessExpr) (v Value, handled bool, err error) {
	id, ok := e.Object.(*ast.Ident)
	if !ok || interp.EnumDefs == nil || e.Member == nil {
		return Value{}, false, nil
	}
	if _, isVar := env.Get(id.Name); isVar {
		return Value{}, false, nil
	}
	def, ok := interp.EnumDefs[strings.ToUpper(id.Name)]
	if !ok {
		return Value{}, false, nil
	}
	if v, ok := def.value(e.Member.Name); ok {
		return v, true, nil
	}
	return Value{}, true, &RuntimeError{
		Msg: fmt.Sprintf("enum '%s' has no value '%s'", id.Name, e.Member.Name),
		Pos: e.Member.Span().Start,
	}
}

// enumString returns the value name of v when v belongs to an enumeration
// with the to_string attribute and its ordinal has a name.
func (interp *Interpreter) enumString(v Value) (string, bool) {
	if v.Enum == "" || interp.EnumDefs == nil {
		return "", false
	}
	def, ok := interp.EnumDefs[v.Enum]
	if !ok || !def.ToString {
		return "", false
	}
	name, ok := def.Names[v.Int]
	return name, ok
}

// zeroEnum returns the default value of an enumeration: its first value,
// typed by its base type.
func zeroEnum(et *ast.EnumType) Value {
	v := Value{Kind: ValInt, IECType: enumBase(et)}
	if ords := ast.EnumOrdinals(et); len(ords) > 0 {
		v.Int = ords[0].Value
	}
	return v
}

// enumBase returns the IEC kind of an enumeration's base type: the declared
// elementary type, or INT when none (or an unknown one) is given.
func enumBase(et *ast.EnumType) types.TypeKind {
	if nt, ok := et.BaseType.(*ast.NamedType); ok && nt.Name != nil {
		if t, found := types.LookupElementaryType(strings.ToUpper(nt.Name.Name)); found {
			return t.Kind()
		}
	}
	return types.KindINT
}
