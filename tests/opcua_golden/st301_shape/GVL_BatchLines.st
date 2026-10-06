VAR_GLOBAL
	Drives_Line1 : ARRAY[1..2] OF FB_Drive;
	{attribute 'OPC.UA.DA' := '2'}
	Internal_Bus_8 : ST_Bus;
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Description' := 'Batches on line $'1$' ($$ counted)'}
	nBatchCount : DINT := 17;
	{attribute 'OPC.UA.DA' := '1'}
	Conveyor : FB_Conveyor;
END_VAR
