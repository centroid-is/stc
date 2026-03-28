package lint

import (
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

// checkMagicNumbers flags integer/real literals that are not 0, 1, or -1
// and are not in VAR CONSTANT init values.
func checkMagicNumbers(file *ast.SourceFile) []diag.Diagnostic {
	return nil // stub
}

// checkNestingDepth flags control flow nesting deeper than maxDepth.
func checkNestingDepth(file *ast.SourceFile, maxDepth int) []diag.Diagnostic {
	return nil // stub
}

// checkPOULength flags POU bodies with more than maxStmts statements.
func checkPOULength(file *ast.SourceFile, maxStmts int) []diag.Diagnostic {
	return nil // stub
}

// checkMissingReturnType flags FunctionDecls without a return type.
func checkMissingReturnType(file *ast.SourceFile) []diag.Diagnostic {
	return nil // stub
}
