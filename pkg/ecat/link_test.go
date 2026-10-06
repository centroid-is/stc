package ecat

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTcLinkTo(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []Link
		wantErr string
	}{
		{
			name: "single form",
			in:   "TIID^Device 1 (EtherCAT)^InfoData^AmsNetId",
			want: []Link{{Target: "TIID^Device 1 (EtherCAT)^InfoData^AmsNetId"}},
		},
		{
			name: "single form trims outer and segment whitespace",
			in:   "  TIID ^ Device 1 (EtherCAT) ^InfoData^ AmsNetId  ",
			want: []Link{{Target: "TIID^Device 1 (EtherCAT)^InfoData^AmsNetId"}},
		},
		{
			name: "multi member with trailing semicolon",
			in:   ".I1 := TIID^A^B; .I2:=TIID^A^C;",
			want: []Link{{Member: "I1", Target: "TIID^A^B"}, {Member: "I2", Target: "TIID^A^C"}},
		},
		{
			name: "multi member wide whitespace and parentheses",
			in: ".p_stat_Enabled                 := TIID^Device 1 (EtherCAT)^ST301.A1.02 (EL9222-5500)^OCP Inputs Channel 1^Status^Enabled;\n" +
				"\t.p_cmd_Reset := TIID^Device 1 (EtherCAT)^ST301.A1.02 (EL9222-5500)^OCP Outputs Channel 1^Control^Reset",
			want: []Link{
				{Member: "p_stat_Enabled", Target: "TIID^Device 1 (EtherCAT)^ST301.A1.02 (EL9222-5500)^OCP Inputs Channel 1^Status^Enabled"},
				{Member: "p_cmd_Reset", Target: "TIID^Device 1 (EtherCAT)^ST301.A1.02 (EL9222-5500)^OCP Outputs Channel 1^Control^Reset"},
			},
		},
		{
			name: "nested member keeps inner dots",
			in:   ".sub.member := TIID^X",
			want: []Link{{Member: "sub.member", Target: "TIID^X"}},
		},
		{name: "empty value", in: "   ", wantErr: "empty"},
		{name: "member item without assign", in: ".I1 TIID^A; .I2 := TIID^B", wantErr: `".I1 TIID^A"`},
		{name: "empty member name", in: ". := TIID^A", wantErr: "empty member"},
		{name: "empty target", in: ".I1 := ;", wantErr: "empty target"},
		{name: "target without TIID", in: ".I1 := Device 1^X", wantErr: "TIID^"},
		{name: "single form without TIID", in: "Device 1^X", wantErr: "TIID^"},
		{name: "mixing single and member forms", in: ".I1 := TIID^A; TIID^B", wantErr: `"TIID^B"`},
		{name: "only semicolons", in: ".;;", wantErr: ":="},
		{name: "empty segment", in: "TIID^^X", wantErr: "empty segment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTcLinkTo(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
