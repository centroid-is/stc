package sim

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

const epsilon = 1e-9

func assertRealClose(t *testing.T, label string, got interp.Value, want float64) {
	t.Helper()
	if got.Kind != interp.ValReal {
		t.Fatalf("%s: expected ValReal, got %v", label, got.Kind)
	}
	if math.Abs(got.Real-want) > epsilon {
		t.Fatalf("%s: expected %f, got %f", label, want, got.Real)
	}
}

// --- Step waveform ---

func TestStepBeforeDelay(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveStep,
		Amplitude: 5.0,
		Offset:    1.0,
		Delay:     1 * time.Second,
	})
	v := g.Generate(500 * time.Millisecond)
	assertRealClose(t, "step before delay", v, 1.0) // offset only
}

func TestStepAfterDelay(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveStep,
		Amplitude: 5.0,
		Offset:    1.0,
		Delay:     1 * time.Second,
	})
	v := g.Generate(1500 * time.Millisecond)
	assertRealClose(t, "step after delay", v, 6.0) // offset + amplitude
}

func TestStepAtExactDelay(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveStep,
		Amplitude: 10.0,
		Delay:     2 * time.Second,
	})
	// At exactly t=delay, should still be offset (< not <=)
	v := g.Generate(2 * time.Second)
	assertRealClose(t, "step at exact delay", v, 10.0) // t >= delay triggers
}

func TestStepZeroDelay(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveStep,
		Amplitude: 3.0,
	})
	v := g.Generate(0)
	assertRealClose(t, "step zero delay at t=0", v, 3.0) // immediate step
}

// --- Ramp waveform ---

func TestRampAtStart(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveRamp,
		Amplitude: 10.0,
		Frequency: 1.0,
		Offset:    2.0,
	})
	v := g.Generate(0)
	assertRealClose(t, "ramp at start", v, 2.0) // offset
}

func TestRampMidway(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveRamp,
		Amplitude: 10.0,
		Frequency: 1.0,
		Offset:    0.0,
	})
	v := g.Generate(500 * time.Millisecond)
	assertRealClose(t, "ramp midway", v, 5.0)
}

func TestRampAtPeriodEnd(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveRamp,
		Amplitude: 10.0,
		Frequency: 1.0,
	})
	// At exactly t=period, wraps to 0 phase
	v := g.Generate(1 * time.Second)
	assertRealClose(t, "ramp at period end wraps", v, 0.0)
}

func TestRampLinearity(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveRamp,
		Amplitude: 100.0,
		Frequency: 2.0, // period = 0.5s
	})
	v1 := g.Generate(100 * time.Millisecond) // 0.1s -> 0.1/0.5 = 0.2 -> 20.0
	v2 := g.Generate(200 * time.Millisecond) // 0.2s -> 0.2/0.5 = 0.4 -> 40.0
	assertRealClose(t, "ramp linearity 0.1s", v1, 20.0)
	assertRealClose(t, "ramp linearity 0.2s", v2, 40.0)
}

// --- Sine waveform ---

func TestSineAtZero(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSine,
		Amplitude: 5.0,
		Frequency: 1.0,
		Offset:    2.0,
	})
	v := g.Generate(0)
	assertRealClose(t, "sine at 0", v, 2.0) // offset + amplitude*sin(0) = offset
}

func TestSinePeak(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSine,
		Amplitude: 5.0,
		Frequency: 1.0,
		Offset:    2.0,
	})
	// Peak at t = period/4 = 0.25s
	v := g.Generate(250 * time.Millisecond)
	assertRealClose(t, "sine peak", v, 7.0) // offset + amplitude
}

func TestSineTrough(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSine,
		Amplitude: 5.0,
		Frequency: 1.0,
		Offset:    2.0,
	})
	// Trough at t = 3*period/4 = 0.75s
	v := g.Generate(750 * time.Millisecond)
	assertRealClose(t, "sine trough", v, -3.0) // offset - amplitude
}

// --- Square waveform ---

func TestSquareHigh(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSquare,
		Amplitude: 10.0,
		Frequency: 0.5, // period = 2s
		Offset:    1.0,
	})
	// At t=0, should be high (offset + amplitude)
	v := g.Generate(0)
	assertRealClose(t, "square high", v, 11.0)
}

func TestSquareLow(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSquare,
		Amplitude: 10.0,
		Frequency: 0.5, // period = 2s
		Offset:    1.0,
		DutyCycle: 0.5,
	})
	// At t=1.5s into period of 2s, past duty cycle, should be low (offset only)
	v := g.Generate(1500 * time.Millisecond)
	assertRealClose(t, "square low", v, 1.0)
}

func TestSquareCustomDutyCycle(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSquare,
		Amplitude: 5.0,
		Frequency: 1.0, // period = 1s
		DutyCycle: 0.25,
	})
	// High for first 0.25s
	v1 := g.Generate(100 * time.Millisecond) // within duty cycle
	assertRealClose(t, "square custom duty high", v1, 5.0)

	// Low after 0.25s
	v2 := g.Generate(500 * time.Millisecond) // past duty cycle
	assertRealClose(t, "square custom duty low", v2, 0.0)
}

// --- Default config ---

func TestDefaultConfig(t *testing.T) {
	// Defaults: amplitude=1, frequency=1, dutyCycle=0.5
	g := NewWaveformGenerator(WaveformConfig{Kind: WaveSine})
	v := g.Generate(250 * time.Millisecond) // peak of sine with amp=1
	assertRealClose(t, "default config sine peak", v, 1.0)
}

// --- Determinism ---

func TestDeterminism(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveSine,
		Amplitude: 3.0,
		Frequency: 2.5,
		Offset:    1.0,
	})
	// Same t must always produce same value
	ts := 333 * time.Millisecond
	v1 := g.Generate(ts)
	v2 := g.Generate(ts)
	if v1.Real != v2.Real {
		t.Fatalf("determinism failed: %f != %f", v1.Real, v2.Real)
	}
}

// --- Return type ---

func TestWaveformReturnsRealValue(t *testing.T) {
	g := NewWaveformGenerator(WaveformConfig{Kind: WaveStep, Amplitude: 1.0})
	v := g.Generate(0)
	if v.Kind != interp.ValReal {
		t.Fatalf("expected ValReal, got %v", v.Kind)
	}
}
