package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// parseStatements parses statements until a stop token or EOF is reached.
func (p *Parser) parseStatements(stop ...lexer.TokenKind) []ast.Statement {
	stopSet := make(map[lexer.TokenKind]bool, len(stop))
	for _, k := range stop {
		stopSet[k] = true
	}

	var stmts []ast.Statement
	for !p.atEnd() {
		if stopSet[p.peek().Kind] {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

// parseStatement dispatches to the appropriate statement parser.
func (p *Parser) parseStatement() ast.Statement {
	switch p.peek().Kind {
	case lexer.KwIf:
		return p.parseIfStmt()
	case lexer.KwCase:
		return p.parseCaseStmt()
	case lexer.KwFor:
		return p.parseForStmt()
	case lexer.KwWhile:
		return p.parseWhileStmt()
	case lexer.KwRepeat:
		return p.parseRepeatStmt()
	case lexer.KwReturn:
		startTok := p.advance()
		endTok := startTok
		if p.at(lexer.Semicolon) {
			endTok = p.advance()
		}
		return &ast.ReturnStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindReturnStmt,
				NodeSpan: spanFromTokens(startTok, endTok),
			},
		}
	case lexer.KwExit:
		startTok := p.advance()
		endTok := startTok
		if p.at(lexer.Semicolon) {
			endTok = p.advance()
		}
		return &ast.ExitStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindExitStmt,
				NodeSpan: spanFromTokens(startTok, endTok),
			},
		}
	case lexer.KwContinue:
		startTok := p.advance()
		endTok := startTok
		if p.at(lexer.Semicolon) {
			endTok = p.advance()
		}
		return &ast.ContinueStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindContinueStmt,
				NodeSpan: spanFromTokens(startTok, endTok),
			},
		}
	case lexer.Semicolon:
		startTok := p.advance()
		return &ast.EmptyStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindEmptyStmt,
				NodeSpan: spanFromTokens(startTok, startTok),
			},
		}
	case lexer.Ident:
		return p.parseAssignOrCall()
	default:
		return p.recoverStatement()
	}
}

// parseAssignOrCall parses either an assignment (x := expr;) or FB call (fb(args);).
func (p *Parser) parseAssignOrCall() ast.Statement {
	startTok := p.peek()
	lhs := p.parseExpr(0)

	// Assignment: lhs := rhs ;
	if p.match(lexer.Assign) {
		rhs := p.parseExpr(0)
		endTok := p.peek()
		p.expect(lexer.Semicolon)
		return &ast.AssignStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindAssignStmt,
				NodeSpan: spanFromTokens(startTok, endTok),
			},
			Target: lhs,
			Value:  rhs,
		}
	}

	// FB call: callee(args) ;
	if p.at(lexer.LParen) {
		p.advance() // consume (
		args := p.parseCallArgs()
		p.expect(lexer.RParen)
		endTok := p.peek()
		p.expect(lexer.Semicolon)
		return &ast.CallStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindCallStmt,
				NodeSpan: spanFromTokens(startTok, endTok),
			},
			Callee: lhs,
			Args:   args,
		}
	}

	// Expression statement — consume semicolon
	p.expect(lexer.Semicolon)
	return &ast.AssignStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindAssignStmt,
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Target: lhs,
	}
}

// parseIfStmt parses IF cond THEN body [ELSIF cond THEN body]* [ELSE body] END_IF [;]
func (p *Parser) parseIfStmt() *ast.IfStmt {
	startTok := p.advance() // consume IF
	cond := p.parseExpr(0)
	p.expect(lexer.KwThen)

	body := p.parseStatements(lexer.KwElsif, lexer.KwElse, lexer.KwEndIf)

	var elsifs []*ast.ElsIf
	for p.match(lexer.KwElsif) {
		elsifCond := p.parseExpr(0)
		p.expect(lexer.KwThen)
		elsifBody := p.parseStatements(lexer.KwElsif, lexer.KwElse, lexer.KwEndIf)
		elsifs = append(elsifs, &ast.ElsIf{
			Condition: elsifCond,
			Body:      elsifBody,
		})
	}

	var elseBody []ast.Statement
	if p.match(lexer.KwElse) {
		elseBody = p.parseStatements(lexer.KwEndIf)
	}

	endTok := p.expect(lexer.KwEndIf)
	p.match(lexer.Semicolon)

	return &ast.IfStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindIfStmt,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Condition: cond,
		Then:      body,
		ElsIfs:    elsifs,
		Else:      elseBody,
	}
}

// parseCaseStmt parses CASE expr OF branches [ELSE body] END_CASE [;]
func (p *Parser) parseCaseStmt() *ast.CaseStmt {
	startTok := p.advance() // consume CASE
	selector := p.parseExpr(0)
	p.expect(lexer.KwOf)

	var branches []*ast.CaseBranch
	var elseBranch []ast.Statement

	for !p.atEnd() && !p.at(lexer.KwElse) && !p.at(lexer.KwEndCase) {
		branch := p.parseCaseBranch()
		if branch != nil {
			branches = append(branches, branch)
		}
	}

	if p.match(lexer.KwElse) {
		elseBranch = p.parseStatements(lexer.KwEndCase)
	}

	endTok := p.expect(lexer.KwEndCase)
	p.match(lexer.Semicolon)

	return &ast.CaseStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindCaseStmt,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Expr:       selector,
		Branches:   branches,
		ElseBranch: elseBranch,
	}
}

// parseCaseBranch parses a case branch: labels : body
func (p *Parser) parseCaseBranch() *ast.CaseBranch {
	startTok := p.peek()

	// Parse labels: comma-separated values or ranges
	var labels []ast.CaseLabel
	labels = append(labels, p.parseCaseLabel())
	for p.match(lexer.Comma) {
		labels = append(labels, p.parseCaseLabel())
	}

	p.expect(lexer.Colon)

	// Parse body until next label-like token, ELSE, or END_CASE
	body := p.parseCaseBranchBody()

	return &ast.CaseBranch{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Labels: labels,
		Body:   body,
	}
}

// parseCaseLabel parses a case label: value or range (low..high).
func (p *Parser) parseCaseLabel() ast.CaseLabel {
	startTok := p.peek()
	expr := p.parseExpr(0)

	if p.match(lexer.DotDot) {
		high := p.parseExpr(0)
		return &ast.CaseLabelRange{
			NodeBase: ast.NodeBase{
				NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
			},
			Low:  expr,
			High: high,
		}
	}

	return &ast.CaseLabelValue{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Value: expr,
	}
}

// parseCaseBranchBody parses statements until a new case label or END_CASE/ELSE.
func (p *Parser) parseCaseBranchBody() []ast.Statement {
	var stmts []ast.Statement
	for !p.atEnd() {
		cur := p.peek().Kind
		if cur == lexer.KwElse || cur == lexer.KwEndCase {
			break
		}
		// Check if this looks like a new case label (integer/ident followed by .. or , or :)
		if p.isCaseLabelStart() {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

// isCaseLabelStart uses lookahead to determine if current position starts a case label.
func (p *Parser) isCaseLabelStart() bool {
	if p.at(lexer.IntLiteral) || p.at(lexer.Ident) {
		saved := p.pos
		defer func() { p.pos = saved }()
		p.advance()
		// If followed by :, .., or , it's a label
		cur := p.peek().Kind
		return cur == lexer.Colon || cur == lexer.DotDot || cur == lexer.Comma
	}
	return false
}

// parseForStmt parses FOR var := from TO to [BY step] DO body END_FOR [;]
func (p *Parser) parseForStmt() *ast.ForStmt {
	startTok := p.advance() // consume FOR
	variable := p.parseIdent()
	p.expect(lexer.Assign)
	from := p.parseExpr(0)
	p.expect(lexer.KwTo)
	to := p.parseExpr(0)

	var by ast.Expr
	if p.match(lexer.KwBy) {
		by = p.parseExpr(0)
	}

	p.expect(lexer.KwDo)
	body := p.parseStatements(lexer.KwEndFor)
	endTok := p.expect(lexer.KwEndFor)
	p.match(lexer.Semicolon)

	return &ast.ForStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindForStmt,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Variable: variable,
		From:     from,
		To:       to,
		By:       by,
		Body:     body,
	}
}

// parseWhileStmt parses WHILE cond DO body END_WHILE [;]
func (p *Parser) parseWhileStmt() *ast.WhileStmt {
	startTok := p.advance() // consume WHILE
	cond := p.parseExpr(0)
	p.expect(lexer.KwDo)
	body := p.parseStatements(lexer.KwEndWhile)
	endTok := p.expect(lexer.KwEndWhile)
	p.match(lexer.Semicolon)

	return &ast.WhileStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindWhileStmt,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Condition: cond,
		Body:      body,
	}
}

// parseRepeatStmt parses REPEAT body UNTIL cond END_REPEAT [;]
func (p *Parser) parseRepeatStmt() *ast.RepeatStmt {
	startTok := p.advance() // consume REPEAT
	body := p.parseStatements(lexer.KwUntil)
	p.expect(lexer.KwUntil)
	cond := p.parseExpr(0)

	// END_REPEAT is optional in some dialects; handle both
	endTok := p.peek()
	if p.at(lexer.KwEndRepeat) {
		endTok = p.advance()
	}
	p.match(lexer.Semicolon)

	return &ast.RepeatStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindRepeatStmt,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Body:      body,
		Condition: cond,
	}
}

// parseCallArgs parses named arguments inside a function block call.
func (p *Parser) parseCallArgs() []*ast.CallArg {
	var args []*ast.CallArg
	if p.at(lexer.RParen) {
		return args
	}

	for {
		arg := p.parseCallArg()
		args = append(args, arg)
		if !p.match(lexer.Comma) {
			break
		}
	}
	return args
}

// parseCallArg parses a single call argument: [Name :=] expr or [Name =>] expr
func (p *Parser) parseCallArg() *ast.CallArg {
	startTok := p.peek()

	// Try named argument: Ident := expr or Ident => expr
	if p.at(lexer.Ident) {
		saved := p.pos
		nameTok := p.advance()
		if p.at(lexer.Assign) {
			p.advance()
			value := p.parseExpr(0)
			return &ast.CallArg{
				NodeBase: ast.NodeBase{
					NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
				},
				Name:  makeIdent(nameTok),
				Value: value,
			}
		}
		if p.at(lexer.Arrow) {
			p.advance()
			value := p.parseExpr(0)
			return &ast.CallArg{
				NodeBase: ast.NodeBase{
					NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
				},
				Name:     makeIdent(nameTok),
				Value:    value,
				IsOutput: true,
			}
		}
		// Not a named arg, backtrack
		p.pos = saved
	}

	// Positional argument
	value := p.parseExpr(0)
	return &ast.CallArg{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Value: value,
	}
}
