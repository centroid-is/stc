package interp

import (
	"testing"
	"time"
)

func TestCTU_RisingEdgeCount(t *testing.T) {
	ctu := &CTU{}
	ctu.SetInput("PV", IntValue(3))
	ctu.SetInput("R", BoolValue(false))

	// 3 rising edges
	for i := 0; i < 3; i++ {
		ctu.SetInput("CU", BoolValue(true))
		ctu.Execute(0)
		ctu.SetInput("CU", BoolValue(false))
		ctu.Execute(0)
	}

	q := ctu.GetOutput("Q")
	cv := ctu.GetOutput("CV")
	if !q.Bool {
		t.Fatal("Q should be TRUE after 3 rising edges with PV=3")
	}
	if cv.Int != 3 {
		t.Fatalf("CV should be 3, got %d", cv.Int)
	}
}

func TestCTU_HeldTRUENoDoubleCount(t *testing.T) {
	ctu := &CTU{}
	ctu.SetInput("PV", IntValue(5))
	ctu.SetInput("R", BoolValue(false))

	// Hold CU=TRUE for multiple scans
	ctu.SetInput("CU", BoolValue(true))
	ctu.Execute(0)
	ctu.Execute(0)
	ctu.Execute(0)

	cv := ctu.GetOutput("CV")
	if cv.Int != 1 {
		t.Fatalf("CV should be 1 (only one rising edge), got %d", cv.Int)
	}
}

func TestCTU_ResetPriority(t *testing.T) {
	ctu := &CTU{}
	ctu.SetInput("PV", IntValue(5))
	ctu.SetInput("R", BoolValue(false))

	// Count up to 2
	for i := 0; i < 2; i++ {
		ctu.SetInput("CU", BoolValue(true))
		ctu.Execute(0)
		ctu.SetInput("CU", BoolValue(false))
		ctu.Execute(0)
	}

	// Reset
	ctu.SetInput("R", BoolValue(true))
	ctu.Execute(0)

	cv := ctu.GetOutput("CV")
	q := ctu.GetOutput("Q")
	if cv.Int != 0 {
		t.Fatalf("CV should be 0 after reset, got %d", cv.Int)
	}
	if q.Bool {
		t.Fatal("Q should be FALSE after reset")
	}
}

func TestCTD_RisingEdgeDecrement(t *testing.T) {
	ctd := &CTD{}
	ctd.SetInput("PV", IntValue(5))

	// Load
	ctd.SetInput("LD", BoolValue(true))
	ctd.Execute(0)
	ctd.SetInput("LD", BoolValue(false))
	ctd.Execute(0)

	cv := ctd.GetOutput("CV")
	if cv.Int != 5 {
		t.Fatalf("CV should be 5 after load, got %d", cv.Int)
	}

	// 5 rising edges to decrement to 0
	for i := 0; i < 5; i++ {
		ctd.SetInput("CD", BoolValue(true))
		ctd.Execute(0)
		ctd.SetInput("CD", BoolValue(false))
		ctd.Execute(0)
	}

	q := ctd.GetOutput("Q")
	cv = ctd.GetOutput("CV")
	if !q.Bool {
		t.Fatal("Q should be TRUE when CV<=0")
	}
	if cv.Int != 0 {
		t.Fatalf("CV should be 0, got %d", cv.Int)
	}
}

func TestCTD_LoadPriority(t *testing.T) {
	ctd := &CTD{}
	ctd.SetInput("PV", IntValue(10))

	// Both LD and CD true: LD takes priority
	ctd.SetInput("LD", BoolValue(true))
	ctd.SetInput("CD", BoolValue(true))
	ctd.Execute(0)

	cv := ctd.GetOutput("CV")
	if cv.Int != 10 {
		t.Fatalf("CV should be PV=10 (LD priority), got %d", cv.Int)
	}
}

func TestCTUD_UpDownReset(t *testing.T) {
	ctud := &CTUD{}
	ctud.SetInput("PV", IntValue(5))
	ctud.SetInput("R", BoolValue(false))
	ctud.SetInput("LD", BoolValue(false))

	// Count up 3 times
	for i := 0; i < 3; i++ {
		ctud.SetInput("CU", BoolValue(true))
		ctud.SetInput("CD", BoolValue(false))
		ctud.Execute(0)
		ctud.SetInput("CU", BoolValue(false))
		ctud.Execute(0)
	}

	cv := ctud.GetOutput("CV")
	if cv.Int != 3 {
		t.Fatalf("CV should be 3 after 3 up counts, got %d", cv.Int)
	}

	// Count down once
	ctud.SetInput("CD", BoolValue(true))
	ctud.Execute(0)
	ctud.SetInput("CD", BoolValue(false))
	ctud.Execute(0)

	cv = ctud.GetOutput("CV")
	if cv.Int != 2 {
		t.Fatalf("CV should be 2 after down count, got %d", cv.Int)
	}

	// Reset
	ctud.SetInput("R", BoolValue(true))
	ctud.Execute(0)
	cv = ctud.GetOutput("CV")
	if cv.Int != 0 {
		t.Fatalf("CV should be 0 after reset, got %d", cv.Int)
	}
}

func TestCTUD_Priority(t *testing.T) {
	ctud := &CTUD{}
	ctud.SetInput("PV", IntValue(5))

	// R takes priority over LD
	ctud.SetInput("R", BoolValue(true))
	ctud.SetInput("LD", BoolValue(true))
	ctud.Execute(0)

	cv := ctud.GetOutput("CV")
	if cv.Int != 0 {
		t.Fatalf("R should take priority, CV should be 0, got %d", cv.Int)
	}

	// LD takes priority over CU/CD
	ctud.SetInput("R", BoolValue(false))
	ctud.SetInput("LD", BoolValue(true))
	ctud.SetInput("CU", BoolValue(true))
	ctud.Execute(0)

	cv = ctud.GetOutput("CV")
	if cv.Int != 5 {
		t.Fatalf("LD should take priority, CV should be PV=5, got %d", cv.Int)
	}
}

// Ensure all counters use no wall clock
func TestCounters_NoWallClock(t *testing.T) {
	// All counter Execute methods accept time.Duration but ignore it
	ctu := &CTU{}
	ctu.SetInput("CU", BoolValue(true))
	ctu.SetInput("PV", IntValue(1))
	ctu.Execute(time.Hour) // Large dt should not affect counting
	if ctu.GetOutput("CV").Int != 1 {
		t.Fatal("counter should not depend on dt")
	}
}
