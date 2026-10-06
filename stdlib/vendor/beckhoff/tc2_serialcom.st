(* Tc2_SerialCom -- serial communication function blocks (TF6340) *)
(* Stub declarations for stc type-checking. No implementation bodies. *)
(* Source: TF6340 TC3 Serial Communication manual section 5. The real type
   names are ComBuffer, ComSerialLineMode_t and ComError_t (ST_ComBuffer and
   E_ComMode do not exist). At the time of writing no plcproj-listed
   sildarvinnsla source uses these names (only Baader/PLC-TASK.md mentions
   them) and Baader does not reference Tc2_SerialCom; this stub is exercised
   by the committed usage fixture only. *)

TYPE ComBuffer :
STRUCT
    Buffer          : ARRAY[0..300] OF BYTE;
    RdIdx           : INT;
    WrIdx           : INT;
    Count           : INT;
    FreeByte        : INT;
    Error           : INT;
    blocked         : BOOL;
END_STRUCT
END_TYPE

TYPE ComSerialLineMode_t : (
    SERIALLINEMODE_DEFAULT,
    SERIALLINEMODE_KL6_3B_ALTERNATIVE,
    SERIALLINEMODE_KL6_5B_STANDARD,
    SERIALLINEMODE_KL6_22B_STANDARD,
    SERIALLINEMODE_PC_COM_PORT,
    SERIALLINEMODE_EL6_22B,
    SERIALLINEMODE_IE6_11B
);
END_TYPE

TYPE ComError_t : (
    COMERROR_NOERROR := 0,
    COMERROR_PARAMETERCHANGED := 1,
    COMERROR_TXBUFFOVERRUN := 2,
    COMERROR_STRINGOVERRUN := 10,
    COMERROR_ZEROCHARINVALID := 11,
    COMERROR_INVALIDPOINTER := 20,
    COMERROR_INVALIDRXPOINTER := 21,
    COMERROR_INVALIDRXLENGTH := 22,
    COMERROR_DATASIZEOVERRUN := 23,
    COMERROR_INVALIDPROCESSDATASIZE := 24,
    COMERROR_MODENOTSUPPORTED := 16#0101,
    COMERROR_INVALIDCHANNELNUMBER := 16#0102,
    COMERROR_INVALIDBAUDRATE := 16#1001,
    COMERROR_INVALIDNUMDATABITS := 16#1002,
    COMERROR_INVALIDNUMSTOPBITS := 16#1003,
    COMERROR_INVALIDPARITY := 16#1004,
    COMERROR_INVALIDHANDSHAKE := 16#1005,
    COMERROR_INVALIDNUMREGISTERS := 16#1006,
    COMERROR_INVALIDREGISTER := 16#1007,
    COMERROR_TIMEOUT := 16#1008
);
END_TYPE

(* Section 5 *)
FUNCTION_BLOCK SerialLineControl
VAR_INPUT
    Mode            : ComSerialLineMode_t;
    pComIn          : POINTER TO BYTE;
    pComOut         : POINTER TO BYTE;
    SizeComIn       : INT;
END_VAR
VAR_IN_OUT
    TxBuffer        : ComBuffer;
    RxBuffer        : ComBuffer;
END_VAR
VAR_OUTPUT
    Error           : BOOL;
    ErrorID         : ComError_t;
END_VAR
END_FUNCTION_BLOCK

(* Section 5 *)
FUNCTION_BLOCK SendByte
VAR_INPUT
    SendByte        : BYTE;
END_VAR
VAR_IN_OUT
    TxBuffer        : ComBuffer;
END_VAR
VAR_OUTPUT
    Busy            : BOOL;
    Error           : ComError_t;
END_VAR
END_FUNCTION_BLOCK

(* Section 5 *)
FUNCTION_BLOCK SendString
VAR_INPUT
    SendString      : STRING;
END_VAR
VAR_IN_OUT
    TxBuffer        : ComBuffer;
END_VAR
VAR_OUTPUT
    Busy            : BOOL;
    Error           : ComError_t;
END_VAR
END_FUNCTION_BLOCK

(* Section 5 *)
FUNCTION_BLOCK ReceiveByte
VAR_IN_OUT
    RXBuffer        : ComBuffer;
END_VAR
VAR_OUTPUT
    ByteReceived    : BOOL;
    ReceivedByte    : BYTE;
    Error           : ComError_t;
END_VAR
END_FUNCTION_BLOCK

(* Section 5 *)
FUNCTION_BLOCK ReceiveString
VAR_INPUT
    Prefix          : STRING;
    Suffix          : STRING;
    Timeout         : TIME;
    Reset           : BOOL;
END_VAR
VAR_IN_OUT
    ReceivedString  : STRING;
    RXBuffer        : ComBuffer;
END_VAR
VAR_OUTPUT
    StringReceived  : BOOL;
    busy            : BOOL;
    Error           : ComError_t;
    RxTimeout       : BOOL;
END_VAR
END_FUNCTION_BLOCK

(* Section 5 *)
FUNCTION_BLOCK ClearComBuffer
VAR_IN_OUT
    Buffer          : ComBuffer;
END_VAR
END_FUNCTION_BLOCK
