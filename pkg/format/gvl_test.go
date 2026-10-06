package format

import (
	"strings"
	"testing"
)

func TestFormatGVL(t *testing.T) {
	for _, name := range []string{"gvl1.st", "gvl2.st", "ECT.st"} {
		t.Run(name, func(t *testing.T) {
			out := formatClean(t, readProbe(t, name))
			if !strings.Contains(out, "VAR_GLOBAL") || !strings.Contains(out, "END_VAR") {
				t.Fatalf("GVL block missing:\n%s", out)
			}
			assertIdempotent(t, out)
		})
	}

	t.Run("ECT attributes and comments stay in place", func(t *testing.T) {
		out := formatClean(t, readProbe(t, "ECT.st"))
		assertInOrder(t, out,
			"END_TYPE",
			"// Synthetic EtherCAT GVL shape",
			`{attribute "qualified_only"}`,
			"VAR_GLOBAL RETAIN PERSISTENT",
			"// ==== Device 1 (EtherCAT) ====",
			"{attribute 'TcLinkTo'",
			"{attribute 'OPC.UA.DA' := '1'}",
			"{attribute 'OPC.UA.DA.StructuredType' := '1'}",
			"X : ST_EL1008;",
			"{attribute 'OPC.UA.DA.Description' := 'The slave controller''s own lost-link count'}",
			"nLost : UINT;",
			"END_VAR",
		)
		// The comment sits directly above the TcLinkTo attribute.
		if !strings.Contains(out, "// ==== Device 1 (EtherCAT) ====\n    {attribute 'TcLinkTo'") {
			t.Fatalf("comment not directly above TcLinkTo:\n%s", out)
		}
	})

	t.Run("aggregated blocks print at the first position", func(t *testing.T) {
		src := "VAR_GLOBAL\n\ta : INT;\nEND_VAR\nPROGRAM P\nEND_PROGRAM\n{attribute 'second'}\nVAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		out := formatClean(t, src)
		assertInOrder(t, out, "VAR_GLOBAL\n", "a : INT;", "{attribute 'second'}\nVAR_GLOBAL CONSTANT", "c : INT := 5;", "END_VAR", "PROGRAM P")
		assertIdempotent(t, out)
	})
}
