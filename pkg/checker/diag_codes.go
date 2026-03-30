// Package checker implements the two-pass type checker for IEC 61131-3
// Structured Text. Pass 1 collects declarations into the symbol table.
// Pass 2 type-checks expressions, assignments, and control flow.
package checker

const (
	// Type errors
	CodeTypeMismatch   = "SEMA001" // type mismatch in assignment/comparison
	CodeNoImplicitConv = "SEMA002" // no implicit conversion from X to Y
	CodeIncompatibleOp = "SEMA003" // operator not defined for type

	// Name resolution
	CodeUndeclared      = "SEMA010" // undeclared identifier
	CodeRedeclared      = "SEMA011" // identifier already declared in scope
	CodeUnusedVar       = "SEMA012" // declared variable never referenced
	CodeUnreachableCode = "SEMA013" // code after RETURN/EXIT

	// Structural
	CodeWrongArgCount    = "SEMA020" // wrong number of arguments
	CodeWrongArgType     = "SEMA021" // argument type mismatch
	CodeNotCallable      = "SEMA022" // identifier is not a function/FB
	CodeNotIndexable     = "SEMA023" // type does not support indexing
	CodeNoMember         = "SEMA024" // type has no member with this name
	CodeInOutRequiresVar = "SEMA025" // VAR_IN_OUT requires variable, not literal

	// I/O address validation
	CodeInvalidATAddress = "SEMA030" // invalid AT address format
	CodeATNotAllowedHere = "SEMA031" // AT address in wrong POU type
	CodeATOverlap        = "SEMA032" // overlapping AT address ranges

	// Vendor warnings
	CodeVendorOOP       = "VEND001" // OOP not supported by target vendor
	CodeVendorPointer   = "VEND002" // POINTER TO not supported
	CodeVendorReference = "VEND003" // REFERENCE TO not supported
	CodeVendorStringLen = "VEND004" // string length exceeds vendor limit
	CodeVendor64Bit     = "VEND005" // 64-bit type not supported
	CodeVendorWString   = "VEND006" // WSTRING not supported
)
