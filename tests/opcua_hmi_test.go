package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/awcullen/opcua/ua"
)

// HMI stand-in (OPCUA-10, D-13): read exactly the node ids the tfc Flutter
// HMI's keymappings.json names, through `stc serve`, decoding structs by
// served field names and enums as name(value).

// keymappings is the current tfc schema; only opcua_node matters here.
type keymappings struct {
	Nodes map[string]struct {
		OpcuaNode *struct {
			Namespace   uint16  `json:"namespace"`
			Identifier  string  `json:"identifier"`
			ArrayIndex  *int    `json:"array_index"`
			ServerAlias *string `json:"server_alias"`
		} `json:"opcua_node"`
		M2400Node json.RawMessage `json:"m2400_node"`
		IO        *bool           `json:"io"`
		Collect   json.RawMessage `json:"collect"`
	} `json:"nodes"`
}

// hmiTarget is one keymapping entry the default server answers.
type hmiTarget struct {
	key string
	id  ua.NodeID
}

// loadKeymappings returns the entries served by the default (un-aliased)
// server, sorted by key, and the number of skipped entries: those with a
// server_alias (other OPC UA servers or tfc's aggregate) or no opcua_node.
func loadKeymappings(t *testing.T, path string) ([]hmiTarget, int) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var km keymappings
	if err := json.Unmarshal(b, &km); err != nil {
		t.Fatalf("%s: %v", filepath.Base(path), err)
	}
	var out []hmiTarget
	skipped := 0
	for key, e := range km.Nodes {
		n := e.OpcuaNode
		if n == nil || n.ServerAlias != nil {
			skipped++
			continue
		}
		out = append(out, hmiTarget{key: key, id: hmiNodeID(n.Namespace, n.Identifier)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out, skipped
}

// hmiNodeID maps a keymapping node to a NodeId: numeric identifiers in
// namespace 0 (for example "2259") are numeric, everything else is a string.
func hmiNodeID(ns uint16, ident string) ua.NodeID {
	if ns == 0 {
		if n, err := strconv.ParseUint(ident, 10, 32); err == nil {
			return ua.NewNodeIDNumeric(0, uint32(n))
		}
	}
	return ua.NewNodeIDString(ns, ident)
}

// TestHMIKeymappings reads every stand-in entry from stc serve on the
// st301_shape fixture and checks the HMI-equivalent decoding.
func TestHMIKeymappings(t *testing.T) {
	targets, skipped := loadKeymappings(t, filepath.Join(opcuaGolden, "hmi_keymappings.json"))
	if skipped != 3 {
		t.Errorf("skipped %d entries, want 3 (two aliased servers, one m2400 node)", skipped)
	}
	byKey := map[string]ua.NodeID{}
	for _, tg := range targets {
		byKey[tg.key] = tg.id
	}
	if byKey["Server.State"] != ua.VariableIDServerServerStatusState {
		t.Fatalf("Server.State maps to %v, want i=2259", byKey["Server.State"])
	}

	r := newUAReader(dialAnonymous(t, execServe(t, st301Fixture)))

	// The first scan sets the drive state; wait for it.
	var motor map[string]any
	deadline := time.Now().Add(10 * time.Second)
	for {
		v, st, err := r.read(byKey["Line1.Motor1"])
		if err != nil || st != ua.Good {
			t.Fatalf("Line1.Motor1: %v %v", st, err)
		}
		motor = v.(map[string]any)
		if motor["p_stat_State"] == "rdy(2)" || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := motor["p_stat_State"]; got != "rdy(2)" {
		t.Errorf("Line1.Motor1 p_stat_State = %v, want rdy(2)", got)
	}
	if got := motor["p_stat_402_State"]; got != "switched_on(4)" {
		t.Errorf("Line1.Motor1 p_stat_402_State = %v, want switched_on(4)", got)
	}
	var names []string
	for k := range motor {
		names = append(names, k)
	}
	sort.Strings(names)
	if want := "p_cfg_AutoFreq p_cmd_JogFwd p_stat_402_State p_stat_Error p_stat_Frequency p_stat_SlaveId p_stat_State"; strings.Join(names, " ") != want {
		t.Errorf("Line1.Motor1 fields %v", names)
	}

	good := 0
	for _, tg := range targets {
		v, st, err := r.read(tg.id)
		if err != nil || st != ua.Good {
			t.Errorf("%s (%v): %v %v", tg.key, tg.id, st, err)
			continue
		}
		good++
		switch tg.key {
		case "Line1.Motor1.State":
			if v != "rdy(2)" {
				t.Errorf("%s = %v, want rdy(2)", tg.key, v)
			}
		case "Conveyor.Mode":
			if s, ok := v.(string); !ok || !strings.HasSuffix(s, ")") {
				t.Errorf("%s = %#v, want name(value)", tg.key, v)
			}
		case "Sensor.IS11":
			if m, ok := v.(map[string]any); !ok || len(m) != 2 {
				t.Errorf("%s = %#v, want 2 fields", tg.key, v)
			}
		case "Line1.Motor1.Error", "Sensor.IS11.Raw":
			if _, ok := v.(bool); !ok {
				t.Errorf("%s = %T, want bool", tg.key, v)
			}
		}
	}
	t.Logf("HMI stand-in: %d good, %d bad, %d skipped", good, len(targets)-good, skipped)
}

// TestHMIKeymappingsReal reads the real HMI keymappings against a real
// project: ST301 by default, or STC_HMI_PROJECT (absolute, or relative to
// STC_SILD_DIR), because the current hmi/keymappings.json targets the
// legacy project. It is local-only: both inputs hold plant identifiers.
func TestHMIKeymappingsReal(t *testing.T) {
	km := os.Getenv("STC_HMI_KEYMAPPINGS")
	dir := os.Getenv("STC_SILD_DIR")
	if km == "" || dir == "" {
		t.Skip("STC_HMI_KEYMAPPINGS and STC_SILD_DIR not both set; real HMI inputs are local-only")
	}
	proj := filepath.Join(dir, "ST301", "ST301 solution.tsproj")
	if p := os.Getenv("STC_HMI_PROJECT"); p != "" {
		proj = p
		if !filepath.IsAbs(p) {
			proj = filepath.Join(dir, p)
		}
	}
	if _, err := os.Stat(proj); err != nil {
		t.Skipf("project unavailable: %v", err)
	}
	targets, skipped := loadKeymappings(t, km)
	r := newUAReader(dialAnonymous(t, execServe(t, proj)))
	var bad []string
	for _, tg := range targets {
		if _, st, err := r.read(tg.id); err != nil {
			bad = append(bad, fmt.Sprintf("%v: %v", tg.id, err))
		} else if st != ua.Good {
			bad = append(bad, fmt.Sprintf("%v: %v", tg.id, st))
		}
	}
	t.Logf("real keymappings: %d good, %d bad, %d skipped", len(targets)-len(bad), len(bad), skipped)
	if len(bad) > 0 {
		shown := bad
		if len(shown) > 20 {
			shown = shown[:20]
		}
		t.Fatalf("%d HMI node ids do not read Good, first %d:\n%s", len(bad), len(shown), strings.Join(shown, "\n"))
	}
}

func TestHMINodeID(t *testing.T) {
	if id := hmiNodeID(0, "2259"); id != ua.NewNodeIDNumeric(0, 2259) {
		t.Errorf("ns0 numeric: %v", id)
	}
	if id := hmiNodeID(0, "Server"); id != ua.NewNodeIDString(0, "Server") {
		t.Errorf("ns0 string: %v", id)
	}
	if id := hmiNodeID(4, "123"); id != ua.NewNodeIDString(4, "123") {
		t.Errorf("ns4 digits stay a string: %v", id)
	}
}
