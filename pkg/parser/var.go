package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// isVarStart returns true if the current token begins a VAR section.
func (p *Parser) isVarStart() bool {
	switch p.peek().Kind {
	case lexer.KwVar, lexer.KwVarInput, lexer.KwVarOutput, lexer.KwVarInOut,
		lexer.KwVarTemp, lexer.KwVarGlobal, lexer.KwVarAccess,
		lexer.KwVarExternal, lexer.KwVarConfig:
		return true
	}
	return false
}

// parseVarBlocks parses zero or more VAR sections.
func (p *Parser) parseVarBlocks() []*ast.VarBlock {
	var blocks []*ast.VarBlock
	for p.isVarStart() {
		blocks = append(blocks, p.parseVarBlock())
	}
	return blocks
}

// parseVarBlock parses a single VAR section: VAR_xxx [CONSTANT|RETAIN|PERSISTENT] ... END_VAR
func (p *Parser) parseVarBlock() *ast.VarBlock {
	startTok := p.peek()

	// Determine section kind
	section := ast.VarLocal
	switch p.peek().Kind {
	case lexer.KwVar:
		section = ast.VarLocal
	case lexer.KwVarInput:
		section = ast.VarInput
	case lexer.KwVarOutput:
		section = ast.VarOutput
	case lexer.KwVarInOut:
		section = ast.VarInOut
	case lexer.KwVarTemp:
		section = ast.VarTemp
	case lexer.KwVarGlobal:
		section = ast.VarGlobal
	case lexer.KwVarAccess:
		section = ast.VarAccess
	case lexer.KwVarExternal:
		section = ast.VarExternal
	case lexer.KwVarConfig:
		section = ast.VarConfig
	}
	p.advance() // consume the VAR keyword

	// Optional modifiers
	isConstant := false
	isRetain := false
	isPersistent := false
	for {
		switch p.peek().Kind {
		case lexer.KwConstant:
			isConstant = true
			p.advance()
			continue
		case lexer.KwRetain:
			isRetain = true
			p.advance()
			continue
		case lexer.KwPersistent:
			isPersistent = true
			p.advance()
			continue
		}
		break
	}

	// Parse variable declarations until END_VAR
	var decls []*ast.VarDecl
	for !p.atEnd() && !p.at(lexer.KwEndVar) {
		decl := p.parseVarDecl()
		if decl != nil {
			decls = append(decls, decl)
		}
	}

	endTok := p.expect(lexer.KwEndVar)
	p.match(lexer.Semicolon)

	return &ast.VarBlock{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindVarBlock,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Section:      section,
		IsConstant:   isConstant,
		IsRetain:     isRetain,
		IsPersistent: isPersistent,
		Declarations: decls,
	}
}

// parseVarDecl parses a single variable declaration: name [, name2] [AT addr] : type [:= init] ;
func (p *Parser) parseVarDecl() *ast.VarDecl {
	startTok := p.peek()

	// Parse one or more names
	var names []*ast.Ident
	names = append(names, p.parseIdent())
	for p.match(lexer.Comma) {
		names = append(names, p.parseIdent())
	}

	// Optional AT address
	var atAddress *ast.Ident
	if p.match(lexer.KwAt) {
		// Address looks like %IX0.0 — lexer produces it as an identifier or special token
		if p.at(lexer.Ident) {
			tok := p.advance()
			atAddress = makeIdent(tok)
		}
	}

	p.expect(lexer.Colon)

	typeSpec := p.parseTypeSpec()

	// Optional initializer
	var initValue ast.Expr
	if p.match(lexer.Assign) {
		initValue = p.parseExpr(0)
	}

	endTok := p.peek()
	p.expect(lexer.Semicolon)

	return &ast.VarDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindVarDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Names:     names,
		Type:      typeSpec,
		InitValue: initValue,
		AtAddress: atAddress,
	}
}
