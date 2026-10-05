package checker

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diagsWithCode(ds []diag.Diagnostic, code string) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

func TestATWildcard(t *testing.T) {
	t.Run("fbat fixture has no diagnostics", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/fbat.st")
		require.NoError(t, err)
		require.Empty(t, runChecker(string(data)))
	})

	t.Run("wildcard in FB VAR_INPUT and VAR_OUTPUT", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Drive
VAR_INPUT
    i_uETA AT %I* : UINT;
END_VAR
VAR_OUTPUT
    q_uCMD AT %Q* : UINT;
END_VAR
VAR
    m AT %M* : INT;
END_VAR
q_uCMD := i_uETA;
END_FUNCTION_BLOCK`
		require.Empty(t, runChecker(src))
	})

	t.Run("explicit address in FB keeps SEMA031", func(t *testing.T) {
		src := "FUNCTION_BLOCK FB\nVAR\n    x AT %IX0.0 : BOOL;\nEND_VAR\nEND_FUNCTION_BLOCK"
		got := diagsWithCode(runChecker(src), CodeATNotAllowedHere)
		require.Len(t, got, 1)
		assert.Equal(t, diag.Warning, got[0].Severity)
	})

	t.Run("explicit address in FUNCTION keeps SEMA031", func(t *testing.T) {
		src := "FUNCTION F : INT\nVAR\n    x AT %QW4 : WORD;\nEND_VAR\nF := 0;\nEND_FUNCTION"
		require.Len(t, diagsWithCode(runChecker(src), CodeATNotAllowedHere), 1)
	})

	t.Run("explicit address in PROGRAM is allowed", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n    x AT %IX0.0 : BOOL;\nEND_VAR\nEND_PROGRAM"
		require.Empty(t, diagsWithCode(runChecker(src), CodeATNotAllowedHere))
	})

	t.Run("explicit address on STRUCT member warns", func(t *testing.T) {
		src := "TYPE S :\nSTRUCT\n    a AT %IX0.0 : BOOL;\nEND_STRUCT\nEND_TYPE"
		ds := runChecker(src)
		got := diagsWithCode(ds, CodeATNotAllowedHere)
		require.Len(t, got, 1, "diags: %v", ds)
		assert.Equal(t, diag.Warning, got[0].Severity)
		assert.Contains(t, got[0].Message, `"a"`)
		assert.Contains(t, got[0].Message, "all instances share the same address")
		assert.Equal(t, 3, got[0].Pos.Line)
	})

	t.Run("unknown area on STRUCT member is a parse error", func(t *testing.T) {
		// %Z9 is rejected by the lexer, so it never reaches the checker.
		r := parser.Parse("s.st", "TYPE S :\nSTRUCT\n    a AT %Z9 : BOOL;\nEND_STRUCT\nEND_TYPE")
		require.NotEmpty(t, r.Diags)
	})

	t.Run("invalid address on STRUCT member is SEMA030", func(t *testing.T) {
		// %IX0.9 lexes as an address but bit 9 is out of range.
		src := "TYPE S :\nSTRUCT\n    a AT %IX0.9 : BOOL;\nEND_STRUCT\nEND_TYPE"
		ds := runChecker(src)
		got := diagsWithCode(ds, CodeInvalidATAddress)
		require.Len(t, got, 1, "diags: %v", ds)
		assert.Equal(t, diag.Error, got[0].Severity)
		assert.Empty(t, diagsWithCode(ds, CodeATNotAllowedHere))
	})

	t.Run("wildcard on STRUCT member is silent", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/structat.st")
		require.NoError(t, err)
		require.Empty(t, runChecker(string(data)))
	})

	t.Run("non-struct type decl is ignored", func(t *testing.T) {
		src := "TYPE E : (a, b);\nEND_TYPE"
		require.Empty(t, runChecker(src))
	})
}

func TestEmptyArgCheck(t *testing.T) {
	const fb = `FUNCTION_BLOCK FB_X
VAR_INPUT
    a : INT;
    b : INT;
END_VAR
VAR_OUTPUT
    q : BOOL;
END_VAR
q := a > b;
END_FUNCTION_BLOCK
`
	t.Run("empty input and output args are accepted", func(t *testing.T) {
		src := fb + "PROGRAM P\nVAR\n    f : FB_X;\nEND_VAR\nf(a := 1, b := , q => );\nEND_PROGRAM"
		require.Empty(t, runChecker(src))
	})

	t.Run("unknown empty arg still reported", func(t *testing.T) {
		src := fb + "PROGRAM P\nVAR\n    f : FB_X;\nEND_VAR\nf(zz := );\nEND_PROGRAM"
		ds := runChecker(src)
		require.Len(t, diagsWithCode(ds, CodeNoMember), 1, "diags: %v", ds)
	})
}
