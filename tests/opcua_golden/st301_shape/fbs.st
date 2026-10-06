FUNCTION_BLOCK FB_Drive
VAR
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Drive_HMI;
	tmrStart : TON;
	pAxis : POINTER TO INT;
END_VAR
IF HMI.p_stat_State = E_DriveState.init THEN
	HMI.p_stat_State := E_DriveState.rdy;
	HMI.p_stat_402_State := E_402State.switched_on;
END_IF;
HMI.p_stat_Frequency := HMI.p_cfg_AutoFreq;
HMI.p_stat_Error := HMI.p_cmd_JogFwd AND HMI.p_cfg_AutoFreq < 0.0;
tmrStart(IN := HMI.p_cmd_JogFwd, PT := T#1S);
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Sensor
VAR
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Sensor_HMI;
	xRawIn : BOOL;
END_VAR
HMI.p_stat_xRaw := xRawIn XOR HMI.p_cfg_Invert;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Conveyor
VAR
	xRun : BOOL;
	{attribute 'OPC.UA.DA' := '0'}
	tmrDelay : TON;
	pDrive : POINTER TO INT;
	{attribute 'OPC.UA.DA.Access' := '1'}
	rSpeed : REAL;
	Cfg : ST_ConveyorCfg;
END_VAR
tmrDelay(IN := xRun, PT := T#500MS);
IF tmrDelay.Q THEN
	rSpeed := Cfg.rMax;
ELSE
	rSpeed := 0.0;
END_IF;
END_FUNCTION_BLOCK
