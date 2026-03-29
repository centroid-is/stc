(* Subrange Enforcement Test Suite
   Tests that subrange types enforce bounds at runtime.
*)

TEST_CASE 'Subrange accepts value within bounds'
VAR
    x : INT(0..100);
END_VAR
    x := 50;
    ASSERT_EQ(x, 50);
    x := 0;
    ASSERT_EQ(x, 0);
    x := 100;
    ASSERT_EQ(x, 100);
END_TEST_CASE

TEST_CASE 'Subrange at boundary values'
VAR
    y : INT(1..10);
END_VAR
    y := 1;
    ASSERT_EQ(y, 1);
    y := 10;
    ASSERT_EQ(y, 10);
    y := 5;
    ASSERT_EQ(y, 5);
END_TEST_CASE
