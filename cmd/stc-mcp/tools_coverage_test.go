package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStcParse_DefaultFilename(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "ast")
}

func TestStcParse_EmptyCode(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{Code: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed, "ast")
}

func TestStcCheck_WithVendor(t *testing.T) {
	tests := []struct {
		name   string
		vendor string
	}{
		{"beckhoff", "beckhoff"},
		{"schneider", "schneider"},
		{"portable", "portable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handleCheck(context.Background(), checkArgs{Code: validST, Vendor: tt.vendor})
			require.NoError(t, err)
			require.NotNil(t, result)
			text := result.Content[0].(*textContent).Text
			var diags []interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &diags))
		})
	}
}

func TestStcCheck_NoVendor(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{Code: validST, Vendor: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcCheck_EmptyCode(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{Code: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcEmit_DefaultTarget(t *testing.T) {
	result, err := handleEmit(context.Background(), emitArgs{Code: validST, Target: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "PROGRAM")
}

func TestStcEmit_AllTargets(t *testing.T) {
	targets := []string{"beckhoff", "schneider", "portable"}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			result, err := handleEmit(context.Background(), emitArgs{Code: validST, Target: target})
			require.NoError(t, err)
			require.NotNil(t, result)
			text := result.Content[0].(*textContent).Text
			assert.Contains(t, text, "PROGRAM")
		})
	}
}

func TestStcEmit_EmptyCode(t *testing.T) {
	result, err := handleEmit(context.Background(), emitArgs{Code: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcLint_EmptyCode(t *testing.T) {
	result, err := handleLint(context.Background(), lintArgs{Code: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcLint_InvalidCode(t *testing.T) {
	result, err := handleLint(context.Background(), lintArgs{Code: invalidST})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcFormat_EmptyCode(t *testing.T) {
	result, err := handleFormat(context.Background(), formatArgs{Code: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcFormat_InvalidCode(t *testing.T) {
	result, err := handleFormat(context.Background(), formatArgs{Code: invalidST})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcFormat_Idempotent(t *testing.T) {
	result1, err := handleFormat(context.Background(), formatArgs{Code: validST})
	require.NoError(t, err)
	text1 := result1.Content[0].(*textContent).Text

	result2, err := handleFormat(context.Background(), formatArgs{Code: text1})
	require.NoError(t, err)
	text2 := result2.Content[0].(*textContent).Text

	assert.Equal(t, text1, text2, "format should be idempotent")
}

func TestStcParse_MalformedCode(t *testing.T) {
	malformed := `PROGRAM
    := ;
END_PROGRAM`
	result, err := handleParse(context.Background(), parseArgs{Code: malformed})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, true, parsed["has_errors"])
}

func TestStcCheck_TypeErrorWithVendor(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{
		Code:   typeErrorST,
		Vendor: "portable",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	var diags []interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &diags))
	assert.Greater(t, len(diags), 0)
}

func TestAllToolDefinitions_Count(t *testing.T) {
	defs := allToolDefinitions()
	assert.Len(t, defs, 6)

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.name] = true
	}
	assert.True(t, names["stc_parse"])
	assert.True(t, names["stc_check"])
	assert.True(t, names["stc_test"])
	assert.True(t, names["stc_emit"])
	assert.True(t, names["stc_lint"])
	assert.True(t, names["stc_format"])
}

func TestToMCPResult(t *testing.T) {
	r := &callToolResult{
		Content: []interface{}{
			&textContent{Text: "hello"},
			&textContent{Text: "world"},
		},
	}
	mcpResult := toMCPResult(r)
	assert.Len(t, mcpResult.Content, 2)
}

func TestToMCPResult_NonTextContent(t *testing.T) {
	r := &callToolResult{
		Content: []interface{}{
			"not a textContent",
			&textContent{Text: "hello"},
		},
	}
	mcpResult := toMCPResult(r)
	// Only the textContent should be included
	assert.Len(t, mcpResult.Content, 1)
}

func TestStcCheck_ComplexCode(t *testing.T) {
	code := `
FUNCTION_BLOCK FB_Motor
VAR_INPUT
    speed : INT;
END_VAR
VAR_OUTPUT
    running : BOOL;
END_VAR
    running := speed > 0;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    m : FB_Motor;
    s : INT;
END_VAR
    s := 100;
END_PROGRAM
`
	result, err := handleCheck(context.Background(), checkArgs{Code: code, Vendor: "beckhoff"})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStcEmit_OOPCode(t *testing.T) {
	code := `
FUNCTION_BLOCK FB_Motor
METHOD PUBLIC Start : BOOL
    Start := TRUE;
END_METHOD
END_FUNCTION_BLOCK
`
	result, err := handleEmit(context.Background(), emitArgs{Code: code, Target: "beckhoff"})
	require.NoError(t, err)
	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "FUNCTION_BLOCK")
}
