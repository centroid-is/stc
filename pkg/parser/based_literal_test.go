package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInvalidBasedLiteral covers review LO-04: a based literal with a base
// other than 2, 8 or 16, or without digits, is a parse error.
func TestInvalidBasedLiteral(t *testing.T) {
	for _, lit := range []string{"WORD#3#10", "DINT#16#", "3#10"} {
		t.Run(lit, func(t *testing.T) {
			res := Parse("t.st", "PROGRAM P\nVAR x : DINT; END_VAR\nx := "+lit+";\nEND_PROGRAM\n")
			require.NotEmpty(t, res.Diags)
			require.True(t, strings.Contains(res.Diags[0].Message, "Illegal"), res.Diags[0].Message)
		})
	}
	res := Parse("t.st", "PROGRAM P\nVAR x : WORD; END_VAR\nx := WORD#2#1010;\nEND_PROGRAM\n")
	require.Empty(t, res.Diags)
}
