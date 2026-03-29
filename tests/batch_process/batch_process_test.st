(* Batch Process Test Suite
   Tests phase sequencing, analog value calculations, timer transitions,
   and string status reporting for a batch mixing process.
*)

TEST_CASE 'Batch starts in IDLE phase'
VAR
    Phase : INT := 0;
    InletValve : BOOL;
    BatchComplete : BOOL;
END_VAR
    ASSERT_EQ(Phase, 0);
    ASSERT_FALSE(InletValve);
    ASSERT_FALSE(BatchComplete);
END_TEST_CASE

TEST_CASE 'Fill phase ramps level to target'
VAR
    CurrentLevel : REAL := 0.0;
    TargetLevel : REAL := 80.0;
    FillRate : REAL := 5.0;
    InletValve : BOOL := TRUE;
    Phase : INT := 1;
    i : INT;
END_VAR
    (* Run filling for 16 scans: 16 * 5.0 = 80.0 *)
    FOR i := 1 TO 16 DO
        CurrentLevel := CurrentLevel + FillRate;
        IF CurrentLevel >= TargetLevel THEN
            CurrentLevel := TargetLevel;
            InletValve := FALSE;
            Phase := 2;
        END_IF;
    END_FOR;

    ASSERT_NEAR(CurrentLevel, 80.0, 0.01);
    ASSERT_FALSE(InletValve);
    ASSERT_EQ(Phase, 2);
END_TEST_CASE

TEST_CASE 'Mix timer completes after preset time'
VAR
    MixTimer : TON;
    Phase : INT := 2;
END_VAR
    MixTimer(IN := TRUE, PT := T#30s);
    ASSERT_FALSE(MixTimer.Q);

    ADVANCE_TIME(T#15s);
    MixTimer(IN := TRUE, PT := T#30s);
    ASSERT_FALSE(MixTimer.Q);

    ADVANCE_TIME(T#20s);
    MixTimer(IN := TRUE, PT := T#30s);
    ASSERT_TRUE(MixTimer.Q);

    (* Transition to heating *)
    IF MixTimer.Q THEN
        Phase := 3;
        MixTimer(IN := FALSE, PT := T#30s);
    END_IF;
    ASSERT_EQ(Phase, 3);
END_TEST_CASE

TEST_CASE 'Heating ramps temperature to target'
VAR
    CurrentTemp : REAL := 20.0;
    TargetTemp : REAL := 75.0;
    TempRampRate : REAL := 2.5;
    HeaterOn : BOOL := TRUE;
    Phase : INT := 3;
    scanCount : INT := 0;
END_VAR
    WHILE CurrentTemp < TargetTemp DO
        CurrentTemp := CurrentTemp + TempRampRate;
        scanCount := scanCount + 1;
        IF CurrentTemp >= TargetTemp THEN
            CurrentTemp := TargetTemp;
            HeaterOn := FALSE;
            Phase := 4;
        END_IF;
    END_WHILE;

    ASSERT_NEAR(CurrentTemp, 75.0, 0.01);
    ASSERT_FALSE(HeaterOn);
    ASSERT_EQ(Phase, 4);
    (* (75-20)/2.5 = 22 scans *)
    ASSERT_EQ(scanCount, 22);
END_TEST_CASE

TEST_CASE 'Drain phase empties vessel'
VAR
    CurrentLevel : REAL := 80.0;
    DrainRate : REAL := 3.0;
    OutletValve : BOOL := TRUE;
    Phase : INT := 4;
    i : INT;
END_VAR
    (* 80.0 / 3.0 = 26.67 scans, need 27 *)
    FOR i := 1 TO 30 DO
        CurrentLevel := CurrentLevel - DrainRate;
        IF CurrentLevel <= 0.0 THEN
            CurrentLevel := 0.0;
            OutletValve := FALSE;
            Phase := 5;
            EXIT;
        END_IF;
    END_FOR;

    ASSERT_NEAR(CurrentLevel, 0.0, 0.01);
    ASSERT_FALSE(OutletValve);
    ASSERT_EQ(Phase, 5);
END_TEST_CASE

TEST_CASE 'Status message reflects current phase'
VAR
    RecipeName : STRING := 'RECIPE_A';
    StatusMsg : STRING;
END_VAR
    (* IDLE status includes recipe name *)
    StatusMsg := CONCAT('IDLE: ', RecipeName);
    ASSERT_EQ(StatusMsg, 'IDLE: RECIPE_A');

    (* Running phases *)
    StatusMsg := 'FILLING';
    ASSERT_EQ(StatusMsg, 'FILLING');

    StatusMsg := 'MIXING';
    ASSERT_EQ(StatusMsg, 'MIXING');

    StatusMsg := 'HEATING';
    ASSERT_EQ(StatusMsg, 'HEATING');

    StatusMsg := 'DRAINING';
    ASSERT_EQ(StatusMsg, 'DRAINING');

    (* Completion *)
    StatusMsg := CONCAT('DONE: ', RecipeName);
    ASSERT_EQ(StatusMsg, 'DONE: RECIPE_A');
END_TEST_CASE

TEST_CASE 'Start edge only triggers once per press'
VAR
    StartEdge : R_TRIG;
    triggered : INT := 0;
END_VAR
    (* No press *)
    StartEdge(CLK := FALSE);
    IF StartEdge.Q THEN triggered := triggered + 1; END_IF;
    ASSERT_EQ(triggered, 0);

    (* Press start *)
    StartEdge(CLK := TRUE);
    IF StartEdge.Q THEN triggered := triggered + 1; END_IF;
    ASSERT_EQ(triggered, 1);

    (* Hold start (no re-trigger) *)
    StartEdge(CLK := TRUE);
    IF StartEdge.Q THEN triggered := triggered + 1; END_IF;
    ASSERT_EQ(triggered, 1);
END_TEST_CASE

TEST_CASE 'Temperature clamping at target'
VAR
    temp : REAL := 73.0;
    target : REAL := 75.0;
    ramp : REAL := 2.5;
END_VAR
    (* One step: 73 + 2.5 = 75.5, exceeds target *)
    temp := temp + ramp;
    IF temp >= target THEN
        temp := target;
    END_IF;
    ASSERT_NEAR(temp, 75.0, 0.01);
END_TEST_CASE

TEST_CASE 'Level clamping at zero during drain'
VAR
    level : REAL := 2.0;
    drainRate : REAL := 3.0;
END_VAR
    (* 2.0 - 3.0 = -1.0, should clamp to 0 *)
    level := level - drainRate;
    IF level <= 0.0 THEN
        level := 0.0;
    END_IF;
    ASSERT_NEAR(level, 0.0, 0.01);
END_TEST_CASE
