package interp

import (
	"testing"
	"time"
)

func TestTON_BasicOnDelay(t *testing.T) {
	ton := &TON{}
	ton.SetInput("PT", TimeValue(500*time.Millisecond))

	// IN=TRUE for 5 ticks of 100ms each
	ton.SetInput("IN", BoolValue(true))
	for i := 0; i < 4; i++ {
		ton.Execute(100 * time.Millisecond)
		q := ton.GetOutput("Q")
		if q.Bool {
			t.Fatalf("tick %d: Q should be FALSE before PT elapsed", i+1)
		}
	}
	// 5th tick: 500ms total, Q should fire
	ton.Execute(100 * time.Millisecond)
	q := ton.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE after 500ms")
	}
	// ET should be capped at PT
	et := ton.GetOutput("ET")
	if et.Time != 500*time.Millisecond {
		t.Fatalf("ET should be 500ms, got %v", et.Time)
	}
}

func TestTON_ETCappedAtPT(t *testing.T) {
	ton := &TON{}
	ton.SetInput("PT", TimeValue(200*time.Millisecond))
	ton.SetInput("IN", BoolValue(true))

	// Run for much longer than PT
	for i := 0; i < 10; i++ {
		ton.Execute(100 * time.Millisecond)
	}
	et := ton.GetOutput("ET")
	if et.Time != 200*time.Millisecond {
		t.Fatalf("ET should be capped at PT=200ms, got %v", et.Time)
	}
}

func TestTON_INFalseResets(t *testing.T) {
	ton := &TON{}
	ton.SetInput("PT", TimeValue(500*time.Millisecond))
	ton.SetInput("IN", BoolValue(true))
	ton.Execute(300 * time.Millisecond)

	// Drop IN
	ton.SetInput("IN", BoolValue(false))
	ton.Execute(100 * time.Millisecond)
	q := ton.GetOutput("Q")
	et := ton.GetOutput("ET")
	if q.Bool {
		t.Fatal("Q should be FALSE after IN dropped")
	}
	if et.Time != 0 {
		t.Fatalf("ET should be 0 after reset, got %v", et.Time)
	}
}

func TestTON_ShortPulseNoFire(t *testing.T) {
	ton := &TON{}
	ton.SetInput("PT", TimeValue(500*time.Millisecond))

	// IN=TRUE for only 200ms
	ton.SetInput("IN", BoolValue(true))
	ton.Execute(100 * time.Millisecond)
	ton.Execute(100 * time.Millisecond)

	// IN drops
	ton.SetInput("IN", BoolValue(false))
	ton.Execute(100 * time.Millisecond)
	q := ton.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should never have fired for short pulse")
	}
}

func TestTON_Retrigger(t *testing.T) {
	ton := &TON{}
	ton.SetInput("PT", TimeValue(500*time.Millisecond))

	// First pulse: 300ms
	ton.SetInput("IN", BoolValue(true))
	ton.Execute(300 * time.Millisecond)

	// Drop and re-raise
	ton.SetInput("IN", BoolValue(false))
	ton.Execute(100 * time.Millisecond)
	ton.SetInput("IN", BoolValue(true))

	// ET should restart from 0
	ton.Execute(100 * time.Millisecond)
	et := ton.GetOutput("ET")
	if et.Time != 100*time.Millisecond {
		t.Fatalf("ET should restart from 0 on re-trigger, got %v", et.Time)
	}
}

func TestTOF_BasicOffDelay(t *testing.T) {
	tof := &TOF{}
	tof.SetInput("PT", TimeValue(500*time.Millisecond))

	// IN rises: Q=TRUE immediately
	tof.SetInput("IN", BoolValue(true))
	tof.Execute(100 * time.Millisecond)
	q := tof.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE when IN rises")
	}

	// IN falls: Q stays TRUE for PT
	tof.SetInput("IN", BoolValue(false))
	for i := 0; i < 4; i++ {
		tof.Execute(100 * time.Millisecond)
		q = tof.GetOutput("Q")
		if !q.Bool {
			t.Fatalf("tick %d: Q should stay TRUE during off-delay", i+1)
		}
	}

	// After PT: Q=FALSE
	tof.Execute(100 * time.Millisecond)
	q = tof.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE after PT elapsed")
	}
}

func TestTOF_ReactivateBeforePT(t *testing.T) {
	tof := &TOF{}
	tof.SetInput("PT", TimeValue(500*time.Millisecond))

	// IN rises
	tof.SetInput("IN", BoolValue(true))
	tof.Execute(100 * time.Millisecond)

	// IN falls, wait 200ms
	tof.SetInput("IN", BoolValue(false))
	tof.Execute(100 * time.Millisecond)
	tof.Execute(100 * time.Millisecond)

	// IN rises again before PT expires
	tof.SetInput("IN", BoolValue(true))
	tof.Execute(100 * time.Millisecond)

	q := tof.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should stay TRUE when IN re-activates before PT")
	}
}

func TestTP_FixedPulse(t *testing.T) {
	tp := &TP{}
	tp.SetInput("PT", TimeValue(300*time.Millisecond))

	// Rising edge: start pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(100 * time.Millisecond)
	q := tp.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE during pulse")
	}

	tp.Execute(100 * time.Millisecond)
	q = tp.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE during pulse at 200ms")
	}

	// 300ms: pulse ends
	tp.Execute(100 * time.Millisecond)
	q = tp.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE after PT elapsed")
	}

	// ET capped at PT
	et := tp.GetOutput("ET")
	if et.Time != 300*time.Millisecond {
		t.Fatalf("ET should be capped at PT=300ms, got %v", et.Time)
	}
}

func TestTP_INChangesIgnoredDuringPulse(t *testing.T) {
	tp := &TP{}
	tp.SetInput("PT", TimeValue(500*time.Millisecond))

	// Start pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(100 * time.Millisecond)

	// Drop IN during pulse
	tp.SetInput("IN", BoolValue(false))
	tp.Execute(100 * time.Millisecond)
	q := tp.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should stay TRUE during pulse even if IN drops")
	}

	// Raise IN again during pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(100 * time.Millisecond)
	q = tp.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should stay TRUE during pulse")
	}
}

func TestTP_ResetAfterPulseAndINFalse(t *testing.T) {
	tp := &TP{}
	tp.SetInput("PT", TimeValue(200*time.Millisecond))

	// Start pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(100 * time.Millisecond)
	tp.Execute(100 * time.Millisecond)

	// Pulse complete, but IN still true
	tp.Execute(100 * time.Millisecond)
	q := tp.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE after PT elapsed")
	}

	// Drop IN
	tp.SetInput("IN", BoolValue(false))
	tp.Execute(100 * time.Millisecond)

	// New rising edge should start new pulse
	tp.SetInput("IN", BoolValue(true))
	tp.Execute(100 * time.Millisecond)
	q = tp.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE for new pulse after reset")
	}
}
