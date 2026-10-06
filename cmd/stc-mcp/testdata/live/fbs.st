FUNCTION_BLOCK FB_Conveyor
VAR_INPUT
	xSensor : BOOL;
END_VAR
VAR_OUTPUT
	xRunning : BOOL;
END_VAR
VAR
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Conveyor_HMI;
END_VAR
IF HMI.p_cmd_Start THEN
	HMI.p_stat_Running := TRUE;
	HMI.p_cmd_Start := FALSE;
END_IF;
IF HMI.p_cmd_Stop THEN
	HMI.p_stat_Running := FALSE;
	HMI.p_cmd_Stop := FALSE;
END_IF;
HMI.p_stat_Sensor := xSensor;
xRunning := HMI.p_stat_Running;
END_FUNCTION_BLOCK
