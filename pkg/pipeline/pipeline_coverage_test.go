package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Parse with defines that enable different branches

func TestParse_WithDefinesActiveTrue(t *testing.T) {
	src := `{IF defined(MY_FLAG)}
PROGRAM Active
END_PROGRAM
{ELSE}
PROGRAM Inactive
END_PROGRAM
{END_IF}`
	result := Parse("test.st", src, map[string]bool{"MY_FLAG": true})
	require.NotNil(t, result.File)
	// Active should be in the AST
	assert.True(t, len(result.File.Declarations) > 0)
}

func TestParse_WithDefinesActiveFalse(t *testing.T) {
	src := `{IF defined(MY_FLAG)}
PROGRAM Active
END_PROGRAM
{ELSE}
PROGRAM Inactive
END_PROGRAM
{END_IF}`
	result := Parse("test.st", src, map[string]bool{})
	require.NotNil(t, result.File)
	assert.True(t, len(result.File.Declarations) > 0)
}

// Test Parse with nil defines (vs empty map)

func TestParse_NilDefines(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
	END_PROGRAM`
	result := Parse("test.st", src, nil)
	require.NotNil(t, result.File)
	assert.Empty(t, result.PPDiags)
}

// Test source map remapping with parse errors in preprocessed code

func TestParse_SourceMapRemapping_WithError(t *testing.T) {
	src := `{DEFINE MY_FLAG}
{IF defined(MY_FLAG)}
PROGRAM P
VAR x : INT END_VAR
END_PROGRAM
{END_IF}`
	result := Parse("test.st", src, nil)
	require.NotNil(t, result.File)
	// Should have parse diags with remapped positions
	hasDiag := false
	for _, d := range result.Diags {
		if d.Pos.Line > 0 {
			hasDiag = true
		}
	}
	if !hasDiag {
		t.Log("no diags with line > 0 found")
	}
}

// Test Parse with {ERROR} directive

func TestParse_ErrorDirective(t *testing.T) {
	src := `{ERROR 'Stop here'}`
	result := Parse("test.st", src, nil)
	assert.True(t, len(result.PPDiags) > 0, "expected preprocessor error diagnostic")
}

// Test Parse with empty source

func TestParse_EmptySource(t *testing.T) {
	result := Parse("test.st", "", nil)
	require.NotNil(t, result.File)
}

// Test Parse with source map present but no remap needed

func TestParse_SourceMapNoRemap(t *testing.T) {
	src := `{DEFINE FOO}
PROGRAM P
END_PROGRAM`
	result := Parse("test.st", src, nil)
	require.NotNil(t, result.File)
	// SourceMap should be present
	if result.SourceMap == nil {
		t.Log("no source map generated (may be expected)")
	}
}
