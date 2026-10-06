(* Tc2_ModbusSrv -- Modbus TCP client function blocks (TF6250) *)
(* Stub declarations for stc type-checking. No implementation bodies. *)
(* Source: TF6250 TC3 Modbus TCP manual section 6.1.2. Section 6.1 states the
   Modbus functions are implemented in the PLC library Tc2_ModbusSrv, which is
   the library name TwinCAT projects reference, so the client FBs live here. *)

VAR_GLOBAL CONSTANT
    MODBUS_TCP_PORT : UINT := 502;      (* value assumed: Modbus TCP standard port *)
END_VAR

(* Modbus function 1 *)
FUNCTION_BLOCK FB_MBReadCoils
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pDestAddr       : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 2 *)
FUNCTION_BLOCK FB_MBReadInputs
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pDestAddr       : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 3 *)
FUNCTION_BLOCK FB_MBReadRegs
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pDestAddr       : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 4 *)
FUNCTION_BLOCK FB_MBReadInputRegs
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pDestAddr       : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 5 *)
FUNCTION_BLOCK FB_MBWriteSingleCoil
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nMBAddr         : WORD;
    nValue          : WORD;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 6 *)
FUNCTION_BLOCK FB_MBWriteSingleReg
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nMBAddr         : WORD;
    nValue          : WORD;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 15 *)
FUNCTION_BLOCK FB_MBWriteCoils
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pSrcAddr        : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 16 *)
FUNCTION_BLOCK FB_MBWriteRegs
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nQuantity       : WORD;
    nMBAddr         : WORD;
    cbLength        : UDINT;
    pSrcAddr        : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Modbus function 23 *)
FUNCTION_BLOCK FB_MBReadWriteRegs
VAR_INPUT
    sIPAddr         : STRING(15);
    nTCPPort        : UINT := MODBUS_TCP_PORT;
    nUnitID         : BYTE := 16#FF;
    nReadQuantity   : WORD;
    nMBReadAddr     : WORD;
    nWriteQuantity  : WORD;
    nMBWriteAddr    : WORD;
    cbDestLength    : UDINT;
    pDestAddr       : POINTER TO BYTE;
    cbSrcLength     : UDINT;
    pSrcAddr        : POINTER TO BYTE;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBUSY           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK
