package parser

import (
	"sort"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// attachTrivia walks the token stream and attaches comment trivia to AST nodes.
// It is called as a post-parse pass: the parser builds the tree without trivia,
// then this function maps comment tokens to their nearest AST nodes.
//
// Algorithm:
// 1. Collect all AST nodes (depth-first), sorted by span start offset.
// 2. Build a map from non-trivia token start offset to the innermost AST node.
// 3. Walk allTokens, grouping comments between non-trivia tokens.
// 4. Same-line comments after a non-trivia token become TrailingTrivia of
//    the previous token's node. Remaining comments become LeadingTrivia of
//    the next token's node.
func attachTrivia(file *ast.SourceFile, allTokens []lexer.Token) {
	if file == nil || len(allTokens) == 0 {
		return
	}

	// Step 1: Collect all attachable nodes sorted by start offset.
	nodes := collectNodes(file)
	if len(nodes) == 0 {
		return
	}

	// Step 2: Build offset-to-node map. For each non-trivia token offset,
	// find the innermost AST node whose span contains that offset.
	offsetToNode := make(map[int]*ast.NodeBase, len(allTokens))
	for _, tok := range allTokens {
		if tok.Kind.IsTrivia() || tok.Kind == lexer.EOF {
			continue
		}
		n := findInnermostNode(nodes, tok.Pos.Offset)
		if n != nil {
			offsetToNode[tok.Pos.Offset] = n
		}
	}

	// Step 3-4: Walk allTokens, collecting comment groups between non-trivia tokens.
	type commentGroup struct {
		comments []ast.Trivia
	}

	var pending []ast.Trivia
	var prevNonTriviaLine int
	var prevNonTriviaNode *ast.NodeBase

	for _, tok := range allTokens {
		if tok.Kind == lexer.EOF {
			break
		}

		switch {
		case tok.Kind == lexer.LineComment || tok.Kind == lexer.BlockComment:
			tri := tokenToTrivia(tok)

			// Is this comment on the same line as the previous non-trivia token?
			if prevNonTriviaNode != nil && tok.Pos.Line == prevNonTriviaLine {
				// Trailing trivia of the previous token's node.
				prevNonTriviaNode.TrailingTrivia = append(prevNonTriviaNode.TrailingTrivia, tri)
			} else {
				// Buffer as potential leading trivia for the next non-trivia token.
				pending = append(pending, tri)
			}

		case tok.Kind == lexer.Whitespace:
			// Skip whitespace — formatter regenerates it.
			continue

		default:
			// Non-trivia token: flush pending comments as leading trivia.
			if len(pending) > 0 {
				node := offsetToNode[tok.Pos.Offset]
				if node != nil {
					node.LeadingTrivia = append(node.LeadingTrivia, pending...)
				}
				pending = nil
			}
			prevNonTriviaLine = tok.EndPos.Line
			prevNonTriviaNode = offsetToNode[tok.Pos.Offset]
		}
	}

	// Any remaining pending comments attach as trailing to the last node.
	if len(pending) > 0 && len(nodes) > 0 {
		last := nodes[len(nodes)-1]
		last.TrailingTrivia = append(last.TrailingTrivia, pending...)
	}
}

// tokenToTrivia converts a lexer comment token to an ast.Trivia value.
func tokenToTrivia(tok lexer.Token) ast.Trivia {
	var kind ast.TriviaKind
	switch tok.Kind {
	case lexer.LineComment:
		kind = ast.TriviaLineComment
	case lexer.BlockComment:
		kind = ast.TriviaBlockComment
	default:
		kind = ast.TriviaWhitespace
	}
	return ast.Trivia{
		Kind: kind,
		Text: tok.Text,
		Span: ast.Span{
			Start: ast.Pos{File: tok.Pos.File, Line: tok.Pos.Line, Col: tok.Pos.Col, Offset: tok.Pos.Offset},
			End:   ast.Pos{File: tok.EndPos.File, Line: tok.EndPos.Line, Col: tok.EndPos.Col, Offset: tok.EndPos.Offset},
		},
	}
}

// collectNodes performs a depth-first walk of the AST and returns pointers
// to each node's NodeBase, sorted by start offset.
func collectNodes(file *ast.SourceFile) []*ast.NodeBase {
	var nodes []*ast.NodeBase
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		nb := nodeBaseOf(n)
		if nb != nil {
			nodes = append(nodes, nb)
		}
		return true
	})
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeSpan.Start.Offset < nodes[j].NodeSpan.Start.Offset
	})
	return nodes
}

// nodeBaseOf extracts a *NodeBase from an ast.Node if it's a type we attach trivia to.
func nodeBaseOf(n ast.Node) *ast.NodeBase {
	switch x := n.(type) {
	case *ast.SourceFile:
		return &x.NodeBase
	case *ast.ProgramDecl:
		return &x.NodeBase
	case *ast.FunctionBlockDecl:
		return &x.NodeBase
	case *ast.FunctionDecl:
		return &x.NodeBase
	case *ast.InterfaceDecl:
		return &x.NodeBase
	case *ast.MethodDecl:
		return &x.NodeBase
	case *ast.PropertyDecl:
		return &x.NodeBase
	case *ast.TypeDecl:
		return &x.NodeBase
	case *ast.ActionDecl:
		return &x.NodeBase
	case *ast.TestCaseDecl:
		return &x.NodeBase
	case *ast.VarBlock:
		return &x.NodeBase
	case *ast.VarDecl:
		return &x.NodeBase
	case *ast.AssignStmt:
		return &x.NodeBase
	case *ast.CallStmt:
		return &x.NodeBase
	case *ast.IfStmt:
		return &x.NodeBase
	case *ast.CaseStmt:
		return &x.NodeBase
	case *ast.ForStmt:
		return &x.NodeBase
	case *ast.WhileStmt:
		return &x.NodeBase
	case *ast.RepeatStmt:
		return &x.NodeBase
	case *ast.ReturnStmt:
		return &x.NodeBase
	case *ast.ExitStmt:
		return &x.NodeBase
	case *ast.ContinueStmt:
		return &x.NodeBase
	case *ast.ErrorNode:
		return &x.NodeBase
	default:
		return nil
	}
}

// findInnermostNode finds the most specific (innermost) AST node whose span
// contains the given offset. Among all nodes containing the offset, the one
// with the largest start offset (closest to the token) wins.
func findInnermostNode(nodes []*ast.NodeBase, offset int) *ast.NodeBase {
	// Binary search: find the last node whose start <= offset.
	idx := sort.Search(len(nodes), func(i int) bool {
		return nodes[i].NodeSpan.Start.Offset > offset
	})
	// Walk backwards to find the innermost containing node.
	for i := idx - 1; i >= 0; i-- {
		n := nodes[i]
		if n.NodeSpan.Start.Offset <= offset && n.NodeSpan.End.Offset >= offset {
			return n
		}
	}
	return nil
}
