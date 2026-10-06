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

// TestEnumConstOrdinals covers review ME-02 at runtime: enum values named by
// a GVL constant are numbered from the constant, although the runner
// registers enums before GVLs, and inline enums use the POU's constants.
func TestEnumConstOrdinals(t *testing.T) {
	result := runGVLSuite(t, "enum_const_test.st", `
VAR_GLOBAL CONSTANT
	C_BASE : INT := 10;
END_VAR
TYPE E_K : (ka := C_BASE, kb, kc := C_BASE * 3 + 1, kd); END_TYPE

TEST_CASE 'values from a GVL constant'
ASSERT_EQ(TO_INT(E_K.ka), 10);
ASSERT_EQ(TO_INT(E_K.kb), 11);
ASSERT_EQ(TO_INT(kd), 32);
END_TEST_CASE

TEST_CASE 'inline enum from a local constant'
VAR CONSTANT C_L : INT := 4; END_VAR
VAR m : (x := C_L, y); END_VAR
m := y;
ASSERT_EQ(TO_INT(m), 5);
END_TEST_CASE
`)
	requireAllPassed(t, result, 2)
}
