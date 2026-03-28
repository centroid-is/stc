package testing

import "time"

// AssertionResultJSON is the JSON-friendly version of an assertion result.
type AssertionResultJSON struct {
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
	Position string `json:"position,omitempty"` // "file:line:col"
}

// TestResult holds the outcome of a single TEST_CASE execution.
type TestResult struct {
	Name       string                `json:"name"`
	File       string                `json:"file"`
	Line       int                   `json:"line"`
	Passed     bool                  `json:"passed"`
	Duration   time.Duration         `json:"duration_ns"`
	Assertions []AssertionResultJSON `json:"assertions,omitempty"`
	Error      string                `json:"error,omitempty"`
}

// SuiteResult holds results for all TEST_CASEs in a single file.
type SuiteResult struct {
	Name     string        `json:"name"`
	Tests    []TestResult  `json:"tests"`
	Duration time.Duration `json:"duration_ns"`
}

// RunResult holds the aggregate results of a test run.
type RunResult struct {
	Suites   []SuiteResult `json:"suites"`
	Total    int           `json:"total"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Errors   int           `json:"errors"`
	Duration time.Duration `json:"duration_ns"`
}

// HasFailures returns true if any test failed or errored.
func (r *RunResult) HasFailures() bool {
	return r.Failed > 0 || r.Errors > 0
}
