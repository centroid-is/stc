package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInlineExpect writes src as a single _test.st file and runs it, returning
// the result for custom assertions.
func runInlineExpect(t *testing.T, src string) *RunResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "inline_test.st")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total == 0 {
		t.Fatal("no tests discovered")
	}
	return result
}

// A user FB declared as a VAR of another user FB must be instantiated and
// callable -- FB composition is the basic reuse mechanism in IEC 61131-3.
func TestNestedUserFB_Executes(t *testing.T) {
	runInline(t, `
FUNCTION_BLOCK FB_Inner
VAR_INPUT
	i_r : REAL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
q_r := i_r + 1.0;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Outer
VAR_INPUT
	i_r : REAL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
VAR
	inner : FB_Inner;
END_VAR
inner(i_r := i_r);
q_r := inner.q_r;
END_FUNCTION_BLOCK

TEST_CASE 'nested user FB call works'
VAR
	o : FB_Outer;
END_VAR
	o(i_r := 1.0);
	ASSERT_NEAR(o.q_r, 2.0, 0.001, 'inner FB executed through outer');
END_TEST_CASE
`)
}

// Nested FB state must persist across calls of the outer FB -- an inner
// counter FB keeps its count between outer invocations.
func TestNestedUserFB_StatePersists(t *testing.T) {
	runInline(t, `
FUNCTION_BLOCK FB_Count
VAR_INPUT
	i_x : BOOL;
END_VAR
VAR_OUTPUT
	q_u : UDINT;
END_VAR
IF i_x THEN
	q_u := q_u + UDINT#1;
END_IF
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Wrap
VAR_INPUT
	i_x : BOOL;
END_VAR
VAR_OUTPUT
	q_u : UDINT;
END_VAR
VAR
	c : FB_Count;
END_VAR
c(i_x := i_x);
q_u := c.q_u;
END_FUNCTION_BLOCK

TEST_CASE 'nested FB state persists across outer calls'
VAR
	w : FB_Wrap;
END_VAR
	w(i_x := TRUE);
	w(i_x := TRUE);
	w(i_x := TRUE);
	ASSERT_EQ(w.q_u, UDINT#3, 'inner counter kept its state');
END_TEST_CASE
`)
}

// A stdlib FB (R_TRIG) nested inside a user FB must also be instantiated.
func TestNestedStdlibFB_Executes(t *testing.T) {
	runInline(t, `
FUNCTION_BLOCK FB_EdgeCount
VAR_INPUT
	i_x : BOOL;
END_VAR
VAR_OUTPUT
	q_u : UDINT;
END_VAR
VAR
	trig : R_TRIG;
END_VAR
trig(CLK := i_x);
IF trig.Q THEN
	q_u := q_u + UDINT#1;
END_IF
END_FUNCTION_BLOCK

TEST_CASE 'nested R_TRIG counts rising edges only'
VAR
	e : FB_EdgeCount;
END_VAR
	e(i_x := FALSE);
	e(i_x := TRUE);
	e(i_x := TRUE);
	e(i_x := FALSE);
	e(i_x := TRUE);
	ASSERT_EQ(e.q_u, UDINT#2, 'two rising edges');
END_TEST_CASE
`)
}

// Two levels of nesting: outer -> mid -> inner.
func TestDoublyNestedUserFB(t *testing.T) {
	runInline(t, `
FUNCTION_BLOCK FB_A
VAR_INPUT
	i_r : REAL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
q_r := i_r * 2.0;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_B
VAR_INPUT
	i_r : REAL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
VAR
	a : FB_A;
END_VAR
a(i_r := i_r);
q_r := a.q_r + 1.0;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_C
VAR_INPUT
	i_r : REAL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
VAR
	b : FB_B;
END_VAR
b(i_r := i_r);
q_r := b.q_r;
END_FUNCTION_BLOCK

TEST_CASE 'two levels of FB nesting'
VAR
	c : FB_C;
END_VAR
	c(i_r := 10.0);
	ASSERT_NEAR(c.q_r, 21.0, 0.001, '10*2+1 through two levels');
END_TEST_CASE
`)
}

// A runtime error inside an FB body must surface as a test error, not be
// silently swallowed leaving outputs stale. Swallowed errors turn real bugs
// into wrong-value mysteries.
func TestFBBodyError_Propagates(t *testing.T) {
	result := runInlineExpect(t, `
FUNCTION_BLOCK FB_Bad
VAR_INPUT
	i_x : BOOL;
END_VAR
VAR_OUTPUT
	q_r : REAL;
END_VAR
q_r := UNDEFINED_FUNCTION_XYZ(1.0);
END_FUNCTION_BLOCK

TEST_CASE 'error in FB body is reported'
VAR
	f : FB_Bad;
END_VAR
	f(i_x := TRUE);
	ASSERT_TRUE(TRUE, 'should not get here silently');
END_TEST_CASE
`)
	if !result.HasFailures() {
		t.Fatal("expected the FB body error to fail the test, but it passed silently")
	}
	found := false
	for _, suite := range result.Suites {
		for _, tc := range suite.Tests {
			if strings.Contains(tc.Error, "UNDEFINED_FUNCTION_XYZ") {
				found = true
			}
		}
	}
	if !found {
		t.Error("test failure should mention the undefined function that caused it")
	}
}
