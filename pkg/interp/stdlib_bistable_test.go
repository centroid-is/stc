package interp

import "testing"

func TestSR_SetDominant(t *testing.T) {
	sr := &SR{}

	// S1=TRUE, R=FALSE: Q1=TRUE
	sr.SetInput("S1", BoolValue(true))
	sr.SetInput("R", BoolValue(false))
	sr.Execute(0)
	if !sr.GetOutput("Q1").Bool {
		t.Fatal("SR: S1=TRUE, R=FALSE should set Q1=TRUE")
	}

	// S1=FALSE, R=TRUE: Q1=FALSE
	sr.SetInput("S1", BoolValue(false))
	sr.SetInput("R", BoolValue(true))
	sr.Execute(0)
	if sr.GetOutput("Q1").Bool {
		t.Fatal("SR: S1=FALSE, R=TRUE should reset Q1=FALSE")
	}

	// S1=TRUE, R=TRUE: Q1=TRUE (set dominant)
	sr.SetInput("S1", BoolValue(true))
	sr.SetInput("R", BoolValue(true))
	sr.Execute(0)
	if !sr.GetOutput("Q1").Bool {
		t.Fatal("SR: S1=TRUE, R=TRUE should be Q1=TRUE (set dominant)")
	}

	// S1=FALSE, R=FALSE: Q1 holds previous (TRUE)
	sr.SetInput("S1", BoolValue(false))
	sr.SetInput("R", BoolValue(false))
	sr.Execute(0)
	if !sr.GetOutput("Q1").Bool {
		t.Fatal("SR: S1=FALSE, R=FALSE should hold Q1=TRUE")
	}
}

func TestRS_ResetDominant(t *testing.T) {
	rs := &RS{}

	// S=TRUE, R1=FALSE: Q1=TRUE
	rs.SetInput("S", BoolValue(true))
	rs.SetInput("R1", BoolValue(false))
	rs.Execute(0)
	if !rs.GetOutput("Q1").Bool {
		t.Fatal("RS: S=TRUE, R1=FALSE should set Q1=TRUE")
	}

	// S=FALSE, R1=TRUE: Q1=FALSE
	rs.SetInput("S", BoolValue(false))
	rs.SetInput("R1", BoolValue(true))
	rs.Execute(0)
	if rs.GetOutput("Q1").Bool {
		t.Fatal("RS: S=FALSE, R1=TRUE should reset Q1=FALSE")
	}

	// S=TRUE, R1=TRUE: Q1=FALSE (reset dominant)
	rs.SetInput("S", BoolValue(true))
	rs.SetInput("R1", BoolValue(true))
	rs.Execute(0)
	if rs.GetOutput("Q1").Bool {
		t.Fatal("RS: S=TRUE, R1=TRUE should be Q1=FALSE (reset dominant)")
	}

	// S=FALSE, R1=FALSE: Q1 holds previous (FALSE)
	rs.SetInput("S", BoolValue(false))
	rs.SetInput("R1", BoolValue(false))
	rs.Execute(0)
	if rs.GetOutput("Q1").Bool {
		t.Fatal("RS: S=FALSE, R1=FALSE should hold Q1=FALSE")
	}
}

func TestSR_MemoryBehavior(t *testing.T) {
	sr := &SR{}

	// Initially Q1=FALSE
	sr.SetInput("S1", BoolValue(false))
	sr.SetInput("R", BoolValue(false))
	sr.Execute(0)
	if sr.GetOutput("Q1").Bool {
		t.Fatal("SR: initial state should be Q1=FALSE")
	}

	// Set
	sr.SetInput("S1", BoolValue(true))
	sr.Execute(0)
	if !sr.GetOutput("Q1").Bool {
		t.Fatal("SR: should be set")
	}

	// Remove set, keep no reset
	sr.SetInput("S1", BoolValue(false))
	sr.Execute(0)
	if !sr.GetOutput("Q1").Bool {
		t.Fatal("SR: should hold set state")
	}
}

func TestRS_MemoryBehavior(t *testing.T) {
	rs := &RS{}

	// Set
	rs.SetInput("S", BoolValue(true))
	rs.SetInput("R1", BoolValue(false))
	rs.Execute(0)
	if !rs.GetOutput("Q1").Bool {
		t.Fatal("RS: should be set")
	}

	// Hold
	rs.SetInput("S", BoolValue(false))
	rs.Execute(0)
	if !rs.GetOutput("Q1").Bool {
		t.Fatal("RS: should hold set state")
	}

	// Reset
	rs.SetInput("R1", BoolValue(true))
	rs.Execute(0)
	if rs.GetOutput("Q1").Bool {
		t.Fatal("RS: should be reset")
	}
}
