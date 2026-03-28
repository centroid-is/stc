package sim

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

func assertPlantReal(t *testing.T, label string, outputs map[string]interp.Value, key string, want float64, tol float64) {
	t.Helper()
	v, ok := outputs[key]
	if !ok {
		t.Fatalf("%s: output %q not found", label, key)
	}
	if v.Kind != interp.ValReal {
		t.Fatalf("%s: output %q expected ValReal, got %v", label, key, v.Kind)
	}
	if math.Abs(v.Real-want) > tol {
		t.Fatalf("%s: output %q expected %f (+/- %f), got %f", label, key, want, tol, v.Real)
	}
}

func assertPlantBool(t *testing.T, label string, outputs map[string]interp.Value, key string, want bool) {
	t.Helper()
	v, ok := outputs[key]
	if !ok {
		t.Fatalf("%s: output %q not found", label, key)
	}
	if v.Kind != interp.ValBool {
		t.Fatalf("%s: output %q expected ValBool, got %v", label, key, v.Kind)
	}
	if v.Bool != want {
		t.Fatalf("%s: output %q expected %v, got %v", label, key, want, v.Bool)
	}
}

func makeInputs(pairs ...any) map[string]interp.Value {
	m := make(map[string]interp.Value)
	for i := 0; i < len(pairs); i += 2 {
		key := pairs[i].(string)
		switch v := pairs[i+1].(type) {
		case bool:
			m[key] = interp.BoolValue(v)
		case float64:
			m[key] = interp.RealValue(v)
		}
	}
	return m
}

// --- MotorModel ---

func TestPlantMotorInterface(t *testing.T) {
	m := NewMotorModel(MotorConfig{})
	var _ PlantModel = m // compile-time interface check
	_ = m
}

func TestPlantMotorRampUp(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	dt := 100 * time.Millisecond
	inputs := makeInputs("RUN", true)

	// After 1s (10 ticks of 100ms), should be at ~750 RPM (halfway)
	var out map[string]interp.Value
	for i := 0; i < 10; i++ {
		out = m.Update(inputs, dt)
	}
	assertPlantReal(t, "motor ramp up 1s", out, "SPEED", 750.0, 1.0)
	assertPlantBool(t, "motor not at speed yet", out, "AT_SPEED", false)
}

func TestPlantMotorAtSpeed(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	dt := 100 * time.Millisecond
	inputs := makeInputs("RUN", true)

	// After 2s (20 ticks), should be at max speed
	var out map[string]interp.Value
	for i := 0; i < 20; i++ {
		out = m.Update(inputs, dt)
	}
	assertPlantReal(t, "motor at max speed", out, "SPEED", 1500.0, 1.0)
	assertPlantBool(t, "motor at speed", out, "AT_SPEED", true)
}

func TestPlantMotorDecelerate(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	dt := 100 * time.Millisecond

	// Ramp up to full speed
	runInputs := makeInputs("RUN", true)
	for i := 0; i < 30; i++ {
		m.Update(runInputs, dt)
	}

	// Stop and decelerate for 1s
	stopInputs := makeInputs("RUN", false)
	var out map[string]interp.Value
	for i := 0; i < 10; i++ {
		out = m.Update(stopInputs, dt)
	}
	assertPlantReal(t, "motor decelerate 1s", out, "SPEED", 750.0, 1.0)
}

func TestPlantMotorStops(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	dt := 100 * time.Millisecond

	// Ramp up
	for i := 0; i < 30; i++ {
		m.Update(makeInputs("RUN", true), dt)
	}

	// Decelerate fully (2s+)
	var out map[string]interp.Value
	for i := 0; i < 25; i++ {
		out = m.Update(makeInputs("RUN", false), dt)
	}
	assertPlantReal(t, "motor fully stopped", out, "SPEED", 0.0, 0.1)
}

func TestPlantMotorDefaults(t *testing.T) {
	m := NewMotorModel(MotorConfig{})
	// Defaults: MaxSpeed=1500, RampTime=2s
	out := m.Update(makeInputs("RUN", true), 2*time.Second)
	assertPlantReal(t, "motor defaults at speed", out, "SPEED", 1500.0, 1.0)
}

func TestPlantMotorZeroDt(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	out := m.Update(makeInputs("RUN", true), 0)
	assertPlantReal(t, "motor zero dt", out, "SPEED", 0.0, 0.001)
}

// --- ValveModel ---

func TestPlantValveInterface(t *testing.T) {
	v := NewValveModel(ValveConfig{})
	var _ PlantModel = v
	_ = v
}

func TestPlantValveOpening(t *testing.T) {
	v := NewValveModel(ValveConfig{TravelTime: 1 * time.Second, MaxFlow: 100.0})
	dt := 100 * time.Millisecond
	inputs := makeInputs("OPEN", true)

	// After 500ms, should be at 50% open
	var out map[string]interp.Value
	for i := 0; i < 5; i++ {
		out = v.Update(inputs, dt)
	}
	assertPlantReal(t, "valve 50% open", out, "POSITION", 0.5, 0.01)
	assertPlantReal(t, "valve 50% flow", out, "FLOW", 50.0, 1.0)
}

func TestPlantValveFullyOpen(t *testing.T) {
	v := NewValveModel(ValveConfig{TravelTime: 1 * time.Second, MaxFlow: 100.0})
	dt := 100 * time.Millisecond
	inputs := makeInputs("OPEN", true)

	// After 1.5s, should be clamped at 1.0
	var out map[string]interp.Value
	for i := 0; i < 15; i++ {
		out = v.Update(inputs, dt)
	}
	assertPlantReal(t, "valve clamped at 1.0", out, "POSITION", 1.0, 0.001)
	assertPlantReal(t, "valve max flow", out, "FLOW", 100.0, 0.1)
}

func TestPlantValveClosing(t *testing.T) {
	v := NewValveModel(ValveConfig{TravelTime: 1 * time.Second, MaxFlow: 100.0})
	dt := 100 * time.Millisecond

	// Fully open
	for i := 0; i < 15; i++ {
		v.Update(makeInputs("OPEN", true), dt)
	}

	// Close for 500ms
	var out map[string]interp.Value
	for i := 0; i < 5; i++ {
		out = v.Update(makeInputs("OPEN", false), dt)
	}
	assertPlantReal(t, "valve closing", out, "POSITION", 0.5, 0.01)
}

func TestPlantValveClampAtZero(t *testing.T) {
	v := NewValveModel(ValveConfig{TravelTime: 1 * time.Second, MaxFlow: 100.0})
	// Never opened, close more
	out := v.Update(makeInputs("OPEN", false), 2*time.Second)
	assertPlantReal(t, "valve clamped at 0", out, "POSITION", 0.0, 0.001)
}

func TestPlantValveDefaults(t *testing.T) {
	v := NewValveModel(ValveConfig{})
	// Defaults: TravelTime=1s, MaxFlow=100
	out := v.Update(makeInputs("OPEN", true), 1*time.Second)
	assertPlantReal(t, "valve defaults fully open", out, "POSITION", 1.0, 0.001)
	assertPlantReal(t, "valve defaults max flow", out, "FLOW", 100.0, 0.1)
}

// --- CylinderModel ---

func TestPlantCylinderInterface(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{})
	var _ PlantModel = c
	_ = c
}

func TestPlantCylinderExtend(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	dt := 100 * time.Millisecond
	inputs := makeInputs("EXTEND", true, "RETRACT", false)

	// After 5 ticks (0.5s), position should be 0.05 (0.1 m/s * 0.5s)
	var out map[string]interp.Value
	for i := 0; i < 5; i++ {
		out = c.Update(inputs, dt)
	}
	assertPlantReal(t, "cylinder extending", out, "POSITION", 0.05, 0.001)
	assertPlantBool(t, "cylinder not at extend", out, "AT_EXTEND", false)
	assertPlantBool(t, "cylinder not at retract", out, "AT_RETRACT", false)
}

func TestPlantCylinderFullyExtended(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	dt := 1 * time.Second
	inputs := makeInputs("EXTEND", true, "RETRACT", false)

	// After 10s at 0.1 m/s, should be at stroke (clamped at 1.0)
	var out map[string]interp.Value
	for i := 0; i < 15; i++ {
		out = c.Update(inputs, dt)
	}
	assertPlantReal(t, "cylinder at stroke", out, "POSITION", 1.0, 0.001)
	assertPlantBool(t, "cylinder at extend", out, "AT_EXTEND", true)
}

func TestPlantCylinderRetract(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	dt := 1 * time.Second

	// Extend fully
	for i := 0; i < 15; i++ {
		c.Update(makeInputs("EXTEND", true, "RETRACT", false), dt)
	}

	// Retract for 5s (should move from 1.0 to 0.5)
	var out map[string]interp.Value
	for i := 0; i < 5; i++ {
		out = c.Update(makeInputs("EXTEND", false, "RETRACT", true), dt)
	}
	assertPlantReal(t, "cylinder retracting", out, "POSITION", 0.5, 0.001)
}

func TestPlantCylinderFullyRetracted(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	// At position 0, retract more — should clamp at 0
	out := c.Update(makeInputs("EXTEND", false, "RETRACT", true), 5*time.Second)
	assertPlantReal(t, "cylinder clamped at 0", out, "POSITION", 0.0, 0.001)
	assertPlantBool(t, "cylinder at retract", out, "AT_RETRACT", true)
}

func TestPlantCylinderBothInputs(t *testing.T) {
	// Both EXTEND and RETRACT true -> no movement
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	// Move to 0.5 first
	for i := 0; i < 50; i++ {
		c.Update(makeInputs("EXTEND", true, "RETRACT", false), 100*time.Millisecond)
	}
	out := c.Update(makeInputs("EXTEND", true, "RETRACT", true), 1*time.Second)
	assertPlantReal(t, "cylinder both inputs no move", out, "POSITION", 0.5, 0.001)
}

func TestPlantCylinderDefaults(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{})
	// Defaults: Stroke=1.0, Speed=0.1
	out := c.Update(makeInputs("EXTEND", true), 10*time.Second)
	assertPlantReal(t, "cylinder defaults fully extended", out, "POSITION", 1.0, 0.001)
}

func TestPlantCylinderZeroDt(t *testing.T) {
	c := NewCylinderModel(CylinderConfig{Stroke: 1.0, Speed: 0.1})
	out := c.Update(makeInputs("EXTEND", true), 0)
	assertPlantReal(t, "cylinder zero dt", out, "POSITION", 0.0, 0.001)
}

// --- Case insensitive inputs ---

func TestPlantCaseInsensitiveInputs(t *testing.T) {
	m := NewMotorModel(MotorConfig{MaxSpeed: 1500, RampTime: 2 * time.Second})
	inputs := map[string]interp.Value{
		"run": interp.BoolValue(true), // lowercase
	}
	out := m.Update(inputs, 1*time.Second)
	assertPlantReal(t, "case insensitive RUN", out, "SPEED", 750.0, 1.0)
}
