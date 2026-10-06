package opcua

import (
	"reflect"
	"testing"
	"time"

	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

func testEnum() *types.EnumType {
	return &types.EnumType{Name: "E_State", BaseType: types.KindINT,
		Values: []string{"Idle", "rdy", "Run"}, Ordinals: []int64{0, 2, 3}}
}

func arr(elem types.Type, dims ...[2]int) *types.ArrayType {
	a := &types.ArrayType{ElementType: elem}
	for _, d := range dims {
		a.Dimensions = append(a.Dimensions, types.ArrayDimension{Low: d[0], High: d[1], Known: true})
	}
	return a
}

func TestUADataType(t *testing.T) {
	tests := []struct {
		typ  types.Type
		want ua.NodeID
	}{
		{types.TypeBOOL, ua.DataTypeIDBoolean},
		{types.TypeSINT, ua.DataTypeIDSByte},
		{types.TypeUSINT, ua.DataTypeIDByte},
		{types.TypeBYTE, ua.DataTypeIDByte},
		{types.TypeCHAR, ua.DataTypeIDByte},
		{types.TypeINT, ua.DataTypeIDInt16},
		{types.TypeUINT, ua.DataTypeIDUInt16},
		{types.TypeWORD, ua.DataTypeIDUInt16},
		{types.TypeWCHAR, ua.DataTypeIDUInt16},
		{types.TypeDINT, ua.DataTypeIDInt32},
		{types.TypeUDINT, ua.DataTypeIDUInt32},
		{types.TypeDWORD, ua.DataTypeIDUInt32},
		{types.TypeLINT, ua.DataTypeIDInt64},
		{types.TypeULINT, ua.DataTypeIDUInt64},
		{types.TypeLWORD, ua.DataTypeIDUInt64},
		{types.TypeREAL, ua.DataTypeIDFloat},
		{types.TypeLREAL, ua.DataTypeIDDouble},
		{types.TypeSTRING, ua.DataTypeIDString},
		{types.TypeWSTRING, ua.DataTypeIDString},
		{types.TypeTIME, ua.DataTypeIDInt64},
		{types.TypeDATE, ua.DataTypeIDDateTime},
		{types.TypeDT, ua.DataTypeIDDateTime},
		{types.TypeTOD, ua.DataTypeIDUInt32},
		{testEnum(), ua.DataTypeIDInt32},
		{arr(types.TypeINT, [2]int{0, 4}), ua.DataTypeIDInt16},
	}
	for _, tc := range tests {
		got, ok := uaDataType(tc.typ)
		if !ok || got != tc.want {
			t.Errorf("uaDataType(%s) = %v, %v; want %v", tc.typ, got, ok, tc.want)
		}
	}
	for _, bad := range []types.Type{
		&types.PointerType{BaseType: types.TypeINT},
		&types.ReferenceType{BaseType: types.TypeINT},
		&types.FunctionType{Name: "F"},
		&types.FunctionBlockType{Name: "FB"},
		&types.StructType{Name: "ST"},
		types.TypeVOID,
		nil,
	} {
		if _, ok := uaDataType(bad); ok {
			t.Errorf("uaDataType(%v) ok, want false", bad)
		}
	}
}

func TestUAGoType(t *testing.T) {
	tests := []struct {
		typ  types.Type
		want any
	}{
		{types.TypeBOOL, false},
		{types.TypeSINT, int8(0)},
		{types.TypeUSINT, uint8(0)},
		{types.TypeBYTE, uint8(0)},
		{types.TypeCHAR, uint8(0)},
		{types.TypeINT, int16(0)},
		{types.TypeUINT, uint16(0)},
		{types.TypeWORD, uint16(0)},
		{types.TypeWCHAR, uint16(0)},
		{types.TypeDINT, int32(0)},
		{types.TypeUDINT, uint32(0)},
		{types.TypeDWORD, uint32(0)},
		{types.TypeLINT, int64(0)},
		{types.TypeULINT, uint64(0)},
		{types.TypeLWORD, uint64(0)},
		{types.TypeREAL, float32(0)},
		{types.TypeLREAL, float64(0)},
		{types.TypeSTRING, ""},
		{types.TypeWSTRING, ""},
		{types.TypeTIME, int64(0)},
		{types.TypeTOD, uint32(0)},
		{types.TypeDATE, time.Time{}},
		{types.TypeDT, time.Time{}},
		{testEnum(), int32(0)},
		{arr(types.TypeREAL, [2]int{1, 3}), []float32(nil)},
	}
	for _, tc := range tests {
		got, ok := uaGoType(tc.typ)
		if !ok || got != reflect.TypeOf(tc.want) {
			t.Errorf("uaGoType(%s) = %v, %v; want %T", tc.typ, got, ok, tc.want)
		}
	}
	for _, bad := range []types.Type{
		&types.PointerType{BaseType: types.TypeINT},
		arr(&types.PointerType{BaseType: types.TypeINT}, [2]int{0, 1}),
		&types.StructType{Name: "ST"},
		nil,
	} {
		if _, ok := uaGoType(bad); ok {
			t.Errorf("uaGoType(%v) ok, want false", bad)
		}
	}
}

func TestArrayShape(t *testing.T) {
	rank, dims, err := arrayShape(arr(types.TypeINT, [2]int{0, 4}))
	if err != nil || rank != 1 || !reflect.DeepEqual(dims, []uint32{5}) {
		t.Errorf("1-D: %d %v %v", rank, dims, err)
	}
	rank, dims, err = arrayShape(arr(types.TypeINT, [2]int{1, 3}, [2]int{-1, 2}))
	if err != nil || rank != 2 || !reflect.DeepEqual(dims, []uint32{3, 4}) {
		t.Errorf("2-D: %d %v %v", rank, dims, err)
	}
	unknown := &types.ArrayType{ElementType: types.TypeINT,
		Dimensions: []types.ArrayDimension{{Text: "1..GVL.N"}}}
	if _, _, err := arrayShape(unknown); err == nil {
		t.Error("unknown bound: want error")
	}
	if _, _, err := arrayShape(arr(types.TypeINT, [2]int{3, 1})); err == nil {
		t.Error("inverted bounds: want error")
	}
	if _, _, err := arrayShape(&types.ArrayType{ElementType: types.TypeINT}); err == nil {
		t.Error("no dimensions: want error")
	}
	if n, err := arrayLen(arr(types.TypeINT, [2]int{1, 3}, [2]int{0, 1})); err != nil || n != 6 {
		t.Errorf("arrayLen = %d, %v", n, err)
	}
}
