// Package parser implements a recursive descent parser with Pratt expression
// parsing for IEC 61131-3 Structured Text with CODESYS OOP extensions.
// It produces a concrete syntax tree that preserves trivia for lossless
// round-tripping and supports error recovery for LSP partial ASTs.
package parser

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lexer"
	"github.com/centroid-is/stc/pkg/source"
)

// ParseResult holds the AST and diagnostics from a parse operation.
// The parser always returns both — File is non-nil even when errors occur.
type ParseResult struct {
	File  *ast.SourceFile
	Diags []diag.Diagnostic
}

// Parse tokenizes the source and produces an AST with diagnostics.
// It always returns a non-nil File, even for invalid source (error recovery).
func Parse(filename, src string) ParseResult {
	allTokens := lexer.Tokenize(filename, src)

	p := &Parser{
		allTokens: allTokens,
		filename:  filename,
		source:    src,
		diags:     diag.NewCollector(),
	}

	// Separate trivia from non-trivia tokens
	for _, tok := range allTokens {
		if tok.Kind.IsTrivia() {
			p.trivia = append(p.trivia, tok)
		} else {
			p.tokens = append(p.tokens, tok)
		}
	}

	file := p.parseSourceFile()
	return ParseResult{
		File:  file,
		Diags: p.diags.All(),
	}
}

// Parser holds the state for recursive descent parsing.
type Parser struct {
	allTokens []lexer.Token // all tokens including trivia
	tokens    []lexer.Token // non-trivia tokens only
	trivia    []lexer.Token // trivia tokens for attachment
	pos       int           // current position in tokens
	filename  string
	source    string
	diags     *diag.Collector
}

// peek returns the current token without consuming it.
func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.tokens[p.pos]
}

// advance consumes and returns the current token.
func (p *Parser) advance() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

// expect checks that the current token matches kind; if so, advances.
// Otherwise adds a diagnostic and returns a zero token.
func (p *Parser) expect(kind lexer.TokenKind) lexer.Token {
	if p.at(kind) {
		return p.advance()
	}
	cur := p.peek()
	p.error("expected %s, got %s", kind.String(), cur.Kind.String())
	return lexer.Token{}
}

// match returns true and advances if the current token matches any of the given kinds.
func (p *Parser) match(kinds ...lexer.TokenKind) bool {
	for _, k := range kinds {
		if p.at(k) {
			p.advance()
			return true
		}
	}
	return false
}

// at returns true if the current token is of the given kind.
func (p *Parser) at(kind lexer.TokenKind) bool {
	return p.peek().Kind == kind
}

// atEnd returns true if the parser is at EOF.
func (p *Parser) atEnd() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Kind == lexer.EOF
}

// tokenPos converts a lexer.Pos to a source.Pos.
func tokenPos(lp lexer.Pos) source.Pos {
	return source.Pos{
		File:   lp.File,
		Line:   lp.Line,
		Col:    lp.Col,
		Offset: lp.Offset,
	}
}

// astPos converts a lexer.Pos to an ast.Pos.
func astPos(lp lexer.Pos) ast.Pos {
	return ast.Pos{
		File:   lp.File,
		Line:   lp.Line,
		Col:    lp.Col,
		Offset: lp.Offset,
	}
}

// spanFromTokens creates an ast.Span from start and end lexer tokens.
func spanFromTokens(start, end lexer.Token) ast.Span {
	return ast.Span{
		Start: astPos(start.Pos),
		End:   astPos(end.EndPos),
	}
}

// makeIdent creates an Ident node from a token.
func makeIdent(tok lexer.Token) *ast.Ident {
	return &ast.Ident{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindIdent,
			NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos)),
		},
		Name: tok.Text,
	}
}

// parseIdent parses an identifier, reporting an error if not found.
func (p *Parser) parseIdent() *ast.Ident {
	if p.at(lexer.Ident) {
		tok := p.advance()
		return makeIdent(tok)
	}
	cur := p.peek()
	p.error("expected identifier, got %s", cur.Kind.String())
	// Return a synthetic ident to allow recovery
	return &ast.Ident{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindIdent,
			NodeSpan: ast.SpanFrom(astPos(cur.Pos), astPos(cur.Pos)),
		},
		Name: "<missing>",
	}
}

// parseSourceFile parses the entire source file into a SourceFile node.
func (p *Parser) parseSourceFile() *ast.SourceFile {
	startTok := p.peek()
	var decls []ast.Declaration

	for !p.atEnd() {
		decl := p.parseDeclaration()
		if decl != nil {
			decls = append(decls, decl)
		}
	}

	endTok := p.peek()
	return &ast.SourceFile{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindSourceFile,
			NodeSpan: spanFromTokens(startTok, endTok),
		},
		Declarations: decls,
	}
}
