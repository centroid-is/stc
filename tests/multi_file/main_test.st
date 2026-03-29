(* Multi-File Test Suite
   Tests that FUNCTION_BLOCK and FUNCTION declarations in the same file
   are available to TEST_CASE blocks.

   NOTE: The current test runner processes each _test.st file independently,
   so cross-file declarations (from common.st) are NOT available here.
   Instead, we redeclare the needed POUs inline.
*)

FUNCTION_BLOCK FB_PID
VAR_INPUT
    SetPoint : REAL;
    ProcessValue : REAL;
    Kp : REAL;
END_VAR
VAR_OUTPUT
    ControlOutput : REAL;
END_VAR
VAR
    error : REAL;
END_VAR
    error := SetPoint - ProcessValue;
    ControlOutput := error * Kp;
END_FUNCTION_BLOCK

FUNCTION Clamp : REAL
VAR_INPUT
    val : REAL;
    lo : REAL;
    hi : REAL;
END_VAR
    IF val < lo THEN
        Clamp := lo;
    ELSIF val > hi THEN
        Clamp := hi;
    ELSE
        Clamp := val;
    END_IF;
END_FUNCTION

TEST_CASE 'PID controller proportional output'
VAR
    pid : FB_PID;
END_VAR
    pid(SetPoint := 100.0, ProcessValue := 80.0, Kp := 2.0);
    ASSERT_NEAR(pid.ControlOutput, 40.0, 0.001);  (* (100-80) * 2 = 40 *)
END_TEST_CASE

TEST_CASE 'PID controller zero error'
VAR
    pid : FB_PID;
END_VAR
    pid(SetPoint := 50.0, ProcessValue := 50.0, Kp := 5.0);
    ASSERT_NEAR(pid.ControlOutput, 0.0, 0.001);
END_TEST_CASE

TEST_CASE 'PID controller negative error'
VAR
    pid : FB_PID;
END_VAR
    pid(SetPoint := 30.0, ProcessValue := 50.0, Kp := 1.0);
    ASSERT_NEAR(pid.ControlOutput, -20.0, 0.001);
END_TEST_CASE

TEST_CASE 'Clamp function limits output'
VAR
    r : REAL;
END_VAR
    r := Clamp(50.0, 0.0, 100.0);
    ASSERT_NEAR(r, 50.0, 0.001);

    r := Clamp(-10.0, 0.0, 100.0);
    ASSERT_NEAR(r, 0.0, 0.001);

    r := Clamp(200.0, 0.0, 100.0);
    ASSERT_NEAR(r, 100.0, 0.001);
END_TEST_CASE

TEST_CASE 'PID with clamped output'
VAR
    pid : FB_PID;
    clamped : REAL;
END_VAR
    pid(SetPoint := 100.0, ProcessValue := 0.0, Kp := 10.0);
    (* Raw output: 1000.0, clamp to 0..100 *)
    clamped := Clamp(pid.ControlOutput, 0.0, 100.0);
    ASSERT_NEAR(clamped, 100.0, 0.001);
END_TEST_CASE
