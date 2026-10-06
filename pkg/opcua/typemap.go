package opcua

import (
	"fmt"
	"reflect"
	"time"

	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

// elementary describes the PLCopen OPC 30000 mapping of one IEC elementary
// kind: the OPC UA DataType and the exact Go type awcullen encodes for it.
type elementary struct {
	dt ua.NodeID
	gt reflect.Type
}

var (
	goBool    = reflect.TypeOf(false)
	goInt8    = reflect.TypeOf(int8(0))
	goUint8   = reflect.TypeOf(uint8(0))
	goInt16   = reflect.TypeOf(int16(0))
	goUint16  = reflect.TypeOf(uint16(0))
	goInt32   = reflect.TypeOf(int32(0))
	goUint32  = reflect.TypeOf(uint32(0))
	goInt64   = reflect.TypeOf(int64(0))
	goUint64  = reflect.TypeOf(uint64(0))
	goFloat32 = reflect.TypeOf(float32(0))
	goFloat64 = reflect.TypeOf(float64(0))
	goString  = reflect.TypeOf("")
	goTime    = reflect.TypeOf(time.Time{})
)

// elementaryMap is the OPCUA-06 table. TIME is Int64 milliseconds and TOD
// UInt32 milliseconds of the day (TF6100). pkg/types has no LTIME, LDATE,
// LDT or LTOD kind, so those rows have no input yet.
var elementaryMap = map[types.TypeKind]elementary{
	types.KindBOOL:    {ua.DataTypeIDBoolean, goBool},
	types.KindSINT:    {ua.DataTypeIDSByte, goInt8},
	types.KindUSINT:   {ua.DataTypeIDByte, goUint8},
	types.KindBYTE:    {ua.DataTypeIDByte, goUint8},
	types.KindCHAR:    {ua.DataTypeIDByte, goUint8},
	types.KindINT:     {ua.DataTypeIDInt16, goInt16},
	types.KindUINT:    {ua.DataTypeIDUInt16, goUint16},
	types.KindWORD:    {ua.DataTypeIDUInt16, goUint16},
	types.KindWCHAR:   {ua.DataTypeIDUInt16, goUint16},
	types.KindDINT:    {ua.DataTypeIDInt32, goInt32},
	types.KindUDINT:   {ua.DataTypeIDUInt32, goUint32},
	types.KindDWORD:   {ua.DataTypeIDUInt32, goUint32},
	types.KindLINT:    {ua.DataTypeIDInt64, goInt64},
	types.KindULINT:   {ua.DataTypeIDUInt64, goUint64},
	types.KindLWORD:   {ua.DataTypeIDUInt64, goUint64},
	types.KindREAL:    {ua.DataTypeIDFloat, goFloat32},
	types.KindLREAL:   {ua.DataTypeIDDouble, goFloat64},
	types.KindSTRING:  {ua.DataTypeIDString, goString},
	types.KindWSTRING: {ua.DataTypeIDString, goString},
	types.KindTIME:    {ua.DataTypeIDInt64, goInt64},
	types.KindDATE:    {ua.DataTypeIDDateTime, goTime},
	types.KindDT:      {ua.DataTypeIDDateTime, goTime},
	types.KindTOD:     {ua.DataTypeIDUInt32, goUint32},
}

// uaDataType returns the OPC UA DataType of an elementary, enum or array
// type (arrays report their element DataType). Enums report Int32; Publish
// substitutes the custom Enumeration DataType node. Structs, FBs, functions,
// pointers and references have no elementary mapping.
func uaDataType(t types.Type) (ua.NodeID, bool) {
	switch tt := t.(type) {
	case nil:
		return nil, false
	case *types.EnumType:
		return ua.DataTypeIDInt32, true
	case *types.ArrayType:
		return uaDataType(tt.ElementType)
	case *types.PrimitiveType:
		e, ok := elementaryMap[tt.Kind_]
		return e.dt, ok
	}
	return nil, false
}

// uaGoType returns the Go type awcullen encodes for t: the elementary Go
// type, int32 for enums, and a slice of the element type for arrays.
func uaGoType(t types.Type) (reflect.Type, bool) {
	switch tt := t.(type) {
	case *types.EnumType:
		return goInt32, true
	case *types.ArrayType:
		et, ok := uaGoType(tt.ElementType)
		if !ok {
			return nil, false
		}
		return reflect.SliceOf(et), true
	case *types.PrimitiveType:
		e, ok := elementaryMap[tt.Kind_]
		return e.gt, ok
	}
	return nil, false
}

// arrayShape returns the OPC UA ValueRank and ArrayDimensions of a: rank 1
// and [High-Low+1] for one dimension, one entry per dimension otherwise.
// Bounds that are not integer literals (Known=false) are an error.
func arrayShape(a *types.ArrayType) (int32, []uint32, error) {
	if len(a.Dimensions) == 0 {
		return 0, nil, fmt.Errorf("%s: array without dimensions", a)
	}
	dims := make([]uint32, len(a.Dimensions))
	for i, d := range a.Dimensions {
		if !d.Known {
			return 0, nil, fmt.Errorf("%s: dimension %d bounds %q are not constant", a, i+1, d.Text)
		}
		if d.High < d.Low {
			return 0, nil, fmt.Errorf("%s: dimension %d bounds %d..%d are inverted", a, i+1, d.Low, d.High)
		}
		dims[i] = uint32(d.High - d.Low + 1)
	}
	return int32(len(dims)), dims, nil
}

// maxServedSlots is the interpreter's array slot limit (direct indexing:
// High+1 slots); a longer array is truncated there, so it cannot be served.
const maxServedSlots = 10000

// arrayServable reports why the runtime cannot serve a's values yet: the
// interpreter models only the first dimension of an array and at most
// maxServedSlots slots. Such arrays would be published but never read.
func arrayServable(a *types.ArrayType) error {
	if len(a.Dimensions) > 1 {
		return fmt.Errorf("%s: multi-dimensional arrays are not served yet", a)
	}
	if len(a.Dimensions) == 1 && a.Dimensions[0].Known && a.Dimensions[0].High+1 > maxServedSlots {
		return fmt.Errorf("%s: arrays beyond index %d are not served", a, maxServedSlots-1)
	}
	return nil
}

// arrayLen returns the total element count of a (the product of all
// dimension lengths).
func arrayLen(a *types.ArrayType) (int, error) {
	_, dims, err := arrayShape(a)
	if err != nil {
		return 0, err
	}
	n := 1
	for _, d := range dims {
		n *= int(d)
	}
	return n, nil
}
