package sim

import (
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

// CycleRecord captures the state of one simulation scan cycle.
type CycleRecord struct {
	Cycle   int                    `json:"cycle"`
	Time    time.Duration          `json:"time"`
	Inputs  map[string]interp.Value `json:"inputs"`
	Outputs map[string]interp.Value `json:"outputs"`
}

// SimResult holds the complete results of a simulation run.
type SimResult struct {
	Cycles    []CycleRecord `json:"cycles"`
	Duration  time.Duration `json:"duration"`
	NumCycles int           `json:"num_cycles"`
}
