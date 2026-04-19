// Package pgformat pretty-prints a subset of PostgreSQL in a darold/pgFormatter-like
// style ([Format], [FormatWithOptions]). The pipeline is lex.go (tokenize) →
// layout.go / layout_helpers.go (clause layout).
//
// Supported top-level statements include SELECT, WITH, INSERT, UPDATE, DELETE,
// MERGE, and CREATE TABLE. Anything else is returned unchanged.
//
// Safety: [FormatWithOptions] compares the normalized token stream of the
// output to the input. If they differ, the original string is returned so the
// caller never gets a silently altered query.
package pgformat

import "strings"

// Format is [FormatWithOptions] with [DefaultOptions]. Empty input returns "".
func Format(sql string) string {
	return FormatWithOptions(sql, DefaultOptions())
}

// FormatWithOptions formats SQL using the given options. Empty input returns "".
//
// Safety: if the formatter would change the meaning of the query (i.e. the
// output token stream differs from the input ignoring whitespace), the
// original input is returned. The formatter only edits whitespace.
func FormatWithOptions(sql string, o Options) string {
	s := strings.TrimSpace(sql)
	if s == "" {
		return ""
	}
	out := layoutSQL(s, o)
	if out == "" {
		return s
	}
	if !sameTokenStream(s, out, o) {
		return s
	}
	return out
}

// sameTokenStream returns true when a and b tokenize to the same sequence of
// token kinds and literals after the same word-normalization the formatter
// applies (so case-folding and similar intentional edits don't trigger the
// safety fallback).
func sameTokenStream(a, b string, o Options) bool {
	ta := normalizeTokens(tokenize(a), o)
	tb := normalizeTokens(tokenize(b), o)
	if len(ta) != len(tb) {
		return false
	}
	for i := range ta {
		if ta[i].kind != tb[i].kind || ta[i].lit != tb[i].lit {
			return false
		}
	}
	return true
}
