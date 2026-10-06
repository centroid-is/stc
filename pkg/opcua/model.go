package opcua

import (
	"errors"
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// Kind classifies a SymbolNode.
type Kind int

const (
	KindRoot Kind = iota // synthetic root; Children() are GVLs and PROGRAMs
	KindGVL
	KindProgram
	KindFBInstance
	KindStruct
	KindArray
	KindScalar
	KindEnum
	KindReference
	KindPointer
)

var kindNames = [...]string{
	KindRoot:       "Root",
	KindGVL:        "GVL",
	KindProgram:    "Program",
	KindFBInstance: "FBInstance",
	KindStruct:     "Struct",
	KindArray:      "Array",
	KindScalar:     "Scalar",
	KindEnum:       "Enum",
	KindReference:  "Reference",
	KindPointer:    "Pointer",
}

// String returns the kind name, or "Kind(n)" for unknown values.
func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// SymbolNode is one symbol of the running project, independent of pkg/symtree.
type SymbolNode interface {
	Path() string                  // "GVL_BatchLines.Drives_Line1[1].HMI", declared case; "" for KindRoot
	Name() string                  // BrowseName text: "HMI"; array elements "Drives_Line1[1]"
	Kind() Kind
	Type() types.Type              // IEC type; nil for KindRoot, KindGVL, KindProgram
	TypeName() string              // declared type name, e.g. "ST_Drive_HMI", "E_State", "INT"
	Attributes() []ast.Attribute   // type-level first (TYPE/FB header, member decl inside the type), then the instance declaration; the LAST occurrence of a name wins
	EnumStrings() map[int64]string // ordinal -> name for enums, else nil
	Children() []SymbolNode        // declaration order; arrays: elements Low..High
}

// NodeSource is the live value seam. Values use the canonical IEC Go forms:
// BOOL bool; integers any Go integer kind; REAL float32/float64; LREAL float64;
// STRING/WSTRING string; TIME and TOD time.Duration; DATE/DT time.Time;
// enums an integer ordinal or the value name string; arrays []any or a typed slice.
// Struct values are never passed whole: the server composes them from member paths.
type NodeSource interface {
	Read(path string) (any, error)
	Write(path string, v any) error                  // queued until the next cycle boundary by real runtimes
	Snapshot(paths []string) (map[string]any, error) // consistent multi-path read from one scan image
}

// Sentinel errors a NodeSource returns (wrapped) so the server can map them
// to OPC UA status codes.
var (
	ErrUnknownSymbol = errors.New("opcua: unknown symbol")
	ErrNotWritable   = errors.New("opcua: symbol not writable")
	ErrTypeMismatch  = errors.New("opcua: type mismatch")
	ErrOutOfRange    = errors.New("opcua: value out of range")
)
