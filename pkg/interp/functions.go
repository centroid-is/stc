package interp

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// RegisterFunctionDecl makes a user-defined FUNCTION callable by name
// (case-insensitive). evalCall looks registered FUNCTIONs up after
// LocalFunctions and before the built-in functions.
func (interp *Interpreter) RegisterFunctionDecl(decl *ast.FunctionDecl) {
	if decl == nil || decl.Name == nil {
		return
	}
	if interp.FuncDecls == nil {
		interp.FuncDecls = make(map[string]*ast.FunctionDecl)
	}
	interp.FuncDecls[strings.ToUpper(decl.Name.Name)] = decl
	interp.RegisterInlineEnums(decl.Name.Name, decl.VarBlocks)
}

// CallFunction calls a user-defined FUNCTION from env. Arguments bind by
// position, by name or mixed (see bindArgs); omitted inputs take their
// declared default. The body runs in a fresh env whose parent is the chain of
// non qualified_only GVLs. VAR_IN_OUT arguments and `=>` outputs are written
// back once the body returns, and the value of the function-name variable is
// the result. The call is bounded by MaxCallDepth.
func (interp *Interpreter) CallFunction(env *Env, decl *ast.FunctionDecl, posArgs []ast.Expr, named []*ast.CallArg, pos ast.Pos) (Value, error) {
	name := strings.ToUpper(decl.Name.Name)
	if err := interp.EnterCall(name, pos); err != nil {
		return Value{}, err
	}
	defer interp.ExitCall()

	callee := NewEnv(interp.GlobalParent())
	callee.Define(decl.Name.Name, zeroFromTypeSpecWith(decl.ReturnType, interp.TypeResolverFunc(), 0))
	bc, err := interp.bindArgs(env, callee, decl.VarBlocks, posArgs, named, decl.Name.Name)
	if err != nil {
		return Value{}, err
	}
	if err := interp.execStatements(callee, decl.Body); err != nil {
		if _, ok := err.(*ErrReturn); !ok {
			return Value{}, err
		}
	}
	if err := interp.writeBack(env, callee, bc); err != nil {
		return Value{}, err
	}
	v, _ := callee.GetLocal(decl.Name.Name)
	return v, nil
}
