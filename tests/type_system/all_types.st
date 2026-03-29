(* All Types Declaration File
   Exercises every IEC 61131-3 type supported by the stc toolchain:
   elementary types, arrays, structs, enums, subranges, strings.
*)

(* Enum type declaration *)
TYPE MachineState : (
    MS_IDLE := 0,
    MS_RUNNING := 1,
    MS_FAULT := 2,
    MS_MAINTENANCE := 3
);
END_TYPE

(* Struct type declaration *)
TYPE SensorReading :
STRUCT
    Value     : REAL;
    RawCount  : INT;
    Valid     : BOOL;
    Timestamp : DINT;
    TagName   : STRING;
END_STRUCT;
END_TYPE

(* Subrange type *)
TYPE PercentRange : INT (0..100);
END_TYPE

TYPE MotorSpeed : REAL (0.0..1500.0);
END_TYPE

(* Program using all types *)
PROGRAM TypeShowcase
VAR
    (* Boolean *)
    bFlag : BOOL := TRUE;
    bLatch : BOOL := FALSE;

    (* Integer types *)
    nSint : SINT := -10;
    nInt  : INT := 1000;
    nDint : DINT := 100000;
    nLint : LINT := 1000000;

    (* Unsigned integer types *)
    nUsint : USINT := 200;
    nUint  : UINT := 50000;
    nUdint : UDINT := 4000000;
    nUlint : ULINT := 8000000;

    (* Bit string types *)
    bByte  : BYTE := 16#FF;
    wWord  : WORD := 16#ABCD;
    dwDword : DWORD := 16#12345678;
    lwLword : LWORD := 16#FEDCBA98;

    (* Real types *)
    fReal  : REAL := 3.14;
    fLreal : LREAL := 2.718281828;

    (* String types *)
    sName  : STRING := 'Hello World';
    sEmpty : STRING := '';

    (* Time types *)
    tDelay : TIME := T#500ms;
    tCycle : TIME := T#100ms;

    (* Array *)
    aTemps : ARRAY[1..10] OF REAL;
    aFlags : ARRAY[0..7] OF BOOL;

    (* Enum instance *)
    eState : MachineState := MS_IDLE;

    (* Struct instance *)
    stSensor : SensorReading;

    (* Loop variable *)
    i : INT;
END_VAR
    (* Exercise boolean operators *)
    bLatch := bFlag AND NOT bLatch;
    bLatch := bFlag OR bLatch;
    bLatch := bFlag XOR bLatch;

    (* Exercise integer arithmetic *)
    nInt := nInt + 1;
    nDint := nDint * 2;
    nLint := nLint - 500;

    (* Exercise real arithmetic *)
    fReal := fReal * 2.0;
    fLreal := fLreal + 1.0;

    (* Array operations *)
    FOR i := 1 TO 10 DO
        aTemps[i] := INT_TO_REAL(i) * 10.5;
    END_FOR;

    aFlags[0] := TRUE;
    aFlags[7] := FALSE;

    (* Enum usage *)
    eState := MS_RUNNING;
    IF eState = MS_RUNNING THEN
        nInt := nInt + 100;
    END_IF;

    (* Struct field access *)
    stSensor.Value := 25.4;
    stSensor.RawCount := 1023;
    stSensor.Valid := TRUE;
    stSensor.TagName := 'TT-101';

    (* String operations *)
    sName := CONCAT(sName, '!');
END_PROGRAM
