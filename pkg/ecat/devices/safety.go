package devices

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// SafetyDiag models the standard (non-safe) diagnostics of TwinSAFE
// terminals: EL2912, EP1918-0002 and EL1904. It only writes field-voltage
// input entries (any entry whose name contains "Fieldvoltage" or "Field
// Voltage"): "...Underrange" and "...Overrange" read 0 while healthy.
// FSoE frames and every other entry pass through untouched; terminals
// without field-voltage entries (EP1918, EL1904 in the exports) get the
// model so they are recognised, and Step leaves their image alone.
//
// The exports carry only the Underrange/Overrange bits (PDO "FIELDVOLTAGE
// Field Voltage Status"); the EL2912 manual documents no separate status
// value, so no other field-voltage entry is written.
type SafetyDiag struct {
	Base
	under, over []ecat.Field
	underOn     bool
	overOn      bool
}

func safetyDiag() ecat.Device { return &SafetyDiag{} }

// Bind stores the layout and picks out the field-voltage entries.
func (s *SafetyDiag) Bind(l *ecat.Layout) {
	s.Base.Bind(l)
	s.under, s.over = nil, nil
	for _, f := range l.Fields(ecat.DirIn) {
		n := strings.ToLower(f.Entry)
		if !strings.Contains(n, "fieldvoltage") && !strings.Contains(n, "field voltage") {
			continue
		}
		switch {
		case strings.Contains(n, "underrange"):
			s.under = append(s.under, f)
		case strings.Contains(n, "overrange"):
			s.over = append(s.over, f)
		}
	}
}

// SetFieldVoltage sets the field-voltage Underrange and Overrange bits.
func (s *SafetyDiag) SetFieldVoltage(under, over bool) { s.underOn, s.overOn = under, over }

// Step writes the field-voltage bits only.
func (s *SafetyDiag) Step(_ time.Duration, out, in []byte) {
	for _, f := range s.under {
		ecat.Put(in, f, b2u(s.underOn))
	}
	for _, f := range s.over {
		ecat.Put(in, f, b2u(s.overOn))
	}
	s.stepIO(out, in)
}
