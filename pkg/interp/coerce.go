package interp

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

// coerce converts the Go value in (as decoded from JSON or given by a Go
// caller) to a Value of the same type as the witness w, the current value at
// the target. External integer writes are range-checked and rejected, not
// wrapped. Arrays take []any (element i goes to slot ArrayLow+i), structs
// and FB instances take map[string]any with case-insensitive keys; missing
// keys are left unchanged.
//
// User and standard FB instances are reference values, so a map written to
// them is applied to the instance by coerce itself. A dry run (apply false)
// validates everything first, so a failing write changes nothing.
func (r *Runtime) coerce(w Value, in any) (Value, error) {
	if _, err := r.coerceWith(w, in, false); err != nil {
		return Value{}, err
	}
	return r.coerceWith(w, in, true)
}

func (r *Runtime) coerceWith(w Value, in any, apply bool) (Value, error) {
	if in == nil {
		return Value{}, fmt.Errorf("cannot write nil to %s", kindName(w))
	}
	switch w.Kind {
	case ValBool:
		return coerceBool(in)
	case ValInt:
		if def := r.interp.EnumDefs[w.Enum]; def != nil {
			return coerceEnum(w, def, in)
		}
		return coerceInt(w, in)
	case ValReal:
		return coerceReal(w, in)
	case ValString:
		s, ok := in.(string)
		if !ok {
			return Value{}, cannotUse(in, w)
		}
		str, err := unquoteIEC(s)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValString, Str: str, IECType: w.IECType}, nil
	case ValTime:
		return r.coerceTime(w, in)
	case ValDate, ValDateTime, ValTod:
		return coerceDate(w, in)
	case ValArray:
		return r.coerceArray(w, in, apply)
	case ValStruct:
		return r.coerceStruct(w, in, apply)
	case ValFBInstance:
		return r.coerceFB(w, in, apply)
	default:
		return Value{}, fmt.Errorf("%s is not writable by path", kindName(w))
	}
}

// kindName names the witness type for error messages.
func kindName(w Value) string {
	if w.IECType != types.KindInvalid {
		return w.IECType.String()
	}
	return w.Kind.String()
}

func cannotUse(in any, w Value) error {
	return fmt.Errorf("cannot use %T as %s", in, kindName(w))
}

// typedPrefix strips an IEC type prefix (INT#, REAL#, E_State#) from s: the
// text before the first '#' when it starts with a letter or underscore.
func typedPrefix(s string) (prefix, rest string) {
	if i := strings.IndexByte(s, '#'); i > 0 && pathIdentStart(s[0]) {
		return s[:i], s[i+1:]
	}
	return "", s
}

// toBigInt converts an integral Go number, json.Number or IEC integer
// literal string to a big.Int.
func toBigInt(in any) (*big.Int, error) {
	switch x := in.(type) {
	case int:
		return big.NewInt(int64(x)), nil
	case int8:
		return big.NewInt(int64(x)), nil
	case int16:
		return big.NewInt(int64(x)), nil
	case int32:
		return big.NewInt(int64(x)), nil
	case int64:
		return big.NewInt(x), nil
	case uint:
		return new(big.Int).SetUint64(uint64(x)), nil
	case uint8:
		return big.NewInt(int64(x)), nil
	case uint16:
		return big.NewInt(int64(x)), nil
	case uint32:
		return big.NewInt(int64(x)), nil
	case uint64:
		return new(big.Int).SetUint64(x), nil
	case float32:
		return floatToBig(float64(x))
	case float64:
		return floatToBig(x)
	case json.Number:
		if n, ok := new(big.Int).SetString(string(x), 10); ok {
			return n, nil
		}
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", string(x))
		}
		return floatToBig(f)
	case string:
		return parseIECInt(x)
	default:
		return nil, fmt.Errorf("cannot use %T as an integer", in)
	}
}

func floatToBig(f float64) (*big.Int, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return nil, fmt.Errorf("%v is not an integer", f)
	}
	n, _ := big.NewFloat(f).Int(nil)
	return n, nil
}

// parseIECInt parses an IEC integer literal: optional type prefix and sign,
// underscores, and base#digits for bases 2, 8 and 16.
func parseIECInt(s string) (*big.Int, error) {
	_, body := typedPrefix(strings.TrimSpace(s))
	body = strings.ReplaceAll(body, "_", "")
	neg := false
	if body != "" && (body[0] == '-' || body[0] == '+') {
		neg = body[0] == '-'
		body = body[1:]
	}
	base := 10
	if i := strings.IndexByte(body, '#'); i > 0 {
		b, err := strconv.Atoi(body[:i])
		if err != nil || (b != 2 && b != 8 && b != 16) {
			return nil, fmt.Errorf("invalid integer %q", s)
		}
		base, body = b, body[i+1:]
	}
	n, ok := new(big.Int).SetString(body, base)
	if !ok || body == "" || body[0] == '-' || body[0] == '+' {
		return nil, fmt.Errorf("invalid integer %q", s)
	}
	if neg {
		n.Neg(n)
	}
	return n, nil
}

// intBounds returns the value range of an integer kind; an unknown kind is
// treated as LINT.
func intBounds(k types.TypeKind) (lo, hi *big.Int) {
	w := bitWidth(k)
	if w == 0 {
		w = 64
	}
	if isUnsignedBits(k) {
		hi = new(big.Int).Lsh(big.NewInt(1), uint(w))
		return big.NewInt(0), hi.Sub(hi, big.NewInt(1))
	}
	hi = new(big.Int).Lsh(big.NewInt(1), uint(w-1))
	lo = new(big.Int).Neg(hi)
	return lo, hi.Sub(hi, big.NewInt(1))
}

func coerceInt(w Value, in any) (Value, error) {
	if _, ok := in.(bool); ok {
		return Value{}, cannotUse(in, w)
	}
	n, err := toBigInt(in)
	if err != nil {
		return Value{}, err
	}
	lo, hi := intBounds(w.IECType)
	if n.Cmp(lo) < 0 || n.Cmp(hi) > 0 {
		return Value{}, fmt.Errorf("%s out of range for %s", n, kindName(w))
	}
	var bits int64
	if n.IsInt64() {
		bits = n.Int64()
	} else {
		bits = int64(n.Uint64())
	}
	return storeAs(w, Value{Kind: ValInt, Int: bits, IECType: w.IECType}), nil
}

func coerceBool(in any) (Value, error) {
	switch x := in.(type) {
	case bool:
		return BoolValue(x), nil
	case string:
		switch strings.ToUpper(strings.TrimPrefix(strings.ToUpper(x), "BOOL#")) {
		case "TRUE", "1":
			return BoolValue(true), nil
		case "FALSE", "0":
			return BoolValue(false), nil
		}
		return Value{}, fmt.Errorf("invalid BOOL %q", x)
	}
	n, err := toBigInt(in)
	if err != nil || (n.Sign() != 0 && n.Cmp(big.NewInt(1)) != 0) {
		return Value{}, fmt.Errorf("cannot use %v as BOOL (want true, false, 0 or 1)", in)
	}
	return BoolValue(n.Sign() != 0), nil
}

// coerceEnum accepts a value name (v, E.v, E#v, case-insensitive) or a
// declared ordinal.
func coerceEnum(w Value, def *EnumDef, in any) (Value, error) {
	var n int64
	if s, ok := in.(string); ok {
		name := s
		if i := strings.LastIndexAny(name, ".#"); i >= 0 {
			name = name[i+1:]
		}
		v, ok := def.Values[strings.ToUpper(strings.TrimSpace(name))]
		if !ok {
			return Value{}, fmt.Errorf("unknown value %q for enum %s", s, def.Name)
		}
		n = v
	} else {
		b, err := toBigInt(in)
		if err != nil {
			return Value{}, err
		}
		if _, ok := def.Names[b.Int64()]; !ok || !b.IsInt64() {
			return Value{}, fmt.Errorf("unknown value %s for enum %s", b, def.Name)
		}
		n = b.Int64()
	}
	return Value{Kind: ValInt, Int: n, IECType: w.IECType, Enum: w.Enum}, nil
}

func coerceReal(w Value, in any) (Value, error) {
	var f float64
	switch x := in.(type) {
	case float64:
		f = x
	case float32:
		f = float64(x)
	case json.Number:
		v, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return Value{}, fmt.Errorf("invalid real %q", string(x))
		}
		f = v
	case string:
		_, body := typedPrefix(strings.TrimSpace(x))
		v, err := strconv.ParseFloat(strings.ReplaceAll(body, "_", ""), 64)
		if err != nil {
			return Value{}, fmt.Errorf("invalid real %q", x)
		}
		f = v
	case bool:
		return Value{}, cannotUse(in, w)
	default:
		n, err := toBigInt(in)
		if err != nil {
			return Value{}, err
		}
		f, _ = new(big.Float).SetInt(n).Float64()
	}
	switch {
	case math.IsNaN(f):
		return Value{}, fmt.Errorf("NaN not allowed for %s", kindName(w))
	case math.IsInf(f, 0):
		return Value{}, fmt.Errorf("infinite value not allowed for %s", kindName(w))
	case w.IECType == types.KindREAL && math.Abs(f) > math.MaxFloat32:
		return Value{}, fmt.Errorf("%g out of range for REAL", f)
	}
	return storeAs(w, Value{Kind: ValReal, Real: f, IECType: w.IECType}), nil
}

// coerceTime accepts a TIME literal string or a number of milliseconds.
func (r *Runtime) coerceTime(w Value, in any) (Value, error) {
	var d time.Duration
	switch x := in.(type) {
	case string:
		v, err := r.interp.parseLitTime(strings.TrimSpace(x))
		if err != nil {
			return Value{}, fmt.Errorf("invalid time %q", x)
		}
		d = v.Time
	case bool:
		return Value{}, cannotUse(in, w)
	case float64, float32, json.Number:
		f, err := strconv.ParseFloat(fmt.Sprint(x), 64)
		if err != nil {
			return Value{}, fmt.Errorf("%v is not a number", x)
		}
		d = time.Duration(f * float64(time.Millisecond))
	default:
		n, err := toBigInt(in)
		if err != nil {
			return Value{}, err
		}
		d = time.Duration(n.Int64()) * time.Millisecond
	}
	return Value{Kind: ValTime, Time: d, IECType: w.IECType}, nil
}

// dateLayouts holds the accepted literal prefixes and layout per kind.
var dateLayouts = map[ValueKind]struct {
	prefixes []string
	layout   string
	name     string
}{
	ValDate:     {[]string{"DATE#", "D#"}, "2006-01-02", "DATE"},
	ValDateTime: {[]string{"DATE_AND_TIME#", "DT#"}, "2006-01-02-15:04:05", "DT"},
	ValTod:      {[]string{"TIME_OF_DAY#", "TOD#"}, "15:04:05", "TOD"},
}

func coerceDate(w Value, in any) (Value, error) {
	s, ok := in.(string)
	if !ok {
		return Value{}, cannotUse(in, w)
	}
	spec := dateLayouts[w.Kind]
	body := strings.TrimSpace(s)
	for _, p := range spec.prefixes {
		if strings.HasPrefix(strings.ToUpper(body), p) {
			body = body[len(p):]
			break
		}
	}
	t, err := time.Parse(spec.layout, body)
	if err != nil {
		return Value{}, fmt.Errorf("invalid %s %q", spec.name, s)
	}
	base := time.Unix(0, 0).UTC()
	if w.Kind == ValTod {
		base = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return Value{Kind: w.Kind, Time: t.Sub(base), IECType: w.IECType}, nil
}

func (r *Runtime) coerceArray(w Value, in any, apply bool) (Value, error) {
	list, ok := in.([]any)
	if !ok {
		return Value{}, cannotUse(in, w)
	}
	if n := len(w.Array) - w.ArrayLow; len(list) > n {
		return Value{}, fmt.Errorf("cannot write %d elements to an array of %d", len(list), n)
	}
	out := w.Clone()
	for i, e := range list {
		slot := w.ArrayLow + i
		v, err := r.coerceWith(out.Array[slot], e, apply)
		if err != nil {
			return Value{}, fmt.Errorf("[%d]: %w", slot, err)
		}
		out.Array[slot] = v
	}
	return out, nil
}

// sortedKeys returns the keys of m in order, for deterministic errors.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (r *Runtime) coerceStruct(w Value, in any, apply bool) (Value, error) {
	m, ok := in.(map[string]any)
	if !ok {
		return Value{}, cannotUse(in, w)
	}
	out := w.Clone()
	for _, k := range sortedKeys(m) {
		upper := strings.ToUpper(k)
		f, ok := out.Struct[upper]
		if !ok {
			return Value{}, fmt.Errorf("struct has no member %q", k)
		}
		v, err := r.coerceWith(f, m[k], apply)
		if err != nil {
			return Value{}, fmt.Errorf("%s: %w", k, err)
		}
		out.Struct[upper] = v
	}
	return out, nil
}

// coerceFB writes a map into a user FB's variables, or into a standard
// FB's inputs (outputs are read-only). Only an apply run changes the
// instance.
func (r *Runtime) coerceFB(w Value, in any, apply bool) (Value, error) {
	m, ok := in.(map[string]any)
	if !ok || w.FBRef == nil {
		return Value{}, cannotUse(in, w)
	}
	inst := w.FBRef
	for _, k := range sortedKeys(m) {
		upper := strings.ToUpper(k)
		if inst.Env == nil {
			cur, out, ok := stdMember(inst.FB, upper)
			if !ok {
				return Value{}, fmt.Errorf("FB %s has no member %q", inst.TypeName, k)
			}
			if out {
				return Value{}, fmt.Errorf("%s is a read-only output of %s", k, inst.TypeName)
			}
			v, err := r.coerceWith(cur, m[k], apply)
			if err != nil {
				return Value{}, fmt.Errorf("%s: %w", k, err)
			}
			if apply {
				inst.FB.SetInput(upper, v)
			}
			continue
		}
		cur, ok := inst.Env.GetLocal(upper)
		if !ok {
			return Value{}, fmt.Errorf("FB %s has no member %q", inst.TypeName, k)
		}
		v, err := r.coerceWith(cur, m[k], apply)
		if err != nil {
			return Value{}, fmt.Errorf("%s: %w", k, err)
		}
		if apply && cur.Kind != ValFBInstance {
			inst.Env.Set(upper, storeAs(cur, v))
		}
	}
	return w, nil
}

// unquoteIEC returns s unchanged unless it is wrapped in single quotes, in
// which case the IEC escapes (” $' $$ $N $L $R $T $P $hh) are decoded.
func unquoteIEC(s string) (string, error) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return s, nil
	}
	body := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == '\'' && i+1 < len(body) && body[i+1] == '\'':
			b.WriteByte('\'')
			i++
		case c == '$' && i+1 < len(body):
			i++
			switch e := body[i]; e {
			case '$', '\'', '"':
				b.WriteByte(e)
			case 'N', 'n', 'L', 'l':
				b.WriteByte('\n')
			case 'R', 'r':
				b.WriteByte('\r')
			case 'T', 't':
				b.WriteByte('\t')
			case 'P', 'p':
				b.WriteByte('\f')
			default:
				if i+1 >= len(body) {
					return "", fmt.Errorf("invalid escape $%c in %q", e, s)
				}
				n, err := strconv.ParseUint(body[i:i+2], 16, 8)
				if err != nil {
					return "", fmt.Errorf("invalid escape $%s in %q", body[i:i+2], s)
				}
				b.WriteByte(byte(n))
				i++
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}
