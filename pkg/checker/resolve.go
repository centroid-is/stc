package checker

import (
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
