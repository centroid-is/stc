(* Batch Mixing Process Program
   Simulates a batch reactor with:
   - Sequential fill/mix/heat/drain phases
   - Timer-based phase transitions
   - Analog value calculations (temperature, level)
   - String-based recipe and status reporting

   Phases:
     0 = IDLE
     1 = FILLING
     2 = MIXING
     3 = HEATING
     4 = DRAINING
     5 = COMPLETE
*)

PROGRAM BatchProcess
VAR_INPUT
    StartBatch    : BOOL;
    RecipeName    : STRING := 'DEFAULT';
    TargetTemp    : REAL := 75.0;     (* Degrees C *)
    TargetLevel   : REAL := 80.0;     (* Percent *)
    FillRate      : REAL := 5.0;      (* Percent per scan *)
    DrainRate     : REAL := 3.0;      (* Percent per scan *)
END_VAR

VAR_OUTPUT
    Phase         : INT;
    InletValve    : BOOL;
    OutletValve   : BOOL;
    MixerMotor    : BOOL;
    HeaterOn      : BOOL;
    CurrentLevel  : REAL;
    CurrentTemp   : REAL;
    StatusMsg     : STRING;
    BatchComplete : BOOL;
END_VAR

VAR
    FillTimer     : TON;
    MixTimer      : TON;
    HeatTimer     : TON;
    DrainTimer    : TON;
    StartEdge     : R_TRIG;
    TempRampRate  : REAL := 2.5;    (* Degrees per scan *)
    AmbientTemp   : REAL := 20.0;
END_VAR

    StartEdge(CLK := StartBatch);

    CASE Phase OF
        0: (* IDLE *)
            InletValve := FALSE;
            OutletValve := FALSE;
            MixerMotor := FALSE;
            HeaterOn := FALSE;
            BatchComplete := FALSE;
            StatusMsg := CONCAT('IDLE: ', RecipeName);

            IF StartEdge.Q THEN
                Phase := 1;
                CurrentLevel := 0.0;
                CurrentTemp := AmbientTemp;
            END_IF;

        1: (* FILLING *)
            InletValve := TRUE;
            StatusMsg := 'FILLING';
            CurrentLevel := CurrentLevel + FillRate;
            IF CurrentLevel >= TargetLevel THEN
                CurrentLevel := TargetLevel;
                InletValve := FALSE;
                Phase := 2;
            END_IF;

        2: (* MIXING *)
            MixerMotor := TRUE;
            MixTimer(IN := TRUE, PT := T#30s);
            StatusMsg := 'MIXING';
            IF MixTimer.Q THEN
                Phase := 3;
                MixTimer(IN := FALSE, PT := T#30s);
            END_IF;

        3: (* HEATING *)
            HeaterOn := TRUE;
            MixerMotor := TRUE;
            StatusMsg := 'HEATING';
            CurrentTemp := CurrentTemp + TempRampRate;
            IF CurrentTemp >= TargetTemp THEN
                CurrentTemp := TargetTemp;
                HeaterOn := FALSE;
                Phase := 4;
            END_IF;

        4: (* DRAINING *)
            MixerMotor := FALSE;
            OutletValve := TRUE;
            StatusMsg := 'DRAINING';
            CurrentLevel := CurrentLevel - DrainRate;
            IF CurrentLevel <= 0.0 THEN
                CurrentLevel := 0.0;
                OutletValve := FALSE;
                Phase := 5;
            END_IF;

        5: (* COMPLETE *)
            BatchComplete := TRUE;
            StatusMsg := CONCAT('DONE: ', RecipeName);
    END_CASE;
END_PROGRAM
