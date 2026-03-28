package testing

import "encoding/json"

// FormatJSON formats test results as indented JSON.
func FormatJSON(result *RunResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
