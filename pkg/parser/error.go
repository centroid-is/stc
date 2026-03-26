package parser

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lexer"
	"github.com/centroid-is/stc/pkg/source"
)

// error adds an Error diagnostic at the current token position.
func (p *Parser) error(format string, args ...any) {
	cur := p.peek()
	p.diags.Errorf(tokenPos(cur.Pos), "P001", format, args...)
}

// errorAt adds an Error diagnostic at a specific position.
func (p *Parser) errorAt(pos source.Pos, format string, args ...any) {
	p.diags.Errorf(pos, "P001", fmt.Sprintf(format, args...))
}

// declarationStarts lists token kinds that begin a declaration.
var declarationStarts = map[lexer.TokenKind]bool{
	lexer.KwProgram:       true,
	lexer.KwFunctionBlock: true,
	lexer.KwFunction:      true,
	lexer.KwType:          true,
	lexer.KwInterface:     true,
}

// statementStarts lists token kinds that begin a statement.
var statementStarts = map[lexer.TokenKind]bool{
	lexer.KwIf:       true,
	lexer.KwCase:     true,
	lexer.KwFor:      true,
	lexer.KwWhile:    true,
	lexer.KwRepeat:   true,
	lexer.KwReturn:   true,
	lexer.KwExit:     true,
	lexer.KwContinue: true,
	lexer.Ident:      true,
}

// synchronize skips tokens until one of stopAt is found, or a declaration/statement
// start keyword is encountered. This is the panic-mode error recovery mechanism.
func (p *Parser) synchronize(stopAt ...lexer.TokenKind) {
	stopSet := make(map[lexer.TokenKind]bool, len(stopAt))
	for _, k := range stopAt {
		stopSet[k] = true
	}

	for !p.atEnd() {
		cur := p.peek().Kind
		if stopSet[cur] {
			return
		}
		if declarationStarts[cur] || statementStarts[cur] {
			return
		}
		p.advance()
	}
}

// recoverDeclaration wraps the current error into an ErrorNode and synchronizes
// to the next declaration keyword. Returns the error as a Declaration.
func (p *Parser) recoverDeclaration() ast.Declaration {
	cur := p.peek()
	msg := fmt.Sprintf("unexpected %s in declaration context", cur.Kind.String())
	node := &ast.ErrorNode{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindErrorNode,
			NodeSpan: ast.SpanFrom(astPos(cur.Pos), astPos(cur.EndPos)),
		},
		Message: msg,
	}
	p.diags.Add(diag.Error, tokenPos(cur.Pos), source.Pos{}, "P002", msg)
	p.synchronize()
	return node
}

// recoverStatement wraps the current error into an ErrorNode and synchronizes
// to the next semicolon, END_*, or statement start. Returns the error as a Statement.
func (p *Parser) recoverStatement() ast.Statement {
	cur := p.peek()
	msg := fmt.Sprintf("unexpected %s in statement context", cur.Kind.String())
	node := &ast.ErrorNode{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindErrorNode,
			NodeSpan: ast.SpanFrom(astPos(cur.Pos), astPos(cur.EndPos)),
		},
		Message: msg,
	}
	p.diags.Add(diag.Error, tokenPos(cur.Pos), source.Pos{}, "P002", msg)
	p.synchronize(
		lexer.Semicolon,
		lexer.KwEndIf, lexer.KwEndCase, lexer.KwEndFor,
		lexer.KwEndWhile, lexer.KwEndRepeat,
		lexer.KwEndProgram, lexer.KwEndFunctionBlock, lexer.KwEndFunction,
		lexer.KwEndMethod,
	)
	// Consume the semicolon if we stopped at one
	p.match(lexer.Semicolon)
	return node
}
