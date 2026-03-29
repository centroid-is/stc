(* Vendor-Portable Test Suite
   Tests the proportional controller, analog scaling,
   and deadband functions using the portable subset of ST.
*)

TEST_CASE 'Proportional controller basic output'
VAR
    Error : REAL;
    RawOutput : REAL;
    Output : REAL;
    Kp : REAL := 2.0;
    Setpoint : REAL := 50.0;
    ProcessVar : REAL := 30.0;
    OutputMin : REAL := 0.0;
    OutputMax : REAL := 100.0;
END_VAR
    Error := Setpoint - ProcessVar;
    ASSERT_NEAR(Error, 20.0, 0.01);

    RawOutput := Kp * Error;
    ASSERT_NEAR(RawOutput, 40.0, 0.01);

    (* Within limits, no clamping *)
    IF RawOutput > OutputMax THEN
        Output := OutputMax;
    ELSIF RawOutput < OutputMin THEN
        Output := OutputMin;
    ELSE
        Output := RawOutput;
    END_IF;
    ASSERT_NEAR(Output, 40.0, 0.01);
END_TEST_CASE

TEST_CASE 'Controller output clamps at maximum'
VAR
    Error : REAL;
    RawOutput : REAL;
    Output : REAL;
    Saturated : BOOL;
    Kp : REAL := 10.0;
    Setpoint : REAL := 80.0;
    ProcessVar : REAL := 20.0;
    OutputMax : REAL := 100.0;
    OutputMin : REAL := 0.0;
END_VAR
    Error := Setpoint - ProcessVar;
    RawOutput := Kp * Error;
    (* 10.0 * 60.0 = 600.0, way above max *)
    ASSERT_TRUE(RawOutput > OutputMax);

    IF RawOutput > OutputMax THEN
        Output := OutputMax;
        Saturated := TRUE;
    ELSIF RawOutput < OutputMin THEN
        Output := OutputMin;
        Saturated := TRUE;
    ELSE
        Output := RawOutput;
        Saturated := FALSE;
    END_IF;

    ASSERT_NEAR(Output, 100.0, 0.01);
    ASSERT_TRUE(Saturated);
END_TEST_CASE

TEST_CASE 'Controller output clamps at minimum'
VAR
    Error : REAL;
    RawOutput : REAL;
    Output : REAL;
    Saturated : BOOL;
    Kp : REAL := 5.0;
    Setpoint : REAL := 10.0;
    ProcessVar : REAL := 90.0;
    OutputMax : REAL := 100.0;
    OutputMin : REAL := 0.0;
END_VAR
    Error := Setpoint - ProcessVar;
    RawOutput := Kp * Error;
    (* 5.0 * (-80.0) = -400.0, below min *)
    ASSERT_TRUE(RawOutput < OutputMin);

    IF RawOutput > OutputMax THEN
        Output := OutputMax;
        Saturated := TRUE;
    ELSIF RawOutput < OutputMin THEN
        Output := OutputMin;
        Saturated := TRUE;
    ELSE
        Output := RawOutput;
        Saturated := FALSE;
    END_IF;

    ASSERT_NEAR(Output, 0.0, 0.01);
    ASSERT_TRUE(Saturated);
END_TEST_CASE

TEST_CASE 'Manual mode bypasses control calculation'
VAR
    ManualMode : BOOL := TRUE;
    ManualValue : REAL := 65.0;
    Output : REAL;
    Error : REAL;
END_VAR
    IF ManualMode THEN
        Output := ManualValue;
        Error := 0.0;
    END_IF;

    ASSERT_NEAR(Output, 65.0, 0.01);
    ASSERT_NEAR(Error, 0.0, 0.01);
END_TEST_CASE

TEST_CASE 'Disabled controller outputs zero'
VAR
    Enable : BOOL := FALSE;
    Output : REAL := 50.0;
    Error : REAL := 10.0;
    Saturated : BOOL := TRUE;
END_VAR
    IF NOT Enable THEN
        Output := 0.0;
        Error := 0.0;
        Saturated := FALSE;
    END_IF;

    ASSERT_NEAR(Output, 0.0, 0.01);
    ASSERT_NEAR(Error, 0.0, 0.01);
    ASSERT_FALSE(Saturated);
END_TEST_CASE

TEST_CASE 'Analog scaling: raw 0..32767 to 0..100'
VAR
    RawValue : INT := 16384;
    RawMin : INT := 0;
    RawMax : INT := 32767;
    EngMin : REAL := 0.0;
    EngMax : REAL := 100.0;
    RawSpan : REAL;
    EngSpan : REAL;
    Fraction : REAL;
    Result : REAL;
END_VAR
    RawSpan := INT_TO_REAL(RawMax - RawMin);
    EngSpan := EngMax - EngMin;
    Fraction := INT_TO_REAL(RawValue - RawMin) / RawSpan;
    Result := EngMin + (Fraction * EngSpan);

    (* 16384/32767 * 100 ~ 50.0015 *)
    ASSERT_NEAR(Result, 50.0, 0.1);
END_TEST_CASE

TEST_CASE 'Analog scaling: offset range'
VAR
    RawValue : INT := 16384;
    RawMin : INT := 4000;
    RawMax : INT := 20000;
    EngMin : REAL := -20.0;
    EngMax : REAL := 120.0;
    RawSpan : REAL;
    EngSpan : REAL;
    Fraction : REAL;
    Result : REAL;
END_VAR
    RawSpan := INT_TO_REAL(RawMax - RawMin);
    EngSpan := EngMax - EngMin;
    Fraction := INT_TO_REAL(RawValue - RawMin) / RawSpan;
    Result := EngMin + (Fraction * EngSpan);

    (* (16384-4000)/(20000-4000) * 140 + (-20) = 12384/16000 * 140 - 20 = 108.36 - 20 = 88.36 *)
    ASSERT_NEAR(Result, 88.36, 0.5);
END_TEST_CASE

TEST_CASE 'Deadband: value within band'
VAR
    Value : REAL := 50.2;
    Target : REAL := 50.0;
    Deadband : REAL := 0.5;
    result : BOOL;
END_VAR
    IF ABS(Value - Target) <= Deadband THEN
        result := TRUE;
    ELSE
        result := FALSE;
    END_IF;
    ASSERT_TRUE(result);
END_TEST_CASE

TEST_CASE 'Deadband: value outside band'
VAR
    Value : REAL := 52.0;
    Target : REAL := 50.0;
    Deadband : REAL := 0.5;
    result : BOOL;
END_VAR
    IF ABS(Value - Target) <= Deadband THEN
        result := TRUE;
    ELSE
        result := FALSE;
    END_IF;
    ASSERT_FALSE(result);
END_TEST_CASE

TEST_CASE 'Controller reaches setpoint in closed loop simulation'
VAR
    Setpoint : REAL := 50.0;
    ProcessVar : REAL := 0.0;
    Kp : REAL := 0.5;
    Output : REAL;
    Error : REAL;
    i : INT;
END_VAR
    (* Simulate 50 iterations of proportional control *)
    FOR i := 1 TO 50 DO
        Error := Setpoint - ProcessVar;
        Output := Kp * Error;
        IF Output > 100.0 THEN
            Output := 100.0;
        ELSIF Output < 0.0 THEN
            Output := 0.0;
        END_IF;
        (* Process model: process responds proportionally to output *)
        ProcessVar := ProcessVar + Output * 0.1;
    END_FOR;

    (* After 50 iterations, should be close to setpoint *)
    ASSERT_NEAR(ProcessVar, 50.0, 1.0);
END_TEST_CASE
