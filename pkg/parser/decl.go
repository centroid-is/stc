package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// parseDeclaration dispatches to the appropriate declaration parser
// based on the current token.
func (p *Parser) parseDeclaration() ast.Declaration {
	switch p.peek().Kind {
	case lexer.Pragma:
		// Skip pragmas between declarations (attach as trivia in future)
		p.advance()
		if !p.atEnd() {
			return p.parseDeclaration()
		}
		return nil
	case lexer.KwProgram:
		return p.parseProgram()
	case lexer.KwFunctionBlock:
		return p.parseFunctionBlock()
	case lexer.KwFunction:
		return p.parseFunction()
	case lexer.KwType:
		return p.parseTypeDecls()
	case lexer.KwInterface:
		return p.parseInterface()
	case lexer.KwTestCase:
		return p.parseTestCase()
	default:
		return p.recoverDeclaration()
	}
}

// parseProgram parses PROGRAM name ... END_PROGRAM
func (p *Parser) parseProgram() *ast.ProgramDecl {
	startTok := p.advance() // consume PROGRAM
	name := p.parseIdent()
	p.match(lexer.Semicolon) // optional trailing semicolon

	varBlocks := p.parseVarBlocks()
	body := p.parseStatements(lexer.KwEndProgram)

	endTok := p.expect(lexer.KwEndProgram)
	p.match(lexer.Semicolon)

	return &ast.ProgramDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindProgramDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:      name,
		VarBlocks: varBlocks,
		Body:      body,
	}
}

// parseFunctionBlock parses FUNCTION_BLOCK name [EXTENDS base] [IMPLEMENTS ifaces] ... END_FUNCTION_BLOCK
func (p *Parser) parseFunctionBlock() *ast.FunctionBlockDecl {
	startTok := p.advance() // consume FUNCTION_BLOCK

	// Optional ABSTRACT/FINAL before name
	isAbstract := p.match(lexer.KwAbstract)
	_ = isAbstract // stored if needed later

	name := p.parseIdent()

	// EXTENDS
	var extends *ast.Ident
	if p.match(lexer.KwExtends) {
		extends = p.parseIdent()
	}

	// IMPLEMENTS
	var implements []*ast.Ident
	if p.match(lexer.KwImplements) {
		implements = append(implements, p.parseIdent())
		for p.match(lexer.Comma) {
			implements = append(implements, p.parseIdent())
		}
	}

	p.match(lexer.Semicolon) // optional

	varBlocks := p.parseVarBlocks()

	// Parse body, methods, and properties
	var body []ast.Statement
	var methods []*ast.MethodDecl
	var properties []*ast.PropertyDecl

	for !p.atEnd() && !p.at(lexer.KwEndFunctionBlock) {
		savedPos := p.pos
		switch p.peek().Kind {
		case lexer.KwMethod, lexer.KwPublic, lexer.KwPrivate, lexer.KwProtected, lexer.KwInternal,
			lexer.KwAbstract, lexer.KwFinal, lexer.KwOverride:
			// Could be a method with access modifier
			if p.isMethodStart() {
				methods = append(methods, p.parseMethod())
			} else {
				body = append(body, p.parseStatement())
			}
		case lexer.KwProperty:
			properties = append(properties, p.parseProperty())
		default:
			stmts := p.parseStatements(
				lexer.KwEndFunctionBlock, lexer.KwMethod, lexer.KwProperty,
				lexer.KwPublic, lexer.KwPrivate, lexer.KwProtected, lexer.KwInternal,
				lexer.KwAbstract, lexer.KwFinal, lexer.KwOverride,
			)
			body = append(body, stmts...)
		}
		// Guard against infinite loops.
		if p.pos == savedPos {
			p.advance()
		}
	}

	endTok := p.expect(lexer.KwEndFunctionBlock)
	p.match(lexer.Semicolon)

	return &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindFunctionBlockDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:       name,
		Extends:    extends,
		Implements: implements,
		VarBlocks:  varBlocks,
		Body:       body,
		Methods:    methods,
		Properties: properties,
	}
}

// isMethodStart looks ahead to determine if the current position starts a METHOD.
// Handles both "METHOD PUBLIC name" and "PUBLIC METHOD name" orderings.
func (p *Parser) isMethodStart() bool {
	// If directly at METHOD, it's a method
	if p.at(lexer.KwMethod) {
		return true
	}
	saved := p.pos
	defer func() { p.pos = saved }()

	// Skip access modifiers
	for p.at(lexer.KwPublic) || p.at(lexer.KwPrivate) || p.at(lexer.KwProtected) || p.at(lexer.KwInternal) {
		p.advance()
	}
	// Skip ABSTRACT/FINAL/OVERRIDE
	for p.at(lexer.KwAbstract) || p.at(lexer.KwFinal) || p.at(lexer.KwOverride) {
		p.advance()
	}
	return p.at(lexer.KwMethod)
}

// parseFunction parses FUNCTION name : returnType ... END_FUNCTION
func (p *Parser) parseFunction() *ast.FunctionDecl {
	startTok := p.advance() // consume FUNCTION
	name := p.parseIdent()

	var returnType ast.TypeSpec
	if p.match(lexer.Colon) {
		returnType = p.parseTypeSpec()
	}

	p.match(lexer.Semicolon) // optional

	varBlocks := p.parseVarBlocks()
	body := p.parseStatements(lexer.KwEndFunction)
	endTok := p.expect(lexer.KwEndFunction)
	p.match(lexer.Semicolon)

	return &ast.FunctionDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindFunctionDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:       name,
		ReturnType: returnType,
		VarBlocks:  varBlocks,
		Body:       body,
	}
}

// parseTypeDecls parses TYPE name : typespec; END_TYPE (possibly multiple)
func (p *Parser) parseTypeDecls() ast.Declaration {
	startTok := p.advance() // consume TYPE

	// A TYPE block can contain multiple type declarations before END_TYPE
	// But we return them as individual TypeDecl nodes within a wrapper,
	// or just parse the first one if there's only one.
	name := p.parseIdent()
	p.expect(lexer.Colon)

	typeSpec := p.parseTypeSpec()
	p.match(lexer.Semicolon)

	endTok := p.expect(lexer.KwEndType)
	p.match(lexer.Semicolon)

	return &ast.TypeDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindTypeDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name: name,
		Type: typeSpec,
	}
}

// parseInterface parses INTERFACE name [EXTENDS bases] ... END_INTERFACE
func (p *Parser) parseInterface() *ast.InterfaceDecl {
	startTok := p.advance() // consume INTERFACE
	name := p.parseIdent()

	var extends []*ast.Ident
	if p.match(lexer.KwExtends) {
		extends = append(extends, p.parseIdent())
		for p.match(lexer.Comma) {
			extends = append(extends, p.parseIdent())
		}
	}

	p.match(lexer.Semicolon)

	// Parse method and property signatures
	var methods []*ast.MethodSignature
	var properties []*ast.PropertySignature

	for !p.atEnd() && !p.at(lexer.KwEndInterface) {
		switch p.peek().Kind {
		case lexer.KwMethod:
			methods = append(methods, p.parseMethodSignature())
		case lexer.KwProperty:
			properties = append(properties, p.parsePropertySignature())
		default:
			// Skip unexpected tokens
			p.error("unexpected %s in interface", p.peek().Kind.String())
			p.advance()
		}
	}

	endTok := p.expect(lexer.KwEndInterface)
	p.match(lexer.Semicolon)

	return &ast.InterfaceDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindInterfaceDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:       name,
		Extends:    extends,
		Methods:    methods,
		Properties: properties,
	}
}

// parseMethod parses METHOD [access] [ABSTRACT|FINAL|OVERRIDE] name [: returnType] ... END_METHOD
// Also supports [access] METHOD ... ordering for pre-METHOD modifiers.
func (p *Parser) parseMethod() *ast.MethodDecl {
	startTok := p.peek()

	access := ast.AccessNone
	var isAbstract, isFinal, isOverride bool

	// Modifiers can appear before or after METHOD keyword.
	// First, consume any pre-METHOD modifiers.
	for {
		switch p.peek().Kind {
		case lexer.KwPublic:
			access = ast.AccessPublic
			p.advance()
			continue
		case lexer.KwPrivate:
			access = ast.AccessPrivate
			p.advance()
			continue
		case lexer.KwProtected:
			access = ast.AccessProtected
			p.advance()
			continue
		case lexer.KwInternal:
			access = ast.AccessInternal
			p.advance()
			continue
		case lexer.KwAbstract:
			isAbstract = true
			p.advance()
			continue
		case lexer.KwFinal:
			isFinal = true
			p.advance()
			continue
		case lexer.KwOverride:
			isOverride = true
			p.advance()
			continue
		}
		break
	}

	p.expect(lexer.KwMethod)

	// Modifiers can also appear after METHOD keyword.
	for {
		switch p.peek().Kind {
		case lexer.KwPublic:
			access = ast.AccessPublic
			p.advance()
			continue
		case lexer.KwPrivate:
			access = ast.AccessPrivate
			p.advance()
			continue
		case lexer.KwProtected:
			access = ast.AccessProtected
			p.advance()
			continue
		case lexer.KwInternal:
			access = ast.AccessInternal
			p.advance()
			continue
		case lexer.KwAbstract:
			isAbstract = true
			p.advance()
			continue
		case lexer.KwFinal:
			isFinal = true
			p.advance()
			continue
		case lexer.KwOverride:
			isOverride = true
			p.advance()
			continue
		}
		break
	}

	name := p.parseIdent()

	var returnType ast.TypeSpec
	if p.match(lexer.Colon) {
		returnType = p.parseTypeSpec()
	}

	p.match(lexer.Semicolon)

	varBlocks := p.parseVarBlocks()
	body := p.parseStatements(lexer.KwEndMethod)
	endTok := p.expect(lexer.KwEndMethod)
	p.match(lexer.Semicolon)

	return &ast.MethodDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		AccessModifier: access,
		Name:           name,
		ReturnType:     returnType,
		VarBlocks:      varBlocks,
		Body:           body,
		IsAbstract:     isAbstract,
		IsFinal:        isFinal,
		IsOverride:     isOverride,
	}
}

// parseProperty parses PROPERTY name : type ... END_PROPERTY
func (p *Parser) parseProperty() *ast.PropertyDecl {
	startTok := p.advance() // consume PROPERTY
	name := p.parseIdent()
	p.expect(lexer.Colon)
	typeSpec := p.parseTypeSpec()
	p.match(lexer.Semicolon)

	// Parse getter/setter (they look like METHOD GET / METHOD SET)
	// For now, just skip to END_PROPERTY
	for !p.atEnd() && !p.at(lexer.KwEndProperty) {
		p.advance()
	}

	endTok := p.expect(lexer.KwEndProperty)
	p.match(lexer.Semicolon)

	return &ast.PropertyDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindPropertyDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name: name,
		Type: typeSpec,
	}
}

// parseMethodSignature parses METHOD name [: returnType] [varBlocks] END_METHOD inside an interface.
func (p *Parser) parseMethodSignature() *ast.MethodSignature {
	startTok := p.advance() // consume METHOD
	name := p.parseIdent()

	var returnType ast.TypeSpec
	if p.match(lexer.Colon) {
		returnType = p.parseTypeSpec()
	}

	p.match(lexer.Semicolon)

	// Parse optional var blocks in signature
	varBlocks := p.parseVarBlocks()

	endTok := p.expect(lexer.KwEndMethod)
	p.match(lexer.Semicolon)

	return &ast.MethodSignature{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:       name,
		ReturnType: returnType,
		VarBlocks:  varBlocks,
	}
}

// parsePropertySignature parses PROPERTY name : type END_PROPERTY inside an interface.
func (p *Parser) parsePropertySignature() *ast.PropertySignature {
	startTok := p.advance() // consume PROPERTY
	name := p.parseIdent()
	p.expect(lexer.Colon)
	typeSpec := p.parseTypeSpec()
	p.match(lexer.Semicolon)

	for !p.atEnd() && !p.at(lexer.KwEndProperty) {
		p.advance()
	}

	endTok := p.expect(lexer.KwEndProperty)
	p.match(lexer.Semicolon)

	return &ast.PropertySignature{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindPropertyDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name: name,
		Type: typeSpec,
	}
}

// parseTestCase parses TEST_CASE 'name' ... END_TEST_CASE
func (p *Parser) parseTestCase() *ast.TestCaseDecl {
	startTok := p.advance() // consume TEST_CASE

	// Expect string literal for test name (single or double quoted)
	var name string
	if p.at(lexer.StringLiteral) || p.at(lexer.WStringLiteral) {
		nameTok := p.advance()
		name = nameTok.Text
		// Strip surrounding quotes (single or double)
		if len(name) >= 2 {
			if (name[0] == '\'' && name[len(name)-1] == '\'') ||
				(name[0] == '"' && name[len(name)-1] == '"') {
				name = name[1 : len(name)-1]
			}
		}
	} else {
		p.error("expected string literal for TEST_CASE name, got %s", p.peek().Kind.String())
	}

	p.match(lexer.Semicolon) // optional

	varBlocks := p.parseVarBlocks()
	body := p.parseStatements(lexer.KwEndTestCase)

	endTok := p.expect(lexer.KwEndTestCase)
	p.match(lexer.Semicolon) // optional

	return &ast.TestCaseDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindTestCaseDecl,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:      name,
		VarBlocks: varBlocks,
		Body:      body,
	}
}
