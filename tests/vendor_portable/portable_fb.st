(* Vendor-Portable Function Block
   Demonstrates a function block that uses preprocessor directives
   for multi-vendor compatibility. The portable subset runs on all
   platforms; vendor-specific sections are conditionally compiled.

   This FB implements a simple PID-like proportional controller
   with configurable parameters and output clamping.
*)

FUNCTION_BLOCK ProportionalController
VAR_INPUT
    Setpoint    : REAL;       (* Desired value *)
    ProcessVar  : REAL;       (* Measured value *)
    Kp          : REAL := 1.0;  (* Proportional gain *)
    OutputMin   : REAL := 0.0;
    OutputMax   : REAL := 100.0;
    ManualMode  : BOOL;
    ManualValue : REAL := 0.0;
    Enable      : BOOL := TRUE;
END_VAR

VAR_OUTPUT
    Output      : REAL;       (* Control output *)
    Error       : REAL;       (* Current error *)
    Saturated   : BOOL;       (* Output is at limit *)
    Active      : BOOL;       (* Controller is running *)
END_VAR

VAR
    RawOutput   : REAL;
END_VAR

    Active := Enable;

    IF NOT Enable THEN
        Output := 0.0;
        Error := 0.0;
        Saturated := FALSE;
        RETURN;
    END_IF;

    IF ManualMode THEN
        Output := ManualValue;
        Error := 0.0;
    ELSE
        (* Calculate error and proportional output *)
        Error := Setpoint - ProcessVar;
        RawOutput := Kp * Error;

        (* Clamp output to limits *)
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
    END_IF;
END_FUNCTION_BLOCK

(* Helper function: Scale analog value from raw to engineering units *)
FUNCTION ScaleAnalog : REAL
VAR_INPUT
    RawValue  : INT;     (* Raw ADC count, e.g., 0..32767 *)
    RawMin    : INT := 0;
    RawMax    : INT := 32767;
    EngMin    : REAL := 0.0;
    EngMax    : REAL := 100.0;
END_VAR
VAR
    RawSpan   : REAL;
    EngSpan   : REAL;
    Fraction  : REAL;
END_VAR
    RawSpan := INT_TO_REAL(RawMax - RawMin);
    EngSpan := EngMax - EngMin;
    Fraction := INT_TO_REAL(RawValue - RawMin) / RawSpan;
    ScaleAnalog := EngMin + (Fraction * EngSpan);
END_FUNCTION

(* Helper function: Deadband comparison *)
FUNCTION InDeadband : BOOL
VAR_INPUT
    Value     : REAL;
    Target    : REAL;
    Deadband  : REAL;
END_VAR
    IF ABS(Value - Target) <= Deadband THEN
        InDeadband := TRUE;
    ELSE
        InDeadband := FALSE;
    END_IF;
END_FUNCTION
