TYPE E_DriveState :
(
	init := 0,
	cnf := 1,
	rdy := 2,
	run := 3
);
END_TYPE

TYPE E_402State :
(
	not_ready := 0,
	switch_on_disabled := 1,
	ready_to_switch_on := 2,
	fault := 3,
	switched_on := 4,
	operation_enabled := 5
);
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_Drive_HMI :
STRUCT
	p_cmd_JogFwd : BOOL;
	{attribute 'OPC.UA.DA.Access' := '1'}
	{attribute 'OPC.UA.DA.Description' := 'Drive fault'}
	p_stat_Error : BOOL;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_State : E_DriveState;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_402_State : E_402State;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_Frequency : REAL;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_SlaveId : UINT;
	p_cfg_AutoFreq : REAL;
END_STRUCT
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_Sensor_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_xRaw : BOOL;
	p_cfg_Invert : BOOL;
END_STRUCT
END_TYPE

TYPE ST_Bus :
STRUCT
	nNodes : INT;
	xOk : BOOL;
END_STRUCT
END_TYPE

TYPE ST_ConveyorCfg :
STRUCT
	rMax : REAL;
	eMode : E_DriveState;
END_STRUCT
END_TYPE
