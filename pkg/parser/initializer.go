package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// maxInitDepth caps initialiser nesting so adversarial input such as
// "[[[[..." or "((((a := ..." cannot exhaust the stack (threat T-20-07).
const maxInitDepth = 64

// parseInitializer parses the value after ":=" in a VAR, STRUCT member or
// TYPE declaration. It accepts a struct initialiser "(a := 1, b := 'x')", an
// array initialiser "[1, 3(0), (a := 1), [2, 3]]", or any expression. A
// repetition "N(value)" keeps N as Count and is never expanded (T-20-08).
// depth is the number of enclosing struct/array initialisers.
func (p *Parser) parseInitializer(depth int) ast.Expr {
	isStruct := p.at(lexer.LParen) && p.kindAt(1) == lexer.Ident && p.kindAt(2) == lexer.Assign
	isArray := p.at(lexer.LBracket)
	if (isStruct || isArray) && depth >= maxInitDepth {
		start := p.peek()
		p.error("initialiser nested too deeply")
		p.advance() // consume ( or [
		p.skipInitTail()
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindErrorNode,
				NodeSpan: spanFromTokens(start, p.tokens[p.pos-1]),
			},
			Message: "initialiser nested too deeply",
		}
	}
	switch {
	case isStruct:
		return p.parseStructInit(depth)
	case isArray:
		return p.parseArrayInit(depth)
	}
	switch p.peek().Kind {
	case lexer.RParen, lexer.RBracket, lexer.Comma, lexer.Semicolon, lexer.EOF:
		// Missing value: report without consuming the delimiter so the
		// enclosing list or declaration still sees it.
		cur := p.peek()
		p.error("expected expression, got %s", cur.Kind.String())
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindErrorNode,
				NodeSpan: ast.SpanFrom(astPos(cur.Pos), astPos(cur.Pos)),
			},
			Message: "expected expression",
		}
	}
	return p.parseExpr(0)
}

// parseStructInit parses "(" field ":=" value {"," field ":=" value} [","] ")".
func (p *Parser) parseStructInit(depth int) *ast.StructInit {
	startTok := p.advance() // consume (
	si := &ast.StructInit{NodeBase: ast.NodeBase{NodeKind: ast.KindStructInit}}
	for !p.at(lexer.RParen) {
		if !p.at(lexer.Ident) {
			p.error("expected field name, got %s", p.peek().Kind.String())
			break
		}
		nameTok := p.peek()
		name := p.parseIdent()
		p.expect(lexer.Assign)
		value := p.parseInitializer(depth + 1)
		si.Fields = append(si.Fields, &ast.FieldInit{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindFieldInit,
				NodeSpan: spanFromTokens(nameTok, p.tokens[p.pos-1]),
			},
			Name:  name,
			Value: value,
		})
		if !p.match(lexer.Comma) {
			break
		}
	}
	p.closeInit(lexer.RParen)
	si.NodeSpan = spanFromTokens(startTok, p.tokens[p.pos-1])
	return si
}

// parseArrayInit parses "[" [elem {"," elem} [","]] "]" where elem is
// either a value or a repetition "N(value)" / "N()".
func (p *Parser) parseArrayInit(depth int) *ast.ArrayInit {
	startTok := p.advance() // consume [
	ai := &ast.ArrayInit{NodeBase: ast.NodeBase{NodeKind: ast.KindArrayInit}}
	for !p.at(lexer.RBracket) {
		elemTok := p.peek()
		elem := &ast.ArrayInitElem{NodeBase: ast.NodeBase{NodeKind: ast.KindArrayInitElem}}
		if p.at(lexer.IntLiteral) && p.kindAt(1) == lexer.LParen {
			countTok := p.advance()
			elem.Count = &ast.Literal{
				NodeBase: ast.NodeBase{
					NodeKind: ast.KindLiteral,
					NodeSpan: ast.SpanFrom(astPos(countTok.Pos), astPos(countTok.EndPos)),
				},
				LitKind: ast.LitInt,
				Value:   countTok.Text,
			}
			p.advance() // consume (
			if !p.at(lexer.RParen) {
				elem.Value = p.parseInitializer(depth + 1)
			}
			p.closeInit(lexer.RParen)
		} else {
			elem.Value = p.parseInitializer(depth + 1)
		}
		elem.NodeSpan = spanFromTokens(elemTok, p.tokens[p.pos-1])
		ai.Elements = append(ai.Elements, elem)
		if !p.match(lexer.Comma) {
			break
		}
	}
	p.closeInit(lexer.RBracket)
	ai.NodeSpan = spanFromTokens(startTok, p.tokens[p.pos-1])
	return ai
}

// closeInit consumes the closing token of an initialiser list. If it is
// missing, it reports an error and skips to the matching close (or the end
// of the declaration) so the rest of the declaration list still parses.
func (p *Parser) closeInit(close lexer.TokenKind) {
	if p.match(close) {
		return
	}
	p.error("expected %s, got %s", close.String(), p.peek().Kind.String())
	p.skipInitTail()
}

// skipInitTail skips tokens up to and including the close that matches an
// already consumed opening bracket or parenthesis. It stops before a ";"
// or at end of file, since neither can appear inside an initialiser.
func (p *Parser) skipInitTail() {
	balance := 0
	for !p.atEnd() {
		switch p.peek().Kind {
		case lexer.Semicolon:
			return
		case lexer.LParen, lexer.LBracket:
			balance++
		case lexer.RParen, lexer.RBracket:
			if balance == 0 {
				p.advance()
				return
			}
			balance--
		}
		p.advance()
	}
}
