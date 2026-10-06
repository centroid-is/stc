{attribute 'pack_mode' := '1'}
TYPE ST_EL1008 :
STRUCT
	I1 AT %I* : BOOL;
	I2 AT %I* : BOOL;
END_STRUCT
END_TYPE

// Synthetic EtherCAT GVL shape: qualified_only, PERSISTENT RETAIN, TcLinkTo, OPC UA attributes
{attribute "qualified_only"}

VAR_GLOBAL PERSISTENT RETAIN
	// ==== Device 1 (EtherCAT) ====
	{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^Term 1 (EK1200)^Term 3 (EL1008)^Channel 1^Input; .I2 := TIID^Device 1 (EtherCAT)^Term 1 (EK1200)^Term 3 (EL1008)^Channel 2^Input'}
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	X : ST_EL1008;

	{attribute 'OPC.UA.DA.Description' := 'The slave controller''s own lost-link count'}
	nLost : UINT;
END_VAR
