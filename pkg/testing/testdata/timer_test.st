TEST_CASE 'TON timer fires after preset'
VAR
    timer : TON;
    done : BOOL;
END_VAR
    timer(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(timer.Q);

    ADVANCE_TIME(T#50ms);
    timer(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(timer.Q);

    ADVANCE_TIME(T#60ms);
    timer(IN := TRUE, PT := T#100ms);
    ASSERT_TRUE(timer.Q);
END_TEST_CASE
