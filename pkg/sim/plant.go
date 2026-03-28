package sim

import (
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

// PlantModel represents a simulated physical system that responds to inputs
// over time. Models are stateful — they maintain internal state across Update
// calls. All models are deterministic: same sequence of Update(inputs, dt)
// always produces the same outputs.
type PlantModel interface {
	Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value
}

// getInput performs case-insensitive lookup of a key in the inputs map.
func getInput(inputs map[string]interp.Value, key string) interp.Value {
	upper := strings.ToUpper(key)
	for k, v := range inputs {
		if strings.ToUpper(k) == upper {
			return v
		}
	}
	return interp.Value{}
}

// boolFromValue extracts a boolean from a Value using IsTruthy.
func boolFromValue(v interp.Value) bool {
	return v.IsTruthy()
}

// --- MotorModel ---

// MotorConfig configures a MotorModel.
type MotorConfig struct {
	MaxSpeed float64       // Maximum speed in RPM (default 1500)
	RampTime time.Duration // Time to reach max speed (default 2s)
}

// MotorModel simulates a simple motor with inertia-based speed ramping.
// Input: "RUN" (bool). Outputs: "SPEED" (real), "AT_SPEED" (bool).
type MotorModel struct {
	cfg          MotorConfig
	currentSpeed float64
}

// NewMotorModel creates a MotorModel with the given config, applying defaults.
func NewMotorModel(cfg MotorConfig) *MotorModel {
	if cfg.MaxSpeed == 0 {
		cfg.MaxSpeed = 1500.0
	}
	if cfg.RampTime == 0 {
		cfg.RampTime = 2 * time.Second
	}
	return &MotorModel{cfg: cfg}
}

// Update advances the motor simulation by dt. Accelerates toward MaxSpeed when
// RUN is true, decelerates toward 0 when false. Rate = MaxSpeed / RampTime.
func (m *MotorModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	run := boolFromValue(getInput(inputs, "RUN"))
	rate := m.cfg.MaxSpeed / m.cfg.RampTime.Seconds() // RPM per second

	if run {
		m.currentSpeed += rate * dt.Seconds()
	} else {
		m.currentSpeed -= rate * dt.Seconds()
	}

	// Clamp to [0, MaxSpeed]
	m.currentSpeed = math.Max(0, math.Min(m.currentSpeed, m.cfg.MaxSpeed))

	atSpeed := run && math.Abs(m.currentSpeed-m.cfg.MaxSpeed) < 0.01*m.cfg.MaxSpeed

	return map[string]interp.Value{
		"SPEED":    interp.RealValue(m.currentSpeed),
		"AT_SPEED": interp.BoolValue(atSpeed),
	}
}

// --- ValveModel ---

// ValveConfig configures a ValveModel.
type ValveConfig struct {
	TravelTime time.Duration // Time for full open/close (default 1s)
	MaxFlow    float64       // Maximum flow rate (default 100.0)
}

// ValveModel simulates a valve with linear open/close dynamics.
// Input: "OPEN" (bool). Outputs: "POSITION" (real 0..1), "FLOW" (real).
type ValveModel struct {
	cfg      ValveConfig
	position float64 // 0.0 = closed, 1.0 = fully open
}

// NewValveModel creates a ValveModel with the given config, applying defaults.
func NewValveModel(cfg ValveConfig) *ValveModel {
	if cfg.TravelTime == 0 {
		cfg.TravelTime = 1 * time.Second
	}
	if cfg.MaxFlow == 0 {
		cfg.MaxFlow = 100.0
	}
	return &ValveModel{cfg: cfg}
}

// Update advances the valve simulation by dt. Opens when OPEN is true, closes
// when false. Rate = 1 / TravelTime.
func (v *ValveModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	open := boolFromValue(getInput(inputs, "OPEN"))
	rate := dt.Seconds() / v.cfg.TravelTime.Seconds()

	if open {
		v.position += rate
	} else {
		v.position -= rate
	}

	// Clamp to [0, 1]
	v.position = math.Max(0, math.Min(v.position, 1.0))

	return map[string]interp.Value{
		"POSITION": interp.RealValue(v.position),
		"FLOW":     interp.RealValue(v.position * v.cfg.MaxFlow),
	}
}

// --- CylinderModel ---

// CylinderConfig configures a CylinderModel.
type CylinderConfig struct {
	Stroke float64 // Full stroke length in meters (default 1.0)
	Speed  float64 // Movement speed in m/s (default 0.1)
}

// CylinderModel simulates a hydraulic/pneumatic cylinder.
// Inputs: "EXTEND" (bool), "RETRACT" (bool).
// Outputs: "POSITION" (real 0..stroke), "AT_EXTEND" (bool), "AT_RETRACT" (bool).
type CylinderModel struct {
	cfg      CylinderConfig
	position float64
}

// NewCylinderModel creates a CylinderModel with the given config, applying defaults.
func NewCylinderModel(cfg CylinderConfig) *CylinderModel {
	if cfg.Stroke == 0 {
		cfg.Stroke = 1.0
	}
	if cfg.Speed == 0 {
		cfg.Speed = 0.1
	}
	return &CylinderModel{cfg: cfg}
}

// Update advances the cylinder simulation by dt. Extends when EXTEND is true
// (and not RETRACT), retracts when RETRACT is true (and not EXTEND).
// Both true = no movement.
func (c *CylinderModel) Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value {
	extend := boolFromValue(getInput(inputs, "EXTEND"))
	retract := boolFromValue(getInput(inputs, "RETRACT"))

	if extend && !retract {
		c.position += c.cfg.Speed * dt.Seconds()
	} else if retract && !extend {
		c.position -= c.cfg.Speed * dt.Seconds()
	}
	// Both true or both false = no movement

	// Clamp to [0, Stroke]
	c.position = math.Max(0, math.Min(c.position, c.cfg.Stroke))

	return map[string]interp.Value{
		"POSITION":   interp.RealValue(c.position),
		"AT_EXTEND":  interp.BoolValue(c.position >= c.cfg.Stroke),
		"AT_RETRACT": interp.BoolValue(c.position <= 0),
	}
}
