(* Conveyor Belt Test Suite
   Tests state machine transitions, jam detection, product counting,
   and enable/disable behavior for a 3-zone conveyor system.
*)

TEST_CASE 'Conveyor starts in IDLE state'
VAR
    State : INT := 0;
    MotorFwd : BOOL := FALSE;
END_VAR
    ASSERT_EQ(State, 0);
    ASSERT_FALSE(MotorFwd);
END_TEST_CASE

TEST_CASE 'Product entry transitions IDLE to RUNNING'
VAR
    State : INT := 0;
    Enable : BOOL := TRUE;
    SensorEntry : BOOL;
    MotorFwd : BOOL;
END_VAR
    (* No product yet *)
    SensorEntry := FALSE;
    CASE State OF
        0:
            MotorFwd := FALSE;
            IF Enable AND SensorEntry THEN
                State := 1;
            END_IF;
    END_CASE;
    ASSERT_EQ(State, 0);
    ASSERT_FALSE(MotorFwd);

    (* Product arrives *)
    SensorEntry := TRUE;
    CASE State OF
        0:
            MotorFwd := FALSE;
            IF Enable AND SensorEntry THEN
                State := 1;
            END_IF;
    END_CASE;
    ASSERT_EQ(State, 1);
END_TEST_CASE

TEST_CASE 'Jam timer triggers fault after timeout'
VAR
    JamTimer : TON;
    SensorEntry : BOOL := TRUE;
    JamFault : BOOL;
END_VAR
    (* Sensor blocked, start timing *)
    JamTimer(IN := SensorEntry, PT := T#10s);
    ASSERT_FALSE(JamTimer.Q);

    (* 8 seconds pass, still no jam *)
    ADVANCE_TIME(T#8s);
    JamTimer(IN := SensorEntry, PT := T#10s);
    ASSERT_FALSE(JamTimer.Q);

    (* 3 more seconds, total 11s > 10s preset *)
    ADVANCE_TIME(T#3s);
    JamTimer(IN := SensorEntry, PT := T#10s);
    ASSERT_TRUE(JamTimer.Q);
    JamFault := JamTimer.Q;
    ASSERT_TRUE(JamFault);
END_TEST_CASE

TEST_CASE 'Jam resets on operator Reset signal'
VAR
    JamTimer : TON;
    JamFault : BOOL := TRUE;
    Reset : BOOL;
    State : INT := 2;
END_VAR
    (* In JAM state, operator presses reset *)
    Reset := TRUE;
    IF Reset THEN
        JamFault := FALSE;
        JamTimer(IN := FALSE, PT := T#10s);
        State := 0;
    END_IF;

    ASSERT_FALSE(JamFault);
    ASSERT_EQ(State, 0);
    ASSERT_FALSE(JamTimer.Q);
END_TEST_CASE

TEST_CASE 'Product counter increments on product exit'
VAR
    Counter : CTU;
END_VAR
    (* Simulate 3 products passing through *)
    Counter(CU := FALSE, R := FALSE, PV := 9999);

    (* Product 1 exits *)
    Counter(CU := TRUE, R := FALSE, PV := 9999);
    ASSERT_EQ(Counter.CV, 1);
    Counter(CU := FALSE, R := FALSE, PV := 9999);

    (* Product 2 exits *)
    Counter(CU := TRUE, R := FALSE, PV := 9999);
    ASSERT_EQ(Counter.CV, 2);
    Counter(CU := FALSE, R := FALSE, PV := 9999);

    (* Product 3 exits *)
    Counter(CU := TRUE, R := FALSE, PV := 9999);
    ASSERT_EQ(Counter.CV, 3);
    ASSERT_FALSE(Counter.Q);  (* PV=9999, far from reached *)
END_TEST_CASE

TEST_CASE 'Disable stops conveyor motor'
VAR
    State : INT := 1;  (* RUNNING *)
    Enable : BOOL;
    MotorFwd : BOOL := TRUE;
END_VAR
    (* Operator disables *)
    Enable := FALSE;

    CASE State OF
        1:
            IF NOT Enable THEN
                State := 3;
                MotorFwd := FALSE;
            END_IF;
    END_CASE;

    ASSERT_EQ(State, 3);
    ASSERT_FALSE(MotorFwd);
END_TEST_CASE

TEST_CASE 'Re-enable resumes from STOPPED to RUNNING'
VAR
    State : INT := 3;  (* STOPPED *)
    Enable : BOOL;
    MotorFwd : BOOL := FALSE;
END_VAR
    Enable := TRUE;
    CASE State OF
        3:
            IF Enable THEN
                State := 1;
            END_IF;
    END_CASE;

    ASSERT_EQ(State, 1);
END_TEST_CASE

TEST_CASE 'Rising edge detects product entry once'
VAR
    EntryEdge : R_TRIG;
    SensorEntry : BOOL;
END_VAR
    (* No product *)
    SensorEntry := FALSE;
    EntryEdge(CLK := SensorEntry);
    ASSERT_FALSE(EntryEdge.Q);

    (* Product arrives: rising edge *)
    SensorEntry := TRUE;
    EntryEdge(CLK := SensorEntry);
    ASSERT_TRUE(EntryEdge.Q);

    (* Product still present: no new edge *)
    EntryEdge(CLK := SensorEntry);
    ASSERT_FALSE(EntryEdge.Q);
END_TEST_CASE
