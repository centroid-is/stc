(* Motor Control Program
   Industrial motor with start/stop pushbuttons, safety interlocks,
   speed ramping, and emergency stop handling.

   Inputs:
     StartPB    - Momentary start pushbutton
     StopPB     - Momentary stop pushbutton (NC, TRUE = not pressed)
     EStopOK    - Emergency stop circuit healthy (TRUE = OK)
     OverloadOK - Thermal overload relay (TRUE = OK)
     LubeOK     - Lubrication pressure confirmed (TRUE = OK)

   Outputs:
     MotorRun   - Motor contactor output
     RunLamp    - Running indicator lamp
     FaultLamp  - Fault indicator lamp
     SpeedRef   - Speed reference 0.0 to 100.0 percent
*)

PROGRAM MotorControl
VAR_INPUT
    StartPB    : BOOL;
    StopPB     : BOOL := TRUE;  (* NC contact, TRUE = safe *)
    EStopOK    : BOOL;
    OverloadOK : BOOL := TRUE;
    LubeOK     : BOOL := TRUE;
    SpeedSetpoint : REAL := 100.0;
END_VAR

VAR_OUTPUT
    MotorRun   : BOOL;
    RunLamp    : BOOL;
    FaultLamp  : BOOL;
    SpeedRef   : REAL;
END_VAR

VAR
    StartEdge  : R_TRIG;
    StartupTmr : TON;
    FaultCount : CTU;
    AllInterlocksOK : BOOL;
    StartCommand : BOOL;
    Fault : BOOL;
    RampStep : REAL := 5.0;
END_VAR

    (* Check all interlocks *)
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;

    (* Detect start button rising edge *)
    StartEdge(CLK := StartPB);

    (* Latch start command on rising edge, clear on fault or interlock loss *)
    IF StartEdge.Q AND AllInterlocksOK AND NOT Fault THEN
        StartCommand := TRUE;
    END_IF;

    IF NOT AllInterlocksOK OR Fault THEN
        StartCommand := FALSE;
    END_IF;

    (* Motor output follows start command *)
    MotorRun := StartCommand AND AllInterlocksOK;

    (* Speed ramping *)
    IF MotorRun THEN
        IF SpeedRef < SpeedSetpoint THEN
            SpeedRef := SpeedRef + RampStep;
            IF SpeedRef > SpeedSetpoint THEN
                SpeedRef := SpeedSetpoint;
            END_IF;
        END_IF;
    ELSE
        SpeedRef := 0.0;
    END_IF;

    (* Startup timer for monitoring *)
    StartupTmr(IN := MotorRun, PT := T#5s);

    (* Fault detection: count interlock trips *)
    FaultCount(CU := NOT AllInterlocksOK, R := FALSE, PV := 3);
    Fault := FaultCount.Q;

    (* Outputs *)
    RunLamp := MotorRun;
    FaultLamp := Fault;
END_PROGRAM
