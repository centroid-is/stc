package testing

import (
	"strings"
	"testing"
)

func TestRunnerCallDepth(t *testing.T) {
	t.Run("recursive FUNCTION fails at the call depth limit", func(t *testing.T) {
		result := runGVLSuite(t, "rec_test.st", `
FUNCTION F_Rec : DINT
VAR_INPUT n : DINT; END_VAR
F_Rec := F_Rec(n + 1);
END_FUNCTION

TEST_CASE 'recursion'
VAR r : DINT; END_VAR
r := F_Rec(0);
END_TEST_CASE
`)
		if result.Total != 1 || result.Passed != 0 {
			t.Fatalf("want 1 failing case, got %d passed of %d", result.Passed, result.Total)
		}
		got := result.Suites[0].Tests[0].Error
		if !strings.Contains(got, "maximum call depth 256 exceeded calling F_REC") {
			t.Fatalf("unexpected error: %q", got)
		}
	})

	t.Run("FB calling its own GVL instance fails only that case", func(t *testing.T) {
		result := runGVLSuite(t, "self_test.st", `
FUNCTION_BLOCK FB_Self
VAR n : DINT; END_VAR
n := n + 1;
g();
END_FUNCTION_BLOCK

VAR_GLOBAL
g : FB_Self;
END_VAR

TEST_CASE 'self call'
g();
END_TEST_CASE

TEST_CASE 'still runs'
ASSERT_EQ(1, 1);
END_TEST_CASE
`)
		if result.Total != 2 || result.Passed != 1 {
			t.Fatalf("want 1 of 2 passing, got %d passed of %d", result.Passed, result.Total)
		}
		got := result.Suites[0].Tests[0].Error
		if !strings.Contains(got, "maximum call depth 256 exceeded calling FB_Self") {
			t.Fatalf("unexpected error: %q", got)
		}
	})

	t.Run("derived FB in a suite calls the base FB action", func(t *testing.T) {
		result := runGVLSuite(t, "ext_test.st", `
FUNCTION_BLOCK FB_Base
VAR_OUTPUT n : DINT; END_VAR
ACTION bump
n := n + 10;
END_ACTION
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
bump();
END_FUNCTION_BLOCK

TEST_CASE 'inherited action'
VAR d : FB_Derived; END_VAR
d();
d.bump();
ASSERT_EQ(d.n, 20);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})
}
