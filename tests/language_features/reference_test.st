(* Reference Test Suite
   Tests REFERENCE TO with auto-dereference semantics.
*)

TEST_CASE 'Reference auto-dereferences on read'
VAR
    x : INT := 42;
    r : REFERENCE TO INT;
END_VAR
    r := REF(x);
    ASSERT_EQ(r, 42);
END_TEST_CASE

TEST_CASE 'Reference write-through modifies target'
VAR
    x : INT := 10;
    r : REFERENCE TO INT;
END_VAR
    r := REF(x);
    r := 99;
    ASSERT_EQ(x, 99);
END_TEST_CASE
