// Package ast defines the concrete syntax tree node types for IEC 61131-3
// Structured Text with CODESYS OOP extensions. Every node carries trivia
// (whitespace, comments) for CST fidelity, enabling lossless round-tripping.
package ast

// NodeKind identifies the type of an AST/CST node.
type NodeKind int

const (
	// File
	KindSourceFile NodeKind = iota

	// Declarations
	KindProgramDecl
	KindFunctionBlockDecl
	KindFunctionDecl
	KindInterfaceDecl
	KindMethodDecl
	KindPropertyDecl
	KindTypeDecl
	KindActionDecl

	// Statements
	KindAssignStmt
	KindCallStmt
	KindIfStmt
	KindCaseStmt
	KindForStmt
	KindWhileStmt
	KindRepeatStmt
	KindReturnStmt
	KindExitStmt
	KindContinueStmt
	KindEmptyStmt
	KindErrorNode

	// Expressions
	KindBinaryExpr
	KindUnaryExpr
	KindLiteral
	KindIdent
	KindCallExpr
	KindMemberAccessExpr
	KindIndexExpr
	KindDerefExpr
	KindParenExpr

	// Types
	KindNamedType
	KindArrayType
	KindPointerType
	KindReferenceType
	KindStringType
	KindSubrangeType
	KindEnumType
	KindStructType

	// Var
	KindVarBlock
	KindVarDecl
)

var nodeKindNames = [...]string{
	KindSourceFile:        "SourceFile",
	KindProgramDecl:       "ProgramDecl",
	KindFunctionBlockDecl: "FunctionBlockDecl",
	KindFunctionDecl:      "FunctionDecl",
	KindInterfaceDecl:     "InterfaceDecl",
	KindMethodDecl:        "MethodDecl",
	KindPropertyDecl:      "PropertyDecl",
	KindTypeDecl:          "TypeDecl",
	KindActionDecl:        "ActionDecl",
	KindAssignStmt:        "AssignStmt",
	KindCallStmt:          "CallStmt",
	KindIfStmt:            "IfStmt",
	KindCaseStmt:          "CaseStmt",
	KindForStmt:           "ForStmt",
	KindWhileStmt:         "WhileStmt",
	KindRepeatStmt:        "RepeatStmt",
	KindReturnStmt:        "ReturnStmt",
	KindExitStmt:          "ExitStmt",
	KindContinueStmt:      "ContinueStmt",
	KindEmptyStmt:         "EmptyStmt",
	KindErrorNode:         "ErrorNode",
	KindBinaryExpr:        "BinaryExpr",
	KindUnaryExpr:         "UnaryExpr",
	KindLiteral:           "Literal",
	KindIdent:             "Ident",
	KindCallExpr:          "CallExpr",
	KindMemberAccessExpr:  "MemberAccessExpr",
	KindIndexExpr:         "IndexExpr",
	KindDerefExpr:         "DerefExpr",
	KindParenExpr:         "ParenExpr",
	KindNamedType:         "NamedType",
	KindArrayType:         "ArrayType",
	KindPointerType:       "PointerType",
	KindReferenceType:     "ReferenceType",
	KindStringType:        "StringType",
	KindSubrangeType:      "SubrangeType",
	KindEnumType:          "EnumType",
	KindStructType:        "StructType",
	KindVarBlock:          "VarBlock",
	KindVarDecl:           "VarDecl",
}

// String returns the human-readable name of a NodeKind.
func (k NodeKind) String() string {
	if int(k) < len(nodeKindNames) {
		return nodeKindNames[k]
	}
	return "Unknown"
}

// Pos represents a position in a source file.
type Pos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
}

// Span represents a range in a source file from Start to End.
type Span struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}

// SpanFrom creates a Span from start and end positions.
func SpanFrom(start, end Pos) Span {
	return Span{Start: start, End: end}
}

// Node is the interface implemented by all AST/CST nodes.
type Node interface {
	Kind() NodeKind
	Span() Span
	Children() []Node
}

// NodeBase is the common base embedded in all concrete node types.
// It carries the node kind, source span, and attached trivia.
type NodeBase struct {
	NodeKind       NodeKind `json:"kind"`
	NodeSpan       Span     `json:"span"`
	LeadingTrivia  []Trivia `json:"leading_trivia,omitempty"`
	TrailingTrivia []Trivia `json:"trailing_trivia,omitempty"`
}

// Kind returns the NodeKind of this node.
func (n *NodeBase) Kind() NodeKind { return n.NodeKind }

// Span returns the source span of this node.
func (n *NodeBase) Span() Span { return n.NodeSpan }

// Declaration is a marker interface for declaration nodes.
type Declaration interface {
	Node
	declNode()
}

// Statement is a marker interface for statement nodes.
type Statement interface {
	Node
	stmtNode()
}

// Expr is a marker interface for expression nodes.
type Expr interface {
	Node
	exprNode()
}

// TypeSpec is a marker interface for type specifier nodes.
type TypeSpec interface {
	Node
	typeSpecNode()
}

// ErrorNode represents a parse error with partial recovery.
type ErrorNode struct {
	NodeBase
	Message string `json:"message"`
}

// Children returns an empty slice (error nodes have no children).
func (n *ErrorNode) Children() []Node { return nil }

// ErrorNode satisfies Declaration, Statement, and Expr interfaces for error recovery.
func (n *ErrorNode) declNode()     {}
func (n *ErrorNode) stmtNode()     {}
func (n *ErrorNode) exprNode()     {}
func (n *ErrorNode) typeSpecNode() {}

// Ident represents an identifier reference.
type Ident struct {
	NodeBase
	Name string `json:"name"`
}

// Children returns an empty slice (identifiers are leaf nodes).
func (n *Ident) Children() []Node { return nil }

func (n *Ident) exprNode() {}
