(* Conveyor Belt Control Function Block
   Models a 3-zone conveyor with photoeye sensors, jam detection,
   and state-machine sequencing.

   States:
     0 = IDLE      - Waiting for product
     1 = RUNNING   - Conveyor moving, product in transit
     2 = JAM       - Sensor blocked too long, fault
     3 = STOPPED   - Operator stop requested
*)

FUNCTION_BLOCK ConveyorZone
VAR_INPUT
    Enable      : BOOL;      (* Master enable *)
    SensorEntry : BOOL;      (* Photoeye at zone entry *)
    SensorExit  : BOOL;      (* Photoeye at zone exit *)
    Reset       : BOOL;      (* Reset from jam fault *)
    JamTimeLimit : TIME := T#10s;  (* Max time sensor can be blocked *)
END_VAR

VAR_OUTPUT
    MotorFwd    : BOOL;      (* Drive motor forward *)
    ZoneOccupied : BOOL;     (* Product present in zone *)
    JamFault    : BOOL;      (* Jam detected *)
    State       : INT;       (* Current state *)
    ProductCount : DINT;     (* Products that have exited *)
END_VAR

VAR
    JamTimer    : TON;       (* Jam detection timer *)
    ExitEdge    : R_TRIG;    (* Detects product leaving zone *)
    EntryEdge   : R_TRIG;    (* Detects product entering zone *)
    Counter     : CTU;       (* Product counter *)
END_VAR

    (* Detect product entry and exit edges *)
    EntryEdge(CLK := SensorEntry);
    ExitEdge(CLK := NOT SensorExit);  (* Photoeye: TRUE = blocked *)

    (* State machine *)
    CASE State OF
        0: (* IDLE *)
            MotorFwd := FALSE;
            IF Enable AND SensorEntry THEN
                State := 1;
                ZoneOccupied := TRUE;
            END_IF;

        1: (* RUNNING *)
            MotorFwd := TRUE;
            (* Check for product exit *)
            IF NOT SensorEntry AND NOT SensorExit THEN
                ZoneOccupied := FALSE;
                Counter(CU := TRUE, R := FALSE, PV := 9999);
                State := 0;
            END_IF;
            (* Check for jam *)
            JamTimer(IN := SensorEntry, PT := JamTimeLimit);
            IF JamTimer.Q THEN
                State := 2;
            END_IF;
            IF NOT Enable THEN
                State := 3;
            END_IF;

        2: (* JAM *)
            MotorFwd := FALSE;
            JamFault := TRUE;
            IF Reset THEN
                JamFault := FALSE;
                JamTimer(IN := FALSE, PT := JamTimeLimit);
                State := 0;
            END_IF;

        3: (* STOPPED *)
            MotorFwd := FALSE;
            IF Enable THEN
                State := 1;
            END_IF;
    END_CASE;

    ProductCount := Counter.CV;

    (* Reset counter edge *)
    Counter(CU := FALSE, R := FALSE, PV := 9999);
END_FUNCTION_BLOCK
