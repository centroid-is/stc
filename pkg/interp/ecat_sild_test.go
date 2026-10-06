package interp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/ecat/devices"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sildGateSrc instantiates the unmodified SVNCoreComponents FB_ATV320 and
// FB_EcDeviceDiag, with the ST301 TcLinkTo shape bound to the ATV320 fixture.
const sildGateSrc = `
{attribute 'qualified_only'}
VAR_GLOBAL
	{attribute 'TcLinkTo' := '.q_uCMD := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^CMD;
				.q_iLFR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^LFR;
				.q_uOL1R := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^OL1R;
				.q_uACC := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^ACC;
				.q_uDEC := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Outputs^DEC;
				.i_uETA := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^ETA;
				.i_iRFR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^RFR;
				.i_uLCR := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^LCR;
				.i_uDI := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^DI;
				.i_eLFT := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^LFT;
				.i_eHMIS := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^Inputs^HMIS;
				.amsaddr := TIID^Device 3 (EtherCAT)^SIM.FD01 (ATV320 EtherCAT)^InfoData^AdsAddr;'}
	drive : FB_ATV320;
	params : ST_MotorParams;
	diag : FB_EcDeviceDiag;
	aInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo;
	aDiag : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveDiag;
	diagBusySeen : BOOL;
END_VAR

PROGRAM MAIN
GATE.params.acc := 30;
GATE.params.dec := 30;
GATE.aInfo[1].p_stat_nPhysAddr := 1001;
GATE.drive(param := GATE.params);
GATE.diag(sNetId := '', nSlaveCount := 1, aInfo := GATE.aInfo, aDiag := GATE.aDiag);
IF GATE.diag.bBusy THEN
	GATE.diagBusySeen := TRUE;
END_IF;
END_PROGRAM
`

// sildFiles imports SVNCoreComponents from STC_SILD_DIR, read-only.
func sildFiles(t *testing.T) []*ast.SourceFile {
	t.Helper()
	dir := os.Getenv("STC_SILD_DIR")
	if dir == "" {
		t.Skip("STC_SILD_DIR not set; the SVNCoreComponents gate runs locally only")
	}
	proj := filepath.Join(dir, "SVNCoreComponents", "SVNCoreComponents", "SVNCoreComponents.plcproj")
	if _, err := os.Stat(proj); err != nil {
		t.Skipf("SVNCoreComponents project not found: %v", err)
	}
	m, _, err := twincat.Import(proj, twincat.Options{})
	require.NoError(t, err)
	user, libs, ds := twincat.ParseModel(m, nil)
	for _, d := range ds {
		require.NotEqual(t, diag.Error, d.Severity, "%s: %s", d.Pos, d.Message)
	}
	return append(libs, user...)
}

// FB_ATV320 inline configState ordinals.
const (
	sildCfgWriteParams = 1
	sildCfgReady       = 6
)

func TestEcatSildATV320Gate(t *testing.T) {
	files := sildFiles(t)
	gate := parseNoErr(t, "gate.st", sildGateSrc)
	ast.SetGVLName(gate, "GATE")
	r := newAtvRigFiles(t, append(files, gate))
	assert.Equal(t, ecat.StatePreOp, r.drv.EcState(), "drive boots in PreOp")

	// Configurator: PreOp, 5 s parameter window, promote to OP, ready.
	r.setG(BoolValue(true), "GATE", "drive", "i_xAuto")
	sawWrite := false
	n := r.until(200, "cfgReady", func() bool {
		cs := r.g("GATE", "drive", "configState").Int
		sawWrite = sawWrite || cs == sildCfgWriteParams
		return cs == sildCfgReady
	})
	t.Logf("FB_ATV320 reached cfgReady after %d scans (%v)", n, atvScan*time.Duration(n))
	assert.True(t, sawWrite, "passed through cfgWriteParams")
	assert.Equal(t, int64(0), r.g("GATE", "drive", "nLastConfigError").Int)
	assert.Equal(t, ecat.StateOP, r.drv.EcState())
	r.tick(3)
	assert.True(t, r.g("GATE", "drive", "q_xReady").Bool, "q_xReady")
	assert.Equal(t, uint16(1500), r.drv.HSP, "HSP synced from ST_MotorParams over SDO")

	// Auto run at 50 Hz: Operation enabled, RFR ramps to 500, HMIS run.
	hmi := r.g("GATE", "drive", "HMI").Clone()
	hmi.Struct["P_CFG_AUTOFREQ"] = Value{Kind: ValReal, Real: 50, IECType: hmi.Struct["P_CFG_AUTOFREQ"].IECType}
	r.setG(hmi, "GATE", "drive", "HMI")
	r.setG(BoolValue(true), "GATE", "drive", "i_xFwd")
	r.until(20, "operation enabled", func() bool { return r.drv.ETA()&0x6F == 0x27 })
	last := int16(-1)
	r.until(200, "RFR 500", func() bool {
		rfr := r.drv.RFR()
		require.GreaterOrEqual(t, rfr, last, "RFR must rise monotonically")
		last = rfr
		return rfr == 500
	})
	r.tick(2)
	assert.Equal(t, int64(500), r.g("GATE", "drive", "q_iLFR").Int)
	assert.Equal(t, uint16(devices.HMISRun), r.drv.HMIS())
	assert.InDelta(t, 50.0, r.g("GATE", "drive", "q_rFreq").Real, 1e-9)
	assert.Equal(t, int64(devices.HMISRun), r.g("GATE", "drive", "HMI").Struct["P_STAT_STATE"].Int)
	assert.False(t, r.g("GATE", "drive", "q_xError").Bool)

	// Fault: q_xError and the last fault on the HMI.
	r.drv.InjectFault(16)
	r.until(10, "q_xError", func() bool { return r.g("GATE", "drive", "q_xError").Bool })
	r.tick(2)
	assert.Equal(t, int64(16), r.g("GATE", "drive", "HMI").Struct["P_STAT_LASTFAULT"].Int)

	// FB_EcDeviceDiag filled the first record and the master state.
	rec := r.g("GATE", "aDiag").Array[1]
	assert.Equal(t, int64(8), rec.Struct["P_STAT_NDEVICESTATE"].Int)
	assert.True(t, rec.Struct["P_STAT_BOK"].Bool)
	assert.Equal(t, int64(8), r.g("GATE", "diag", "nMasterDevState").Int)
	assert.True(t, r.g("GATE", "diag", "bAllOp").Bool)
	assert.True(t, r.g("GATE", "diagBusySeen").Bool, "diag bBusy seen")
}
