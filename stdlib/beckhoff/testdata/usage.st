(* Usage fixture: one instance of every shipped Tc2_EtherCAT, Tc2_ModbusSrv,
   Tc2_SerialCom FB plus FB_LocalSystemTime, each called with the Beckhoff
   parameter names. Checked against all stub closures by TestStubUsage. *)
PROGRAM StubUsage
VAR
    sNetId          : T_AmsNetId := '5.1.2.3.4.1';
    bExec           : BOOL;
    nSlave          : UINT := UINT#1001;
    pVoid           : PVOID;
    pByte           : POINTER TO BYTE;
    nBufLen         : UDINT := UDINT#64;
    tTmo            : TIME := T#2S;
    nWord           : WORD := WORD#16#0008;
    nSub            : BYTE := BYTE#1;
    nIdx            : WORD := WORD#16#8000;
    sIp             : STRING(15) := '192.168.1.10';
    nPort           : UINT := UINT#502;
    nUnit           : BYTE := BYTE#1;
    nQty            : WORD := WORD#4;
    nAddr           : WORD := WORD#100;
    pAddrs          : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF UINT;
    pCrcs           : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF DWORD;
    pStates         : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF ST_EcSlaveState;
    aStates         : ARRAY[0..127] OF ST_EcSlaveState;
    amsAddr         : AMSADDR;
    stAmsAddr       : ST_AmsAddr;
    aNetIds         : T_AmsNetIdArr;
    sCreated        : T_AmsNetId;
    nCopied         : UDINT;
    nCmp            : DINT;
    dtNow           : DT;
    bOp             : BOOL;
    txBuf           : ComBuffer;
    rxBuf           : ComBuffer;
    sRx             : STRING;
    eMode           : ComSerialLineMode_t := SERIALLINEMODE_EL6_22B;

    fbPhysWrite     : FB_EcPhysicalWriteCmd;
    fbAllAddr       : FB_EcGetAllSlaveAddr;
    fbAllCrc        : FB_EcGetAllSlaveCrcErrors;
    fbCrc           : FB_EcGetSlaveCrcError;
    fbCrcEx         : FB_EcGetSlaveCrcErrorEx;
    fbAllStates     : FB_EcGetAllSlaveStates;
    fbMaster        : FB_EcGetMasterState;
    fbState         : FB_EcGetSlaveState;
    fbSetState      : FB_EcSetSlaveState;
    fbSdoRead       : FB_EcCoESDoRead;
    fbSdoWrite      : FB_EcCoESdoWrite;

    fbReadCoils     : FB_MBReadCoils;
    fbReadInputs    : FB_MBReadInputs;
    fbReadRegs      : FB_MBReadRegs;
    fbReadInRegs    : FB_MBReadInputRegs;
    fbWriteCoil     : FB_MBWriteSingleCoil;
    fbWriteReg      : FB_MBWriteSingleReg;
    fbWriteCoils    : FB_MBWriteCoils;
    fbWriteRegs     : FB_MBWriteRegs;
    fbReadWrite     : FB_MBReadWriteRegs;

    fbLine          : SerialLineControl;
    fbSendByte      : SendByte;
    fbSendString    : SendString;
    fbRecvByte      : ReceiveByte;
    fbRecvString    : ReceiveString;
    fbClear         : ClearComBuffer;

    fbLocalTime     : FB_LocalSystemTime;
    fbRtc           : RTC;
END_VAR

(* Tc2_EtherCAT *)
fbPhysWrite(sNetId := sNetId, adp := nSlave, ado := UINT#16#0120, len := nBufLen,
            eType := eAdressingType_Fixed, pSrcBuf := pVoid, bExecute := bExec, tTimeout := tTmo);
fbAllAddr(sNetId := sNetId, pAddrBuf := pAddrs, cbBufLen := nBufLen, bExecute := bExec, tTimeout := tTmo);
fbAllCrc(sNetId := sNetId, pCrcErrorBuf := pCrcs, cbBufLen := nBufLen, bExecute := bExec, tTimeout := tTmo);
fbCrc(sNetId := sNetId, nSlaveAddr := nSlave, bExecute := bExec, tTimeout := tTmo);
fbCrcEx(sNetId := sNetId, nSlaveAddr := nSlave, bExecute := bExec, tTimeout := tTmo);
fbAllStates(sNetId := sNetId, pStateBuf := pStates, cbBufLen := nBufLen, bExecute := bExec, tTimeout := tTmo);
fbMaster(sNetId := sNetId, bExecute := bExec, tTimeout := tTmo);
fbState(sNetId := sNetId, nSlaveAddr := nSlave, bExecute := bExec, tTimeout := tTmo);
fbSetState(sNetId := sNetId, nSlaveAddr := nSlave, bExecute := bExec, tTimeout := tTmo, reqState := nWord);
fbSdoRead(sNetId := sNetId, nSlaveAddr := nSlave, nSubIndex := nSub, nIndex := nIdx,
          pDstBuf := pVoid, cbBufLen := nBufLen, bExecute := bExec, tTimeout := tTmo);
fbSdoWrite(sNetId := sNetId, nSlaveAddr := nSlave, nSubIndex := nSub, nIndex := nIdx,
           pSrcBuf := pVoid, cbBufLen := nBufLen, bExecute := bExec, tTimeout := tTmo);

bOp := fbState.state.deviceState = EC_DEVICE_STATE_OP
       AND aStates[0].linkState = EC_LINK_STATE_OK
       AND fbCrc.crcError.portA = fbCrcEx.CrcError.portD
       AND fbAllStates.nSlaves > UINT#0
       AND fbSetState.currState.deviceState = fbMaster.state
       AND fbSdoRead.cbRead > UDINT#0
       AND fbPhysWrite.wkc = UINT#1
       AND NOT (fbAllAddr.bBusy OR fbAllCrc.bError OR fbSdoWrite.nErrId <> UDINT#0);

(* Tc2_ModbusSrv *)
fbReadCoils(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
            cbLength := nBufLen, pDestAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbReadInputs(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
             cbLength := nBufLen, pDestAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbReadRegs(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
           cbLength := nBufLen, pDestAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbReadInRegs(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
             cbLength := nBufLen, pDestAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbWriteCoil(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nMBAddr := nAddr, nValue := WORD#16#FF00,
            bExecute := bExec, tTimeout := tTmo);
fbWriteReg(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nMBAddr := nAddr, nValue := nWord,
           bExecute := bExec, tTimeout := tTmo);
fbWriteCoils(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
             cbLength := nBufLen, pSrcAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbWriteRegs(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit, nQuantity := nQty, nMBAddr := nAddr,
            cbLength := nBufLen, pSrcAddr := pByte, bExecute := bExec, tTimeout := tTmo);
fbReadWrite(sIPAddr := sIp, nTCPPort := nPort, nUnitID := nUnit,
            nReadQuantity := nQty, nMBReadAddr := nAddr, nWriteQuantity := nQty, nMBWriteAddr := nAddr,
            cbDestLength := nBufLen, pDestAddr := pByte, cbSrcLength := nBufLen, pSrcAddr := pByte,
            bExecute := bExec, tTimeout := tTmo);
bExec := fbReadCoils.bBUSY OR fbReadRegs.bError OR fbReadWrite.cbRead > UDINT#0
         OR fbReadInputs.cbRead > fbReadInRegs.cbRead OR fbWriteCoil.bBUSY OR fbWriteReg.bError
         OR fbWriteCoils.nErrId <> fbWriteRegs.nErrId;

(* Tc2_SerialCom *)
fbLine(Mode := eMode, pComIn := pByte, pComOut := pByte, SizeComIn := INT#22,
       TxBuffer := txBuf, RxBuffer := rxBuf);
fbSendByte(SendByte := nSub, TxBuffer := txBuf);
fbSendString(SendString := sRx, TxBuffer := txBuf);
fbRecvByte(RXBuffer := rxBuf);
fbRecvString(Prefix := sRx, Suffix := sRx, Timeout := tTmo, Reset := bExec,
             ReceivedString := sRx, RXBuffer := rxBuf);
fbClear(Buffer := rxBuf);
bOp := fbLine.Error OR fbLine.ErrorID <> COMERROR_NOERROR OR fbSendByte.Busy
       OR fbSendString.Error = COMERROR_TIMEOUT OR fbRecvByte.ByteReceived
       OR fbRecvByte.ReceivedByte = nSub OR fbRecvString.StringReceived OR fbRecvString.RxTimeout;

(* Tc2_Utilities and Tc2_System *)
fbLocalTime(sNetID := sNetId, bEnable := bExec, dwCycle := DWORD#5, dwOpt := DWORD#1, tTimeout := tTmo);
IF fbLocalTime.bValid AND fbLocalTime.tzID <> eTimeZoneID_Invalid THEN
    dtNow := SYSTEMTIME_TO_DT(TIMESTR := fbLocalTime.systemTime);
END_IF;
fbRtc(EN := bExec, PDT := dtNow);
dtNow := fbRtc.CDT;

aNetIds[0] := BYTE#5;
sCreated := F_CreateAmsNetId(nIds := aNetIds);
amsAddr.netId[5] := BYTE#1;
amsAddr.port := WORD#851;
stAmsAddr.port := UINT#851;
nCopied := MEMCPY(destAddr := pVoid, srcAddr := pVoid, n := nBufLen);
nCopied := MEMSET(destAddr := pVoid, fillByte := USINT#0, n := nBufLen);
nCopied := MEMMOVE(destAddr := pVoid, srcAddr := pVoid, n := nBufLen);
nCmp := MEMCMP(pBuf1 := pVoid, pBuf2 := pVoid, n := nBufLen);
END_PROGRAM
