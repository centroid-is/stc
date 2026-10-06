package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/interp"
	"github.com/stretchr/testify/require"
)

// TestLoadProjectSild runs the imported production projects when
// STC_SILD_DIR points at a sildarvinnsla checkout (RUNT-09). It is skipped
// otherwise so CI stays hermetic. ST301 is not listed: its sources reference
// an FB_TwoWayConveyor type and a ST_LineRecipe member that the project does
// not declare, so `stc check` rejects it before it can run.
func TestLoadProjectSild(t *testing.T) {
	dir := os.Getenv("STC_SILD_DIR")
	if dir == "" {
		t.Skip("STC_SILD_DIR not set")
	}
	for _, name := range []string{"ST101"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name, name+" solution.tsproj")
			spec, _, err := loadProjectSpec([]string{path}, nil)
			require.NoError(t, err)
			p, err := interp.LoadProject(spec)
			require.NoError(t, err)
			for i := 0; i < 1000; i++ {
				require.NoError(t, p.Tick(), "tick %d", i)
			}
		})
	}
}
