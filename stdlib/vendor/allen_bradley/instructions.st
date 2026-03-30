(* Allen Bradley Logix 5000 -- Common instruction stubs *)
(* These map AB instruction mnemonics to IEC FUNCTION declarations. *)
(* AB uses these as instructions; stc stubs them as functions for type-checking. *)

(* Arithmetic instructions *)
FUNCTION ADD : DINT
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION SUB : DINT
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION MUL : DINT
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION DIV : DINT
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

(* Data move instruction *)
FUNCTION MOV : DINT
VAR_INPUT
    Source      : DINT;
END_VAR
END_FUNCTION

(* Comparison instructions -- return BOOL for condition evaluation *)
FUNCTION CMP : BOOL
VAR_INPUT
    Expression  : STRING(80);
END_VAR
END_FUNCTION

FUNCTION EQU : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION NEQ : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION GRT : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION LES : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION GEQ : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION

FUNCTION LEQ : BOOL
VAR_INPUT
    SourceA     : DINT;
    SourceB     : DINT;
END_VAR
END_FUNCTION
