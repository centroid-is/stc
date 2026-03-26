PROGRAM Main
VAR
    x : INT;
END_VAR
{IF defined(VENDOR_BECKHOFF)}
    x := 1; (* Beckhoff path *)
{ELSIF defined(VENDOR_SCHNEIDER)}
    x := 2; (* Schneider path *)
{ELSE}
    x := 0; (* Generic path *)
{END_IF}
END_PROGRAM
