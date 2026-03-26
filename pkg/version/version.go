// Package version provides build version information for the stc binary.
// Variables are set via ldflags at build time.
package version

import "fmt"

// Set via ldflags: -ldflags "-X github.com/centroid-is/stc/pkg/version.Version=1.0.0"
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return fmt.Sprintf("stc %s (commit: %s, built: %s)", Version, Commit, Date)
}
