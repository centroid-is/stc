package ast

// TriviaKind identifies the type of trivia (non-semantic tokens).
type TriviaKind int

const (
	// TriviaWhitespace represents whitespace (spaces, tabs, newlines).
	TriviaWhitespace TriviaKind = iota
	// TriviaLineComment represents a single-line comment (// ...).
	TriviaLineComment
	// TriviaBlockComment represents a block comment ((* ... *)).
	TriviaBlockComment
)

var triviaKindNames = [...]string{
	TriviaWhitespace:    "Whitespace",
	TriviaLineComment:   "LineComment",
	TriviaBlockComment:  "BlockComment",
}

// String returns the human-readable name of a TriviaKind.
func (k TriviaKind) String() string {
	if int(k) < len(triviaKindNames) {
		return triviaKindNames[k]
	}
	return "Unknown"
}

// Trivia represents a non-semantic token attached to a node (whitespace or comment).
type Trivia struct {
	Kind TriviaKind `json:"kind"`
	Text string     `json:"text"`
	Span Span       `json:"span"`
}
