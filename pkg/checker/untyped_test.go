package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseExpr parses src as the right-hand side of an assignment inside a
// throwaway program and returns that expression.
func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	res := parser.Parse("e.st", "PROGRAM P\nx := "+src+";\nEND_PROGRAM\n")
	require.Empty(t, res.Diags, src)
	prog, ok := res.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	require.Len(t, prog.Body, 1)
	return prog.Body[0].(*ast.AssignStmt).Value
}

func TestUntypedConst(t *testing.T) {
	tests := []struct {
		src    string
		kind   untypedKind
		val    int64
		hasVal bool
	}{
		{"1", untypedInt, 1, true},
		{"-5", untypedInt, -5, true},
		{"(3)", untypedInt, 3, true},
		{"2 * 3 + 1", untypedInt, 7, true},
		{"7 / 2", untypedInt, 3, true},
		{"7 MOD 4", untypedInt, 3, true},
		{"10 - 4", untypedInt, 6, true},
		{"16#FF", untypedInt, 255, true},
		{"2#1010", untypedInt, 10, true},
		{"9223372036854775807 + 1", untypedInt, 0, false},
		{"16#FFFF_FFFF_FFFF_FFFF", untypedInt, 0, false},
		{"1 / 0", untypedInt, 0, false},
		{"1 MOD 0", untypedInt, 0, false},
		{"-(-9223372036854775807 - 1)", untypedInt, 0, false},
		{"5.0", untypedReal, 0, false},
		{"1e3", untypedReal, 0, false},
		{"1 + 2.5", untypedReal, 0, false},
		{"-2.5", untypedReal, 0, false},
		{"INT#1", untypedNone, 0, false},
		{"x + 1", untypedNone, 0, false},
		{"1 + x", untypedNone, 0, false},
		{"NOT 1", untypedNone, 0, false},
		{"1 = 1", untypedNone, 0, false},
		{"2 ** 3", untypedNone, 0, false},
		{"TRUE", untypedNone, 0, false},
		{"'s'", untypedNone, 0, false},
		{"x", untypedNone, 0, false},
		{"16#FFFF_FFFF_FFFF_FFFF + 1", untypedInt, 0, false},
		{"-x", untypedNone, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			k, v, has := untypedConst(parseExpr(t, tt.src))
			assert.Equal(t, tt.kind, k)
			assert.Equal(t, tt.hasVal, has)
			if tt.hasVal {
				assert.Equal(t, tt.val, v)
			}
		})
	}
	k, _, _ := untypedConst(nil)
	assert.Equal(t, untypedNone, k)

	// Unary plus is not produced by the parser today; build it directly.
	plus := &ast.UnaryExpr{Op: ast.Token{Text: "+"}, Operand: parseExpr(t, "5")}
	k, v, has := untypedConst(plus)
	assert.Equal(t, untypedInt, k)
	assert.True(t, has)
	assert.Equal(t, int64(5), v)
}

func TestUntypedAssignable(t *testing.T) {
	prim := func(k types.TypeKind) types.Type { return &types.PrimitiveType{Kind_: k} }
	tests := []struct {
		src    string
		target types.TypeKind
		ok     bool
		msg    string
	}{
		{"300", types.KindBYTE, false, "constant 300 out of range for BYTE"},
		{"255", types.KindBYTE, true, ""},
		{"-1", types.KindUINT, false, "constant -1 out of range for UINT"},
		{"-1", types.KindBYTE, false, "constant -1 out of range for BYTE"},
		{"65535", types.KindWORD, true, ""},
		{"16#2001", types.KindWORD, true, ""},
		{"16#FFFFFFFF", types.KindDWORD, true, ""},
		{"16#1_0000_0000", types.KindDWORD, false, "constant 4294967296 out of range for DWORD"},
		{"16#FFFF_FFFF_FFFF_FFFF", types.KindLWORD, true, ""},
		{"16#FFFF_FFFF_FFFF_FFFF", types.KindULINT, true, ""},
		{"-128", types.KindSINT, true, ""},
		{"-129", types.KindSINT, false, "constant -129 out of range for SINT"},
		{"32768", types.KindINT, false, "constant 32768 out of range for INT"},
		{"-2147483648", types.KindDINT, true, ""},
		{"2147483648", types.KindDINT, false, "constant 2147483648 out of range for DINT"},
		{"-9223372036854775807", types.KindLINT, true, ""},
		{"256", types.KindUSINT, false, "constant 256 out of range for USINT"},
		{"4294967295", types.KindUDINT, true, ""},
		{"0", types.KindREAL, true, ""},
		{"0", types.KindLREAL, true, ""},
		{"5.0", types.KindREAL, true, ""},
		{"5.0", types.KindINT, false, ""},
		{"5.0", types.KindBYTE, false, ""},
		{"1", types.KindBOOL, false, ""},
		{"1", types.KindTIME, false, ""},
		{"1", types.KindSTRING, false, ""},
		{"INT#1", types.KindSINT, false, ""},
		{"x", types.KindINT, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.src+"->"+tt.target.String(), func(t *testing.T) {
			ok, msg := untypedAssignable(parseExpr(t, tt.src), prim(tt.target))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.msg, msg)
		})
	}
	ok, _ := untypedAssignable(parseExpr(t, "1"), nil)
	assert.False(t, ok)
}

func TestIntRange(t *testing.T) {
	lo, hi := intRange(types.KindINT)
	assert.Equal(t, int64(-32768), lo)
	assert.Equal(t, uint64(32767), hi)
	lo, hi = intRange(types.KindLWORD)
	assert.Equal(t, int64(0), lo)
	assert.Equal(t, uint64(1<<64-1), hi)
	lo, hi = intRange(types.KindREAL)
	assert.Equal(t, int64(0), lo)
	assert.Equal(t, uint64(0), hi)
}
