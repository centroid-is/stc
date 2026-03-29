(* Pointer and Reference Test Suite
   Tests POINTER TO, REFERENCE TO, dereferencing with ^, and ADR().
*)

TEST_CASE 'Pointer to INT'
VAR
    x : INT := 42;
    p : POINTER TO INT;
END_VAR
    p := ADR(x);
    ASSERT_EQ(p^, 42);
END_TEST_CASE

TEST_CASE 'Basic variable assignment without pointers'
VAR
    x : INT := 42;
    y : INT;
END_VAR
    y := x;
    ASSERT_EQ(y, 42);
END_TEST_CASE
