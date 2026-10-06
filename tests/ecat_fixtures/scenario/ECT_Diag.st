// Scenario fixture: per-slave EtherCAT diagnostics for Device 1, filled by
// MAIN like SVNCoreComponents FB_EcDeviceDiag. The GVL name comes from the
// file stem (ECT_Diag), so keep the file name.
TYPE ST_SimSlaveDiag :
STRUCT
	p_stat_nDeviceState : BYTE;  // ST_EcSlaveState.deviceState
	p_stat_nLinkState   : BYTE;  // ST_EcSlaveState.linkState
	p_stat_bError       : BOOL;  // deviceState error bit 16#10
	p_stat_bLinkDown    : BOOL;  // linkState <> 0
	p_stat_bOk          : BOOL;  // OP, no error, link up
END_STRUCT
END_TYPE

{attribute 'qualified_only'}
VAR_GLOBAL
	// Index i is bus position i - 1 (FB_EcGetAllSlaveStates buffer entry i - 1).
	Device_1_Diag : ARRAY[1..16] OF ST_SimSlaveDiag;
	nSlaves       : UINT;     // slaves reported by the last poll
	nPolls        : UDINT;    // completed diag polls
	nScanCount    : UDINT;    // MAIN scans
	rSetpoint     : REAL;     // ramped by the jam scenario
END_VAR
