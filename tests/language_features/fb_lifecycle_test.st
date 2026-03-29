(* Function Block Lifecycle Test Suite
   Tests FB instantiation, state persistence across calls, and multiple instances.
*)

FUNCTION_BLOCK FB_Counter
VAR
    count : INT := 0;
END_VAR
VAR_INPUT
    increment : BOOL;
END_VAR
    IF increment THEN
        count := count + 1;
    END_IF;
END_FUNCTION_BLOCK

TEST_CASE 'FB state persists across calls'
VAR
    c : FB_Counter;
END_VAR
    c(increment := TRUE);
    c(increment := TRUE);
    c(increment := TRUE);
    ASSERT_EQ(c.count, 3);
END_TEST_CASE

TEST_CASE 'Multiple FB instances are independent'
VAR
    c1 : FB_Counter;
    c2 : FB_Counter;
END_VAR
    c1(increment := TRUE);
    c1(increment := TRUE);
    c2(increment := TRUE);
    ASSERT_EQ(c1.count, 2);
    ASSERT_EQ(c2.count, 1);
END_TEST_CASE
