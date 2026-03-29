(* Preprocessor Conditional Compilation Test
   Tests {IF}, {ELSIF}, {ELSE}, {END_IF}, {DEFINE}, and nested blocks.
   Run with: stc pp tests/preprocessor/conditional.st --define <flag>
*)

{IF defined(VENDOR_BECKHOFF)}
(* Beckhoff-specific code *)
PROGRAM VendorSpecific
VAR
    platform : STRING := 'Beckhoff TwinCAT';
    maxCycles : INT := 10000;
END_VAR
END_PROGRAM
{ELSIF defined(VENDOR_SCHNEIDER)}
(* Schneider-specific code *)
PROGRAM VendorSpecific
VAR
    platform : STRING := 'Schneider EcoStruxure';
    maxCycles : INT := 5000;
END_VAR
END_PROGRAM
{ELSIF defined(VENDOR_CODESYS)}
(* CODESYS-specific code *)
PROGRAM VendorSpecific
VAR
    platform : STRING := 'CODESYS V3';
    maxCycles : INT := 8000;
END_VAR
END_PROGRAM
{ELSE}
(* Generic/portable code *)
PROGRAM VendorSpecific
VAR
    platform : STRING := 'Generic';
    maxCycles : INT := 1000;
END_VAR
END_PROGRAM
{END_IF}

(* ===== DEFINE AND USE ===== *)

{DEFINE FEATURE_LOGGING}

{IF defined(FEATURE_LOGGING)}
FUNCTION_BLOCK FB_Logger
VAR
    logEnabled : BOOL := TRUE;
END_VAR
END_FUNCTION_BLOCK
{END_IF}

(* ===== NESTED CONDITIONALS ===== *)

{IF defined(VENDOR_BECKHOFF)}
    {IF defined(DEBUG)}
    FUNCTION_BLOCK FB_DebugBeckhoff
    VAR
        debugLevel : INT := 3;
    END_VAR
    END_FUNCTION_BLOCK
    {ELSE}
    FUNCTION_BLOCK FB_ProdBeckhoff
    VAR
        debugLevel : INT := 0;
    END_VAR
    END_FUNCTION_BLOCK
    {END_IF}
{END_IF}
