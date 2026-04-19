package pgformat

import "strings"

// WordCase controls how a class of identifiers is cased in formatted output.
// It mirrors pgFormatter-style -u / -l style behavior per token category.
type WordCase int

const (
	// PreserveCase leaves spelling as in the source (after lexing).
	PreserveCase WordCase = iota
	// UpperCase forces UPPERCASE.
	UpperCase
	// LowerCase forces lowercase.
	LowerCase
)

// Options configures the layout engine. Defaults match typical pgFormatter
// snapshot behavior (4-space indent, uppercased clause keywords).
type Options struct {
	// Spaces is the number of spaces per indent level. If <= 0, DefaultOptions uses 4.
	Spaces int

	KeywordCase    WordCase
	TypeCase       WordCase
	FunctionCase   WordCase
	IdentifierCase WordCase

	// CommaBreak requests line breaks after commas in SELECT/GROUP BY/ORDER BY lists.
	// The layout engine honors this where list formatting is implemented.
	CommaBreak bool

	// WrapLimit is a target maximum line width before wrapping (0 = disabled).
	// Not yet implemented; reserved for pgFormatter parity work.
	WrapLimit int
}

// DefaultOptions returns settings aligned with common pgFormatter / snapshot usage.
func DefaultOptions() Options {
	return Options{
		Spaces:         4,
		KeywordCase:    UpperCase,
		TypeCase:       LowerCase,
		FunctionCase:   PreserveCase,
		IdentifierCase: PreserveCase,
		CommaBreak:     true,
		WrapLimit:      0,
	}
}

func (o Options) resolvedSpaces() int {
	if o.Spaces <= 0 {
		return 4
	}
	if o.Spaces > 64 {
		return 64
	}
	return o.Spaces
}

func (o Options) indentUnit() string {
	return strings.Repeat(" ", o.resolvedSpaces())
}
