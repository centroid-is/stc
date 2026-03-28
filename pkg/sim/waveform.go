package sim

import (
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
type WaveformGenerator struct {
	cfg WaveformConfig
}

// NewWaveformGenerator creates a WaveformGenerator with the given config.
// Applies defaults: amplitude=1 if 0, frequency=1 if 0, dutyCycle=0.5 if 0.
func NewWaveformGenerator(cfg WaveformConfig) *WaveformGenerator {
	// TODO: stub - to be implemented
	return &WaveformGenerator{cfg: cfg}
}

// Generate returns the waveform value at time t. Pure function of t.
func (w *WaveformGenerator) Generate(t time.Duration) interp.Value {
	// TODO: stub - to be implemented
	return interp.RealValue(0)
}
