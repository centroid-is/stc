package emit

import (
	"os"
	"strings"
	"testing"
)

func TestEmitGVL(t *testing.T) {
	for _, name := range []string{"gvl1.st", "gvl2.st", "ECT.st"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../tests/twincat_probes/" + name)
			if err != nil {
				t.Fatal(err)
			}
			out := emitClean(t, string(data), DefaultOptions())
			if !strings.Contains(out, "VAR_GLOBAL") || !strings.Contains(out, "END_VAR") {
				t.Fatalf("GVL block missing:\n%s", out)
			}
			if again := emitClean(t, out, DefaultOptions()); again != out {
				t.Fatalf("emit not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}

	t.Run("ECT attributes in order", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/ECT.st")
		if err != nil {
			t.Fatal(err)
		}
		out := emitClean(t, string(data), DefaultOptions())
		assertEmitInOrder(t, out,
			"END_TYPE",
			`{attribute "qualified_only"}`,
			"VAR_GLOBAL RETAIN PERSISTENT",
			"{attribute 'TcLinkTo'",
			"X : ST_EL1008;",
			"{attribute 'OPC.UA.DA.Description'",
			"nLost : UINT;",
			"END_VAR",
		)
	})

	t.Run("aggregated blocks with block attribute", func(t *testing.T) {
		src := "VAR_GLOBAL\n\ta : INT;\nEND_VAR\n{attribute 'second'}\nVAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		out := emitClean(t, src, DefaultOptions())
		assertEmitInOrder(t, out, "VAR_GLOBAL\n", "a : INT;", "{attribute 'second'}", "VAR_GLOBAL CONSTANT", "c : INT := 5;")
	})
}
