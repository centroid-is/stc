(* Behavioral mocks for Beckhoff Tc2_MC2 motion control function blocks *)
(* These mocks simulate multi-cycle FB execution for realistic testing. *)
(* Users can inspect, modify, or use these as templates for custom mocks. *)

(* Mock MC_MoveAbsolute: simulates position change over multiple cycles *)
(* After Execute rising edge, goes Busy for 5 cycles, then sets Done *)
FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Position        : LREAL;
    Velocity        : LREAL;
    Acceleration    : LREAL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
    Direction       : MC_Direction;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
VAR
    prevExecute     : BOOL;
    cycleCount      : INT;
    targetPosition  : LREAL;
    startPosition   : LREAL;
    currentPosition : LREAL;
END_VAR

(* Detect rising edge of Execute *)
IF Execute AND NOT prevExecute THEN
    Busy := TRUE;
    Active := TRUE;
    Done := FALSE;
    CommandAborted := FALSE;
    Error := FALSE;
    ErrorID := 0;
    cycleCount := 0;
    targetPosition := Position;
    startPosition := currentPosition;
END_IF;

(* Simulate motion over 5 cycles *)
IF Busy THEN
    cycleCount := cycleCount + 1;
    (* Linear interpolation toward target *)
    IF cycleCount >= 5 THEN
        currentPosition := targetPosition;
        Done := TRUE;
        Busy := FALSE;
        Active := FALSE;
    ELSE
        currentPosition := startPosition + (targetPosition - startPosition) * INT_TO_LREAL(cycleCount) / 5.0;
    END_IF;
END_IF;

(* Abort on falling edge of Execute while busy *)
IF NOT Execute AND prevExecute AND Busy THEN
    CommandAborted := TRUE;
    Busy := FALSE;
    Active := FALSE;
END_IF;

prevExecute := Execute;
END_FUNCTION_BLOCK

(* Mock MC_Power: simulates axis enable/disable *)
(* Sets Status after 2 cycles of Enable *)
FUNCTION_BLOCK MC_Power
VAR_INPUT
    Axis            : AXIS_REF;
    Enable          : BOOL;
    Enable_Positive : BOOL;
    Enable_Negative : BOOL;
    Override_V      : LREAL := 100.0;
END_VAR
VAR_OUTPUT
    Status      : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
VAR
    enableCount : INT;
END_VAR

IF Enable THEN
    IF NOT Status THEN
        enableCount := enableCount + 1;
        Busy := TRUE;
        Active := TRUE;
        IF enableCount >= 2 THEN
            Status := TRUE;
            Busy := FALSE;
        END_IF;
    ELSE
        Busy := FALSE;
        Active := TRUE;
    END_IF;
ELSE
    Status := FALSE;
    Busy := FALSE;
    Active := FALSE;
    enableCount := 0;
END_IF;

Error := FALSE;
ErrorID := 0;
END_FUNCTION_BLOCK

(* Mock MC_Home: simulates homing sequence *)
(* After Execute rising edge, goes Busy for 3 cycles, then Done *)
FUNCTION_BLOCK MC_Home
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Position        : LREAL;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
VAR
    prevExecute     : BOOL;
    cycleCount      : INT;
END_VAR

IF Execute AND NOT prevExecute THEN
    Busy := TRUE;
    Active := TRUE;
    Done := FALSE;
    CommandAborted := FALSE;
    Error := FALSE;
    ErrorID := 0;
    cycleCount := 0;
END_IF;

IF Busy THEN
    cycleCount := cycleCount + 1;
    IF cycleCount >= 3 THEN
        Done := TRUE;
        Busy := FALSE;
        Active := FALSE;
    END_IF;
END_IF;

prevExecute := Execute;
END_FUNCTION_BLOCK

(* Mock MC_Stop: simulates deceleration *)
(* After Execute rising edge, goes Busy for 2 cycles (deceleration), then Done *)
FUNCTION_BLOCK MC_Stop
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
VAR
    prevExecute     : BOOL;
    cycleCount      : INT;
END_VAR

IF Execute AND NOT prevExecute THEN
    Busy := TRUE;
    Active := TRUE;
    Done := FALSE;
    CommandAborted := FALSE;
    Error := FALSE;
    ErrorID := 0;
    cycleCount := 0;
END_IF;

IF Busy THEN
    cycleCount := cycleCount + 1;
    IF cycleCount >= 2 THEN
        Done := TRUE;
        Busy := FALSE;
        Active := FALSE;
    END_IF;
END_IF;

prevExecute := Execute;
END_FUNCTION_BLOCK
