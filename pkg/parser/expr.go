package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// Operator precedence levels (from IEC 61131-3).
const (
	precNone   = 0
	precOr     = 1
	precXor    = 2
	precAnd    = 3
	precEq     = 4  // =, <>
	precCmp    = 5  // <, >, <=, >=
	precAdd    = 6  // +, -
	precMul    = 7  // *, /, MOD
	precPower  = 8  // **
)

// infixPrecedence returns the precedence for a binary operator token, or 0 if not an operator.
func infixPrecedence(kind lexer.TokenKind) int {
	switch kind {
	case lexer.KwOr:
		return precOr
	case lexer.KwXor:
		return precXor
	case lexer.KwAnd, lexer.Ampersand:
		return precAnd
	case lexer.Eq, lexer.NotEq:
		return precEq
	case lexer.Less, lexer.Greater, lexer.LessEq, lexer.GreaterEq:
		return precCmp
	case lexer.Plus, lexer.Minus:
		return precAdd
	case lexer.Star, lexer.Slash, lexer.KwMod:
		return precMul
	case lexer.Power:
		return precPower
	default:
		return precNone
	}
}

// isRightAssociative returns true for right-associative operators.
func isRightAssociative(kind lexer.TokenKind) bool {
	return kind == lexer.Power
}

// parseExpr is the Pratt expression parser entry point.
func (p *Parser) parseExpr(minPrec int) ast.Expr {
	left := p.parseUnaryExpr()

	for {
		prec := infixPrecedence(p.peek().Kind)
		if prec == precNone || prec < minPrec {
			break
		}

		opTok := p.advance()
		op := ast.Token{
			Kind: int(opTok.Kind),
			Text: opTok.Text,
			Span: ast.SpanFrom(astPos(opTok.Pos), astPos(opTok.EndPos)),
		}

		// Right-associative operators use same prec; left-associative use prec+1
		nextMinPrec := prec + 1
		if isRightAssociative(opTok.Kind) {
			nextMinPrec = prec
		}

		right := p.parseExpr(nextMinPrec)

		left = &ast.BinaryExpr{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindBinaryExpr,
				NodeSpan: ast.SpanFrom(left.Span().Start, right.Span().End),
			},
			Left:  left,
			Op:    op,
			Right: right,
		}
	}

	return left
}

// parseUnaryExpr parses unary prefix operators (NOT, -).
func (p *Parser) parseUnaryExpr() ast.Expr {
	switch p.peek().Kind {
	case lexer.KwNot:
		opTok := p.advance()
		operand := p.parseUnaryExpr()
		return &ast.UnaryExpr{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindUnaryExpr,
				NodeSpan: ast.SpanFrom(astPos(opTok.Pos), operand.Span().End),
			},
			Op: ast.Token{
				Kind: int(opTok.Kind),
				Text: opTok.Text,
				Span: ast.SpanFrom(astPos(opTok.Pos), astPos(opTok.EndPos)),
			},
			Operand: operand,
		}
	case lexer.Minus:
		opTok := p.advance()
		operand := p.parseUnaryExpr()
		return &ast.UnaryExpr{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindUnaryExpr,
				NodeSpan: ast.SpanFrom(astPos(opTok.Pos), operand.Span().End),
			},
			Op: ast.Token{
				Kind: int(opTok.Kind),
				Text: opTok.Text,
				Span: ast.SpanFrom(astPos(opTok.Pos), astPos(opTok.EndPos)),
			},
			Operand: operand,
		}
	default:
		return p.parsePrimaryExpr()
	}
}

// parsePrimaryExpr parses primary expressions: literals, identifiers, parens, and postfix ops.
func (p *Parser) parsePrimaryExpr() ast.Expr {
	tok := p.peek()

	switch tok.Kind {
	case lexer.IntLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitInt,
			Value:   tok.Text,
		}

	case lexer.RealLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitReal,
			Value:   tok.Text,
		}

	case lexer.StringLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitString,
			Value:   tok.Text,
		}

	case lexer.WStringLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitWString,
			Value:   tok.Text,
		}

	case lexer.TimeLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitTime,
			Value:   tok.Text,
		}

	case lexer.DateLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitDate,
			Value:   tok.Text,
		}

	case lexer.DateTimeLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitDateTime,
			Value:   tok.Text,
		}

	case lexer.TodLiteral:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitTod,
			Value:   tok.Text,
		}

	case lexer.TypedLiteral:
		p.advance()
		return p.parseTypedLiteral(tok)

	case lexer.KwTrue:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitBool,
			Value:   tok.Text,
		}

	case lexer.KwFalse:
		p.advance()
		return &ast.Literal{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindLiteral,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			LitKind: ast.LitBool,
			Value:   tok.Text,
		}

	case lexer.Ident:
		return p.parseIdentExpr()

	case lexer.LParen:
		return p.parseParenExpr()

	default:
		p.error("expected expression, got %s", tok.Kind.String())
		p.advance() // skip the bad token
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindErrorNode,
				NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
			},
			Message: "expected expression",
		}
	}
}

// parseTypedLiteral parses a typed literal like INT#5 or REAL#2.0.
func (p *Parser) parseTypedLiteral(tok lexer.Token) *ast.Literal {
	// The text contains "TYPE#value" — split at #
	text := tok.Text
	prefix := ""
	value := text
	for i, ch := range text {
		if ch == '#' {
			prefix = text[:i]
			value = text[i+1:]
			break
		}
	}
	return &ast.Literal{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindLiteral,
			NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
		},
		LitKind:    ast.LitTyped,
		Value:      value,
		TypePrefix: prefix,
	}
}

// parseIdentExpr parses an identifier and its postfix operations (call, member, index, deref).
func (p *Parser) parseIdentExpr() ast.Expr {
	tok := p.advance()
	var expr ast.Expr = makeIdent(tok)

	return p.parsePostfix(expr)
}

// parsePostfix parses postfix operations: member access, call, index, deref.
func (p *Parser) parsePostfix(expr ast.Expr) ast.Expr {
	for {
		switch p.peek().Kind {
		case lexer.Dot:
			p.advance()
			member := p.parseIdent()
			expr = &ast.MemberAccessExpr{
				NodeBase: ast.NodeBase{
					NodeKind: ast.KindMemberAccessExpr,
					NodeSpan: ast.SpanFrom(expr.Span().Start, member.Span().End),
				},
				Object: expr,
				Member: member,
			}

		case lexer.LParen:
			// Check for named-argument FB call: ident(name := ...) or ident(name => ...)
			// These are statement-level constructs, not expression calls.
			// Use lookahead: if ( is followed by ident then := or =>, leave for stmt parser.
			if p.isNamedArgCall() {
				return expr
			}
			p.advance()
			var args []ast.Expr
			if !p.at(lexer.RParen) {
				args = append(args, p.parseExpr(0))
				for p.match(lexer.Comma) {
					args = append(args, p.parseExpr(0))
				}
			}
			endTok := p.expect(lexer.RParen)
			expr = &ast.CallExpr{
				NodeBase: ast.NodeBase{
					NodeKind: ast.KindCallExpr,
					NodeSpan: ast.SpanFrom(expr.Span().Start, astPos(endTok.EndPos)),
				},
				Callee: expr,
				Args:   args,
			}

		case lexer.LBracket:
			p.advance()
			var indices []ast.Expr
			indices = append(indices, p.parseExpr(0))
			for p.match(lexer.Comma) {
				indices = append(indices, p.parseExpr(0))
			}
			endTok := p.expect(lexer.RBracket)
			expr = &ast.IndexExpr{
				NodeBase: ast.NodeBase{
					NodeKind: ast.KindIndexExpr,
					NodeSpan: ast.SpanFrom(expr.Span().Start, astPos(endTok.EndPos)),
				},
				Object:  expr,
				Indices: indices,
			}

		case lexer.Caret:
			caretTok := p.advance()
			expr = &ast.DerefExpr{
				NodeBase: ast.NodeBase{
					NodeKind: ast.KindDerefExpr,
					NodeSpan: ast.SpanFrom(expr.Span().Start, astPos(caretTok.EndPos)),
				},
				Operand: expr,
			}

		case lexer.Hash:
			// TypeName#Value — typed enum literal (e.g., Color#Green)
			// Only valid when expression so far is a simple identifier
			if ident, ok := expr.(*ast.Ident); ok {
				p.advance() // consume #
				valueTok := p.peek()
				if valueTok.Kind == lexer.Ident {
					p.advance()
					expr = &ast.Literal{
						NodeBase: ast.NodeBase{
							NodeKind: ast.KindLiteral,
							NodeSpan: ast.SpanFrom(ident.Span().Start, astPos(valueTok.EndPos)),
						},
						LitKind:    ast.LitTyped,
						Value:      valueTok.Text,
						TypePrefix: ident.Name,
					}
				} else {
					// Not an ident after #, treat as error
					return expr
				}
			} else {
				return expr
			}

		default:
			return expr
		}
	}
}

// isNamedArgCall checks whether the current LParen starts a named-argument
// FB call (e.g., fb(IN := val)). It peeks ahead without consuming tokens.
// Returns true if the pattern is ( ident := or ( ident =>
func (p *Parser) isNamedArgCall() bool {
	// Current token should be LParen
	if !p.at(lexer.LParen) {
		return false
	}
	// Also return false for empty parens -- that's a regular call
	nextIdx := p.pos + 1
	if nextIdx >= len(p.tokens) {
		return false
	}
	// Check if next token after ( is an identifier
	if p.tokens[nextIdx].Kind != lexer.Ident {
		return false
	}
	// Check if the token after that is := or =>
	afterIdx := nextIdx + 1
	if afterIdx >= len(p.tokens) {
		return false
	}
	afterKind := p.tokens[afterIdx].Kind
	return afterKind == lexer.Assign || afterKind == lexer.Arrow
}

// parseParenExpr parses (expr).
func (p *Parser) parseParenExpr() ast.Expr {
	startTok := p.advance() // consume (
	inner := p.parseExpr(0)
	endTok := p.expect(lexer.RParen)

	return &ast.ParenExpr{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindParenExpr,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Inner: inner,
	}
}
