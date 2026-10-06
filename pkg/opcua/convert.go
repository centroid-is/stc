package opcua

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/centroid-is/stc/pkg/types"
)

// TF6100 publishes TIME as Int64 milliseconds limited to the IEC TIME range
// 0..4294967295 ms; TOD is milliseconds of the day.
const (
	maxTimeMS = math.MaxUint32
	dayMS     = 24 * 60 * 60 * 1000
)

var durationType = reflect.TypeOf(time.Duration(0))

func mismatch(t types.Type, v any) error {
	return fmt.Errorf("%w: %v (%T) for %s", ErrTypeMismatch, v, v, typeName(t))
}

func outOfRange(t types.Type, v any) error {
	return fmt.Errorf("%w: %v for %s", ErrOutOfRange, v, typeName(t))
}

func typeName(t types.Type) string {
	if t == nil {
		return "<nil type>"
	}
	return t.String()
}

// intOf reports v as a signed or unsigned integer. neg is true when the
// value is negative (then s holds it); otherwise u holds it.
func intOf(v any) (s int64, u uint64, neg, ok bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s = rv.Int()
		if s < 0 {
			return s, 0, true, true
		}
		return s, uint64(s), false, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u = rv.Uint()
		return int64(u), u, false, true
	}
	return 0, 0, false, false
}

// signedIn converts v to an int64 within [lo, hi].
func signedIn(t types.Type, v any, lo, hi int64) (int64, error) {
	s, u, neg, ok := intOf(v)
	if !ok {
		return 0, mismatch(t, v)
	}
	if neg {
		if s < lo {
			return 0, outOfRange(t, v)
		}
		return s, nil
	}
	if u > uint64(hi) {
		return 0, outOfRange(t, v)
	}
	return int64(u), nil
}

// unsignedIn converts v to a uint64 within [0, hi].
func unsignedIn(t types.Type, v any, hi uint64) (uint64, error) {
	_, u, neg, ok := intOf(v)
	if !ok {
		return 0, mismatch(t, v)
	}
	if neg || u > hi {
		return 0, outOfRange(t, v)
	}
	return u, nil
}

// floatOf accepts any Go float or integer kind.
func floatOf(t types.Type, v any) (float64, error) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float(), nil
	}
	s, u, neg, ok := intOf(v)
	if !ok {
		return 0, mismatch(t, v)
	}
	if neg {
		return float64(s), nil
	}
	return float64(u), nil
}

// charOf accepts an integer code or a one-character string.
func charOf(t types.Type, v any, hi uint64) (uint64, error) {
	if s, ok := v.(string); ok {
		r, n := utf8.DecodeRuneInString(s)
		if n == 0 || n != len(s) || r == utf8.RuneError {
			return 0, mismatch(t, v)
		}
		if uint64(r) > hi {
			return 0, outOfRange(t, v)
		}
		return uint64(r), nil
	}
	return unsignedIn(t, v, hi)
}

// millisOf accepts a time.Duration or an integer count of milliseconds.
func millisOf(t types.Type, v any) (int64, error) {
	if d, ok := v.(time.Duration); ok {
		return d.Milliseconds(), nil
	}
	s, u, neg, ok := intOf(v)
	if !ok {
		return 0, mismatch(t, v)
	}
	if neg {
		return s, nil
	}
	if u > math.MaxInt64 {
		return 0, outOfRange(t, v)
	}
	return int64(u), nil
}

// enumOrdinal resolves an ordinal or value name (case-sensitive first,
// then case-insensitive) to a declared ordinal of et.
func enumOrdinal(et *types.EnumType, v any) (int64, error) {
	if name, ok := v.(string); ok {
		for i, n := range et.Values {
			if n == name && i < len(et.Ordinals) {
				return et.Ordinals[i], nil
			}
		}
		for i, n := range et.Values {
			if strings.EqualFold(n, name) && i < len(et.Ordinals) {
				return et.Ordinals[i], nil
			}
		}
		return 0, outOfRange(et, v)
	}
	s, u, neg, ok := intOf(v)
	if !ok {
		return 0, mismatch(et, v)
	}
	if !neg {
		if u > math.MaxInt64 {
			return 0, outOfRange(et, v)
		}
		s = int64(u)
	}
	for _, o := range et.Ordinals {
		if o == s {
			return s, nil
		}
	}
	return 0, outOfRange(et, v)
}

// toUA converts a NodeSource value of IEC type t to the exact Go value
// awcullen encodes for t's OPC UA DataType (see uaGoType).
func toUA(t types.Type, v any) (any, error) {
	if v == nil {
		return nil, mismatch(t, v)
	}
	switch tt := t.(type) {
	case *types.EnumType:
		o, err := enumOrdinal(tt, v)
		if err != nil {
			return nil, err
		}
		if o < math.MinInt32 || o > math.MaxInt32 {
			return nil, outOfRange(t, v)
		}
		return int32(o), nil
	case *types.ArrayType:
		return arrayToUA(tt, v)
	case *types.PrimitiveType:
		return primitiveToUA(tt, v)
	}
	return nil, mismatch(t, v)
}

func primitiveToUA(t *types.PrimitiveType, v any) (any, error) {
	switch t.Kind_ {
	case types.KindBOOL:
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return nil, mismatch(t, v)
	case types.KindSINT:
		n, err := signedIn(t, v, math.MinInt8, math.MaxInt8)
		return int8(n), err
	case types.KindINT:
		n, err := signedIn(t, v, math.MinInt16, math.MaxInt16)
		return int16(n), err
	case types.KindDINT:
		n, err := signedIn(t, v, math.MinInt32, math.MaxInt32)
		return int32(n), err
	case types.KindLINT:
		return signedIn(t, v, math.MinInt64, math.MaxInt64)
	case types.KindUSINT, types.KindBYTE:
		n, err := unsignedIn(t, v, math.MaxUint8)
		return uint8(n), err
	case types.KindCHAR:
		n, err := charOf(t, v, math.MaxUint8)
		return uint8(n), err
	case types.KindUINT, types.KindWORD:
		n, err := unsignedIn(t, v, math.MaxUint16)
		return uint16(n), err
	case types.KindWCHAR:
		n, err := charOf(t, v, math.MaxUint16)
		return uint16(n), err
	case types.KindUDINT, types.KindDWORD:
		n, err := unsignedIn(t, v, math.MaxUint32)
		return uint32(n), err
	case types.KindULINT, types.KindLWORD:
		return unsignedIn(t, v, math.MaxUint64)
	case types.KindREAL:
		f, err := floatOf(t, v)
		if err != nil {
			return nil, err
		}
		if !math.IsInf(f, 0) && !math.IsNaN(f) && math.Abs(f) > math.MaxFloat32 {
			return nil, outOfRange(t, v)
		}
		return float32(f), nil
	case types.KindLREAL:
		return floatOf(t, v)
	case types.KindSTRING, types.KindWSTRING:
		if s, ok := v.(string); ok {
			return s, nil
		}
		return nil, mismatch(t, v)
	case types.KindTIME:
		ms, err := millisOf(t, v)
		if err != nil {
			return nil, err
		}
		if ms < 0 || ms > maxTimeMS {
			return nil, outOfRange(t, v)
		}
		return ms, nil
	case types.KindTOD:
		ms, err := millisOf(t, v)
		if err != nil {
			return nil, err
		}
		if ms < 0 || ms >= dayMS {
			return nil, outOfRange(t, v)
		}
		return uint32(ms), nil
	case types.KindDATE, types.KindDT:
		if tm, ok := v.(time.Time); ok {
			return tm.UTC(), nil
		}
		return nil, mismatch(t, v)
	}
	return nil, mismatch(t, v)
}

// sliceOf returns v as a reflect slice holding exactly the element count of a.
func sliceOf(a *types.ArrayType, v any) (reflect.Value, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return reflect.Value{}, mismatch(a, v)
	}
	n, err := arrayLen(a)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("%w: %v", ErrTypeMismatch, err)
	}
	if rv.Len() != n {
		return reflect.Value{}, fmt.Errorf("%w: %s holds %d elements, got %d", ErrTypeMismatch, a, n, rv.Len())
	}
	return rv, nil
}

// arrayToUA converts element-wise into the typed slice of uaGoType;
// multi-dimensional values are flattened row-major.
func arrayToUA(a *types.ArrayType, v any) (any, error) {
	gt, ok := uaGoType(a)
	if !ok {
		return nil, mismatch(a, v)
	}
	rv, err := sliceOf(a, v)
	if err != nil {
		return nil, err
	}
	out := reflect.MakeSlice(gt, rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		e, err := toUA(a.ElementType, rv.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", a, i, err)
		}
		out.Index(i).Set(reflect.ValueOf(e))
	}
	return out.Interface(), nil
}

// fromUA converts a decoded OPC UA value into the canonical IEC Go form a
// NodeSource accepts: signed integers int64, unsigned and bit strings
// uint64, REAL float32, LREAL float64, TIME/TOD time.Duration, DATE/DT
// time.Time, enums an int64 ordinal, arrays []any.
func fromUA(t types.Type, v any) (any, error) {
	if v == nil {
		return nil, mismatch(t, v)
	}
	switch tt := t.(type) {
	case *types.EnumType:
		return enumOrdinal(tt, v)
	case *types.ArrayType:
		rv, err := sliceOf(tt, v)
		if err != nil {
			return nil, err
		}
		out := make([]any, rv.Len())
		for i := range out {
			if out[i], err = fromUA(tt.ElementType, rv.Index(i).Interface()); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", tt, i, err)
			}
		}
		return out, nil
	case *types.PrimitiveType:
		return primitiveFromUA(tt, v)
	}
	return nil, mismatch(t, v)
}

func primitiveFromUA(t *types.PrimitiveType, v any) (any, error) {
	switch t.Kind_ {
	case types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT:
		// Range-check through toUA's width rules, then widen.
		if _, err := primitiveToUA(t, v); err != nil {
			return nil, err
		}
		s, _, _, _ := intOf(v)
		return s, nil
	case types.KindUSINT, types.KindBYTE, types.KindCHAR, types.KindUINT, types.KindWORD,
		types.KindWCHAR, types.KindUDINT, types.KindDWORD, types.KindULINT, types.KindLWORD:
		if _, ok := v.(string); ok {
			return nil, mismatch(t, v)
		}
		if _, err := primitiveToUA(t, v); err != nil {
			return nil, err
		}
		_, u, _, _ := intOf(v)
		return u, nil
	case types.KindTIME, types.KindTOD:
		if reflect.TypeOf(v) == durationType {
			return nil, mismatch(t, v)
		}
		ms, err := primitiveToUA(t, v)
		if err != nil {
			return nil, err
		}
		n := reflect.ValueOf(ms)
		if n.CanInt() {
			return time.Duration(n.Int()) * time.Millisecond, nil
		}
		return time.Duration(n.Uint()) * time.Millisecond, nil
	}
	return primitiveToUA(t, v)
}
