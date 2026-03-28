package lexer

import "strings"

// keywords maps uppercased keyword strings to their TokenKind.
var keywords map[string]TokenKind

func init() {
	keywords = map[string]TokenKind{
		// POU declarations
		"PROGRAM":            KwProgram,
		"END_PROGRAM":        KwEndProgram,
		"FUNCTION_BLOCK":     KwFunctionBlock,
		"END_FUNCTION_BLOCK": KwEndFunctionBlock,
		"FUNCTION":           KwFunction,
		"END_FUNCTION":       KwEndFunction,
		"TYPE":               KwType,
		"END_TYPE":           KwEndType,
		"INTERFACE":          KwInterface,
		"END_INTERFACE":      KwEndInterface,
		"METHOD":             KwMethod,
		"END_METHOD":         KwEndMethod,
		"PROPERTY":           KwProperty,
		"END_PROPERTY":       KwEndProperty,
		"ACTION":             KwAction,
		"END_ACTION":         KwEndAction,

		// Variable sections
		"VAR":          KwVar,
		"VAR_INPUT":    KwVarInput,
		"VAR_OUTPUT":   KwVarOutput,
		"VAR_IN_OUT":   KwVarInOut,
		"VAR_TEMP":     KwVarTemp,
		"VAR_GLOBAL":   KwVarGlobal,
		"VAR_ACCESS":   KwVarAccess,
		"VAR_EXTERNAL": KwVarExternal,
		"VAR_CONFIG":   KwVarConfig,
		"END_VAR":      KwEndVar,
		"CONSTANT":     KwConstant,
		"RETAIN":       KwRetain,
		"PERSISTENT":   KwPersistent,
		"AT":           KwAt,

		// Control flow
		"IF":         KwIf,
		"THEN":       KwThen,
		"ELSIF":      KwElsif,
		"ELSE":       KwElse,
		"END_IF":     KwEndIf,
		"CASE":       KwCase,
		"OF":         KwOf,
		"END_CASE":   KwEndCase,
		"FOR":        KwFor,
		"TO":         KwTo,
		"BY":         KwBy,
		"DO":         KwDo,
		"END_FOR":    KwEndFor,
		"WHILE":      KwWhile,
		"END_WHILE":  KwEndWhile,
		"REPEAT":     KwRepeat,
		"UNTIL":      KwUntil,
		"END_REPEAT": KwEndRepeat,
		"EXIT":       KwExit,
		"CONTINUE":   KwContinue,
		"RETURN":     KwReturn,

		// OOP
		"EXTENDS":    KwExtends,
		"IMPLEMENTS": KwImplements,
		"THIS":       KwThis,
		"SUPER":      KwSuper,
		"ABSTRACT":   KwAbstract,
		"FINAL":      KwFinal,
		"OVERRIDE":   KwOverride,
		"PUBLIC":     KwPublic,
		"PRIVATE":    KwPrivate,
		"PROTECTED":  KwProtected,
		"INTERNAL":   KwInternal,

		// Type system
		"ARRAY":     KwArray,
		"STRUCT":    KwStruct,
		"END_STRUCT": KwEndStruct,
		"POINTER":   KwPointer,
		"REFERENCE": KwReference,
		"STRING":    KwString,
		"WSTRING":   KwWString,

		// Primitive types
		"BOOL":          KwBool,
		"BYTE":          KwByte,
		"WORD":          KwWord,
		"DWORD":         KwDword,
		"LWORD":         KwLword,
		"SINT":          KwSint,
		"INT":           KwInt,
		"DINT":          KwDint,
		"LINT":          KwLint,
		"USINT":         KwUsint,
		"UINT":          KwUint,
		"UDINT":         KwUdint,
		"ULINT":         KwUlint,
		"REAL":          KwReal,
		"LREAL":         KwLreal,
		"TIME":          KwTime,
		"DATE":          KwDate,
		"TIME_OF_DAY":   KwTimeOfDay,
		"TOD":           KwTod,
		"DATE_AND_TIME": KwDateAndTime,
		"DT":            KwDt,

		// Boolean/logical
		"TRUE":  KwTrue,
		"FALSE": KwFalse,
		"AND":   KwAnd,
		"OR":    KwOr,
		"XOR":   KwXor,
		"NOT":   KwNot,
		"MOD":   KwMod,

		// Testing
		"TEST_CASE":     KwTestCase,
		"END_TEST_CASE": KwEndTestCase,
	}
}

// LookupKeyword checks if ident is a keyword (case-insensitive).
// Returns the keyword TokenKind and true if found, or Ident and false if not.
func LookupKeyword(ident string) (TokenKind, bool) {
	if kind, ok := keywords[strings.ToUpper(ident)]; ok {
		return kind, true
	}
	return Ident, false
}
