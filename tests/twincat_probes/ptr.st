PROGRAM MAIN
VAR n : INT; p : POINTER TO INT; r : REFERENCE TO INT; arr : ARRAY[0..3] OF INT; END_VAR
p := ADR(n);
p^ := 5;
r REF= n;
r := 7;
n := SIZEOF(arr);
END_PROGRAM
