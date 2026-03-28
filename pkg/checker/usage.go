package checker

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
)

// CheckUsage performs unused variable detection and unreachable code detection.
// It checks all scopes in the symbol table for unused variables and walks
// all statement lists in the AST for unreachable code after RETURN/EXIT.
func CheckUsage(files []*ast.SourceFile, table *symbols.Table, diags *diag.Collector) {
	// 1. Unused variable check: walk all scopes
	checkUnusedVars(table.GlobalScope(), diags)

	// 2. Unreachable code check: walk all statement lists
	if files != nil {
		for _, file := range files {
			for _, decl := range file.Declarations {
				checkUnreachableDecl(decl, diags)
			}
		}
	}
}

// isInterfaceVar returns true if the variable section is an interface point
// (VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_GLOBAL) and should not trigger
// unused variable warnings.
func isInterfaceVar(section ast.VarSection) bool {
	switch section {
	case ast.VarInput, ast.VarOutput, ast.VarInOut, ast.VarGlobal, ast.VarExternal:
		return true
	}
	return false
}

// checkUnusedVars recursively walks the scope tree checking for unused variables.
func checkUnusedVars(scope *symbols.Scope, diags *diag.Collector) {
	for _, sym := range scope.Symbols() {
		if sym.Kind == symbols.KindVariable && !sym.Used && !isInterfaceVar(sym.ParamDir) {
			diags.Warnf(sym.Pos, CodeUnusedVar,
				"variable '%s' is declared but never used", sym.Name)
		}
	}
	for _, child := range scope.Children {
		checkUnusedVars(child, diags)
	}
}

// checkUnreachableDecl checks a declaration for unreachable code.
func checkUnreachableDecl(decl ast.Declaration, diags *diag.Collector) {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		checkUnreachableStmts(d.Body, diags)
	case *ast.FunctionDecl:
		checkUnreachableStmts(d.Body, diags)
	case *ast.FunctionBlockDecl:
		checkUnreachableStmts(d.Body, diags)
		for _, m := range d.Methods {
			checkUnreachableStmts(m.Body, diags)
		}
	case *ast.MethodDecl:
		checkUnreachableStmts(d.Body, diags)
	}
}

// checkUnreachableStmts checks a statement list for unreachable code after
// RETURN or EXIT statements.
func checkUnreachableStmts(stmts []ast.Statement, diags *diag.Collector) {
	for i, stmt := range stmts {
		// Recursively check nested statement lists
		checkUnreachableNested(stmt, diags)

		// Check if this is a RETURN or EXIT and there are statements after it
		var keyword string
		switch stmt.(type) {
		case *ast.ReturnStmt:
			keyword = "RETURN"
		case *ast.ExitStmt:
			keyword = "EXIT"
		}

		if keyword != "" && i < len(stmts)-1 {
			// All subsequent statements are unreachable
			nextStmt := stmts[i+1]
			span := nextStmt.Span()
			pos := spanPos(span)
			diags.Warnf(pos, CodeUnreachableCode,
				"unreachable code after %s statement", keyword)
			// Only warn once per block
			return
		}
	}
}

// checkUnreachableNested checks nested statement lists within compound statements.
func checkUnreachableNested(stmt ast.Statement, diags *diag.Collector) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		checkUnreachableStmts(s.Then, diags)
		for _, elsif := range s.ElsIfs {
			checkUnreachableStmts(elsif.Body, diags)
		}
		checkUnreachableStmts(s.Else, diags)
	case *ast.ForStmt:
		checkUnreachableStmts(s.Body, diags)
	case *ast.WhileStmt:
		checkUnreachableStmts(s.Body, diags)
	case *ast.RepeatStmt:
		checkUnreachableStmts(s.Body, diags)
	case *ast.CaseStmt:
		for _, branch := range s.Branches {
			checkUnreachableStmts(branch.Body, diags)
		}
		checkUnreachableStmts(s.ElseBranch, diags)
	}
}
