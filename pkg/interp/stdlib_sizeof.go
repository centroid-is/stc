package interp

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// Minimal runtime SIZEOF (Phase 26-04). Phase 23-01 adds SIZEOF to the
// checker and interpreter as well; reconcile on merge.
//
// Widths follow the packed layout of the Tc2_EtherCAT pointer codec
// (no alignment padding), so SIZEOF(buf) equals the bytes ADR(buf) exposes:
// BOOL, BYTE, SINT, USINT 1; WORD, INT, UINT 2; DWORD, DINT, UDINT, REAL,
// TIME 4; LWORD, LINT, ULINT, LREAL 8; enums their base type (INT when
// unknown); STRING the current length + 1 (declared lengths are not
// tracked on values); arrays the sum of their declared elements; structs
// the sum of their members.

// sizeofDefaultString is the size of a STRING whose declared length is
// unknown: the default STRING(80) plus its terminator.
const sizeofDefaultString = 81

// evalSizeOf evaluates SIZEOF(x) to a UDINT byte count.
func (interp *Interpreter) evalSizeOf(env *Env, e *ast.CallExpr) (Value, error) {
	if len(e.Args) != 1 || len(e.NamedArgs) != 0 {
		return Value{}, &RuntimeError{Msg: "SIZEOF requires exactly 1 argument", Pos: e.Span().Start}
	}
	v, err := interp.evalExpr(env, e.Args[0])
	if err != nil {
		return Value{}, err
	}
	n, err := sizeOfValue(v)
	if err != nil {
		return Value{}, &RuntimeError{Msg: "SIZEOF: " + err.Error(), Pos: e.Span().Start}
	}
	return Value{Kind: ValInt, Int: int64(n), IECType: types.KindUDINT}, nil
}

// sizeOfValue is the packed byte size of v.
func sizeOfValue(v Value) (int, error) {
	switch v.Kind {
	case ValBool, ValInt, ValReal, ValTime:
		n, _ := scalarBytes(v)
		return n, nil
	case ValString:
		n := len(v.Str) + 1
		if n < sizeofDefaultString {
			n = sizeofDefaultString
		}
		return n, nil
	case ValArray:
		total := 0
		for i := v.ArrayLow; i < len(v.Array); i++ {
			n, err := sizeOfValue(v.Array[i])
			if err != nil {
				return 0, err
			}
			total += n
		}
		return total, nil
	case ValStruct:
		total := 0
		for _, m := range v.Struct {
			n, err := sizeOfValue(m)
			if err != nil {
				return 0, err
			}
			total += n
		}
		return total, nil
	}
	return 0, fmt.Errorf("size of a %s is unknown", v.Kind)
}
