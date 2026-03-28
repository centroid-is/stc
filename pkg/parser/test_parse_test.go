package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestCase_BasicParsing(t *testing.T) {
	src := `TEST_CASE 'my test'
VAR
    x : INT;
END_VAR
    x := 42;
    ASSERT_TRUE(x > 0);
END_TEST_CASE`

	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)
	require.Len(t, result.File.Declarations, 1)

	tc, ok := result.File.Declarations[0].(*ast.TestCaseDecl)
	require.True(t, ok, "expected TestCaseDecl, got %T", result.File.Declarations[0])
	assert.Equal(t, "my test", tc.Name)
	assert.Len(t, tc.VarBlocks, 1)
	assert.Len(t, tc.Body, 2, "expected 2 body statements (assign + call)")
}

func TestTestCase_WithProgramDecl(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 1;
END_PROGRAM

TEST_CASE 'test main'
    ASSERT_TRUE(TRUE);
END_TEST_CASE`

	result := Parse("mixed.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)
	require.Len(t, result.File.Declarations, 2)

	_, ok1 := result.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok1, "expected ProgramDecl, got %T", result.File.Declarations[0])

	tc, ok2 := result.File.Declarations[1].(*ast.TestCaseDecl)
	require.True(t, ok2, "expected TestCaseDecl, got %T", result.File.Declarations[1])
	assert.Equal(t, "test main", tc.Name)
}

func TestTestCase_MissingNameError(t *testing.T) {
	src := `TEST_CASE
    ASSERT_TRUE(TRUE);
END_TEST_CASE`

	result := Parse("bad.st", src)
	require.NotNil(t, result.File)
	require.NotEmpty(t, result.Diags, "expected parse error for missing test name")
}

func TestTestCase_OptionalSemicolon(t *testing.T) {
	// Semicolon after END_TEST_CASE is optional
	src := `TEST_CASE 'with semi'
    ASSERT_TRUE(TRUE);
END_TEST_CASE;`

	result := Parse("semi.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)
	require.Len(t, result.File.Declarations, 1)

	tc, ok := result.File.Declarations[0].(*ast.TestCaseDecl)
	require.True(t, ok, "expected TestCaseDecl")
	assert.Equal(t, "with semi", tc.Name)
}

func TestTestCase_DoubleQuotedName(t *testing.T) {
	src := `TEST_CASE "double quoted"
    ASSERT_TRUE(TRUE);
END_TEST_CASE`

	result := Parse("dquote.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)
	require.Len(t, result.File.Declarations, 1)

	tc, ok := result.File.Declarations[0].(*ast.TestCaseDecl)
	require.True(t, ok, "expected TestCaseDecl")
	assert.Equal(t, "double quoted", tc.Name)
}

func TestTestCase_KindIsTestCaseDecl(t *testing.T) {
	src := `TEST_CASE 'kind check'
END_TEST_CASE`

	result := Parse("kind.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)
	require.Len(t, result.File.Declarations, 1)

	tc, ok := result.File.Declarations[0].(*ast.TestCaseDecl)
	require.True(t, ok, "expected TestCaseDecl")
	assert.Equal(t, ast.KindTestCaseDecl, tc.Kind())
}
