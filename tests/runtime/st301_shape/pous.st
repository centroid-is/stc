TYPE ST_EcSlaveInfo :
STRUCT
	p_stat_sName : STRING;
	p_stat_sModel : STRING;
	p_stat_nPhysAddr : UINT;
END_STRUCT
END_TYPE

{attribute 'to_string'}
TYPE hmis_e :
(
	Idle := 0,
	Running := 1,
	Faulted := 2
);
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_Drive_HMI :
STRUCT
	p_cfg_ManualFreq : REAL := 20.0;
	p_stat_xAuto : BOOL := TRUE;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_State : hmis_e;
	p_cmd_Start : BOOL;
	p_stat_Error : BOOL;
END_STRUCT
END_TYPE

FUNCTION_BLOCK FB_ATV320
VAR
	HMI : ST_Drive_HMI;
	n : INT;
END_VAR
n := n + 1;
IF HMI.p_cmd_Start THEN
	HMI.p_stat_State := hmis_e.Running;
END_IF
END_FUNCTION_BLOCK

PROGRAM MAIN
GVL.fb[1]();
GVL.fb[2]();
GVL.x();
END_PROGRAM
