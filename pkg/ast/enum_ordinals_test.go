package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ordEnum(vals ...*EnumValue) *EnumType {
	return &EnumType{NodeBase: NodeBase{NodeKind: KindEnumType}, Values: vals}
}

func ordVal(name string, v Expr) *EnumValue {
	return &EnumValue{Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: name}, Value: v}
}

func ordNeg(e Expr) Expr {
	return &UnaryExpr{NodeBase: NodeBase{NodeKind: KindUnaryExpr}, Op: Token{Text: "-"}, Operand: e}
}

func TestEnumOrdinals(t *testing.T) {
	tests := []struct {
		name string
		enum *EnumType
		want []EnumOrdinal
	}{
		{"implicit from zero",
			ordEnum(ordVal("a", nil), ordVal("b", nil), ordVal("c", nil)),
			[]EnumOrdinal{{"a", 0, true}, {"b", 1, true}, {"c", 2, true}}},
		{"previous plus one after explicit",
			ordEnum(ordVal("tun", p20Int("0")), ordVal("rdy", p20Int("2")), ordVal("nst", nil)),
			[]EnumOrdinal{{"tun", 0, true}, {"rdy", 2, true}, {"nst", 3, true}}},
		{"hex literal",
			ordEnum(ordVal("a", p20Int("16#0006")), ordVal("b", nil)),
			[]EnumOrdinal{{"a", 6, true}, {"b", 7, true}}},
		{"binary literal",
			ordEnum(ordVal("a", p20Int("2#101"))),
			[]EnumOrdinal{{"a", 5, true}}},
		{"octal literal",
			ordEnum(ordVal("a", p20Int("8#17"))),
			[]EnumOrdinal{{"a", 15, true}}},
		{"underscores",
			ordEnum(ordVal("a", p20Int("1_000"))),
			[]EnumOrdinal{{"a", 1000, true}}},
		{"hex with underscores",
			ordEnum(ordVal("a", p20Int("16#FF_FF"))),
			[]EnumOrdinal{{"a", 65535, true}}},
		{"unary minus",
			ordEnum(ordVal("a", ordNeg(p20Int("1"))), ordVal("b", nil)),
			[]EnumOrdinal{{"a", -1, true}, {"b", 0, true}}},
		{"unary plus",
			ordEnum(ordVal("a", &UnaryExpr{Op: Token{Text: "+"}, Operand: p20Int("4")})),
			[]EnumOrdinal{{"a", 4, true}}},
		{"signed literal text",
			ordEnum(ordVal("a", p20Int("-3")), ordVal("b", nil)),
			[]EnumOrdinal{{"a", -3, true}, {"b", -2, true}}},
		{"parenthesised literal",
			ordEnum(ordVal("a", &ParenExpr{Inner: p20Int("9")})),
			[]EnumOrdinal{{"a", 9, true}}},
		{"typed literal",
			ordEnum(ordVal("a", &Literal{LitKind: LitTyped, TypePrefix: "UINT", Value: "5"}), ordVal("b", nil)),
			[]EnumOrdinal{{"a", 5, true}, {"b", 6, true}}},
		{"typed based literal",
			ordEnum(ordVal("a", &Literal{LitKind: LitTyped, TypePrefix: "BYTE", Value: "16#10"})),
			[]EnumOrdinal{{"a", 16, true}}},
		{"typed literal with prefix left in value",
			ordEnum(ordVal("a", &Literal{LitKind: LitTyped, Value: "UINT#5"})),
			[]EnumOrdinal{{"a", 5, true}}},
		{"non-literal value continues from last known plus two",
			ordEnum(ordVal("a", &Ident{Name: "C_X"}), ordVal("b", nil), ordVal("c", p20Int("10")), ordVal("d", nil)),
			[]EnumOrdinal{{"a", 0, false}, {"b", 1, false}, {"c", 10, true}, {"d", 11, true}}},
		{"non-literal after known value",
			ordEnum(ordVal("a", p20Int("4")), ordVal("b", &Ident{Name: "C_X"}), ordVal("c", nil)),
			[]EnumOrdinal{{"a", 4, true}, {"b", 5, false}, {"c", 6, false}}},
		{"typed enum literal is not numeric",
			ordEnum(ordVal("a", &Literal{LitKind: LitTyped, TypePrefix: "E", Value: "x"})),
			[]EnumOrdinal{{"a", 0, false}}},
		{"real literal is not an ordinal",
			ordEnum(ordVal("a", &Literal{LitKind: LitReal, Value: "1.5"})),
			[]EnumOrdinal{{"a", 0, false}}},
		{"overflowing literal",
			ordEnum(ordVal("a", p20Int("16#FFFFFFFFFFFFFFFFFF")), ordVal("b", nil)),
			[]EnumOrdinal{{"a", 0, false}, {"b", 1, false}}},
		{"overflowing decimal",
			ordEnum(ordVal("a", p20Int("99999999999999999999"))),
			[]EnumOrdinal{{"a", 0, false}}},
		{"most negative int64",
			ordEnum(ordVal("a", p20Int("-9223372036854775808"))),
			[]EnumOrdinal{{"a", -9223372036854775808, true}}},
		{"negative overflow",
			ordEnum(ordVal("a", p20Int("-9223372036854775809"))),
			[]EnumOrdinal{{"a", 0, false}}},
		{"malformed literal",
			ordEnum(ordVal("a", p20Int("16#"))),
			[]EnumOrdinal{{"a", 0, false}}},
		{"bad base",
			ordEnum(ordVal("a", p20Int("x#1"))),
			[]EnumOrdinal{{"a", 0, false}}},
		{"implicit successor overflows",
			ordEnum(ordVal("a", p20Int("9223372036854775807")), ordVal("b", nil)),
			[]EnumOrdinal{{"a", 9223372036854775807, true}, {"b", 0, false}}},
		{"unary minus on non-literal",
			ordEnum(ordVal("a", ordNeg(&Ident{Name: "C"}))),
			[]EnumOrdinal{{"a", 0, false}}},
		{"other unary operator",
			ordEnum(ordVal("a", &UnaryExpr{Op: Token{Text: "NOT"}, Operand: p20Int("1")})),
			[]EnumOrdinal{{"a", 0, false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EnumOrdinals(tt.enum))
		})
	}
}

func TestEnumOrdinals_EdgeInputs(t *testing.T) {
	assert.Nil(t, EnumOrdinals(nil))
	assert.Empty(t, EnumOrdinals(&EnumType{}))
	// A value with a nil name is reported with an empty name, not a panic.
	got := EnumOrdinals(&EnumType{Values: []*EnumValue{{}, nil}})
	assert.Equal(t, []EnumOrdinal{{"", 0, true}}, got)
}
