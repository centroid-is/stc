package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validST = `PROGRAM Main
VAR
    x : INT := 0;
END_VAR
    x := x + 1;
END_PROGRAM
`

const invalidST = `PROGRAM Main
VAR
    x : INT
END_VAR
    x := ;
END_PROGRAM
`

const typeErrorST = `PROGRAM Main
VAR
    x : INT := 0;
END_VAR
    x := 'hello';
END_PROGRAM
`

func TestStcParse_ValidCode(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{Code: validST, Filename: "test.st"})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(text), &parsed)
	require.NoError(t, err)

	assert.Contains(t, parsed, "ast")
	assert.Contains(t, parsed, "diagnostics")
	assert.Contains(t, parsed, "has_errors")
	assert.Equal(t, false, parsed["has_errors"])
}

func TestStcParse_InvalidCode(t *testing.T) {
	result, err := handleParse(context.Background(), parseArgs{Code: invalidST, Filename: "bad.st"})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(text), &parsed)
	require.NoError(t, err)

	assert.Equal(t, true, parsed["has_errors"])
	diags, ok := parsed["diagnostics"].([]interface{})
	require.True(t, ok)
	assert.Greater(t, len(diags), 0)
}

func TestStcCheck_ValidCode(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	var diags []interface{}
	err = json.Unmarshal([]byte(text), &diags)
	require.NoError(t, err)
	// Valid code may have warnings but no errors expected from just parsing+analyzing
}

func TestStcCheck_TypeError(t *testing.T) {
	result, err := handleCheck(context.Background(), checkArgs{Code: typeErrorST})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	var diags []interface{}
	err = json.Unmarshal([]byte(text), &diags)
	require.NoError(t, err)
	assert.Greater(t, len(diags), 0)
}

func TestStcTest_Directory(t *testing.T) {
	// stc_test requires a directory with *_test.st files.
	// We test with a non-existent dir -- should return error or empty results.
	result, err := handleTest(context.Background(), testArgs{Directory: "/tmp/stc-test-nonexistent"})
	// Nonexistent dir should either error or return empty results
	if err != nil {
		return // acceptable
	}
	require.NotNil(t, result)
	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "total")
}

func TestStcEmit_Beckhoff(t *testing.T) {
	result, err := handleEmit(context.Background(), emitArgs{Code: validST, Target: "beckhoff"})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "PROGRAM")
	assert.Contains(t, text, "END_PROGRAM")
}

func TestStcLint_ValidCode(t *testing.T) {
	result, err := handleLint(context.Background(), lintArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	var diags []interface{}
	err = json.Unmarshal([]byte(text), &diags)
	require.NoError(t, err)
	// Lint result is an array (may or may not have items)
}

func TestStcFormat_ValidCode(t *testing.T) {
	result, err := handleFormat(context.Background(), formatArgs{Code: validST})
	require.NoError(t, err)
	require.NotNil(t, result)

	text := result.Content[0].(*textContent).Text
	assert.Contains(t, text, "PROGRAM")
	assert.Contains(t, text, "END_PROGRAM")
}

func TestToolDescriptionsUnder100Tokens(t *testing.T) {
	tools := allToolDefinitions()
	for _, tool := range tools {
		tokens := strings.Fields(tool.description)
		assert.LessOrEqual(t, len(tokens), 100,
			"Tool %q description exceeds 100 tokens: %d", tool.name, len(tokens))
	}
	assert.Len(t, tools, 6, "Expected exactly 6 tools")
}
