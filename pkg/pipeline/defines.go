package pipeline

// ParseDefines converts a slice of define strings (from CLI --define flags)
// into the map[string]bool format expected by Parse.
func ParseDefines(flags []string) map[string]bool {
	if len(flags) == 0 {
		return nil
	}
	defines := make(map[string]bool, len(flags))
	for _, d := range flags {
		defines[d] = true
	}
	return defines
}
