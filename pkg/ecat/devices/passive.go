package devices

import "time"

// Passive models couplers and terminals without simulated process data
// (EK1100, EK1110, EK1200, EL6070, EL9011, CU2508). Step only keeps the
// generic Set/Get stimulus working.
type Passive struct{ Base }

// Step applies any Set overrides and otherwise leaves the image alone.
func (p *Passive) Step(_ time.Duration, out, in []byte) { p.stepIO(out, in) }
