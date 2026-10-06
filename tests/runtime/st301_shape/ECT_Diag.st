{attribute 'qualified_only'}
VAR_GLOBAL
	Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [
		(p_stat_sName := 'S.A1.01 (EL6070)', p_stat_sModel := 'EL6070', p_stat_nPhysAddr := 1001),
		(p_stat_sName := 'S.A1.02 (EL1008)', p_stat_sModel := 'EL1008', p_stat_nPhysAddr := 1002),
		(p_stat_sName := 'S.A1.03 (EL2008)', p_stat_sModel := 'EL2008', p_stat_nPhysAddr := 1003)
	];
END_VAR
