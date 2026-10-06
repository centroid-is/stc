PROGRAM Main
VAR
    pump : FB_Pump;
    a : ST_A;
    t1 : T1;
    ta : TA;
    i : I_Motor;
    s : INT;
END_VAR
s := pump.motor.speed;
s := a.b.field;
t1 := s;
ta[0].field := s;
pump(cmd := TRUE);
END_PROGRAM

FUNCTION_BLOCK FB_Pump
VAR_INPUT
    cmd : BOOL;
END_VAR
VAR_OUTPUT
    motor : FB_Motor;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Motor
VAR_OUTPUT
    speed : INT;
END_VAR
END_FUNCTION_BLOCK

TYPE ST_A :
STRUCT
    b : ST_B;
END_STRUCT
END_TYPE

TYPE ST_B :
STRUCT
    field : INT;
END_STRUCT
END_TYPE

TYPE T1 : T2; END_TYPE

TYPE T2 : INT; END_TYPE

TYPE TA : ARRAY[0..1] OF ST_B; END_TYPE

INTERFACE I_Motor
END_INTERFACE
