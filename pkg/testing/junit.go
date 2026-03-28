package testing

import (
	"encoding/xml"
	"strings"
)

// JUnitTestSuites is the top-level JUnit XML element.
type JUnitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Time     float64          `xml:"time,attr"`
	Suites   []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a single test suite (one file).
type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test case.
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

// JUnitFailure describes a test failure.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// FormatJUnit formats test results as JUnit XML with XML declaration header.
func FormatJUnit(result *RunResult) ([]byte, error) {
	suites := JUnitTestSuites{
		Tests:    result.Total,
		Failures: result.Failed,
		Errors:   result.Errors,
		Time:     result.Duration.Seconds(),
	}

	for _, sr := range result.Suites {
		suite := JUnitTestSuite{
			Name:  sr.Name,
			Tests: len(sr.Tests),
			Time:  sr.Duration.Seconds(),
		}

		for _, tr := range sr.Tests {
			tc := JUnitTestCase{
				Name:      tr.Name,
				Classname: sr.Name,
				Time:      tr.Duration.Seconds(),
			}

			if !tr.Passed {
				// Collect failure messages
				var messages []string
				for _, a := range tr.Assertions {
					if !a.Passed {
						msg := a.Message
						if a.Position != "" {
							msg = a.Position + ": " + msg
						}
						messages = append(messages, msg)
					}
				}
				if tr.Error != "" {
					messages = append(messages, "runtime error: "+tr.Error)
				}

				failMsg := strings.Join(messages, "\n")
				tc.Failure = &JUnitFailure{
					Message: failMsg,
					Type:    "AssertionFailure",
					Content: failMsg,
				}
				suite.Failures++
			}

			suite.TestCases = append(suite.TestCases, tc)
		}

		suite.Errors = 0 // errors tracked at suite level if needed
		suites.Suites = append(suites.Suites, suite)
	}

	data, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return nil, err
	}

	// Prepend XML declaration
	return append([]byte(xml.Header), data...), nil
}
