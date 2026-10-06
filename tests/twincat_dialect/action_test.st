(* TwinCAT dialect: ACTIONs on a FUNCTION_BLOCK.
   A_Step is declared inside the FB, A_Reset after END_FUNCTION_BLOCK; both
   belong to FB_Counter and run against the instance's own variables, whether
   the FB body calls them or a caller writes inst.A_Step().
*)

FUNCTION_BLOCK FB_Counter
VAR_INPUT
    step : INT;
END_VAR
VAR_OUTPUT
    n : INT;
    edges : INT;
END_VAR
VAR
    edge : R_TRIG;
    pulse : BOOL;
END_VAR
    A_Step();

ACTION A_Step
    n := n + step;
    edge(CLK := pulse);
    IF edge.Q THEN
        edges := edges + 1;
    END_IF
END_ACTION
END_FUNCTION_BLOCK

ACTION A_Reset
    n := 0;
END_ACTION

TEST_CASE 'FB body runs its action on every call'
VAR
    c : FB_Counter;
END_VAR
    c(step := 2);
    c(step := 2);
    ASSERT_EQ(c.n, 4);
END_TEST_CASE

TEST_CASE 'External call to an action declared after the FB'
VAR
    c : FB_Counter;
END_VAR
    c(step := 3);
    ASSERT_EQ(c.n, 3);
    c.A_Reset();
    ASSERT_EQ(c.n, 0);
END_TEST_CASE

TEST_CASE 'External call to an action declared inside the FB'
VAR
    c : FB_Counter;
END_VAR
    c(step := 5);
    c.A_Step();
    ASSERT_EQ(c.n, 10);
END_TEST_CASE

TEST_CASE 'R_TRIG inside an action keeps state across calls'
VAR
    c : FB_Counter;
END_VAR
    c.pulse := TRUE;
    c(step := 1);
    c(step := 1);
    ASSERT_EQ(c.edges, 1);
    c.pulse := FALSE;
    c(step := 1);
    c.pulse := TRUE;
    c(step := 1);
    ASSERT_EQ(c.edges, 2);
    ASSERT_EQ(c.n, 4);
END_TEST_CASE
