// Types for the EtherCAT demo fixtures (Demo Device 1/2). Local stand-ins
// for AMSADDR and T_AmsNetIdArr keep the fixture self-contained.
TYPE T_AmsNetIdArr : ARRAY[0..5] OF BYTE;
END_TYPE

TYPE AMSADDR :
STRUCT
	netId : ARRAY[0..5] OF BYTE;
	port  : UINT;
END_STRUCT
END_TYPE

TYPE ST_DemoEL1008 :
STRUCT
	I1 AT %I* : BOOL;
	I2 AT %I* : BOOL;
	I3 AT %I* : BOOL;
	I4 AT %I* : BOOL;
	I5 AT %I* : BOOL;
	I6 AT %I* : BOOL;
	I7 AT %I* : BOOL;
	I8 AT %I* : BOOL;
END_STRUCT
END_TYPE

TYPE ST_DemoEL2008 :
STRUCT
	O1 AT %Q* : BOOL;
	O2 AT %Q* : BOOL;
	O3 AT %Q* : BOOL;
	O4 AT %Q* : BOOL;
	O5 AT %Q* : BOOL;
	O6 AT %Q* : BOOL;
	O7 AT %Q* : BOOL;
	O8 AT %Q* : BOOL;
END_STRUCT
END_TYPE

TYPE ST_DemoEL9222 :
STRUCT
	p_stat_Enabled AT %I* : BOOL;
	p_stat_Tripped AT %I* : BOOL;
	p_Current      AT %I* : UINT;
	p_cmd_Reset    AT %Q* : BOOL;
END_STRUCT
END_TYPE

FUNCTION_BLOCK FB_DemoDrive
VAR_OUTPUT
	uEtaSeen : UINT;
END_VAR
VAR
	q_uCMD  AT %Q* : UINT;
	q_iLFR  AT %Q* : INT;
	i_uETA  AT %I* : UINT;
	i_iRFR  AT %I* : INT;
	amsaddr AT %I* : AMSADDR;
END_VAR
uEtaSeen := i_uETA;
q_uCMD := 16#000F;
END_FUNCTION_BLOCK
