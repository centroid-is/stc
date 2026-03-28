package lint

import (
	"fmt"
	"strconv"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// spanPos converts an AST span start to a source.Pos.
func spanPos(n ast.Node) source.Pos {
	s := n.Span().Start
	return source.Pos{
		File:   s.File,
		Line:   s.Line,
		Col:    s.Col,
		Offset: s.Offset,
	}
}

// isAllowedLiteral returns true if the literal value is 0 or 1 (commonly allowed).
func isAllowedLiteral(lit *ast.Literal) bool {
	if lit.LitKind != ast.LitInt && lit.LitKind != ast.LitReal {
		return true // only flag numeric literals
	}
	v := lit.Value
	// Parse as integer
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n == 0 || n == 1 || n == -1
	}
	// Parse as float
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f == 0.0 || f == 1.0 || f == -1.0
	}
	return false
}

// checkMagicNumbers flags integer/real literals that are not 0, 1, or -1
// and appear in statement bodies (not in VAR CONSTANT init values).
func checkMagicNumbers(file *ast.SourceFile) []diag.Diagnostic {
	var diags []diag.Diagnostic

	for _, decl := range file.Declarations {
		body := pouBody(decl)
		if body == nil {
			continue
		}
		for _, stmt := range body {
			walkExprsInStmt(stmt, func(expr ast.Expr) {
				lit, ok := expr.(*ast.Literal)
				if !ok {
					return
				}
				if lit.LitKind != ast.LitInt && lit.LitKind != ast.LitReal {
					return
				}
				if isAllowedLiteral(lit) {
					return
				}
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Warning,
					Pos:      spanPos(lit),
					Code:     CodeMagicNumber,
					Message:  fmt.Sprintf("magic number %s: consider extracting to a named constant", lit.Value),
				})
			})
		}
	}
	return diags
}

// pouBody returns the body statements for a POU declaration.
func pouBody(decl ast.Declaration) []ast.Statement {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		return d.Body
	case *ast.FunctionBlockDecl:
		return d.Body
	case *ast.FunctionDecl:
		return d.Body
	default:
		return nil
	}
}

// pouVarBlocks returns the var blocks for a POU declaration.
func pouVarBlocks(decl ast.Declaration) []*ast.VarBlock {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		return d.VarBlocks
	case *ast.FunctionBlockDecl:
		return d.VarBlocks
	case *ast.FunctionDecl:
		return d.VarBlocks
	default:
		return nil
	}
}

// walkExprsInStmt visits all expressions in a statement tree.
func walkExprsInStmt(stmt ast.Statement, visit func(ast.Expr)) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		walkExpr(s.Value, visit)
		walkExpr(s.Target, visit)
	case *ast.IfStmt:
		walkExpr(s.Condition, visit)
		for _, st := range s.Then {
			walkExprsInStmt(st, visit)
		}
		for _, ei := range s.ElsIfs {
			walkExpr(ei.Condition, visit)
			for _, st := range ei.Body {
				walkExprsInStmt(st, visit)
			}
		}
		for _, st := range s.Else {
			walkExprsInStmt(st, visit)
		}
	case *ast.ForStmt:
		walkExpr(s.From, visit)
		walkExpr(s.To, visit)
		if s.By != nil {
			walkExpr(s.By, visit)
		}
		for _, st := range s.Body {
			walkExprsInStmt(st, visit)
		}
	case *ast.WhileStmt:
		walkExpr(s.Condition, visit)
		for _, st := range s.Body {
			walkExprsInStmt(st, visit)
		}
	case *ast.RepeatStmt:
		walkExpr(s.Condition, visit)
		for _, st := range s.Body {
			walkExprsInStmt(st, visit)
		}
	case *ast.CaseStmt:
		walkExpr(s.Expr, visit)
		for _, br := range s.Branches {
			for _, st := range br.Body {
				walkExprsInStmt(st, visit)
			}
		}
		for _, st := range s.ElseBranch {
			walkExprsInStmt(st, visit)
		}
	case *ast.CallStmt:
		walkExpr(s.Callee, visit)
		for _, arg := range s.Args {
			walkExpr(arg.Value, visit)
		}
	}
}

// walkExpr recursively walks an expression tree.
func walkExpr(expr ast.Expr, visit func(ast.Expr)) {
	if expr == nil {
		return
	}
	visit(expr)
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		walkExpr(e.Left, visit)
		walkExpr(e.Right, visit)
	case *ast.UnaryExpr:
		walkExpr(e.Operand, visit)
	case *ast.CallExpr:
		walkExpr(e.Callee, visit)
		for _, a := range e.Args {
			walkExpr(a, visit)
		}
	case *ast.MemberAccessExpr:
		walkExpr(e.Object, visit)
	case *ast.IndexExpr:
		walkExpr(e.Object, visit)
		for _, idx := range e.Indices {
			walkExpr(idx, visit)
		}
	case *ast.ParenExpr:
		walkExpr(e.Inner, visit)
	case *ast.DerefExpr:
		walkExpr(e.Operand, visit)
	}
}

// checkNestingDepth flags control flow nesting deeper than maxDepth.
func checkNestingDepth(file *ast.SourceFile, maxDepth int) []diag.Diagnostic {
	var diags []diag.Diagnostic

	for _, decl := range file.Declarations {
		body := pouBody(decl)
		if body == nil {
			continue
		}
		for _, stmt := range body {
			checkNestingInStmt(stmt, 0, maxDepth, &diags)
		}
	}
	return diags
}

func checkNestingInStmt(stmt ast.Statement, depth, maxDepth int, diags *[]diag.Diagnostic) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		depth++
		if depth > maxDepth {
			*diags = append(*diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(s),
				Code:     CodeDeepNesting,
				Message:  fmt.Sprintf("control flow nested %d levels deep (max %d)", depth, maxDepth),
			})
		}
		for _, st := range s.Then {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
		for _, ei := range s.ElsIfs {
			for _, st := range ei.Body {
				checkNestingInStmt(st, depth, maxDepth, diags)
			}
		}
		for _, st := range s.Else {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
	case *ast.ForStmt:
		depth++
		if depth > maxDepth {
			*diags = append(*diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(s),
				Code:     CodeDeepNesting,
				Message:  fmt.Sprintf("control flow nested %d levels deep (max %d)", depth, maxDepth),
			})
		}
		for _, st := range s.Body {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
	case *ast.WhileStmt:
		depth++
		if depth > maxDepth {
			*diags = append(*diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(s),
				Code:     CodeDeepNesting,
				Message:  fmt.Sprintf("control flow nested %d levels deep (max %d)", depth, maxDepth),
			})
		}
		for _, st := range s.Body {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
	case *ast.RepeatStmt:
		depth++
		if depth > maxDepth {
			*diags = append(*diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(s),
				Code:     CodeDeepNesting,
				Message:  fmt.Sprintf("control flow nested %d levels deep (max %d)", depth, maxDepth),
			})
		}
		for _, st := range s.Body {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
	case *ast.CaseStmt:
		depth++
		if depth > maxDepth {
			*diags = append(*diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(s),
				Code:     CodeDeepNesting,
				Message:  fmt.Sprintf("control flow nested %d levels deep (max %d)", depth, maxDepth),
			})
		}
		for _, br := range s.Branches {
			for _, st := range br.Body {
				checkNestingInStmt(st, depth, maxDepth, diags)
			}
		}
		for _, st := range s.ElseBranch {
			checkNestingInStmt(st, depth, maxDepth, diags)
		}
	}
}

// checkPOULength flags POU bodies with more than maxStmts statements.
func checkPOULength(file *ast.SourceFile, maxStmts int) []diag.Diagnostic {
	var diags []diag.Diagnostic

	for _, decl := range file.Declarations {
		body := pouBody(decl)
		if body == nil {
			continue
		}
		if len(body) > maxStmts {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(decl),
				Code:     CodeLongPOU,
				Message:  fmt.Sprintf("POU body has %d statements (max %d)", len(body), maxStmts),
			})
		}
	}
	return diags
}

// checkMissingReturnType flags FunctionDecls without a return type.
func checkMissingReturnType(file *ast.SourceFile) []diag.Diagnostic {
	var diags []diag.Diagnostic

	for _, decl := range file.Declarations {
		fd, ok := decl.(*ast.FunctionDecl)
		if !ok {
			continue
		}
		if fd.ReturnType == nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Warning,
				Pos:      spanPos(fd),
				Code:     CodeMissingReturnType,
				Message:  "FUNCTION without return type",
			})
		}
	}
	return diags
}
