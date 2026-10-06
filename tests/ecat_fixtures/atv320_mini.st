// Trimmed FB_ATV320 for the CI end-to-end test (Phase 26-04). Same I/O
// names and configurator states as SVNCoreComponents FB_ATV320:
// 0 cfgWaitForPreOp, 1 cfgWriteParams, 2 cfgCommitEEPROM, 3 cfgWaitCommit,
// 4 cfgPromoteOp, 5 cfgWaitForOp, 6 cfgReady, 7 cfgErrorState.
FUNCTION_BLOCK FB_MiniATV320
VAR_INPUT
	amsaddr AT %I* : AMSADDR;
	i_xFwd : BOOL;
	i_xReset : BOOL;
	i_uETA AT %I* : UINT;
	i_iRFR AT %I* : INT;
	i_uLCR AT %I* : UINT;
	i_uDI AT %I* : UINT;
	i_eLFT AT %I* : UINT;
	i_eHMIS AT %I* : UINT;
END_VAR
VAR_OUTPUT
	q_uCMD AT %Q* : UINT;
	q_iLFR AT %Q* : INT;
	q_uOL1R AT %Q* : UINT;
	q_uACC AT %Q* : UINT;
	q_uDEC AT %Q* : UINT;
	q_xError : BOOL;
	q_xReady : BOOL;
	configState : INT;
	nLastConfigError : UDINT;
END_VAR
VAR
	sNetId : T_AmsNetId;
	nSlaveAddr : UINT;
	ecState : FB_EcGetSlaveState;
	ec : ST_EcSlaveState;
	fbSetState : FB_EcSetSlaveState;
	fbWrite : FB_EcCoESdoWrite;
	nParam : INT;
	nHSP : UINT := 500;
	nACC : UINT := 30;
	nCommitMagic : UDINT := 16#65766173;
	eta : UINT;
END_VAR
sNetId := F_CreateAmsNetId(nIds := amsaddr.netId);
nSlaveAddr := amsaddr.port;
q_uACC := 30;
q_uDEC := 30;

ecState(sNetId := sNetId, nSlaveAddr := nSlaveAddr);
IF NOT ecState.bBusy AND NOT ecState.bError THEN
	ec := ecState.state;
	ecState(bExecute := FALSE);
	ecState(bExecute := TRUE);
ELSIF ecState.bError THEN
	ecState(bExecute := FALSE);
	ecState(bExecute := TRUE);
	ec.deviceState := 0;
END_IF;

CASE configState OF
	0:
		IF ec.deviceState = 2 THEN
			configState := 1;
			nParam := 0;
		END_IF;
	1:
		CASE nParam OF
			0:
				fbWrite(sNetId := sNetId, nSlaveAddr := nSlaveAddr, nIndex := 16#2001, nSubIndex := 5,
					pSrcBuf := ADR(nHSP), cbBufLen := SIZEOF(nHSP), bExecute := TRUE);
				nParam := 1;
			1, 3:
				fbWrite();
				IF NOT fbWrite.bBusy THEN
					IF fbWrite.bError THEN
						nLastConfigError := fbWrite.nErrId;
						configState := 7;
					END_IF;
					fbWrite(bExecute := FALSE);
					nParam := nParam + 1;
				END_IF;
			2:
				fbWrite(sNetId := sNetId, nSlaveAddr := nSlaveAddr, nIndex := 16#203C, nSubIndex := 2,
					pSrcBuf := ADR(nACC), cbBufLen := SIZEOF(nACC), bExecute := TRUE);
				nParam := 3;
			4:
				configState := 2;
		END_CASE;
	2:
		fbWrite(sNetId := sNetId, nSlaveAddr := nSlaveAddr, nIndex := 16#2032, nSubIndex := 1,
			pSrcBuf := ADR(nCommitMagic), cbBufLen := SIZEOF(nCommitMagic), bExecute := TRUE);
		configState := 3;
	3:
		fbWrite();
		IF NOT fbWrite.bBusy THEN
			IF fbWrite.bError THEN
				nLastConfigError := fbWrite.nErrId;
				configState := 7;
			ELSE
				configState := 4;
			END_IF;
			fbWrite(bExecute := FALSE);
		END_IF;
	4:
		fbSetState(sNetId := sNetId, nSlaveAddr := nSlaveAddr, reqState := 16#08, bExecute := TRUE);
		configState := 5;
	5:
		fbSetState();
		IF NOT fbSetState.bBusy THEN
			IF fbSetState.bError THEN
				nLastConfigError := fbSetState.nErrId;
				configState := 7;
			ELSE
				configState := 6;
			END_IF;
			fbSetState(bExecute := FALSE);
		END_IF;
	6:
		IF ec.deviceState <> 8 THEN
			configState := 0;
		END_IF;
END_CASE;
q_xReady := configState = 6 AND ec.deviceState = 8;

// CiA402 sequencing from the decoded status word
eta := i_uETA AND 16#6F;
q_xError := (i_uETA AND 16#4F) = 16#08 OR (i_uETA AND 16#4F) = 16#0F;
IF q_xError THEN
	IF i_xReset THEN
		q_uCMD := 16#80;
	ELSE
		q_uCMD := 0;
	END_IF;
ELSIF NOT i_xFwd OR NOT q_xReady THEN
	q_uCMD := 16#06;
ELSIF eta = 16#21 THEN
	q_uCMD := 16#07;
ELSIF eta = 16#23 OR eta = 16#27 THEN
	q_uCMD := 16#0F;
ELSE
	q_uCMD := 16#06;
END_IF;
IF i_xFwd THEN
	q_iLFR := 500;
ELSE
	q_iLFR := 0;
END_IF;
END_FUNCTION_BLOCK

{attribute 'qualified_only'}
VAR_GLOBAL
	{attribute 'TcLinkTo' := '.q_uCMD := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^CMD;
				.q_iLFR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^LFR;
				.q_uOL1R := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^OL1R;
				.q_uACC := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^ACC;
				.q_uDEC := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^DEC;
				.i_uETA := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^ETA;
				.i_iRFR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^RFR;
				.i_uLCR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^LCR;
				.i_uDI := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^DI;
				.i_eLFT := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^LFT;
				.i_eHMIS := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^HMIS;
				.amsaddr := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^InfoData^AdsAddr;'}
	drive : FB_MiniATV320;
	{attribute 'TcLinkTo' := 'TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^InfoData^State'}
	driveState AT %I* : UINT;
END_VAR

PROGRAM MAIN
VAR
	diagGo : BOOL;
	fbStates : FB_EcGetAllSlaveStates;
	fbMaster : FB_EcGetMasterState;
	fbCrc : FB_EcGetAllSlaveCrcErrors;
	aStates : ARRAY[0..3] OF ST_EcSlaveState;
	aCrc : ARRAY[0..3] OF UDINT;
	statesBusy, masterBusy, crcBusy : BOOL;
	masterState : WORD;
END_VAR
ECT.drive();
fbStates(sNetId := '', pStateBuf := ADR(aStates), cbBufLen := SIZEOF(aStates), bExecute := diagGo);
fbMaster(sNetId := '', bExecute := diagGo);
fbCrc(sNetId := '', pCrcErrorBuf := ADR(aCrc), cbBufLen := SIZEOF(aCrc), bExecute := diagGo);
statesBusy := fbStates.bBusy;
masterBusy := fbMaster.bBusy;
crcBusy := fbCrc.bBusy;
masterState := fbMaster.state;
END_PROGRAM
