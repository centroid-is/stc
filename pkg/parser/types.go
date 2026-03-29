package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// primitiveTypeKeywords maps keyword token kinds to their type name text.
var primitiveTypeKeywords = map[lexer.TokenKind]string{
	lexer.KwBool:       "BOOL",
	lexer.KwByte:       "BYTE",
	lexer.KwWord:       "WORD",
	lexer.KwDword:      "DWORD",
	lexer.KwLword:      "LWORD",
	lexer.KwSint:       "SINT",
	lexer.KwInt:        "INT",
	lexer.KwDint:       "DINT",
	lexer.KwLint:       "LINT",
	lexer.KwUsint:      "USINT",
	lexer.KwUint:       "UINT",
	lexer.KwUdint:      "UDINT",
	lexer.KwUlint:      "ULINT",
	lexer.KwReal:       "REAL",
	lexer.KwLreal:      "LREAL",
	lexer.KwTime:       "TIME",
	lexer.KwDate:       "DATE",
	lexer.KwTimeOfDay:  "TIME_OF_DAY",
	lexer.KwTod:        "TOD",
	lexer.KwDateAndTime: "DATE_AND_TIME",
	lexer.KwDt:         "DT",
}

// parseTypeSpec parses a type specifier.
func (p *Parser) parseTypeSpec() ast.TypeSpec {
	switch p.peek().Kind {
	case lexer.KwArray:
		return p.parseArrayType()

	case lexer.KwPointer:
		return p.parsePointerType()

	case lexer.KwReference:
		return p.parseReferenceType()

	case lexer.KwString:
		return p.parseStringType(false)

	case lexer.KwWString:
		return p.parseStringType(true)

	case lexer.KwStruct:
		return p.parseStructType()

	case lexer.LParen:
		// Enum type: (Val1, Val2 := 3, Val3)
		return p.parseEnumType()

	case lexer.Ident:
		return p.parseNamedTypeOrSubrange()

	default:
		// Check for primitive type keywords
		if name, ok := primitiveTypeKeywords[p.peek().Kind]; ok {
			return p.parsePrimitiveTypeOrSubrange(name)
		}

		// Error
		p.error("expected type specifier, got %s", p.peek().Kind.String())
		cur := p.peek()
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindErrorNode,
				NodeSpan: ast.SpanFrom(astPos(cur.Pos), astPos(cur.EndPos)),
			},
			Message: "expected type specifier",
		}
	}
}

// parseArrayType parses ARRAY[ranges] OF elementType
func (p *Parser) parseArrayType() *ast.ArrayType {
	startTok := p.advance() // consume ARRAY
	p.expect(lexer.LBracket)

	var ranges []*ast.SubrangeSpec
	ranges = append(ranges, p.parseSubrangeSpec())
	for p.match(lexer.Comma) {
		ranges = append(ranges, p.parseSubrangeSpec())
	}

	p.expect(lexer.RBracket)
	p.expect(lexer.KwOf)

	elemType := p.parseTypeSpec()

	return &ast.ArrayType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindArrayType,
			NodeSpan: spanFromTokens(startTok, p.tokens[p.pos-1]),
		},
		Ranges:      ranges,
		ElementType: elemType,
	}
}

// parseSubrangeSpec parses low..high
func (p *Parser) parseSubrangeSpec() *ast.SubrangeSpec {
	startTok := p.peek()
	low := p.parseExpr(0)
	p.expect(lexer.DotDot)
	high := p.parseExpr(0)

	return &ast.SubrangeSpec{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Low:  low,
		High: high,
	}
}

// parsePointerType parses POINTER TO baseType
func (p *Parser) parsePointerType() *ast.PointerType {
	startTok := p.advance() // consume POINTER
	p.expect(lexer.KwTo)
	baseType := p.parseTypeSpec()

	return &ast.PointerType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindPointerType,
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		BaseType: baseType,
	}
}

// parseReferenceType parses REFERENCE TO baseType
func (p *Parser) parseReferenceType() *ast.ReferenceType {
	startTok := p.advance() // consume REFERENCE
	p.expect(lexer.KwTo)
	baseType := p.parseTypeSpec()

	return &ast.ReferenceType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindReferenceType,
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		BaseType: baseType,
	}
}

// parseStringType parses STRING[(length)] or WSTRING[(length)]
func (p *Parser) parseStringType(isWide bool) *ast.StringType {
	startTok := p.advance() // consume STRING/WSTRING

	var length ast.Expr
	if p.match(lexer.LParen) {
		length = p.parseExpr(0)
		p.expect(lexer.RParen)
	}

	return &ast.StringType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindStringType,
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		IsWide: isWide,
		Length: length,
	}
}

// parseStructType parses STRUCT members END_STRUCT
func (p *Parser) parseStructType() *ast.StructType {
	startTok := p.advance() // consume STRUCT

	var members []*ast.StructMember
	for !p.atEnd() && !p.at(lexer.KwEndStruct) {
		savedPos := p.pos
		member := p.parseStructMember()
		if member != nil {
			members = append(members, member)
		}
		// Guard against infinite loops when parseStructMember makes no progress.
		if p.pos == savedPos {
			p.advance()
		}
	}

	endTok := p.expect(lexer.KwEndStruct)

	return &ast.StructType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindStructType,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Members: members,
	}
}

// parseStructMember parses name : type [:= init] ;
func (p *Parser) parseStructMember() *ast.StructMember {
	startTok := p.peek()
	name := p.parseIdent()
	p.expect(lexer.Colon)
	typeSpec := p.parseTypeSpec()

	var initValue ast.Expr
	if p.match(lexer.Assign) {
		initValue = p.parseExpr(0)
	}

	endTok := p.peek()
	p.expect(lexer.Semicolon)

	return &ast.StructMember{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Name:      name,
		Type:      typeSpec,
		InitValue: initValue,
	}
}

// parseEnumType parses (Val1, Val2 := expr, ...)
func (p *Parser) parseEnumType() *ast.EnumType {
	startTok := p.advance() // consume (

	var values []*ast.EnumValue
	for !p.atEnd() && !p.at(lexer.RParen) {
		ev := p.parseEnumValue()
		values = append(values, ev)
		if !p.match(lexer.Comma) {
			break
		}
	}

	endTok := p.expect(lexer.RParen)

	return &ast.EnumType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindEnumType,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Values: values,
	}
}

// parseEnumValue parses Name [:= expr]
func (p *Parser) parseEnumValue() *ast.EnumValue {
	startTok := p.peek()
	name := p.parseIdent()

	var value ast.Expr
	if p.match(lexer.Assign) {
		value = p.parseExpr(0)
	}

	return &ast.EnumValue{
		NodeBase: ast.NodeBase{
			NodeSpan: spanFromTokens(startTok, p.tokens[maxInt(p.pos-1, 0)]),
		},
		Name:  name,
		Value: value,
	}
}

// parseNamedTypeOrSubrange parses an Ident type, possibly followed by (low..high) for subrange.
func (p *Parser) parseNamedTypeOrSubrange() ast.TypeSpec {
	tok := p.advance() // consume Ident
	ident := makeIdent(tok)

	namedType := &ast.NamedType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindNamedType,
			NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
		},
		Name: ident,
	}

	// Check for subrange: Ident(low..high)
	if p.at(lexer.LParen) {
		return p.tryParseSubrange(namedType)
	}

	return namedType
}

// parsePrimitiveTypeOrSubrange parses a primitive type keyword, possibly followed by (low..high).
func (p *Parser) parsePrimitiveTypeOrSubrange(name string) ast.TypeSpec {
	tok := p.advance()
	ident := &ast.Ident{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindIdent,
			NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
		},
		Name: name,
	}

	namedType := &ast.NamedType{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindNamedType,
			NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
		},
		Name: ident,
	}

	// Check for subrange: INT(0..100)
	if p.at(lexer.LParen) {
		return p.tryParseSubrange(namedType)
	}

	return namedType
}

// tryParseSubrange checks if LParen starts a subrange (low..high) or not.
func (p *Parser) tryParseSubrange(baseType *ast.NamedType) ast.TypeSpec {
	// Save position for backtracking
	saved := p.pos
	p.advance() // consume (

	// Try to parse as subrange: expr .. expr )
	low := p.parseExpr(0)
	if p.at(lexer.DotDot) {
		p.advance() // consume ..
		high := p.parseExpr(0)
		endTok := p.expect(lexer.RParen)

		return &ast.SubrangeType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindSubrangeType,
				NodeSpan: spanFromTokens(p.tokens[saved], endTok),
			},
			BaseType: baseType,
			Low:      low,
			High:     high,
		}
	}

	// Not a subrange — backtrack
	p.pos = saved
	return baseType
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
