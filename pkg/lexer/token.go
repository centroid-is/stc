// Package lexer implements a hand-written scanner for IEC 61131-3
// Structured Text with CODESYS OOP extensions. It produces a token stream
// with full position tracking and trivia preservation for CST fidelity.
package lexer

import "fmt"

// TokenKind identifies the type of a lexer token.
type TokenKind int

const (
	// Special tokens
	Illegal TokenKind = iota
	EOF

	// Identifiers and literals
	Ident
	IntLiteral
	RealLiteral
	StringLiteral
	WStringLiteral
	TimeLiteral
	DateLiteral
	DateTimeLiteral
	TodLiteral
	TypedLiteral

	// Punctuation
	LParen    // (
	RParen    // )
	LBracket  // [
	RBracket  // ]
	Comma     // ,
	Semicolon // ;
	Colon     // :
	Dot       // .
	DotDot    // ..
	Caret     // ^
	Hash      // #
	Arrow     // =>

	// Operators
	Assign    // :=
	Plus      // +
	Minus     // -
	Star      // *
	Slash     // /
	Power     // **
	Eq        // =
	NotEq     // <>
	Less      // <
	LessEq    // <=
	Greater   // >
	GreaterEq // >=
	Ampersand // &

	// Trivia tokens
	Whitespace
	LineComment
	BlockComment
	Pragma

	// --- Keywords ---

	// POU keywords
	KwProgram
	KwEndProgram
	KwFunctionBlock
	KwEndFunctionBlock
	KwFunction
	KwEndFunction
	KwType
	KwEndType
	KwInterface
	KwEndInterface
	KwMethod
	KwEndMethod
	KwProperty
	KwEndProperty
	KwAction
	KwEndAction

	// Variable keywords
	KwVar
	KwVarInput
	KwVarOutput
	KwVarInOut
	KwVarTemp
	KwVarGlobal
	KwVarAccess
	KwVarExternal
	KwVarConfig
	KwEndVar
	KwConstant
	KwRetain
	KwPersistent
	KwAt

	// Control flow keywords
	KwIf
	KwThen
	KwElsif
	KwElse
	KwEndIf
	KwCase
	KwOf
	KwEndCase
	KwFor
	KwTo
	KwBy
	KwDo
	KwEndFor
	KwWhile
	KwEndWhile
	KwRepeat
	KwUntil
	KwEndRepeat
	KwExit
	KwContinue
	KwReturn

	// OOP keywords
	KwExtends
	KwImplements
	KwThis
	KwSuper
	KwAbstract
	KwFinal
	KwOverride
	KwPublic
	KwPrivate
	KwProtected
	KwInternal

	// Type keywords
	KwArray
	KwStruct
	KwEndStruct
	KwPointer
	KwReference
	KwString
	KwWString

	// Primitive type keywords
	KwBool
	KwByte
	KwWord
	KwDword
	KwLword
	KwSint
	KwInt
	KwDint
	KwLint
	KwUsint
	KwUint
	KwUdint
	KwUlint
	KwReal
	KwLreal
	KwTime
	KwDate
	KwTimeOfDay
	KwTod
	KwDateAndTime
	KwDt

	// Boolean and logical keywords
	KwTrue
	KwFalse
	KwAnd
	KwOr
	KwXor
	KwNot
	KwMod

	// Testing keywords
	KwTestCase
	KwEndTestCase
)

var tokenKindNames = [...]string{
	Illegal:        "Illegal",
	EOF:            "EOF",
	Ident:          "Ident",
	IntLiteral:     "IntLiteral",
	RealLiteral:    "RealLiteral",
	StringLiteral:  "StringLiteral",
	WStringLiteral: "WStringLiteral",
	TimeLiteral:    "TimeLiteral",
	DateLiteral:    "DateLiteral",
	DateTimeLiteral: "DateTimeLiteral",
	TodLiteral:     "TodLiteral",
	TypedLiteral:   "TypedLiteral",
	LParen:         "LParen",
	RParen:         "RParen",
	LBracket:       "LBracket",
	RBracket:       "RBracket",
	Comma:          "Comma",
	Semicolon:      "Semicolon",
	Colon:          "Colon",
	Dot:            "Dot",
	DotDot:         "DotDot",
	Caret:          "Caret",
	Hash:           "Hash",
	Arrow:          "Arrow",
	Assign:         "Assign",
	Plus:           "Plus",
	Minus:          "Minus",
	Star:           "Star",
	Slash:          "Slash",
	Power:          "Power",
	Eq:             "Eq",
	NotEq:          "NotEq",
	Less:           "Less",
	LessEq:         "LessEq",
	Greater:        "Greater",
	GreaterEq:      "GreaterEq",
	Ampersand:      "Ampersand",
	Whitespace:     "Whitespace",
	LineComment:    "LineComment",
	BlockComment:   "BlockComment",
	Pragma:         "Pragma",

	KwProgram:          "KwProgram",
	KwEndProgram:       "KwEndProgram",
	KwFunctionBlock:    "KwFunctionBlock",
	KwEndFunctionBlock: "KwEndFunctionBlock",
	KwFunction:         "KwFunction",
	KwEndFunction:      "KwEndFunction",
	KwType:             "KwType",
	KwEndType:          "KwEndType",
	KwInterface:        "KwInterface",
	KwEndInterface:     "KwEndInterface",
	KwMethod:           "KwMethod",
	KwEndMethod:        "KwEndMethod",
	KwProperty:         "KwProperty",
	KwEndProperty:      "KwEndProperty",
	KwAction:           "KwAction",
	KwEndAction:        "KwEndAction",

	KwVar:         "KwVar",
	KwVarInput:    "KwVarInput",
	KwVarOutput:   "KwVarOutput",
	KwVarInOut:    "KwVarInOut",
	KwVarTemp:     "KwVarTemp",
	KwVarGlobal:   "KwVarGlobal",
	KwVarAccess:   "KwVarAccess",
	KwVarExternal: "KwVarExternal",
	KwVarConfig:   "KwVarConfig",
	KwEndVar:      "KwEndVar",
	KwConstant:    "KwConstant",
	KwRetain:      "KwRetain",
	KwPersistent:  "KwPersistent",
	KwAt:          "KwAt",

	KwIf:        "KwIf",
	KwThen:      "KwThen",
	KwElsif:     "KwElsif",
	KwElse:      "KwElse",
	KwEndIf:     "KwEndIf",
	KwCase:      "KwCase",
	KwOf:        "KwOf",
	KwEndCase:   "KwEndCase",
	KwFor:       "KwFor",
	KwTo:        "KwTo",
	KwBy:        "KwBy",
	KwDo:        "KwDo",
	KwEndFor:    "KwEndFor",
	KwWhile:     "KwWhile",
	KwEndWhile:  "KwEndWhile",
	KwRepeat:    "KwRepeat",
	KwUntil:     "KwUntil",
	KwEndRepeat: "KwEndRepeat",
	KwExit:      "KwExit",
	KwContinue:  "KwContinue",
	KwReturn:    "KwReturn",

	KwExtends:    "KwExtends",
	KwImplements: "KwImplements",
	KwThis:       "KwThis",
	KwSuper:      "KwSuper",
	KwAbstract:   "KwAbstract",
	KwFinal:      "KwFinal",
	KwOverride:   "KwOverride",
	KwPublic:     "KwPublic",
	KwPrivate:    "KwPrivate",
	KwProtected:  "KwProtected",
	KwInternal:   "KwInternal",

	KwArray:     "KwArray",
	KwStruct:    "KwStruct",
	KwEndStruct: "KwEndStruct",
	KwPointer:   "KwPointer",
	KwReference: "KwReference",
	KwString:    "KwString",
	KwWString:   "KwWString",

	KwBool:       "KwBool",
	KwByte:       "KwByte",
	KwWord:       "KwWord",
	KwDword:      "KwDword",
	KwLword:      "KwLword",
	KwSint:       "KwSint",
	KwInt:        "KwInt",
	KwDint:       "KwDint",
	KwLint:       "KwLint",
	KwUsint:      "KwUsint",
	KwUint:       "KwUint",
	KwUdint:      "KwUdint",
	KwUlint:      "KwUlint",
	KwReal:       "KwReal",
	KwLreal:      "KwLreal",
	KwTime:       "KwTime",
	KwDate:       "KwDate",
	KwTimeOfDay:  "KwTimeOfDay",
	KwTod:        "KwTod",
	KwDateAndTime: "KwDateAndTime",
	KwDt:         "KwDt",

	KwTrue:  "KwTrue",
	KwFalse: "KwFalse",
	KwAnd:   "KwAnd",
	KwOr:    "KwOr",
	KwXor:   "KwXor",
	KwNot:   "KwNot",
	KwMod:   "KwMod",

	KwTestCase:    "KwTestCase",
	KwEndTestCase: "KwEndTestCase",
}

// String returns the human-readable name of a TokenKind.
func (k TokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// firstKeyword and lastKeyword bracket the keyword token range.
const (
	firstKeyword = KwProgram
	lastKeyword  = KwEndTestCase
)

// IsKeyword returns true if this token kind is a keyword.
func (k TokenKind) IsKeyword() bool {
	return k >= firstKeyword && k <= lastKeyword
}

// IsOperator returns true if this token kind is an operator.
func (k TokenKind) IsOperator() bool {
	switch k {
	case Assign, Plus, Minus, Star, Slash, Power,
		Eq, NotEq, Less, LessEq, Greater, GreaterEq,
		Ampersand, KwAnd, KwOr, KwXor, KwNot, KwMod:
		return true
	}
	return false
}

// IsTrivia returns true if this token kind is trivia (whitespace or comment).
func (k TokenKind) IsTrivia() bool {
	return k == Whitespace || k == LineComment || k == BlockComment
}

// Pos represents a position in a source file.
type Pos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
}

// Token represents a single lexer token with its kind, text, and position.
type Token struct {
	Kind   TokenKind `json:"kind"`
	Text   string    `json:"text"`
	Pos    Pos       `json:"pos"`
	EndPos Pos       `json:"end_pos"`
}
