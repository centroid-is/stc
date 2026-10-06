package testing

import "testing"

func TestEnumRegister(t *testing.T) {
	result := runGVLSuite(t, "enum_register_test.st", `
TYPE E : (tun := 0, rdy := 2, nst) UINT; END_TYPE
TYPE H : (a := 16#0006, b); END_TYPE
{attribute 'to_string'}
TYPE S : (idle, run, stop); END_TYPE

TEST_CASE 'previous plus one numbering'
ASSERT_EQ(E.nst, 3);
ASSERT_EQ(E.rdy, 2);
ASSERT_EQ(nst, 3);
END_TEST_CASE

TEST_CASE 'hex explicit value'
ASSERT_EQ(H.a, 6);
ASSERT_EQ(H.b, 7);
END_TEST_CASE

TEST_CASE 'to_string attribute'
VAR sv : S; n : E; END_VAR
sv := S.run;
ASSERT_EQ(TO_STRING(sv), 'run');
n := E.nst;
ASSERT_EQ(TO_STRING(n), '3');
END_TEST_CASE

TEST_CASE 'inline enum in a test case'
VAR eStep : (E_IDLE, E_RUN) := E_RUN; END_VAR
ASSERT_TRUE(eStep = E_RUN);
eStep := E_IDLE;
ASSERT_EQ(eStep, 0);
END_TEST_CASE
`)
	requireAllPassed(t, result, 4)
}
