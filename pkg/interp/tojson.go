package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
)

// maxJSONDepth bounds ToJSON recursion; a reference cycle renders null
// past it instead of recursing forever.
const maxJSONDepth = 64

// orderedObject is a JSON object that keeps its key order.
type orderedObject struct {
	keys []string
	vals []any
}

func (o *orderedObject) add(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

// MarshalJSON writes the members in insertion order.
func (o *orderedObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// stdFBMembers lists the members of the standard FBs in declaration order
// (inputs, then outputs) for ToJSON.
var stdFBMembers = map[string][]string{
	"TON": {"IN", "PT", "Q", "ET"}, "TOF": {"IN", "PT", "Q", "ET"}, "TP": {"IN", "PT", "Q", "ET"},
	"CTU": {"CU", "R", "PV", "Q", "CV"}, "CTD": {"CD", "LD", "PV", "Q", "CV"},
	"CTUD":   {"CU", "CD", "R", "LD", "PV", "QU", "QD", "CV"},
	"R_TRIG": {"CLK", "Q"}, "F_TRIG": {"CLK", "Q"},
	"SR": {"S1", "R", "Q1"}, "RS": {"S", "R1", "Q1"},
}

// ToJSON converts v to a JSON-ready value with stable output:
//
//   - BOOL: bool; integers: number (ULINT/LWORD unsigned); an enum value:
//     its value name, or the number when no name matches;
//   - REAL/LREAL: number, NaN and infinities as "NaN", "+Inf", "-Inf";
//   - STRING: string; TIME: {"ms": n, "iso": "PT5S"};
//   - DATE, DT, TOD: IEC literal strings (D#..., DT#..., TOD#...);
//   - ARRAY: list of the elements ArrayLow..High;
//   - STRUCT: object in declared member order and case (sorted upper-case
//     keys when the order is unknown);
//   - FB instance: object of its variables in declaration order, EXTENDS
//     bases first; a standard FB: its inputs and outputs;
//   - a bound reference renders its target, an unbound one null; pointers
//     render as their debug string.
func (r *Runtime) ToJSON(v Value) any {
	return r.toJSON(v, 0)
}

func (r *Runtime) toJSON(v Value, depth int) any {
	if depth > maxJSONDepth {
		return nil
	}
	switch v.Kind {
	case ValBool:
		return v.Bool
	case ValInt:
		if def := r.interp.EnumDefs[v.Enum]; def != nil {
			if s, ok := def.Names[v.Int]; ok {
				return s
			}
		}
		if isUnsigned64(v.IECType) {
			return uint64(v.Int)
		}
		return v.Int
	case ValReal:
		switch {
		case math.IsNaN(v.Real):
			return "NaN"
		case math.IsInf(v.Real, 1):
			return "+Inf"
		case math.IsInf(v.Real, -1):
			return "-Inf"
		}
		return v.Real
	case ValString:
		return v.Str
	case ValTime:
		o := &orderedObject{}
		o.add("ms", v.Time.Milliseconds())
		o.add("iso", isoDuration(v.Time))
		return o
	case ValDate:
		return "D#" + epoch(v.Time).Format("2006-01-02")
	case ValDateTime:
		return "DT#" + epoch(v.Time).Format("2006-01-02-15:04:05") + fracMillis(v.Time)
	case ValTod:
		return "TOD#" + epoch(v.Time).Format("15:04:05") + fracMillis(v.Time)
	case ValArray:
		low := v.ArrayLow
		if low > len(v.Array) {
			low = len(v.Array)
		}
		out := make([]any, 0, len(v.Array)-low)
		for _, e := range v.Array[low:] {
			out = append(out, r.toJSON(e, depth+1))
		}
		return out
	case ValStruct:
		return r.structJSON(v, depth)
	case ValFBInstance:
		return r.fbJSON(v.FBRef, depth)
	case ValReference:
		p, ok := refPathOf(v)
		if !ok {
			return nil
		}
		t, err := readRef(p)
		if err != nil {
			return nil
		}
		return r.toJSON(t, depth+1)
	default:
		return v.String()
	}
}

func (r *Runtime) structJSON(v Value, depth int) any {
	o := &orderedObject{}
	names := v.Fields
	if len(names) == 0 {
		for k := range v.Struct {
			names = append(names, k)
		}
		sort.Strings(names)
	}
	for _, n := range names {
		if f, ok := v.Struct[strings.ToUpper(n)]; ok {
			o.add(n, r.toJSON(f, depth+1))
		}
	}
	return o
}

func (r *Runtime) fbJSON(inst *FBInstance, depth int) any {
	if inst == nil {
		return nil
	}
	o := &orderedObject{}
	if inst.FB != nil {
		for _, n := range stdFBMembers[strings.ToUpper(inst.TypeName)] {
			if m, _, ok := stdMember(inst.FB, n); ok {
				o.add(n, r.toJSON(m, depth+1))
			}
		}
		return o
	}
	for _, n := range r.fbVarNames(inst) {
		if m, ok := inst.Env.GetLocal(n); ok {
			o.add(n, r.toJSON(m, depth+1))
		}
	}
	return o
}

// fbVarNames lists the declared-case variable names of a user FB instance,
// EXTENDS bases first.
func (r *Runtime) fbVarNames(inst *FBInstance) []string {
	var chain [][]*ast.VarBlock
	seen := map[string]bool{}
	for d := inst.Decl; d != nil && d.Name != nil && !seen[strings.ToUpper(d.Name.Name)]; {
		seen[strings.ToUpper(d.Name.Name)] = true
		chain = append([][]*ast.VarBlock{d.VarBlocks}, chain...)
		if d.Extends == nil {
			break
		}
		d = r.interp.FBDecls[strings.ToUpper(d.Extends.Name)]
	}
	var names []string
	for _, blocks := range chain {
		names = append(names, varNames(blocks)...)
	}
	return names
}

// varNames lists the declared-case variable names of blocks in order.
func varNames(blocks []*ast.VarBlock) []string {
	var names []string
	for _, vb := range blocks {
		if vb == nil {
			continue
		}
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// epoch returns the UTC time d after 1970-01-01.
func epoch(d time.Duration) time.Time {
	return time.Unix(0, 0).UTC().Add(d)
}

// fracMillis returns ".fff" for a non-zero millisecond part, else "".
func fracMillis(d time.Duration) string {
	ms := (d % time.Second) / time.Millisecond
	if ms == 0 {
		return ""
	}
	return fmt.Sprintf(".%03d", ms)
}

// isoDuration renders d as an ISO 8601 duration (PT1H30M1.5S, PT0S).
func isoDuration(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	var b strings.Builder
	if d < 0 {
		b.WriteByte('-')
		d = -d
	}
	b.WriteString("PT")
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	if h > 0 {
		fmt.Fprintf(&b, "%dH", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dM", m)
	}
	if d > 0 {
		b.WriteString(strconv.FormatFloat(d.Seconds(), 'f', -1, 64))
		b.WriteByte('S')
	}
	return b.String()
}
