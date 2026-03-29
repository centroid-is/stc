(* Advanced Timer Test Suite
   Tests advanced timer patterns including re-triggering and sequencing.
*)

TEST_CASE 'TON with re-trigger'
VAR
    t : TON;
END_VAR
    (* Start timer *)
    t(IN := TRUE, PT := T#100ms);
    ADVANCE_TIME(T#50ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(t.Q);

    (* Drop and re-raise before done *)
    t(IN := FALSE, PT := T#100ms);
    ADVANCE_TIME(T#1ms);
    t(IN := FALSE, PT := T#100ms);
    t(IN := TRUE, PT := T#100ms);
    ADVANCE_TIME(T#100ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_TRUE(t.Q);

    (* ET should equal PT when done *)
    ASSERT_EQ(t.ET, T#100ms);
END_TEST_CASE

TEST_CASE 'Multiple timers in sequence'
VAR
    t1 : TON;
    t2 : TON;
END_VAR
    t1(IN := TRUE, PT := T#50ms);
    ADVANCE_TIME(T#60ms);
    t1(IN := TRUE, PT := T#50ms);
    ASSERT_TRUE(t1.Q);

    (* Start t2 after t1 fires *)
    t2(IN := t1.Q, PT := T#30ms);
    ADVANCE_TIME(T#40ms);
    t2(IN := t1.Q, PT := T#30ms);
    ASSERT_TRUE(t2.Q);
END_TEST_CASE
