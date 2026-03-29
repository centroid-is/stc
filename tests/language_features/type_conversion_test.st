(* Type Conversion Test Suite
   Tests all standard type conversions between elementary types.
*)

TEST_CASE 'INT to REAL'
VAR
    i : INT := 42;
    r : REAL;
END_VAR
    r := INT_TO_REAL(i);
    ASSERT_NEAR(r, 42.0, 0.01);
END_TEST_CASE

TEST_CASE 'REAL to INT banker rounding'
VAR
    r : REAL := 2.5;
    i : INT;
END_VAR
    i := REAL_TO_INT(r);
    ASSERT_EQ(i, 2);  (* banker's rounding: 2.5 -> 2 *)
END_TEST_CASE
