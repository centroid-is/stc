(* Type Declarations Test Suite
   Tests TYPE blocks: structs, enums (via integer values), arrays,
   subranges, and type aliases used in programs.
*)

(* ===== STRUCT TYPE ===== *)

TYPE Point :
STRUCT
    x : INT;
    y : INT;
END_STRUCT;
END_TYPE

TYPE Sensor :
STRUCT
    Value : REAL;
    RawCount : INT;
    Valid : BOOL;
    Name : STRING;
END_STRUCT;
END_TYPE

TYPE NestedStruct :
STRUCT
    id : INT;
    label : STRING;
END_STRUCT;
END_TYPE

(* ===== ENUM TYPE ===== *)

TYPE TrafficLight : (TL_Red := 0, TL_Yellow := 1, TL_Green := 2);
END_TYPE

TYPE MachineState : (
    MS_IDLE := 0,
    MS_RUNNING := 1,
    MS_ERROR := 2,
    MS_MAINTENANCE := 3
);
END_TYPE

(* ===== STRUCT TESTS ===== *)

TEST_CASE 'Struct field read and write'
VAR
    p : Point;
END_VAR
    p.x := 10;
    p.y := 20;
    ASSERT_EQ(p.x, 10);
    ASSERT_EQ(p.y, 20);
END_TEST_CASE

TEST_CASE 'Struct with multiple field types'
VAR
    s : Sensor;
END_VAR
    s.Value := 3.14;
    s.RawCount := 1023;
    s.Valid := TRUE;
    s.Name := 'PT-100';

    ASSERT_NEAR(s.Value, 3.14, 0.001);
    ASSERT_EQ(s.RawCount, 1023);
    ASSERT_TRUE(s.Valid);
    ASSERT_EQ(s.Name, 'PT-100');
END_TEST_CASE

TEST_CASE 'Struct fields default to zero values'
VAR
    p : Point;
END_VAR
    ASSERT_EQ(p.x, 0);
    ASSERT_EQ(p.y, 0);
END_TEST_CASE

TEST_CASE 'Struct field arithmetic'
VAR
    p : Point;
    dist : INT;
END_VAR
    p.x := 3;
    p.y := 4;
    dist := p.x * p.x + p.y * p.y;
    ASSERT_EQ(dist, 25);  (* 9 + 16 = 25, sqrt would be 5 *)
END_TEST_CASE

(* ===== ARRAY TESTS ===== *)

TEST_CASE 'Array basic indexing'
VAR
    arr : ARRAY[1..5] OF INT;
END_VAR
    arr[1] := 10;
    arr[2] := 20;
    arr[3] := 30;
    arr[4] := 40;
    arr[5] := 50;
    ASSERT_EQ(arr[1], 10);
    ASSERT_EQ(arr[3], 30);
    ASSERT_EQ(arr[5], 50);
END_TEST_CASE

TEST_CASE 'Array with FOR loop initialization'
VAR
    arr : ARRAY[1..10] OF INT;
    i : INT;
    sum : INT := 0;
END_VAR
    FOR i := 1 TO 10 DO
        arr[i] := i * i;
    END_FOR;
    FOR i := 1 TO 10 DO
        sum := sum + arr[i];
    END_FOR;
    (* 1+4+9+16+25+36+49+64+81+100 = 385 *)
    ASSERT_EQ(sum, 385);
END_TEST_CASE

TEST_CASE 'Array of BOOL'
VAR
    flags : ARRAY[0..7] OF BOOL;
END_VAR
    flags[0] := TRUE;
    flags[1] := FALSE;
    flags[7] := TRUE;
    ASSERT_TRUE(flags[0]);
    ASSERT_FALSE(flags[1]);
    ASSERT_TRUE(flags[7]);
END_TEST_CASE

TEST_CASE 'Array of REAL'
VAR
    temps : ARRAY[1..3] OF REAL;
END_VAR
    temps[1] := 98.6;
    temps[2] := 100.0;
    temps[3] := 99.1;
    ASSERT_NEAR(temps[1], 98.6, 0.001);
    ASSERT_NEAR(temps[2], 100.0, 0.001);
END_TEST_CASE

TEST_CASE 'Array of STRING'
VAR
    names : ARRAY[1..3] OF STRING;
END_VAR
    names[1] := 'Alice';
    names[2] := 'Bob';
    names[3] := 'Charlie';
    ASSERT_EQ(names[2], 'Bob');
END_TEST_CASE

(* ===== ENUM TESTS (integer fallback) ===== *)

TEST_CASE 'Enum used as integer'
VAR
    state : MachineState;
END_VAR
    state := 0;  (* MS_IDLE *)
    ASSERT_EQ(state, 0);

    state := 1;  (* MS_RUNNING *)
    ASSERT_EQ(state, 1);
END_TEST_CASE

TEST_CASE 'Enum in CASE statement'
VAR
    light : INT := 2;  (* TL_Green *)
    act : STRING;
END_VAR
    CASE light OF
        0: act := 'STOP';
        1: act := 'CAUTION';
        2: act := 'GO';
    ELSE
        act := 'UNKNOWN';
    END_CASE;
    ASSERT_EQ(act, 'GO');
END_TEST_CASE

(* ===== COMBINED TYPE USAGE ===== *)

TEST_CASE 'Struct and array together'
VAR
    points : ARRAY[1..3] OF INT;
    sensor : Sensor;
END_VAR
    points[1] := 100;
    points[2] := 200;
    points[3] := 300;

    sensor.Value := INT_TO_REAL(points[2]);
    sensor.RawCount := points[1] + points[3];

    ASSERT_NEAR(sensor.Value, 200.0, 0.001);
    ASSERT_EQ(sensor.RawCount, 400);
END_TEST_CASE
