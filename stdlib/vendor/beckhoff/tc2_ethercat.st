(* Tc2_EtherCAT -- EtherCAT diagnostics and mailbox function blocks *)
(* Stub declarations for stc type-checking. No implementation bodies. *)
(* Source: TwinCAT 3 PLC Lib Tc2_EtherCAT manual v1.10.0. *)
(* E_EcSlaveState, EcDiagParam and FB_EcDeviceDiag are deliberately NOT
   declared here: they are SVNCoreComponents declarations, not Beckhoff
   ones, and must resolve from that library. *)

TYPE E_EcAdressingType : (
    eAdressingType_AutoInc := 1,
    eAdressingType_Fixed,
    eAdressingType_Broadcast
);
END_TYPE

TYPE ST_EcSlaveState :
STRUCT
    deviceState     : BYTE;
    linkState       : BYTE;
END_STRUCT
END_TYPE

TYPE ST_EcCrcError :
STRUCT
    portA           : UDINT;
    portB           : UDINT;
    portC           : UDINT;
END_STRUCT
END_TYPE

TYPE ST_EcCrcErrorEx :
STRUCT
    portA           : UDINT;
    portB           : UDINT;
    portC           : UDINT;
    portD           : UDINT;
END_STRUCT
END_TYPE

(* Global constants, section 14.1 (subset) *)
VAR_GLOBAL CONSTANT
    EC_AMSPORT_MASTER               : UINT := 16#FFFF;
    EC_MAX_SLAVES                   : UINT := 16#FFFF;
    EC_DEVICE_STATE_MASK            : BYTE := 16#0F;
    EC_DEVICE_STATE_INIT            : BYTE := 16#01;
    EC_DEVICE_STATE_PREOP           : BYTE := 16#02;
    EC_DEVICE_STATE_BOOTSTRAP       : BYTE := 16#03;
    EC_DEVICE_STATE_SAFEOP          : BYTE := 16#04;
    EC_DEVICE_STATE_OP              : BYTE := 16#08;
    EC_DEVICE_STATE_ERROR           : BYTE := 16#10;
    EC_DEVICE_STATE_INVALID_VPRS    : BYTE := 16#20;
    EC_DEVICE_STATE_INITCMD_ERROR   : BYTE := 16#40;
    EC_LINK_STATE_OK                : BYTE := 16#00;
    EC_LINK_STATE_NOT_PRESENT       : BYTE := 16#01;
    EC_LINK_STATE_LINK_WITHOUT_COMM : BYTE := 16#02;
    EC_LINK_STATE_MISSING_LINK      : BYTE := 16#04;
    EC_LINK_STATE_ADDITIONAL_LINK   : BYTE := 16#08;
    EC_LINK_STATE_PORT_A            : BYTE := 16#10;
    EC_LINK_STATE_PORT_B            : BYTE := 16#20;
    EC_LINK_STATE_PORT_C            : BYTE := 16#40;
    EC_LINK_STATE_PORT_D            : BYTE := 16#80;
END_VAR

(* Section 3.2 *)
FUNCTION_BLOCK FB_EcPhysicalWriteCmd
VAR_INPUT
    sNetId          : T_AmsNetId;
    adp             : UINT;
    ado             : UINT;
    len             : UDINT;
    eType           : E_EcAdressingType := eAdressingType_Fixed;
    pSrcBuf         : PVOID;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    wkc             : UINT;
END_VAR
END_FUNCTION_BLOCK

(* Section 4.3 *)
FUNCTION_BLOCK FB_EcGetAllSlaveAddr
VAR_INPUT
    sNetId          : T_AmsNetId;
    pAddrBuf        : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF UINT;
    cbBufLen        : UDINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    nSlaves         : UINT;
END_VAR
END_FUNCTION_BLOCK

(* Section 4.4 *)
FUNCTION_BLOCK FB_EcGetAllSlaveCrcErrors
VAR_INPUT
    sNetId          : T_AmsNetId;
    pCrcErrorBuf    : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF DWORD;
    cbBufLen        : UDINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    nSlaves         : UINT;
END_VAR
END_FUNCTION_BLOCK

(* Section 4.12 *)
FUNCTION_BLOCK FB_EcGetSlaveCrcError
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    crcError        : ST_EcCrcError;
END_VAR
END_FUNCTION_BLOCK

(* Section 4.13 (tTimeout has no default in the manual) *)
FUNCTION_BLOCK FB_EcGetSlaveCrcErrorEx
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    bExecute        : BOOL;
    tTimeout        : TIME;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    CrcError        : ST_EcCrcErrorEx;
END_VAR
END_FUNCTION_BLOCK

(* Section 5.1 *)
FUNCTION_BLOCK FB_EcGetAllSlaveStates
VAR_INPUT
    sNetId          : T_AmsNetId;
    pStateBuf       : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF ST_EcSlaveState;
    cbBufLen        : UDINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    nSlaves         : UINT;
END_VAR
END_FUNCTION_BLOCK

(* Section 5.3 *)
FUNCTION_BLOCK FB_EcGetMasterState
VAR_INPUT
    sNetId          : T_AmsNetId;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    state           : WORD;
END_VAR
END_FUNCTION_BLOCK

(* Section 5.4 *)
FUNCTION_BLOCK FB_EcGetSlaveState
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    state           : ST_EcSlaveState;
END_VAR
END_FUNCTION_BLOCK

(* Section 5.9 *)
FUNCTION_BLOCK FB_EcSetSlaveState
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    bExecute        : BOOL;
    tTimeout        : TIME := T#10S;
    reqState        : WORD;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    currState       : ST_EcSlaveState;
END_VAR
END_FUNCTION_BLOCK

(* Section 7.1 (Beckhoff also spells it FB_EcCoeSdoRead; names are case-insensitive) *)
FUNCTION_BLOCK FB_EcCoESdoRead
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    nSubIndex       : BYTE;
    nIndex          : WORD;
    pDstBuf         : PVOID;
    cbBufLen        : UDINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
    cbRead          : UDINT;
END_VAR
END_FUNCTION_BLOCK

(* Section 7.3 *)
FUNCTION_BLOCK FB_EcCoESdoWrite
VAR_INPUT
    sNetId          : T_AmsNetId;
    nSlaveAddr      : UINT;
    nSubIndex       : BYTE;
    nIndex          : WORD;
    pSrcBuf         : PVOID;
    cbBufLen        : UDINT;
    bExecute        : BOOL;
    tTimeout        : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bBusy           : BOOL;
    bError          : BOOL;
    nErrId          : UDINT;
END_VAR
END_FUNCTION_BLOCK
