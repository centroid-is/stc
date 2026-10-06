package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestBitwiseOps(t *testing.T) {
	eng := exprEngine(t, "b : BYTE := 16#A5;\nw : WORD := 16#00FF;\nx : BOOL := TRUE;")
	tests := []struct {
		src  string
		want int64
		kind types.TypeKind
	}{
		{"b AND 16#0F", 0x05, types.KindBYTE},
		{"b OR 16#0F", 0xAF, types.KindBYTE},
		{"b XOR 16#FF", 0x5A, types.KindBYTE},
		{"b AND w", 0xA5, types.KindWORD},
		{"16#F0 AND 16#3C", 0x30, types.KindDINT},
	}
	for _, tt := range tests {
		v := mustEval(t, eng, tt.src)
		assert.Equal(t, ValInt, v.Kind, tt.src)
		assert.Equal(t, tt.want, v.Int, tt.src)
		assert.Equal(t, tt.kind, v.IECType, tt.src)
	}
	// BOOL operands stay logical.
	assert.Equal(t, BoolValue(true), mustEval(t, eng, "x AND TRUE"))
	// Non-bitwise operators fall through.
	_, ok := bitwise("+", 1, 2, types.KindDINT)
	assert.False(t, ok)
}
