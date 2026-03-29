(* Parse-Only Pointer and Reference Test
   Exercises pointer and reference types that parse correctly but are not
   yet fully supported in the interpreter. Validated with `stc parse`.
*)

PROGRAM PointerDemo
VAR
    x : INT := 42;
    y : REAL := 3.14;
    pInt : POINTER TO INT;
    pReal : POINTER TO REAL;
    refInt : REFERENCE TO INT;
END_VAR
    (* These would be runtime operations if the interpreter supported them:
       pInt := ADR(x);
       pReal := ADR(y);
       x := pInt^;
    *)
    x := 42;
END_PROGRAM

FUNCTION_BLOCK FB_WithPointers
VAR_INPUT
    pData : POINTER TO INT;
END_VAR
VAR
    localPtr : POINTER TO INT;
    localRef : REFERENCE TO INT;
END_VAR
END_FUNCTION_BLOCK
