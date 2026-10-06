package testing

import "testing"

func TestRunnerUserFunctions(t *testing.T) {
	t.Run("named arguments defaults and outputs", func(t *testing.T) {
		result := runGVLSuite(t, "fn_test.st", `
FUNCTION F_X : INT
VAR_INPUT a : INT; b : INT := 10; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
F_X := a + b;
q := a > 0;
END_FUNCTION

TEST_CASE 'named and defaults'
VAR flag : BOOL; END_VAR
ASSERT_EQ(F_X(a := 1, b := 2), 3);
ASSERT_EQ(F_X(1, 2), 3);
ASSERT_EQ(F_X(b := 2, a := 1), 3);
ASSERT_EQ(F_X(1, b := 2), 3);
ASSERT_EQ(F_X(a := 1), 11);
ASSERT_EQ(F_X(a := 1, q => flag), 11);
ASSERT_TRUE(flag);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})

	t.Run("function calling another function", func(t *testing.T) {
		result := runGVLSuite(t, "fn2_test.st", `
FUNCTION F_Sq : DINT
VAR_INPUT x : DINT; END_VAR
F_Sq := x * x;
END_FUNCTION

FUNCTION F_SumSq : DINT
VAR_INPUT a, b : DINT; END_VAR
F_SumSq := F_Sq(x := a) + F_Sq(b);
END_FUNCTION

TEST_CASE 'nested'
ASSERT_EQ(F_SumSq(b := 4, a := 3), 25);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})
}
