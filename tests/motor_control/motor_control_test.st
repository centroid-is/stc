(* Motor Control Test Suite
   Exercises start/stop sequencing, interlock logic, E-stop, speed ramp,
   and fault counting for a realistic industrial motor control program.
*)

TEST_CASE 'Motor does not start without interlocks'
VAR
    StartEdge : R_TRIG;
    AllInterlocksOK : BOOL;
    MotorRun : BOOL;
    StartCommand : BOOL;
    EStopOK : BOOL := FALSE;
    OverloadOK : BOOL := TRUE;
    LubeOK : BOOL := TRUE;
    StopPB : BOOL := TRUE;
    StartPB : BOOL;
END_VAR
    (* E-stop not healthy, interlocks fail *)
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;
    ASSERT_FALSE(AllInterlocksOK);

    (* Press start button *)
    StartPB := TRUE;
    StartEdge(CLK := StartPB);

    (* Start command should NOT latch because interlocks are bad *)
    IF StartEdge.Q AND AllInterlocksOK THEN
        StartCommand := TRUE;
    END_IF;
    ASSERT_FALSE(StartCommand);

    MotorRun := StartCommand AND AllInterlocksOK;
    ASSERT_FALSE(MotorRun);
END_TEST_CASE

TEST_CASE 'Motor starts with all interlocks OK'
VAR
    StartEdge : R_TRIG;
    AllInterlocksOK : BOOL;
    MotorRun : BOOL;
    StartCommand : BOOL;
    EStopOK : BOOL := TRUE;
    OverloadOK : BOOL := TRUE;
    LubeOK : BOOL := TRUE;
    StopPB : BOOL := TRUE;
    StartPB : BOOL;
END_VAR
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;
    ASSERT_TRUE(AllInterlocksOK);

    (* First scan: CLK=FALSE, no edge yet *)
    StartPB := FALSE;
    StartEdge(CLK := StartPB);

    (* Second scan: CLK=TRUE, rising edge detected *)
    StartPB := TRUE;
    StartEdge(CLK := StartPB);
    ASSERT_TRUE(StartEdge.Q);

    (* Latch start command *)
    IF StartEdge.Q AND AllInterlocksOK THEN
        StartCommand := TRUE;
    END_IF;
    ASSERT_TRUE(StartCommand);

    MotorRun := StartCommand AND AllInterlocksOK;
    ASSERT_TRUE(MotorRun);
END_TEST_CASE

TEST_CASE 'E-stop kills running motor'
VAR
    StartEdge : R_TRIG;
    AllInterlocksOK : BOOL;
    MotorRun : BOOL;
    StartCommand : BOOL;
    EStopOK : BOOL := TRUE;
    OverloadOK : BOOL := TRUE;
    LubeOK : BOOL := TRUE;
    StopPB : BOOL := TRUE;
END_VAR
    (* Get motor running first *)
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;
    StartEdge(CLK := FALSE);
    StartEdge(CLK := TRUE);
    IF StartEdge.Q AND AllInterlocksOK THEN
        StartCommand := TRUE;
    END_IF;
    MotorRun := StartCommand AND AllInterlocksOK;
    ASSERT_TRUE(MotorRun);

    (* Now trip E-stop *)
    EStopOK := FALSE;
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;
    ASSERT_FALSE(AllInterlocksOK);

    (* Clear start command on interlock loss *)
    IF NOT AllInterlocksOK THEN
        StartCommand := FALSE;
    END_IF;
    MotorRun := StartCommand AND AllInterlocksOK;
    ASSERT_FALSE(MotorRun);
END_TEST_CASE

TEST_CASE 'Speed ramps up while running'
VAR
    SpeedRef : REAL := 0.0;
    SpeedSetpoint : REAL := 100.0;
    RampStep : REAL := 5.0;
    MotorRun : BOOL := TRUE;
    i : INT;
END_VAR
    (* Simulate 20 scan cycles of ramping *)
    FOR i := 1 TO 20 DO
        IF MotorRun THEN
            IF SpeedRef < SpeedSetpoint THEN
                SpeedRef := SpeedRef + RampStep;
                IF SpeedRef > SpeedSetpoint THEN
                    SpeedRef := SpeedSetpoint;
                END_IF;
            END_IF;
        END_IF;
    END_FOR;

    ASSERT_NEAR(SpeedRef, 100.0, 0.01);
END_TEST_CASE

TEST_CASE 'Speed resets to zero when motor stops'
VAR
    SpeedRef : REAL := 75.0;
    MotorRun : BOOL := FALSE;
END_VAR
    IF NOT MotorRun THEN
        SpeedRef := 0.0;
    END_IF;
    ASSERT_NEAR(SpeedRef, 0.0, 0.01);
END_TEST_CASE

TEST_CASE 'Fault counter counts interlock trips'
VAR
    FaultCount : CTU;
    Fault : BOOL;
    interlockBad : BOOL;
END_VAR
    (* Reset state *)
    FaultCount(CU := FALSE, R := FALSE, PV := 3);

    (* First trip *)
    interlockBad := TRUE;
    FaultCount(CU := interlockBad, R := FALSE, PV := 3);
    ASSERT_FALSE(FaultCount.Q);
    ASSERT_EQ(FaultCount.CV, 1);

    (* Clear and re-trip for edge detection *)
    FaultCount(CU := FALSE, R := FALSE, PV := 3);
    FaultCount(CU := TRUE, R := FALSE, PV := 3);
    ASSERT_EQ(FaultCount.CV, 2);

    (* Third trip reaches preset *)
    FaultCount(CU := FALSE, R := FALSE, PV := 3);
    FaultCount(CU := TRUE, R := FALSE, PV := 3);
    ASSERT_EQ(FaultCount.CV, 3);
    ASSERT_TRUE(FaultCount.Q);
    Fault := FaultCount.Q;
    ASSERT_TRUE(Fault);
END_TEST_CASE

TEST_CASE 'Startup timer tracks running time'
VAR
    StartupTmr : TON;
END_VAR
    (* Motor starts running *)
    StartupTmr(IN := TRUE, PT := T#5s);
    ASSERT_FALSE(StartupTmr.Q);

    (* Advance 3 seconds *)
    ADVANCE_TIME(T#3s);
    StartupTmr(IN := TRUE, PT := T#5s);
    ASSERT_FALSE(StartupTmr.Q);

    (* Advance 3 more seconds, total 6s > 5s preset *)
    ADVANCE_TIME(T#3s);
    StartupTmr(IN := TRUE, PT := T#5s);
    ASSERT_TRUE(StartupTmr.Q);
END_TEST_CASE

TEST_CASE 'Rising edge only fires once per press'
VAR
    StartEdge : R_TRIG;
END_VAR
    (* Initial state *)
    StartEdge(CLK := FALSE);
    ASSERT_FALSE(StartEdge.Q);

    (* Press: rising edge fires *)
    StartEdge(CLK := TRUE);
    ASSERT_TRUE(StartEdge.Q);

    (* Hold: no new edge *)
    StartEdge(CLK := TRUE);
    ASSERT_FALSE(StartEdge.Q);

    (* Release and re-press *)
    StartEdge(CLK := FALSE);
    ASSERT_FALSE(StartEdge.Q);
    StartEdge(CLK := TRUE);
    ASSERT_TRUE(StartEdge.Q);
END_TEST_CASE
