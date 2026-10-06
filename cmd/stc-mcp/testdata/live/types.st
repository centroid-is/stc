TYPE ST_Conveyor_HMI :
STRUCT
	p_cmd_Start : BOOL;
	p_cmd_Stop : BOOL;
	p_stat_Running : BOOL;
	p_stat_Sensor : BOOL;
	p_cfg_Speed : INT := 50;
END_STRUCT
END_TYPE
