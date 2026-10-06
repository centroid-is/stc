package checker

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// ResolveOpts provides optional configuration for CollectDeclarations.
type ResolveOpts struct {
	// LibraryFiles are parsed library stub files that are registered before
	// user code. Symbols from library files are marked with IsLibrary=true
	// and can be overridden by user code without redeclaration errors.
	LibraryFiles []*ast.SourceFile

	// MockFiles are parsed mock FB files that override library symbols.
	// Mock files are registered after user files and can only override
	// symbols with IsLibrary=true. Attempting to override user-defined
	// symbols produces a redeclaration error. Mock symbols are NOT marked
	// IsLibrary (they have bodies and are real implementations).
	MockFiles []*ast.SourceFile
}

// Resolver performs Pass 1 of semantic analysis: collecting all
// declarations into the symbol table before any body is type-checked.
// This ensures forward references between POUs work correctly.
type Resolver struct {
	table *symbols.Table
	diags *diag.Collector

	// pendingGVLs holds GVL declarations seen during collection. They are
	// resolved after every file's TYPE declarations are registered, so a GVL
	// typed with a DUT from a later file gets the real struct type instead of
	// resolveTypeSpec's placeholder FunctionBlockType.
	pendingGVLs []pendingGVL
}

type pendingGVL struct {
	decl      *ast.GVLDecl
	isLibrary bool
}

// NewResolver creates a new Resolver that populates the given symbol table.
func NewResolver(table *symbols.Table, diags *diag.Collector) *Resolver {
	return &Resolver{table: table, diags: diags}
}

// CollectDeclarations walks all source files and registers POU declarations,
// type declarations, and their variables in the symbol table. The variadic
// opts parameter preserves backward compatibility -- existing callers pass
// no opts. When opts are provided, LibraryFiles are registered first with
// IsLibrary=true, before user code is processed.
func (r *Resolver) CollectDeclarations(files []*ast.SourceFile, opts ...ResolveOpts) {
	// Register library files first (if provided)
	if len(opts) > 0 && opts[0].LibraryFiles != nil {
		for _, libFile := range opts[0].LibraryFiles {
			r.collectFileDeclarations(libFile, true)
		}
	}

	// Register user files
	for _, file := range files {
		r.collectFileDeclarations(file, false)
	}

	// Register mock files (highest priority, override library symbols)
	if len(opts) > 0 && opts[0].MockFiles != nil {
		for _, mockFile := range opts[0].MockFiles {
			r.collectFileDeclarations(mockFile, false) // false = not library
		}
	}

	// Second pass: GVLs, now that every TYPE is in the global scope.
	for _, pg := range r.pendingGVLs {
		r.resolveGVL(pg.decl, pg.isLibrary)
	}
	r.pendingGVLs = nil
}

// collectFileDeclarations processes a single source file's declarations.
// When isLibrary is true, symbols are marked with IsLibrary=true.
func (r *Resolver) collectFileDeclarations(file *ast.SourceFile, isLibrary bool) {
	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.ProgramDecl:
			r.resolveProgram(d, isLibrary)
		case *ast.FunctionBlockDecl:
			r.resolveFunctionBlock(d, isLibrary)
		case *ast.FunctionDecl:
			r.resolveFunction(d, isLibrary)
		case *ast.TypeDecl:
			r.resolveTypeDecl(d, isLibrary)
		case *ast.InterfaceDecl:
			r.resolveInterface(d, isLibrary)
		case *ast.GVLDecl:
			r.pendingGVLs = append(r.pendingGVLs, pendingGVL{decl: d, isLibrary: isLibrary})
		}
	}
}

// resolveGVL registers a GVL as a global symbol whose type is a struct of
// its variables, so GVL.x resolves through ordinary member access. Variables
// of a GVL without qualified_only are also inserted into the global scope so
// bare x resolves. Variables of a qualified_only GVL are not: two such GVLs
// may declare the same name, and the checker reports SEMA033 on bare access.
func (r *Resolver) resolveGVL(d *ast.GVLDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)
	global := r.table.GlobalScope()

	// registerQualified is false when a file-derived GVL name clashes with a
	// POU: a single main.st holding VAR_GLOBAL and PROGRAM Main is a normal
	// layout, so the variables still register bare and only GVL.x is lost.
	registerQualified := true
	if existing := r.table.LookupGlobal(name); existing != nil {
		switch {
		case isLibrary && existing.IsLibrary:
			return
		case !isLibrary && existing.IsLibrary:
			r.removeGVL(existing)
		case d.NameDerived && existing.Kind != symbols.KindGVL:
			r.diags.Warnf(pos, CodeRedeclared,
				"GVL name %q (from the file name) clashes with %s %q declared at %s; qualified access is unavailable, use --gvl-name to rename the GVL",
				name, existing.Kind, existing.Name, existing.Pos)
			registerQualified = false
		default:
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	info := &symbols.GVLInfo{
		QualifiedOnly: ast.HasAttribute(d.Attributes, "qualified_only"),
		Vars:          make(map[string]bool),
		Constants:     make(map[string]bool),
	}
	for _, vb := range d.Blocks {
		if ast.HasAttribute(vb.Attributes, "qualified_only") {
			info.QualifiedOnly = true
		}
	}

	st := &types.StructType{Name: name}
	var bare []*symbols.Symbol
	for _, vb := range d.Blocks {
		for _, vd := range vb.Declarations {
			typ := r.resolveTypeSpec(vd.Type)
			for _, id := range vd.Names {
				key := strings.ToUpper(id.Name)
				st.Members = append(st.Members, types.StructMember{Name: id.Name, Type: typ})
				info.Vars[key] = true
				if vb.IsConstant {
					info.Constants[key] = true
				}
				if !info.QualifiedOnly {
					bare = append(bare, &symbols.Symbol{
						Name:       id.Name,
						Kind:       symbols.KindVariable,
						Pos:        astPosToSource(id.Span().Start),
						ParamDir:   ast.VarGlobal,
						Type:       typ,
						IsLibrary:  isLibrary,
						IsConstant: vb.IsConstant,
					})
				}
			}
		}
	}

	if registerQualified {
		_ = global.Insert(&symbols.Symbol{
			Name:      name,
			Kind:      symbols.KindGVL,
			Pos:       pos,
			Type:      st,
			IsLibrary: isLibrary,
			GVL:       info,
		})
	}
	for _, sym := range bare {
		if err := global.Insert(sym); err != nil {
			r.diags.Errorf(sym.Pos, CodeRedeclared, "%s", err.Error())
		}
	}
}

// removeGVL deletes a library symbol that user code overrides. For a library
// GVL its bare variables are deleted too, so the user GVL can redeclare them.
func (r *Resolver) removeGVL(existing *symbols.Symbol) {
	global := r.table.GlobalScope()
	r.table.RemovePOU(existing.Name)
	if existing.Kind != symbols.KindGVL || existing.GVL.QualifiedOnly {
		return
	}
	for key := range existing.GVL.Vars {
		if v := global.LookupLocal(key); v != nil && v.IsLibrary && v.ParamDir == ast.VarGlobal {
			global.Delete(key)
		}
	}
}

func (r *Resolver) resolveProgram(d *ast.ProgramDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	// Check for redeclaration
	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary {
			// Duplicate library symbol -- silently ignore (first library wins)
			return
		}
		if !isLibrary && existing.IsLibrary {
			// User code overrides library symbol -- remove library entry
			r.table.RemovePOU(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	pouScope := r.table.RegisterPOU(name, symbols.KindProgram, pos)

	// Set type on the global symbol
	if sym := r.table.LookupGlobal(name); sym != nil {
		sym.Type = &types.FunctionBlockType{Name: name}
		sym.IsLibrary = isLibrary
	}

	r.resolveVarBlocksInScope(d.VarBlocks, pouScope)
	r.resolveActions(d.Actions, pouScope)
}

// resolveActions inserts each ACTION into its POU's scope as a VOID,
// parameterless function so A1(); resolves through checkCallExpr. Actions
// share the POU scope, so a clash with a variable or method is reported as
// a redeclaration.
func (r *Resolver) resolveActions(actions []*ast.ActionDecl, scope *symbols.Scope) {
	for _, a := range actions {
		r.insertCallable(scope, a.Name, symbols.KindAction,
			&types.FunctionType{Name: a.Name.Name, ReturnType: types.TypeVOID})
	}
}

// resolveMethods inserts each METHOD of a FUNCTION_BLOCK into the FB scope
// as a function with the method's inputs as parameters, so the FB body and
// its actions can call M(...) unqualified.
func (r *Resolver) resolveMethods(methods []*ast.MethodDecl, scope *symbols.Scope) {
	for _, m := range methods {
		var ret types.Type = types.TypeVOID
		if m.ReturnType != nil {
			ret = r.resolveTypeSpec(m.ReturnType)
		}
		fn := &types.FunctionType{Name: m.Name.Name, ReturnType: ret}
		fn.Params = r.callParams(m.VarBlocks)
		r.insertCallable(scope, m.Name, symbols.KindMethod, fn)
	}
}

// callParams lists the VAR_INPUT and VAR_IN_OUT parameters of a callable in
// declaration order.
func (r *Resolver) callParams(blocks []*ast.VarBlock) []types.Parameter {
	var params []types.Parameter
	for _, vb := range blocks {
		var dir types.ParamDirection
		switch vb.Section {
		case ast.VarInput:
			dir = types.DirInput
		case ast.VarInOut:
			dir = types.DirInOut
		default:
			continue
		}
		for _, vd := range vb.Declarations {
			typ := r.resolveTypeSpec(vd.Type)
			for _, n := range vd.Names {
				params = append(params, types.Parameter{Name: n.Name, Type: typ, Direction: dir})
			}
		}
	}
	return params
}

func (r *Resolver) insertCallable(scope *symbols.Scope, name *ast.Ident, kind symbols.SymbolKind, fn *types.FunctionType) {
	pos := astPosToSource(name.Span().Start)
	sym := &symbols.Symbol{Name: name.Name, Kind: kind, Pos: pos, Type: fn}
	if err := scope.Insert(sym); err != nil {
		r.diags.Errorf(pos, CodeRedeclared, "%s", err.Error())
	}
}

func (r *Resolver) resolveFunctionBlock(d *ast.FunctionBlockDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary {
			// Duplicate library symbol -- silently ignore (first library wins)
			return
		}
		if !isLibrary && existing.IsLibrary {
			// User code overrides library symbol -- remove library entry
			r.table.RemovePOU(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	pouScope := r.table.RegisterPOU(name, symbols.KindFunctionBlock, pos)

	// Build the FunctionBlockType from var blocks
	fbType := &types.FunctionBlockType{Name: name}

	r.resolveVarBlocksInScope(d.VarBlocks, pouScope)
	r.resolveMethods(d.Methods, pouScope)
	r.resolveActions(d.Actions, pouScope)

	// Collect parameters from var blocks
	for _, vb := range d.VarBlocks {
		for _, vd := range vb.Declarations {
			resolvedType := r.resolveTypeSpec(vd.Type)
			for _, n := range vd.Names {
				param := types.Parameter{
					Name: n.Name,
					Type: resolvedType,
				}
				switch vb.Section {
				case ast.VarInput:
					param.Direction = types.DirInput
					fbType.Inputs = append(fbType.Inputs, param)
				case ast.VarOutput:
					param.Direction = types.DirOutput
					fbType.Outputs = append(fbType.Outputs, param)
				case ast.VarInOut:
					param.Direction = types.DirInOut
					fbType.InOuts = append(fbType.InOuts, param)
				}
			}
		}
	}

	// Set type on the global symbol
	if sym := r.table.LookupGlobal(name); sym != nil {
		sym.Type = fbType
		sym.IsLibrary = isLibrary
	}
}

func (r *Resolver) resolveFunction(d *ast.FunctionDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary {
			return
		}
		if !isLibrary && existing.IsLibrary {
			r.table.RemovePOU(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	pouScope := r.table.RegisterPOU(name, symbols.KindFunction, pos)

	// Resolve return type
	var retType types.Type = types.TypeVOID
	if d.ReturnType != nil {
		retType = r.resolveTypeSpec(d.ReturnType)
	}

	fnType := &types.FunctionType{
		Name:       name,
		ReturnType: retType,
	}

	r.resolveVarBlocksInScope(d.VarBlocks, pouScope)

	// Collect parameters
	for _, vb := range d.VarBlocks {
		for _, vd := range vb.Declarations {
			resolvedType := r.resolveTypeSpec(vd.Type)
			for _, n := range vd.Names {
				param := types.Parameter{
					Name: n.Name,
					Type: resolvedType,
				}
				switch vb.Section {
				case ast.VarInput:
					param.Direction = types.DirInput
					fnType.Params = append(fnType.Params, param)
				case ast.VarOutput:
					param.Direction = types.DirOutput
				case ast.VarInOut:
					param.Direction = types.DirInOut
					fnType.Params = append(fnType.Params, param)
				}
			}
		}
	}

	// Set type on the global symbol
	if sym := r.table.LookupGlobal(name); sym != nil {
		sym.Type = fnType
		sym.IsLibrary = isLibrary
	}
}

func (r *Resolver) resolveTypeDecl(d *ast.TypeDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary {
			return
		}
		if !isLibrary && existing.IsLibrary {
			r.table.GlobalScope().Delete(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	resolvedType := r.resolveTypeSpec(d.Type)

	// Set the name on struct types that don't have one
	if st, ok := resolvedType.(*types.StructType); ok && st.Name == "" {
		st.Name = name
	}
	// Set the name on enum types that don't have one
	if et, ok := resolvedType.(*types.EnumType); ok && et.Name == "" {
		et.Name = name
	}

	sym := &symbols.Symbol{
		Name:      name,
		Kind:      symbols.KindType,
		Pos:       pos,
		Type:      resolvedType,
		IsLibrary: isLibrary,
	}
	_ = r.table.GlobalScope().Insert(sym)

	// For enum types, register each enum value in the global scope
	if et, ok := resolvedType.(*types.EnumType); ok {
		for _, val := range et.Values {
			enumSym := &symbols.Symbol{
				Name: val,
				Kind: symbols.KindEnumValue,
				Pos:  pos,
				Type: resolvedType,
			}
			_ = r.table.GlobalScope().Insert(enumSym)
		}
	}
}

func (r *Resolver) resolveInterface(d *ast.InterfaceDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary {
			return
		}
		if !isLibrary && existing.IsLibrary {
			r.table.GlobalScope().Delete(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	sym := &symbols.Symbol{
		Name:      name,
		Kind:      symbols.KindInterface,
		Pos:       pos,
		IsLibrary: isLibrary,
	}
	_ = r.table.GlobalScope().Insert(sym)
}

// resolveVarBlocksInScope registers variable declarations directly
// into the given scope (bypassing the table's scope stack).
func (r *Resolver) resolveVarBlocksInScope(blocks []*ast.VarBlock, scope *symbols.Scope) {
	for _, vb := range blocks {
		for _, vd := range vb.Declarations {
			resolvedType := r.resolveTypeSpec(vd.Type)
			for _, name := range vd.Names {
				pos := astPosToSource(name.Span().Start)
				sym := &symbols.Symbol{
					Name:     name.Name,
					Kind:     symbols.KindVariable,
					Pos:      pos,
					ParamDir: vb.Section,
					Type:     resolvedType,
				}
				if err := scope.Insert(sym); err != nil {
					r.diags.Errorf(pos, CodeRedeclared, "%s", err.Error())
				}
			}
		}
	}
}

// resolveTypeSpec converts an AST type specification to a types.Type.
func (r *Resolver) resolveTypeSpec(ts ast.TypeSpec) types.Type {
	if ts == nil {
		return types.Invalid
	}

	switch t := ts.(type) {
	case *ast.NamedType:
		if t.Name == nil {
			return types.Invalid
		}
		name := t.Name.Name
		// Try elementary type first
		if typ, ok := types.LookupElementaryType(name); ok {
			return typ
		}
		// Look up user-defined type in table
		if sym := r.table.GlobalScope().Lookup(name); sym != nil {
			if sym.Type != nil {
				if typ, ok := sym.Type.(types.Type); ok {
					return typ
				}
			}
		}
		// Forward reference -- create a placeholder FunctionBlockType.
		// This handles cases where an FB is referenced before its declaration.
		return &types.FunctionBlockType{Name: name}

	case *ast.ArrayType:
		elemType := r.resolveTypeSpec(t.ElementType)
		dims := make([]types.ArrayDimension, len(t.Ranges))
		for i, rng := range t.Ranges {
			low := evalConstInt(rng.Low)
			high := evalConstInt(rng.High)
			dims[i] = types.ArrayDimension{Low: low, High: high}
		}
		return &types.ArrayType{ElementType: elemType, Dimensions: dims}

	case *ast.StructType:
		members := make([]types.StructMember, len(t.Members))
		for i, m := range t.Members {
			memberType := r.resolveTypeSpec(m.Type)
			name := ""
			if m.Name != nil {
				name = m.Name.Name
			}
			members[i] = types.StructMember{Name: name, Type: memberType}
		}
		return &types.StructType{Members: members}

	case *ast.EnumType:
		values := make([]string, len(t.Values))
		for i, v := range t.Values {
			if v.Name != nil {
				values[i] = v.Name.Name
			}
		}
		return &types.EnumType{
			BaseType: types.KindINT,
			Values:   values,
		}

	case *ast.PointerType:
		baseType := r.resolveTypeSpec(t.BaseType)
		return &types.PointerType{BaseType: baseType}

	case *ast.ReferenceType:
		baseType := r.resolveTypeSpec(t.BaseType)
		return &types.ReferenceType{BaseType: baseType}

	case *ast.StringType:
		if t.IsWide {
			return types.TypeWSTRING
		}
		return types.TypeSTRING

	case *ast.SubrangeType:
		return r.resolveTypeSpec(t.BaseType)

	case *ast.ErrorNode:
		return types.Invalid
	}

	return types.Invalid
}

// evalConstInt evaluates a constant integer expression from an AST node.
// Used for array dimension bounds. Returns 0 if not a simple integer literal.
func evalConstInt(expr ast.Expr) int {
	if expr == nil {
		return 0
	}
	if lit, ok := expr.(*ast.Literal); ok && lit.LitKind == ast.LitInt {
		val := 0
		for _, c := range lit.Value {
			if c >= '0' && c <= '9' {
				val = val*10 + int(c-'0')
			}
		}
		return val
	}
	return 0
}

// astPosToSource converts an ast.Pos to a source.Pos.
func astPosToSource(p ast.Pos) source.Pos {
	return source.Pos{
		File:   p.File,
		Line:   p.Line,
		Col:    p.Col,
		Offset: p.Offset,
	}
}
