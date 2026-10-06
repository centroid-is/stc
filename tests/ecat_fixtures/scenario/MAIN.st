// Scenario fixture program: an interlock, a scan counter, the Device 1
// diagnostics poll and the ATV320 drive (requested to OP once at start).
PROGRAM MAIN
VAR
	sNetId      : T_AmsNetId;
	fbStates    : FB_EcGetAllSlaveStates;
	aStateBuf   : ARRAY[0..15] OF ST_EcSlaveState;
	fbDriveOp   : FB_EcSetSlaveState;
	i           : INT;
	nCnt        : INT;
	xDriveFault : BOOL;
END_VAR

ECT_Diag.nScanCount := ECT_Diag.nScanCount + 1;

// Interlock: the output follows the input while the OCP channel is enabled.
ECT.A1_02.O1 := ECT.A1_01.I1 AND ECT.A1_03.p_stat_Enabled;

// Drive: the ATV320 boots in PreOp and gates its PDOs until OP.
sNetId := F_CreateAmsNetId(ECT.Dev1_AmsNetId);
fbDriveOp(sNetId := sNetId, nSlaveAddr := ECT.CN01.amsaddr.port, reqState := 16#0008, bExecute := TRUE);
ECT.CN01();
xDriveFault := (ECT.CN01.i_uETA AND 16#0008) <> 0;

// Diagnostics: poll every slave state of Device 1 (FB_EcDeviceDiag rules).
fbStates(sNetId := sNetId, pStateBuf := ADR(aStateBuf), cbBufLen := SIZEOF(aStateBuf), bExecute := TRUE);
IF NOT fbStates.bBusy THEN
	IF NOT fbStates.bError THEN
		ECT_Diag.nSlaves := fbStates.nSlaves;
		nCnt := UINT_TO_INT(fbStates.nSlaves);
		IF nCnt > 16 THEN
			nCnt := 16;
		END_IF
		FOR i := 1 TO nCnt DO
			ECT_Diag.Device_1_Diag[i].p_stat_nDeviceState := aStateBuf[i - 1].deviceState;
			ECT_Diag.Device_1_Diag[i].p_stat_nLinkState := aStateBuf[i - 1].linkState;
			ECT_Diag.Device_1_Diag[i].p_stat_bError := (ECT_Diag.Device_1_Diag[i].p_stat_nDeviceState AND 16#10) <> 0;
			ECT_Diag.Device_1_Diag[i].p_stat_bLinkDown := ECT_Diag.Device_1_Diag[i].p_stat_nLinkState <> 0;
			ECT_Diag.Device_1_Diag[i].p_stat_bOk := (ECT_Diag.Device_1_Diag[i].p_stat_nDeviceState AND 16#0F) = 8
				AND NOT ECT_Diag.Device_1_Diag[i].p_stat_bError
				AND NOT ECT_Diag.Device_1_Diag[i].p_stat_bLinkDown;
		END_FOR
		ECT_Diag.nPolls := ECT_Diag.nPolls + 1;
	END_IF
	fbStates(bExecute := FALSE);
END_IF
END_PROGRAM
