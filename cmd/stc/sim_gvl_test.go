package main

import (
	"encoding/json"
	"testing"
)

func TestSimCommandGVL(t *testing.T) {
	src := `VAR_GLOBAL
    gain : INT := 7;
END_VAR

PROGRAM P
VAR_OUTPUT
    OUT : INT;
END_VAR
    OUT := Plant.gain;
    gain := gain + 1;
END_PROGRAM
`
	path := writeTestST(t, t.TempDir(), "Plant.st", src)

	stdout, stderr, exitCode := runStc(t, "sim", path, "--cycles", "2", "--dt", "10ms", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	var result struct {
		Cycles []struct {
			Outputs map[string]struct {
				Int int64 `json:"Int"`
			} `json:"outputs"`
		} `json:"cycles"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(result.Cycles) != 2 {
		t.Fatalf("want 2 cycles, got %d:\n%s", len(result.Cycles), stdout)
	}
	if got := result.Cycles[1].Outputs["OUT"].Int; got != 8 {
		t.Errorf("cycle 1 OUT = %d, want 8 (program must see the GVL value):\n%s", got, stdout)
	}
}
