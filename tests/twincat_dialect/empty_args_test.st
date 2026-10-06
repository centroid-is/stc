(* TwinCAT dialect: empty formal arguments.
   TwinCAT writes unused outputs of a call as "Q => ," and treats them as
   omitted. t1 is called with empty output bindings and t2 without them; both
   timers must behave identically.
*)

TEST_CASE 'Empty output bindings run like omitted ones'
VAR
    t1 : TON;
    t2 : TON;
    i : INT;
END_VAR
    FOR i := 1 TO 5 DO
        t1(IN := TRUE, PT := T#100MS, Q => , ET => );
        t2(IN := TRUE, PT := T#100MS);
        ADVANCE_TIME(T#10MS);
    END_FOR
    (* 50 ms elapsed *)
    t1(IN := TRUE, PT := T#100MS, Q => , ET => );
    t2(IN := TRUE, PT := T#100MS);
    ASSERT_FALSE(t1.Q);
    ASSERT_EQ(t1.Q, t2.Q);
    ASSERT_EQ(t1.ET, t2.ET);
    ASSERT_EQ(t1.ET, T#50MS);
END_TEST_CASE

TEST_CASE 'Empty bindings still let the timer elapse'
VAR
    t1 : TON;
    t2 : TON;
    i : INT;
END_VAR
    FOR i := 1 TO 15 DO
        t1(IN := TRUE, PT := T#100MS, Q => , ET => );
        t2(IN := TRUE, PT := T#100MS);
        ADVANCE_TIME(T#10MS);
    END_FOR
    (* 150 ms elapsed *)
    t1(IN := TRUE, PT := T#100MS, Q => , ET => );
    t2(IN := TRUE, PT := T#100MS);
    ASSERT_TRUE(t1.Q);
    ASSERT_EQ(t1.Q, t2.Q);
    ASSERT_EQ(t1.ET, t2.ET);
END_TEST_CASE
