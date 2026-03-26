package preprocess

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreprocess(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		defines      map[string]bool
		filename     string
		wantOutput   string
		wantDiagLen  int
		wantDiagCode string
		wantDiagSev  diag.Severity
		wantDiagMsg  string
	}{
		{
			name:       "no directives passes through unchanged",
			source:     "VAR\n  x : INT;\nEND_VAR",
			wantOutput: "VAR\n  x : INT;\nEND_VAR",
		},
		{
			name:       "IF defined true includes block",
			source:     "{IF defined(X)}\nincluded\n{END_IF}",
			defines:    map[string]bool{"X": true},
			wantOutput: "included",
		},
		{
			name:       "IF defined false excludes block",
			source:     "{IF defined(X)}\nexcluded\n{END_IF}",
			defines:    map[string]bool{},
			wantOutput: "",
		},
		{
			name:       "IF ELSIF ELSE selects correct branch - first",
			source:     "{IF defined(A)}\nbranch_a\n{ELSIF defined(B)}\nbranch_b\n{ELSE}\nbranch_c\n{END_IF}",
			defines:    map[string]bool{"A": true},
			wantOutput: "branch_a",
		},
		{
			name:       "IF ELSIF ELSE selects correct branch - second",
			source:     "{IF defined(A)}\nbranch_a\n{ELSIF defined(B)}\nbranch_b\n{ELSE}\nbranch_c\n{END_IF}",
			defines:    map[string]bool{"B": true},
			wantOutput: "branch_b",
		},
		{
			name:       "IF ELSIF ELSE selects correct branch - else",
			source:     "{IF defined(A)}\nbranch_a\n{ELSIF defined(B)}\nbranch_b\n{ELSE}\nbranch_c\n{END_IF}",
			defines:    map[string]bool{},
			wantOutput: "branch_c",
		},
		{
			name:       "nested IF blocks",
			source:     "{IF defined(OUTER)}\nouter_start\n{IF defined(INNER)}\ninner\n{END_IF}\nouter_end\n{END_IF}",
			defines:    map[string]bool{"OUTER": true, "INNER": true},
			wantOutput: "outer_start\ninner\nouter_end",
		},
		{
			name:       "nested IF outer false skips inner",
			source:     "{IF defined(OUTER)}\nouter_start\n{IF defined(INNER)}\ninner\n{END_IF}\nouter_end\n{END_IF}",
			defines:    map[string]bool{"INNER": true},
			wantOutput: "",
		},
		{
			name:       "DEFINE makes symbol available",
			source:     "{DEFINE MY_FLAG}\n{IF defined(MY_FLAG)}\nflag_active\n{END_IF}",
			defines:    map[string]bool{},
			wantOutput: "flag_active",
		},
		{
			name:         "ERROR in active branch emits diagnostic",
			source:       "{ERROR \"Unsupported vendor\"}",
			defines:      map[string]bool{},
			filename:     "test.st",
			wantOutput:   "",
			wantDiagLen:  1,
			wantDiagCode: "PP001",
			wantDiagSev:  diag.Error,
			wantDiagMsg:  "Unsupported vendor",
		},
		{
			name:        "ERROR in inactive branch does not emit",
			source:      "{IF defined(X)}\n{ERROR \"should not fire\"}\n{END_IF}",
			defines:     map[string]bool{},
			wantOutput:  "",
			wantDiagLen: 0,
		},
		{
			name:       "non-directive pragma passes through",
			source:     "{attribute 'qualified_only'}\nVAR\nEND_VAR",
			wantOutput: "{attribute 'qualified_only'}\nVAR\nEND_VAR",
		},
		{
			name:         "unmatched END_IF produces error",
			source:       "{END_IF}",
			defines:      map[string]bool{},
			wantOutput:   "",
			wantDiagLen:  1,
			wantDiagCode: "PP002",
		},
		{
			name:         "missing END_IF at EOF produces error",
			source:       "{IF defined(X)}\nsome line",
			defines:      map[string]bool{"X": true},
			wantOutput:   "some line",
			wantDiagLen:  1,
			wantDiagCode: "PP003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Filename: tt.filename,
				Defines:  tt.defines,
			}
			if opts.Defines == nil {
				opts.Defines = map[string]bool{}
			}
			result := Preprocess(tt.source, opts)
			assert.Equal(t, tt.wantOutput, result.Output)
			if tt.wantDiagLen > 0 {
				require.Len(t, result.Diags, tt.wantDiagLen)
				if tt.wantDiagCode != "" {
					assert.Equal(t, tt.wantDiagCode, result.Diags[0].Code)
				}
				if tt.wantDiagMsg != "" {
					assert.Equal(t, tt.wantDiagMsg, result.Diags[0].Message)
				}
				if tt.wantDiagSev != 0 || tt.wantDiagCode == "PP001" {
					assert.Equal(t, tt.wantDiagSev, result.Diags[0].Severity)
				}
			} else {
				assert.Empty(t, result.Diags, "expected no diagnostics")
			}
		})
	}
}

func TestPreprocess_SourceMap(t *testing.T) {
	t.Run("identity mapping when no directives", func(t *testing.T) {
		result := Preprocess("line1\nline2\nline3", Options{Filename: "f.st"})
		assert.Equal(t, "line1\nline2\nline3", result.Output)
		require.NotNil(t, result.SourceMap)

		pos := result.SourceMap.OriginalPos(1, 1)
		assert.Equal(t, 1, pos.Line)
		assert.Equal(t, "f.st", pos.File)

		pos = result.SourceMap.OriginalPos(3, 5)
		assert.Equal(t, 3, pos.Line)
		assert.Equal(t, 5, pos.Col)
	})

	t.Run("mapping shifts when lines removed", func(t *testing.T) {
		// Lines: 1=IF, 2=excluded, 3=END_IF, 4=kept
		source := "{IF defined(X)}\nexcluded\n{END_IF}\nkept_line"
		result := Preprocess(source, Options{Filename: "f.st"})
		assert.Equal(t, "kept_line", result.Output)
		require.NotNil(t, result.SourceMap)

		// Preprocessed line 1 should map to original line 4
		pos := result.SourceMap.OriginalPos(1, 1)
		assert.Equal(t, 4, pos.Line)
	})
}

func TestPreprocess_SourceMapDiagPos(t *testing.T) {
	t.Run("ERROR diagnostic has original position", func(t *testing.T) {
		source := "line1\n{ERROR \"fail here\"}"
		result := Preprocess(source, Options{Filename: "test.st"})
		require.Len(t, result.Diags, 1)
		assert.Equal(t, "test.st", result.Diags[0].Pos.File)
		assert.Equal(t, 2, result.Diags[0].Pos.Line)
	})
}
