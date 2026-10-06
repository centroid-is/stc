package ast

import (
	"math"
	"strconv"
	"strings"
)

// EnumOrdinal is the numeric value assigned to one enumeration value.
//
// Known is false when the value could not be computed from the AST alone:
// the explicit value is not an integer literal (for example a constant
// name), the literal overflows int64, or the value is an implicit
// successor of such an entry. Value then holds the positional guess
// (previous + 1), and callers that can evaluate constants should do so.
type EnumOrdinal struct {
	Name  string
	Value int64
	Known bool
}

// EnumOrdinals numbers the values of an enumeration with the IEC 61131-3 /
// Beckhoff previous+1 rule: the first implicit value is 0, an explicit
// value sets the counter, and each following implicit value is the
// previous value plus one. So (tun := 0, rdy := 2, nst) gives nst = 3.
//
// Explicit values may be integer literals with an optional base prefix
// (2#, 8#, 16#) and underscores, typed integer literals (UINT#5,
// BYTE#16#10), a leading unary minus or plus, and parentheses.
//
// When an explicit value is not computable, that entry and its implicit
// successors report Known=false; numbering continues positionally, so in
// (a := C_X, b) b gets the last known ordinal + 2. Nil values are skipped.
//
// This is the single numbering routine shared by the checker and the
// interpreter. It never panics on malformed or overflowing input.
func EnumOrdinals(e *EnumType) []EnumOrdinal {
	return EnumOrdinalsWith(e, nil)
}

// EnumOrdinalsWith numbers an enumeration like EnumOrdinals and evaluates an
// explicit value that is not an integer literal with eval, when eval is
// non-nil. A value eval resolves is Known, and so are its implicit
// successors: (ka := C_BASE, kb) with C_BASE = 10 gives kb = 11.
func EnumOrdinalsWith(e *EnumType, eval func(Expr) (int64, bool)) []EnumOrdinal {
	if e == nil {
		return nil
	}
	out := make([]EnumOrdinal, 0, len(e.Values))
	var prev int64 = -1
	prevKnown := true
	for _, v := range e.Values {
		if v == nil {
			continue
		}
		ord := EnumOrdinal{}
		if v.Name != nil {
			ord.Name = v.Name.Name
		}
		if v.Value != nil {
			val, ok := enumLiteralValue(v.Value)
			if !ok && eval != nil {
				val, ok = eval(v.Value)
			}
			if ok {
				ord.Value, ord.Known = val, true
			} else {
				ord.Value, ord.Known = nextOrdinal(prev)
				ord.Known = false
			}
		} else {
			var ok bool
			ord.Value, ok = nextOrdinal(prev)
			ord.Known = ok && prevKnown
		}
		prev, prevKnown = ord.Value, ord.Known
		out = append(out, ord)
	}
	return out
}

// nextOrdinal returns prev + 1, or (0, false) if that overflows int64.
func nextOrdinal(prev int64) (int64, bool) {
	if prev == math.MaxInt64 {
		return 0, false
	}
	return prev + 1, true
}

// IntLiteralValue evaluates an integer constant expression made of an
// integer literal (any base, typed or not), an optional sign and
// parentheses. ok is false for anything else and on int64 overflow.
func IntLiteralValue(x Expr) (int64, bool) {
	return enumLiteralValue(x)
}

// enumLiteralValue evaluates an enumeration value expression that is an
// integer literal, optionally signed or parenthesised.
func enumLiteralValue(x Expr) (int64, bool) {
	switch v := x.(type) {
	case *ParenExpr:
		return enumLiteralValue(v.Inner)
	case *UnaryExpr:
		switch v.Op.Text {
		case "-":
			n, ok := enumLiteralValue(v.Operand)
			if !ok {
				return 0, false
			}
			return -n, true
		case "+":
			return enumLiteralValue(v.Operand)
		}
		return 0, false
	case *Literal:
		switch v.LitKind {
		case LitInt:
			return parseIECInt(v.Value)
		case LitTyped:
			text := v.Value
			// Some producers leave the type prefix in Value (UINT#5).
			if i := strings.IndexByte(text, '#'); i > 0 && !isDigits(text[:i]) {
				text = text[i+1:]
			}
			return parseIECInt(text)
		}
	}
	return 0, false
}

// parseIECInt parses an IEC integer literal: optional sign, optional
// base prefix (2#, 8#, 16#), digits with underscores.
func parseIECInt(text string) (int64, bool) {
	text = strings.ReplaceAll(text, "_", "")
	neg := false
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		neg = text[0] == '-'
		text = text[1:]
	}
	base := 10
	if i := strings.IndexByte(text, '#'); i >= 0 {
		switch text[:i] {
		case "2":
			base = 2
		case "8":
			base = 8
		case "16":
			base = 16
		default:
			return 0, false
		}
		text = text[i+1:]
	}
	if text == "" {
		return 0, false
	}
	u, err := strconv.ParseUint(text, base, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		if u > uint64(math.MaxInt64)+1 {
			return 0, false
		}
		return -int64(u), true
	}
	if u > math.MaxInt64 {
		return 0, false
	}
	return int64(u), true
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// ConstIntValue evaluates an integer constant expression: integer literals,
// parentheses, unary + and -, binary +, - and *, and names. A bare name C
// resolves through lookup("", "C") and a qualified name G.C through
// lookup("G", "C"). ok is false for anything else, for an unresolved name
// and on int64 overflow.
func ConstIntValue(x Expr, lookup func(qual, name string) (int64, bool)) (int64, bool) {
	if n, ok := enumLiteralValue(x); ok {
		return n, true
	}
	switch v := x.(type) {
	case *ParenExpr:
		return ConstIntValue(v.Inner, lookup)
	case *UnaryExpr:
		n, ok := ConstIntValue(v.Operand, lookup)
		switch {
		case !ok:
			return 0, false
		case v.Op.Text == "+":
			return n, true
		case v.Op.Text == "-" && n != math.MinInt64:
			return -n, true
		}
	case *Ident:
		if lookup != nil {
			return lookup("", v.Name)
		}
	case *MemberAccessExpr:
		if obj, ok := v.Object.(*Ident); ok && lookup != nil && v.Member != nil {
			return lookup(obj.Name, v.Member.Name)
		}
	case *BinaryExpr:
		a, okA := ConstIntValue(v.Left, lookup)
		b, okB := ConstIntValue(v.Right, lookup)
		if okA && okB {
			return constBinary(v.Op.Text, a, b)
		}
	}
	return 0, false
}

// constBinary applies +, - or * to a and b, failing on overflow.
func constBinary(op string, a, b int64) (int64, bool) {
	switch op {
	case "+":
		r := a + b
		return r, (r > a) == (b > 0) || b == 0
	case "-":
		r := a - b
		return r, (r < a) == (b > 0) || b == 0
	case "*":
		if a == 0 || b == 0 {
			return 0, true
		}
		r := a * b
		return r, r/b == a && !(a == -1 && b == math.MinInt64) && !(b == -1 && a == math.MinInt64)
	}
	return 0, false
}
