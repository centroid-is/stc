package interp

import "testing"

func TestRTRIG_RisingEdge(t *testing.T) {
	rt := &RTRIG{}

	// CLK FALSE -> TRUE: Q should be TRUE for one scan
	rt.SetInput("CLK", BoolValue(true))
	rt.Execute(0)
	q := rt.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE on rising edge")
	}

	// Second scan with CLK still TRUE: Q=FALSE
	rt.Execute(0)
	q = rt.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE on second scan with CLK=TRUE")
	}
}

func TestRTRIG_StaysFalse(t *testing.T) {
	rt := &RTRIG{}

	// CLK stays FALSE
	rt.SetInput("CLK", BoolValue(false))
	rt.Execute(0)
	q := rt.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE when CLK stays FALSE")
	}

	// Multiple scans
	rt.Execute(0)
	rt.Execute(0)
	q = rt.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should remain FALSE")
	}
}

func TestRTRIG_MultipleTransitions(t *testing.T) {
	rt := &RTRIG{}

	// First rising edge
	rt.SetInput("CLK", BoolValue(true))
	rt.Execute(0)
	if !rt.GetOutput("Q").Bool {
		t.Fatal("Q should be TRUE on first rising edge")
	}

	// Drop CLK
	rt.SetInput("CLK", BoolValue(false))
	rt.Execute(0)
	if rt.GetOutput("Q").Bool {
		t.Fatal("Q should be FALSE when CLK drops")
	}

	// Second rising edge
	rt.SetInput("CLK", BoolValue(true))
	rt.Execute(0)
	if !rt.GetOutput("Q").Bool {
		t.Fatal("Q should be TRUE on second rising edge")
	}

	// Stays TRUE: no more edge
	rt.Execute(0)
	if rt.GetOutput("Q").Bool {
		t.Fatal("Q should be FALSE after edge passes")
	}
}

func TestFTRIG_FallingEdge(t *testing.T) {
	ft := &FTRIG{}

	// Set CLK to TRUE first
	ft.SetInput("CLK", BoolValue(true))
	ft.Execute(0)
	q := ft.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE on rising edge (F_TRIG)")
	}

	// CLK TRUE -> FALSE: Q should be TRUE for one scan
	ft.SetInput("CLK", BoolValue(false))
	ft.Execute(0)
	q = ft.GetOutput("Q")
	if !q.Bool {
		t.Fatal("Q should be TRUE on falling edge")
	}

	// Second scan with CLK still FALSE: Q=FALSE
	ft.Execute(0)
	q = ft.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE on second scan after falling edge")
	}
}

func TestFTRIG_StaysTRUE(t *testing.T) {
	ft := &FTRIG{}

	// CLK stays TRUE
	ft.SetInput("CLK", BoolValue(true))
	ft.Execute(0)
	ft.Execute(0)
	ft.Execute(0)
	q := ft.GetOutput("Q")
	if q.Bool {
		t.Fatal("Q should be FALSE when CLK stays TRUE")
	}
}
