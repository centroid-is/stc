package sim

import (
	"math"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
)

// WaveformKind identifies the type of waveform pattern.
type WaveformKind int

const (
	WaveStep   WaveformKind = iota // Step function
	WaveRamp                       // Linear ramp (sawtooth)
	WaveSine                       // Sinusoidal
	WaveSquare                     // Square wave
)

// WaveformConfig configures a WaveformGenerator.
type WaveformConfig struct {
	Kind      WaveformKind
	Amplitude float64       // Peak amplitude (default 1.0)
	Frequency float64       // Hz (default 1.0, period = 1/frequency)
	Offset    float64       // DC offset (default 0.0)
	Delay     time.Duration // For Step: delay before stepping (default 0)
	DutyCycle float64       // For Square: fraction of period that is high (default 0.5)
}

// WaveformGenerator produces deterministic signal values as a pure function of time.
// It has no internal state — Generate is a pure function of t.
type WaveformGenerator struct {
	cfg WaveformConfig
}

// NewWaveformGenerator creates a WaveformGenerator with the given config.
// Applies defaults: amplitude=1 if 0, frequency=1 if 0, dutyCycle=0.5 if 0.
func NewWaveformGenerator(cfg WaveformConfig) *WaveformGenerator {
	if cfg.Amplitude == 0 {
		cfg.Amplitude = 1.0
	}
	if cfg.Frequency == 0 {
		cfg.Frequency = 1.0
	}
	if cfg.DutyCycle == 0 {
		cfg.DutyCycle = 0.5
	}
	return &WaveformGenerator{cfg: cfg}
}

// Generate returns the waveform value at time t. Pure function — same t always
// produces the same output. No wall-clock dependency.
func (w *WaveformGenerator) Generate(t time.Duration) interp.Value {
	switch w.cfg.Kind {
	case WaveStep:
		return w.generateStep(t)
	case WaveRamp:
		return w.generateRamp(t)
	case WaveSine:
		return w.generateSine(t)
	case WaveSquare:
		return w.generateSquare(t)
	default:
		return interp.RealValue(w.cfg.Offset)
	}
}

// generateStep returns offset before delay, offset+amplitude at/after delay.
func (w *WaveformGenerator) generateStep(t time.Duration) interp.Value {
	if t < w.cfg.Delay {
		return interp.RealValue(w.cfg.Offset)
	}
	return interp.RealValue(w.cfg.Offset + w.cfg.Amplitude)
}

// generateRamp produces a linear sawtooth that ramps from offset to offset+amplitude
// over each period, then wraps.
func (w *WaveformGenerator) generateRamp(t time.Duration) interp.Value {
	sec := t.Seconds()
	period := 1.0 / w.cfg.Frequency
	phase := math.Mod(sec, period)
	// Handle floating-point edge: negative mod result
	if phase < 0 {
		phase += period
	}
	frac := phase / period
	return interp.RealValue(w.cfg.Offset + w.cfg.Amplitude*frac)
}

// generateSine produces a sinusoidal waveform.
func (w *WaveformGenerator) generateSine(t time.Duration) interp.Value {
	sec := t.Seconds()
	val := w.cfg.Offset + w.cfg.Amplitude*math.Sin(2*math.Pi*w.cfg.Frequency*sec)
	return interp.RealValue(val)
}

// generateSquare produces a square wave with configurable duty cycle.
func (w *WaveformGenerator) generateSquare(t time.Duration) interp.Value {
	sec := t.Seconds()
	period := 1.0 / w.cfg.Frequency
	phase := math.Mod(sec, period)
	if phase < 0 {
		phase += period
	}
	if phase < w.cfg.DutyCycle*period {
		return interp.RealValue(w.cfg.Offset + w.cfg.Amplitude)
	}
	return interp.RealValue(w.cfg.Offset)
}
