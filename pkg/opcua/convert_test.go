package opcua

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

func TestToUA(t *testing.T) {
	dt := time.Date(2026, 10, 6, 12, 30, 0, 0, time.FixedZone("X", 3600))
	tests := []struct {
		name string
		typ  types.Type
		in   any
		want any
	}{
		{"bool", types.TypeBOOL, true, true},
		{"sint", types.TypeSINT, int64(-5), int8(-5)},
		{"usint", types.TypeUSINT, 200, uint8(200)},
		{"byte from uint", types.TypeBYTE, uint64(255), uint8(255)},
		{"char int", types.TypeCHAR, 65, uint8(65)},
		{"char string", types.TypeCHAR, "A", uint8(65)},
		{"int", types.TypeINT, 300, int16(300)},
		{"uint", types.TypeUINT, uint16(65535), uint16(65535)},
		{"word", types.TypeWORD, int32(7), uint16(7)},
		{"wchar", types.TypeWCHAR, 0x263A, uint16(0x263A)},
		{"wchar string", types.TypeWCHAR, "☺", uint16(0x263A)},
		{"dint", types.TypeDINT, int64(-70000), int32(-70000)},
		{"udint", types.TypeUDINT, uint(70000), uint32(70000)},
		{"dword", types.TypeDWORD, int8(1), uint32(1)},
		{"lint", types.TypeLINT, int64(math.MinInt64), int64(math.MinInt64)},
		{"ulint", types.TypeULINT, uint64(math.MaxUint64), uint64(math.MaxUint64)},
		{"lword", types.TypeLWORD, 5, uint64(5)},
		{"real f64", types.TypeREAL, 1.5, float32(1.5)},
		{"real f32", types.TypeREAL, float32(2.5), float32(2.5)},
		{"real int", types.TypeREAL, 3, float32(3)},
		{"lreal", types.TypeLREAL, float32(0.5), float64(0.5)},
		{"lreal uint", types.TypeLREAL, uint8(4), float64(4)},
		{"string", types.TypeSTRING, "hi", "hi"},
		{"wstring", types.TypeWSTRING, "hæ", "hæ"},
		{"time", types.TypeTIME, 1500 * time.Millisecond, int64(1500)},
		{"time int ms", types.TypeTIME, 42, int64(42)},
		{"tod", types.TypeTOD, time.Hour, uint32(3600000)},
		{"tod int ms", types.TypeTOD, uint32(5), uint32(5)},
		{"dt", types.TypeDT, dt, dt.UTC()},
		{"date", types.TypeDATE, dt, dt.UTC()},
		{"enum name", testEnum(), "rdy", int32(2)},
		{"enum name fold", testEnum(), "RUN", int32(3)},
		{"enum ordinal", testEnum(), int64(3), int32(3)},
		{"array any", arr(types.TypeINT, [2]int{1, 3}), []any{1, 2, 3}, []int16{1, 2, 3}},
		{"array typed", arr(types.TypeREAL, [2]int{0, 1}), []float64{1, 2}, []float32{1, 2}},
		{"array 2d flat", arr(types.TypeBOOL, [2]int{0, 1}, [2]int{0, 1}), []bool{true, false, false, true}, []bool{true, false, false, true}},
		{"array enum", arr(testEnum(), [2]int{0, 1}), []any{"Idle", 2}, []int32{0, 2}},
	}
	for _, tc := range tests {
		got, err := toUA(tc.typ, tc.in)
		if err != nil {
			t.Errorf("%s: toUA error %v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: toUA = %#v (%T), want %#v (%T)", tc.name, got, got, tc.want, tc.want)
		}
	}
}

func TestToUAErrors(t *testing.T) {
	tests := []struct {
		name string
		typ  types.Type
		in   any
		want error
	}{
		{"int overflow", types.TypeINT, 40000, ErrOutOfRange},
		{"int underflow", types.TypeINT, -40000, ErrOutOfRange},
		{"usint negative", types.TypeUSINT, -1, ErrOutOfRange},
		{"udint overflow", types.TypeUDINT, uint64(1 << 40), ErrOutOfRange},
		{"lint from huge uint", types.TypeLINT, uint64(math.MaxUint64), ErrOutOfRange},
		{"time too long", types.TypeTIME, 5_000_000_000 * time.Millisecond, ErrOutOfRange},
		{"time negative", types.TypeTIME, -time.Millisecond, ErrOutOfRange},
		{"tod over a day", types.TypeTOD, 25 * time.Hour, ErrOutOfRange},
		{"real overflow", types.TypeREAL, math.MaxFloat64, ErrOutOfRange},
		{"enum ordinal unknown", testEnum(), 1, ErrOutOfRange},
		{"enum name unknown", testEnum(), "nope", ErrOutOfRange},
		{"enum bool", testEnum(), true, ErrTypeMismatch},
		{"string into bool", types.TypeBOOL, "true", ErrTypeMismatch},
		{"float into int", types.TypeINT, 1.5, ErrTypeMismatch},
		{"int into string", types.TypeSTRING, 1, ErrTypeMismatch},
		{"string into real", types.TypeREAL, "1", ErrTypeMismatch},
		{"int into dt", types.TypeDT, 1, ErrTypeMismatch},
		{"string into time", types.TypeTIME, "1s", ErrTypeMismatch},
		{"string into tod", types.TypeTOD, "1s", ErrTypeMismatch},
		{"char long string", types.TypeCHAR, "AB", ErrTypeMismatch},
		{"wchar wide rune", types.TypeWCHAR, "😀", ErrOutOfRange},
		{"nil", types.TypeINT, nil, ErrTypeMismatch},
		{"array wrong length", arr(types.TypeINT, [2]int{1, 3}), []any{1, 2}, ErrTypeMismatch},
		{"array not slice", arr(types.TypeINT, [2]int{1, 3}), 1, ErrTypeMismatch},
		{"array element range", arr(types.TypeINT, [2]int{1, 1}), []any{70000}, ErrOutOfRange},
		{"array unknown dim", &types.ArrayType{ElementType: types.TypeINT, Dimensions: []types.ArrayDimension{{}}}, []any{}, ErrTypeMismatch},
		{"pointer", &types.PointerType{BaseType: types.TypeINT}, 1, ErrTypeMismatch},
		{"nil type", nil, 1, ErrTypeMismatch},
	}
	for _, tc := range tests {
		_, err := toUA(tc.typ, tc.in)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: toUA error %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestFromUA(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		typ  types.Type
		in   any
		want any
	}{
		{"bool", types.TypeBOOL, true, true},
		{"sint", types.TypeSINT, int8(-3), int64(-3)},
		{"int", types.TypeINT, int16(300), int64(300)},
		{"dint", types.TypeDINT, int32(-9), int64(-9)},
		{"lint", types.TypeLINT, int64(1 << 40), int64(1 << 40)},
		{"byte", types.TypeBYTE, uint8(9), uint64(9)},
		{"uint", types.TypeUINT, uint16(9), uint64(9)},
		{"ulint", types.TypeULINT, uint64(math.MaxUint64), uint64(math.MaxUint64)},
		{"real", types.TypeREAL, float32(1.25), float32(1.25)},
		{"lreal", types.TypeLREAL, float64(1.25), float64(1.25)},
		{"string", types.TypeSTRING, "x", "x"},
		{"time", types.TypeTIME, int64(1500), 1500 * time.Millisecond},
		{"tod", types.TypeTOD, uint32(3600000), time.Hour},
		{"dt", types.TypeDT, now, now},
		{"enum", testEnum(), int32(2), int64(2)},
		{"array", arr(types.TypeINT, [2]int{0, 2}), []int16{1, 2, 3}, []any{int64(1), int64(2), int64(3)}},
		{"array real", arr(types.TypeREAL, [2]int{0, 0}), []float32{0.5}, []any{float32(0.5)}},
	}
	for _, tc := range tests {
		got, err := fromUA(tc.typ, tc.in)
		if err != nil {
			t.Errorf("%s: fromUA error %v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: fromUA = %#v (%T), want %#v (%T)", tc.name, got, got, tc.want, tc.want)
		}
	}
}

func TestFromUAErrors(t *testing.T) {
	tests := []struct {
		name string
		typ  types.Type
		in   any
		want error
	}{
		{"enum unknown ordinal", testEnum(), int32(1), ErrOutOfRange},
		{"int wrong type", types.TypeINT, "1", ErrTypeMismatch},
		{"int range", types.TypeINT, int32(40000), ErrOutOfRange},
		{"bool wrong", types.TypeBOOL, int32(1), ErrTypeMismatch},
		{"time negative", types.TypeTIME, int64(-1), ErrOutOfRange},
		{"time wrong", types.TypeTIME, "x", ErrTypeMismatch},
		{"array length", arr(types.TypeINT, [2]int{0, 2}), []int16{1}, ErrTypeMismatch},
		{"array oversize", arr(types.TypeINT, [2]int{0, 2}), make([]int16, 1000), ErrTypeMismatch},
		{"array scalar", arr(types.TypeINT, [2]int{0, 2}), int16(1), ErrTypeMismatch},
		{"real string", types.TypeREAL, "x", ErrTypeMismatch},
		{"string int", types.TypeSTRING, 1, ErrTypeMismatch},
		{"dt int", types.TypeDT, 1, ErrTypeMismatch},
		{"pointer", &types.PointerType{BaseType: types.TypeINT}, 1, ErrTypeMismatch},
		{"nil", types.TypeINT, nil, ErrTypeMismatch},
		{"uint string", types.TypeUINT, "1", ErrTypeMismatch},
		{"uint negative", types.TypeUINT, int16(-1), ErrOutOfRange},
		{"time duration", types.TypeTIME, time.Second, ErrTypeMismatch},
		{"tod range", types.TypeTOD, uint32(dayMS), ErrOutOfRange},
		{"array element", arr(types.TypeBOOL, [2]int{0, 0}), []int32{1}, ErrTypeMismatch},
		{"enum huge", testEnum(), uint64(math.MaxUint64), ErrOutOfRange},
	}
	for _, tc := range tests {
		_, err := fromUA(tc.typ, tc.in)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: fromUA error %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestToUARoundTrip(t *testing.T) {
	for _, typ := range []types.Type{types.TypeINT, types.TypeTIME, types.TypeTOD, testEnum()} {
		var in any = int64(2)
		if typ.Kind() == types.KindTIME || typ.Kind() == types.KindTOD {
			in = 2 * time.Millisecond
		}
		u, err := toUA(typ, in)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		back, err := fromUA(typ, u)
		if err != nil || !reflect.DeepEqual(back, in) {
			t.Errorf("%s round trip: %#v -> %#v -> %#v (%v)", typ, in, u, back, err)
		}
	}
}
