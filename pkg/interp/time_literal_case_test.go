package interp

import (
	"testing"
	"time"
)

// IEC 61131-3 time literal units are case-insensitive: TwinCAT accepts
// T#1D, T#7S, T#100MS and mixed forms just as it does the lowercase ones.
func TestParseLitTime_CaseInsensitiveUnits(t *testing.T) {
	interp := New()
	cases := []struct {
		lit  string
		want time.Duration
	}{
		{"T#1D", 24 * time.Hour},
		{"T#7S", 7 * time.Second},
		{"T#100MS", 100 * time.Millisecond},
		{"T#1M30S", 90 * time.Second},
		{"T#0S", 0},
		{"TIME#2H", 2 * time.Hour},
		{"t#1d", 24 * time.Hour}, // lowercase still works
	}
	for _, c := range cases {
		v, err := interp.parseLitTime(c.lit)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.lit, err)
			continue
		}
		if v.Time != c.want {
			t.Errorf("%s: got %v, want %v", c.lit, v.Time, c.want)
		}
	}
}
