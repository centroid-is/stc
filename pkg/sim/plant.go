package sim

import (
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

// PlantModel represents a simulated physical system that responds to inputs
// over time. Models are stateful — they maintain internal state across calls.
type PlantModel interface {
	Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value
}

// MotorConfig configures a MotorModel.
type MotorConfig struct {
	MaxSpeed float64       // Maximum speed in RPM (default 1500)
	RampTime time.Duration // Time to reach max speed (default 2s)
}

// MotorModel simulates a simple motor with inertia-based speed ramping.
type MotorModel struct {
	cfg          MotorConfig
	currentSpeed float64
}

// NewMotorModel creates a MotorModel with the given config, applying defaults.
func NewMotorModel(cfg MotorConfig) *MotorModel {
	return &MotorModel{cfg: cfg}
}

func (m *MotorModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	return map[string]interp.Value{}
}

// ValveConfig configures a ValveModel.
type ValveConfig struct {
	TravelTime time.Duration // Time for full open/close (default 1s)
	MaxFlow    float64       // Maximum flow rate (default 100.0)
}

// ValveModel simulates a valve with linear open/close dynamics.
type ValveModel struct {
	cfg      ValveConfig
	position float64
}

// NewValveModel creates a ValveModel with the given config, applying defaults.
func NewValveModel(cfg ValveConfig) *ValveModel {
	return &ValveModel{cfg: cfg}
}

func (v *ValveModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	return map[string]interp.Value{}
}

// CylinderConfig configures a CylinderModel.
type CylinderConfig struct {
	Stroke float64 // Full stroke length in meters (default 1.0)
	Speed  float64 // Movement speed in m/s (default 0.1)
}

// CylinderModel simulates a hydraulic/pneumatic cylinder.
type CylinderModel struct {
	cfg      CylinderConfig
	position float64
}

// NewCylinderModel creates a CylinderModel with the given config, applying defaults.
func NewCylinderModel(cfg CylinderConfig) *CylinderModel {
	return &CylinderModel{cfg: cfg}
}

func (c *CylinderModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	return map[string]interp.Value{}
}
