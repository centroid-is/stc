package checker

import (
	"math"
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

	// shells maps each FB, PROGRAM, FUNCTION, INTERFACE and STRUCT or enum
	// TYPE declaration to a type object allocated before any declaration is
	// resolved. The resolveX functions fill these objects in place, so a
	// type resolved before its declaration in file order is the final one.
	shells map[ast.Declaration]types.Type
	// aliases holds the resolved type of each winning alias-like TYPE
	// declaration (named, ARRAY, POINTER, REFERENCE, subrange, STRING spec).
	aliases map[ast.Declaration]types.Type
	// forward maps an upper-cased type name to the type of the declaration
	// that ends up owning the name: the first user or mock declaration,
	// otherwise the first library declaration.
	forward map[string]types.Type

	// probing makes resolveTypeSpec record an unknown name in missed instead
	// of reporting it. Used by the alias fixpoint sweep.
	probing bool
	missed  bool

	// inLibrary is set while library declarations are resolved: unknown
	// type names inside vendor stubs are not reported.
	inLibrary bool
	// reportedTypes deduplicates SEMA037: resolveTypeSpec runs twice on the
	// var declarations of FBs and FUNCTIONs (scope and parameter passes).
	reportedTypes map[*ast.NamedType]bool

	// std maps an upper-cased standard FB name to its symbol.
	std map[string]*symbols.Symbol
	// fbs maps an upper-cased FUNCTION_BLOCK name to the declaration that
	// owns it, in registration order (fbOrder), for the EXTENDS pass.
	fbs     map[string]*fbEntry
	fbOrder []string

	// enums caches the resolved type of each enum spec, so the scope and
	// parameter passes over one var declaration share a single EnumType
	// (and report its diagnostics once). TYPE enums map to their shell.
	enums map[*ast.EnumType]*types.EnumType
}

type fbEntry struct {
	decl      *ast.FunctionBlockDecl
	scope     *symbols.Scope
	typ       *types.FunctionBlockType
	isLibrary bool
}

// fileGroup is a set of source files that share an IsLibrary flag.
type fileGroup struct {
	files     []*ast.SourceFile
	isLibrary bool
}

type pendingGVL struct {
	decl      *ast.GVLDecl
	isLibrary bool
}

// NewResolver creates a new Resolver that populates the given symbol table.
func NewResolver(table *symbols.Table, diags *diag.Collector) *Resolver {
	return &Resolver{
		table:         table,
		diags:         diags,
		reportedTypes: make(map[*ast.NamedType]bool),
		enums:         make(map[*ast.EnumType]*types.EnumType),
	}
}

// CollectDeclarations walks all source files and registers POU declarations,
// type declarations, and their variables in the symbol table. The variadic
// opts parameter preserves backward compatibility -- existing callers pass
// no opts. When opts are provided, LibraryFiles are registered first with
// IsLibrary=true, before user code is processed.
func (r *Resolver) CollectDeclarations(files []*ast.SourceFile, opts ...ResolveOpts) {
	var opt ResolveOpts
	if len(opts) > 0 {
		opt = opts[0]
	}
	// Library files first, then user files, then mock files (highest
	// priority, they override library symbols).
	groups := []fileGroup{
		{files: opt.LibraryFiles, isLibrary: true},
		{files: files},
		{files: opt.MockFiles},
	}

	// The IEC standard FBs come before any library file, as library
	// symbols, so stubs and user code can redeclare them.
	r.registerStdFBs()

	// Pass 0: allocate a type object for every named type so forward
	// references resolve to the object that is filled later.
	r.preRegister(groups)

	r.fbs = make(map[string]*fbEntry)
	for _, g := range groups {
		r.inLibrary = g.isLibrary
		for _, file := range g.files {
			r.collectFileDeclarations(file, g.isLibrary)
		}
	}
	r.inLibrary = false

	// Inherited scope: every FB now has its own members filled in.
	r.resolveExtends()

	// Second pass: GVLs, now that every TYPE is in the global scope.
	for _, pg := range r.pendingGVLs {
		r.resolveGVL(pg.decl, pg.isLibrary)
	}
	r.pendingGVLs = nil
}

// preRegister allocates the shell type of every declaration, chooses the
// declaration that owns each name (mirroring the redeclaration rules of the
// resolveX functions) and resolves alias-like TYPE declarations in a
// fixpoint sweep.
func (r *Resolver) preRegister(groups []fileGroup) {
	r.shells = make(map[ast.Declaration]types.Type)
	r.aliases = make(map[ast.Declaration]types.Type)
	r.forward = make(map[string]types.Type)

	type owner struct {
		decl      ast.Declaration
		isLibrary bool
	}
	owners := make(map[string]owner)
	var order []string
	for _, g := range groups {
		for _, file := range g.files {
			for _, decl := range file.Declarations {
				name, shell := declShell(decl)
				if name == "" {
					continue
				}
				if shell != nil {
					r.shells[decl] = shell
				}
				key := strings.ToUpper(name)
				prev, seen := owners[key]
				if !seen {
					order = append(order, key)
				}
				if !seen || (prev.isLibrary && !g.isLibrary) {
					owners[key] = owner{decl: decl, isLibrary: g.isLibrary}
				}
			}
		}
	}

	var pending []*ast.TypeDecl
	library := make(map[*ast.TypeDecl]bool)
	for _, key := range order {
		o := owners[key]
		if shell, ok := r.shells[o.decl]; ok {
			r.forward[key] = shell
		} else if td, ok := o.decl.(*ast.TypeDecl); ok {
			pending = append(pending, td)
			library[td] = o.isLibrary
		}
	}

	// Alias chains may point forward (TYPE T1 : T2; ... TYPE T2 : INT;).
	// Each sweep resolves every alias whose names are all known; stop when
	// a sweep makes no progress. Aliases left over (self-referential,
	// mutually recursive, or naming an unknown type) resolve with
	// diagnostics in resolveTypeDecl.
	for len(pending) > 0 {
		before := len(pending)
		var next []*ast.TypeDecl
		for _, td := range pending {
			if typ, ok := r.probe(td.Type); ok {
				r.aliases[td] = typ
				r.forward[strings.ToUpper(td.Name.Name)] = typ
			} else {
				next = append(next, td)
			}
		}
		pending = next
		if len(next) == before {
			break
		}
	}

	// Leftover aliases resolve with diagnostics now, so the root cause is
	// reported once at the alias declaration and later uses of the alias
	// get its (possibly Invalid) type silently.
	for _, td := range pending {
		r.inLibrary = library[td]
		typ := r.resolveTypeSpec(td.Type)
		r.aliases[td] = typ
		r.forward[strings.ToUpper(td.Name.Name)] = typ
	}
	r.inLibrary = false
}

// declShell returns the declared name and a fresh, empty type object for a
// declaration. Alias-like TYPE declarations return a nil shell; GVLs and
// unnamed declarations return an empty name.
func declShell(decl ast.Declaration) (string, types.Type) {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		if d.Name != nil {
			return d.Name.Name, &types.FunctionBlockType{Name: d.Name.Name}
		}
	case *ast.FunctionBlockDecl:
		if d.Name != nil {
			return d.Name.Name, &types.FunctionBlockType{Name: d.Name.Name}
		}
	case *ast.FunctionDecl:
		if d.Name != nil {
			return d.Name.Name, &types.FunctionType{Name: d.Name.Name}
		}
	case *ast.InterfaceDecl:
		// Interfaces have no type of their own yet; a variable of interface
		// type resolves to an empty FB type carrying the interface name.
		if d.Name != nil {
			return d.Name.Name, &types.FunctionBlockType{Name: d.Name.Name}
		}
	case *ast.TypeDecl:
		if d.Name == nil {
			return "", nil
		}
		switch d.Type.(type) {
		case *ast.StructType:
			return d.Name.Name, &types.StructType{Name: d.Name.Name}
		case *ast.EnumType:
			return d.Name.Name, &types.EnumType{Name: d.Name.Name, BaseType: types.KindINT}
		}
		return d.Name.Name, nil
	}
	return "", nil
}

// probe resolves ts without reporting diagnostics. ok is false when any
// named type inside ts is unknown.
func (r *Resolver) probe(ts ast.TypeSpec) (typ types.Type, ok bool) {
	r.probing, r.missed = true, false
	typ = r.resolveTypeSpec(ts)
	ok = !r.missed
	r.probing, r.missed = false, false
	return typ, ok
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
	r.yieldStdFB(name)
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
					setConstInt(bare[len(bare)-1], vb, vd)
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
		r.yieldStdFB(sym.Name)
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
		if isLibrary && existing.IsLibrary && !r.isStdFB(existing) {
			// Duplicate library symbol -- silently ignore (first library wins)
			return
		}
		if existing.IsLibrary {
			// User code (or a library stub over a standard FB) overrides
			// the library symbol -- remove library entry
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
		sym.Type = r.fbShell(d, name)
		sym.IsLibrary = isLibrary
	}

	r.resolveVarBlocksInScope(name, d.VarBlocks, pouScope)
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
		fn.Outputs = r.callOutputs(m.VarBlocks)
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

// callOutputs lists the VAR_OUTPUT parameters of a callable in declaration
// order, so name => target arguments can bind to them.
func (r *Resolver) callOutputs(blocks []*ast.VarBlock) []types.Parameter {
	var outs []types.Parameter
	for _, vb := range blocks {
		if vb.Section != ast.VarOutput {
			continue
		}
		for _, vd := range vb.Declarations {
			typ := r.resolveTypeSpec(vd.Type)
			for _, n := range vd.Names {
				outs = append(outs, types.Parameter{Name: n.Name, Type: typ, Direction: types.DirOutput})
			}
		}
	}
	return outs
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
		if isLibrary && existing.IsLibrary && !r.isStdFB(existing) {
			// Duplicate library symbol -- silently ignore (first library wins)
			return
		}
		if existing.IsLibrary {
			// User code (or a library stub over a standard FB) overrides
			// the library symbol -- remove library entry
			r.table.RemovePOU(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	pouScope := r.table.RegisterPOU(name, symbols.KindFunctionBlock, pos)

	// Fill the pre-registered FunctionBlockType from the var blocks
	fbType := r.fbShell(d, name)

	r.resolveVarBlocksInScope(name, d.VarBlocks, pouScope)
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

	key := strings.ToUpper(name)
	if _, seen := r.fbs[key]; !seen {
		r.fbOrder = append(r.fbOrder, key)
	}
	r.fbs[key] = &fbEntry{decl: d, scope: pouScope, typ: fbType, isLibrary: isLibrary}
}

// resolveExtends gives every FUNCTION_BLOCK with EXTENDS the members of its
// base chain. The derived POU scope is re-parented onto the base POU scope,
// so the body, actions and methods of the derived FB see inherited
// variables, methods and actions through the ordinary scope walk, and
// usage marks the base symbol itself. Base inputs, outputs and in-outs are
// prepended to the derived FunctionBlockType in place. A derived variable
// that redeclares a base variable is a redeclaration error. Cycles are
// left unlinked, so lookups always terminate.
func (r *Resolver) resolveExtends() {
	done := make(map[string]bool)
	var visit func(key string)
	visit = func(key string) {
		if done[key] {
			return
		}
		done[key] = true
		e := r.fbs[key]
		if e == nil || e.decl.Extends == nil {
			return
		}
		baseName := e.decl.Extends.Name
		baseKey := strings.ToUpper(baseName)
		visit(baseKey) // the base inherits first (no-op on a cycle)

		baseScope := r.table.LookupPOU(baseName)
		baseSym := r.table.LookupGlobal(baseName)
		if baseScope == nil || baseSym == nil {
			if !e.isLibrary {
				r.diags.Errorf(astPosToSource(e.decl.Extends.Span().Start), CodeUndeclaredType,
					"undeclared type '%s'", baseName)
			}
			return
		}
		baseType, ok := baseSym.Type.(*types.FunctionBlockType)
		if !ok {
			return
		}
		for s := baseScope; s != nil; s = s.Parent {
			if s == e.scope {
				return // EXTENDS cycle
			}
		}

		own := make(map[string]bool)
		for _, vb := range e.decl.VarBlocks {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					own[strings.ToUpper(n.Name)] = true
					if prev := lookupInPOUChain(baseScope, n.Name); prev != nil && prev.Kind == symbols.KindVariable {
						pos := astPosToSource(n.Span().Start)
						r.diags.Errorf(pos, CodeRedeclared,
							"redeclaration of %q (inherited from %s, declared at %s)", n.Name, baseType.Name, prev.Pos)
					}
				}
			}
		}
		e.scope.Parent = baseScope
		e.typ.Inputs = append(inheritParams(baseType.Inputs, own), e.typ.Inputs...)
		e.typ.Outputs = append(inheritParams(baseType.Outputs, own), e.typ.Outputs...)
		e.typ.InOuts = append(inheritParams(baseType.InOuts, own), e.typ.InOuts...)
	}
	for _, key := range r.fbOrder {
		visit(key)
	}
}

// lookupInPOUChain looks name up in scope and its POU-scope ancestors,
// stopping before the global scope.
func lookupInPOUChain(scope *symbols.Scope, name string) *symbols.Symbol {
	for s := scope; s != nil && s.Kind == symbols.ScopePOU; s = s.Parent {
		if sym := s.LookupLocal(name); sym != nil {
			return sym
		}
	}
	return nil
}

// inheritParams copies the base parameters the derived FB does not redeclare.
func inheritParams(base []types.Parameter, own map[string]bool) []types.Parameter {
	var out []types.Parameter
	for _, p := range base {
		if !own[strings.ToUpper(p.Name)] {
			out = append(out, p)
		}
	}
	return out
}

func (r *Resolver) resolveFunction(d *ast.FunctionDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary && !r.isStdFB(existing) {
			return
		}
		if existing.IsLibrary {
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

	fnType, ok := r.shells[d].(*types.FunctionType)
	if !ok {
		fnType = &types.FunctionType{Name: name}
	}
	fnType.ReturnType = retType

	r.resolveVarBlocksInScope(name, d.VarBlocks, pouScope)

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
					fnType.Outputs = append(fnType.Outputs, param)
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
		if isLibrary && existing.IsLibrary && !r.isStdFB(existing) {
			return
		}
		if existing.IsLibrary {
			r.table.RemovePOU(name)
		} else {
			r.diags.Errorf(pos, CodeRedeclared,
				"redeclaration of %q (previously declared at %s)", name, existing.Pos)
			return
		}
	}

	resolvedType := r.typeDeclType(d)

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

	// An enum TYPE records its attributes (DIAL-07). Its values are
	// inserted into the global scope unless the enum is qualified_only:
	// then only E.v names them, so a GVL or POU variable may reuse a value
	// name. Aliases of an enum share its EnumType and leave the flags alone.
	et, ok := resolvedType.(*types.EnumType)
	if !ok {
		return
	}
	if _, spec := d.Type.(*ast.EnumType); spec {
		et.Qualified = ast.HasAttribute(d.Attributes, "qualified_only")
		et.Strict = ast.HasAttribute(d.Attributes, "strict")
		et.ToString = ast.HasAttribute(d.Attributes, "to_string")
	}
	if !et.Qualified {
		for _, val := range et.Values {
			enumSym := &symbols.Symbol{
				Name: val,
				Kind: symbols.KindEnumValue,
				Pos:  pos,
				Type: resolvedType,
			}
			r.yieldStdFB(val)
			_ = r.table.GlobalScope().Insert(enumSym)
		}
	}
}

// fbShell returns the pre-registered FunctionBlockType of a PROGRAM or
// FUNCTION_BLOCK declaration, or a new one when the declaration has none.
func (r *Resolver) fbShell(d ast.Declaration, pouName string) *types.FunctionBlockType {
	if fb, ok := r.shells[d].(*types.FunctionBlockType); ok {
		return fb
	}
	return &types.FunctionBlockType{Name: pouName}
}

// typeDeclType resolves a TYPE declaration's spec. STRUCT and enum specs
// fill the pre-registered shell in place; aliases resolved by the fixpoint
// sweep reuse that result; anything else resolves now.
func (r *Resolver) typeDeclType(d *ast.TypeDecl) types.Type {
	if typ, ok := r.aliases[d]; ok {
		return typ
	}
	if sh, ok := r.shells[d].(*types.EnumType); ok {
		// An enum shell exists only for an enum spec.
		return r.resolveEnumSpec(d.Type.(*ast.EnumType), sh)
	}
	resolved := r.resolveTypeSpec(d.Type)
	// A STRUCT shell exists only for a STRUCT spec, which resolveTypeSpec
	// always turns into a StructType.
	if sh, ok := r.shells[d].(*types.StructType); ok {
		sh.Members = resolved.(*types.StructType).Members
		return sh
	}
	return resolved
}

func (r *Resolver) resolveInterface(d *ast.InterfaceDecl, isLibrary bool) {
	if d.Name == nil {
		return
	}
	name := d.Name.Name
	pos := astPosToSource(d.Name.Span().Start)

	if existing := r.table.LookupGlobal(name); existing != nil {
		if isLibrary && existing.IsLibrary && !r.isStdFB(existing) {
			return
		}
		if existing.IsLibrary {
			r.table.RemovePOU(name)
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
// into the given scope (bypassing the table's scope stack). An inline enum
// (eStep : (E_IDLE, E_RUN);) is named <POU>.<var> and its values are
// inserted into the POU scope, so two POUs may reuse value names.
func (r *Resolver) resolveVarBlocksInScope(pouName string, blocks []*ast.VarBlock, scope *symbols.Scope) {
	for _, vb := range blocks {
		for _, vd := range vb.Declarations {
			resolvedType := r.resolveTypeSpec(vd.Type)
			if et, ok := resolvedType.(*types.EnumType); ok && len(vd.Names) > 0 {
				if _, inline := vd.Type.(*ast.EnumType); inline {
					et.Name = pouName + "." + vd.Names[0].Name
					r.insertInlineEnumValues(vd.Type.(*ast.EnumType), et, scope)
				}
			}
			for _, name := range vd.Names {
				pos := astPosToSource(name.Span().Start)
				sym := &symbols.Symbol{
					Name:     name.Name,
					Kind:     symbols.KindVariable,
					Pos:      pos,
					ParamDir: vb.Section,
					Type:     resolvedType,
				}
				setConstInt(sym, vb, vd)
				if err := scope.Insert(sym); err != nil {
					r.diags.Errorf(pos, CodeRedeclared, "%s", err.Error())
				}
			}
		}
	}
}

// setConstInt records the value of a CONSTANT variable whose initialiser is
// an integer literal.
func setConstInt(sym *symbols.Symbol, vb *ast.VarBlock, vd *ast.VarDecl) {
	if !vb.IsConstant || vd.InitValue == nil {
		return
	}
	sym.ConstInt, sym.HasConstInt = ast.IntLiteralValue(vd.InitValue)
}

// insertInlineEnumValues inserts the values of an inline enum into the POU
// scope. A value that clashes with a variable is a redeclaration.
func (r *Resolver) insertInlineEnumValues(spec *ast.EnumType, et *types.EnumType, scope *symbols.Scope) {
	for _, v := range spec.Values {
		if v == nil || v.Name == nil {
			continue
		}
		pos := astPosToSource(v.Name.Span().Start)
		sym := &symbols.Symbol{Name: v.Name.Name, Kind: symbols.KindEnumValue, Pos: pos, Type: et}
		if err := scope.Insert(sym); err != nil {
			r.diags.Errorf(pos, CodeRedeclared, "%s", err.Error())
		}
	}
}

// resolveEnumSpec resolves an enum spec into an EnumType: the base type
// (an integer or bit-string type, INT when omitted) and the value ordinals
// (ast.EnumOrdinals, previous+1 rule). into is the pre-registered shell of
// a TYPE declaration, or nil for an inline enum. A non-integer base type
// and a known ordinal outside the base type's range report SEMA036.
func (r *Resolver) resolveEnumSpec(t *ast.EnumType, into *types.EnumType) *types.EnumType {
	if et, ok := r.enums[t]; ok {
		return et
	}
	et := into
	if et == nil {
		et = &types.EnumType{}
	}
	et.BaseType = types.KindINT
	if t.BaseType != nil {
		bt := r.resolveTypeSpec(t.BaseType)
		if _, _, ok := intKindRange(bt.Kind()); ok {
			et.BaseType = bt.Kind()
		} else if bt != types.Invalid && !r.probing {
			r.diags.Errorf(astPosToSource(t.BaseType.Span().Start), CodeEnumRule,
				"enum base type must be an integer type, got %s", bt)
		}
	}

	ords := ast.EnumOrdinals(t)
	et.Values = make([]string, len(ords))
	et.Ordinals = make([]int64, len(ords))
	lo, hi, _ := intKindRange(et.BaseType)
	i := 0
	for _, v := range t.Values {
		if v == nil {
			continue
		}
		o := ords[i]
		et.Values[i], et.Ordinals[i] = o.Name, o.Value
		if o.Known && (o.Value < lo || o.Value > hi) && !r.probing {
			r.diags.Errorf(astPosToSource(v.Span().Start), CodeEnumRule,
				"enum value %s = %d is out of range for %s (%d..%d)", o.Name, o.Value, et.BaseType, lo, hi)
		}
		i++
	}
	if !r.probing {
		r.enums[t] = et
	}
	return et
}

// intKindRange returns the value range of an integer or bit-string kind.
// ok is false for every other kind (BOOL included). The 64-bit unsigned
// kinds are capped at MaxInt64, the largest ordinal an int64 holds.
func intKindRange(k types.TypeKind) (lo, hi int64, ok bool) {
	switch k {
	case types.KindSINT:
		return math.MinInt8, math.MaxInt8, true
	case types.KindINT:
		return math.MinInt16, math.MaxInt16, true
	case types.KindDINT:
		return math.MinInt32, math.MaxInt32, true
	case types.KindLINT:
		return math.MinInt64, math.MaxInt64, true
	case types.KindUSINT, types.KindBYTE:
		return 0, math.MaxUint8, true
	case types.KindUINT, types.KindWORD:
		return 0, math.MaxUint16, true
	case types.KindUDINT, types.KindDWORD:
		return 0, math.MaxUint32, true
	case types.KindULINT, types.KindLWORD:
		return 0, math.MaxInt64, true
	}
	return 0, 0, false
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
		// Lib.Type: the qualified name first, then the bare name.
		if t.Namespace != nil {
			if typ, ok := r.lookupTypeName(t.Namespace.Name + "." + name); ok {
				return typ
			}
		}
		if typ, ok := types.LookupElementaryType(name); ok {
			return typ
		}
		if typ, ok := r.lookupTypeName(name); ok {
			return typ
		}
		if r.probing {
			r.missed = true
			return types.Invalid
		}
		r.reportUndeclaredType(t)
		return types.Invalid

	case *ast.ArrayType:
		elemType := r.resolveTypeSpec(t.ElementType)
		dims := make([]types.ArrayDimension, len(t.Ranges))
		for i, rng := range t.Ranges {
			low, lowOK := ast.IntLiteralValue(rng.Low)
			high, highOK := ast.IntLiteralValue(rng.High)
			if lowOK && highOK {
				dims[i] = types.ArrayDimension{Low: int(low), High: int(high), Known: true}
				continue
			}
			dims[i] = types.ArrayDimension{Low: evalConstInt(rng.Low), High: evalConstInt(rng.High)}
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
		return r.resolveEnumSpec(t, nil)

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

// lookupTypeName finds a declared type by name: the pre-registered object
// of the declaration that owns the name (so forward references get the
// final, filled type), else any global symbol with a type (library symbols
// registered outside the pre-pass, such as the standard FBs).
func (r *Resolver) lookupTypeName(name string) (types.Type, bool) {
	if typ, ok := r.forward[strings.ToUpper(name)]; ok {
		return typ, true
	}
	if sym := r.table.GlobalScope().Lookup(name); sym != nil {
		if typ, ok := sym.Type.(types.Type); ok {
			return typ, true
		}
	}
	return nil, false
}

// reportUndeclaredType reports SEMA037 once per type reference. Names inside
// library declarations are not reported: a stub may name a type from a
// library that is not loaded, and the user cannot fix the stub.
func (r *Resolver) reportUndeclaredType(t *ast.NamedType) {
	if r.inLibrary || r.reportedTypes[t] {
		return
	}
	r.reportedTypes[t] = true
	name := t.Name.Name
	if t.Namespace != nil {
		name = t.Namespace.Name + "." + name
	}
	r.diags.Errorf(astPosToSource(t.Span().Start), CodeUndeclaredType, "undeclared type '%s'", name)
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
