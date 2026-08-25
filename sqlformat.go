package expect

import (
	"strings"
	"unicode"
)

// This file implements the SQL pretty-printer used by SQL. It is a pure Go
// reimplementation of the subset of pgFormatter's layout rules that this
// package relies on, so that formatting no longer requires Docker or any
// external binary. It uses only the standard library.
//
// Layout rules, derived from pgFormatter's output:
//
//   - Reserved words are upper-cased; identifiers and literals are left alone.
//   - Each top-level clause keyword (SELECT, FROM, WHERE, ...) starts a line at
//     the current indent, and the clause body is indented one level (4 spaces).
//   - Select lists, GROUP BY / ORDER BY lists and UPDATE ... SET assignments
//     are split on top-level commas, one item per line, comma trailing.
//   - Boolean operators (AND / OR) in WHERE and HAVING start a new line.
//   - JOIN clauses start a new line at the same indent as the driving table.
//   - LIMIT and OFFSET stay on one line with their argument.
//   - INSERT INTO keeps the target and column list on the first line and
//     indents VALUES; DELETE FROM keeps the target on the first line and keeps
//     its WHERE condition inline.
//   - Common table expressions are formatted recursively one level in.

const sqlIndent = "    "

// ---------------------------------------------------------------------------
// tokenizer
// ---------------------------------------------------------------------------

type tokKind int

const (
	tokWord tokKind = iota
	tokNumber
	tokString  // 'literal', "quoted ident", $$dollar quoted$$
	tokComment // -- line or /* block */
	tokPunct
)

type token struct {
	text  string // text as it should be emitted
	kind  tokKind
	space bool // whitespace separated this token from the previous one in the input
}

func (t token) upper() string { return strings.ToUpper(t.text) }

func (t token) isWord(words ...string) bool {
	if t.kind != tokWord {
		return false
	}
	u := t.upper()
	for _, w := range words {
		if u == w {
			return true
		}
	}
	return false
}

func isIdentRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// tokenize splits raw SQL into tokens, keeping string literals, quoted
// identifiers and comments byte-for-byte intact.
func tokenize(sql string) []token {
	var toks []token
	rs := []rune(sql)
	space := false

	for i := 0; i < len(rs); {
		r := rs[i]

		if unicode.IsSpace(r) {
			space = true
			i++
			continue
		}

		start := i
		var kind tokKind

		switch {
		// line comment
		case r == '-' && i+1 < len(rs) && rs[i+1] == '-':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			kind = tokComment

		// block comment (nesting, as PostgreSQL allows)
		case r == '/' && i+1 < len(rs) && rs[i+1] == '*':
			depth := 0
			for i < len(rs) {
				if rs[i] == '/' && i+1 < len(rs) && rs[i+1] == '*' {
					depth++
					i += 2
					continue
				}
				if rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '/' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				i++
			}
			kind = tokComment

		// dollar-quoted string: $tag$ ... $tag$
		case r == '$' && dollarTag(rs, i) != "":
			tag := dollarTag(rs, i)
			i += len([]rune(tag))
			for i < len(rs) {
				if strings.HasPrefix(string(rs[i:]), tag) {
					i += len([]rune(tag))
					break
				}
				i++
			}
			kind = tokString

		// single-quoted literal, '' escapes an inner quote
		case r == '\'':
			i++
			for i < len(rs) {
				if rs[i] == '\'' {
					if i+1 < len(rs) && rs[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			kind = tokString

		// double-quoted identifier, "" escapes an inner quote
		case r == '"':
			i++
			for i < len(rs) {
				if rs[i] == '"' {
					if i+1 < len(rs) && rs[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			kind = tokString

		case unicode.IsDigit(r):
			for i < len(rs) && unicode.IsDigit(rs[i]) {
				i++
			}
			// A dot belongs to the number only when a digit follows it.
			// Consuming it greedily makes "0.A" tokenize as "0." + "A", which
			// re-renders as "0. A" and breaks idempotency.
			if i+1 < len(rs) && rs[i] == '.' && unicode.IsDigit(rs[i+1]) {
				i++
				for i < len(rs) && unicode.IsDigit(rs[i]) {
					i++
				}
			}
			// Optional exponent, again only when it really is one.
			if i < len(rs) && (rs[i] == 'e' || rs[i] == 'E') {
				j := i + 1
				if j < len(rs) && (rs[j] == '+' || rs[j] == '-') {
					j++
				}
				if j < len(rs) && unicode.IsDigit(rs[j]) {
					i = j
					for i < len(rs) && unicode.IsDigit(rs[i]) {
						i++
					}
				}
			}
			kind = tokNumber

		case isIdentRune(r):
			for i < len(rs) && isIdentRune(rs[i]) {
				i++
			}
			kind = tokWord

		default:
			// multi-character operators first, then single characters
			if op := matchOperator(rs, i); op != "" {
				i += len([]rune(op))
			} else {
				i++
			}
			kind = tokPunct
		}

		text := string(rs[start:i])
		if kind == tokWord && isReservedWord(text) {
			text = strings.ToUpper(text)
		}
		toks = append(toks, token{text: text, kind: kind, space: space})
		space = false
	}

	return toks
}

var operators = []string{"<=", ">=", "<>", "!=", "||", "::", "->>", "->", "=", "<", ">"}

func matchOperator(rs []rune, i int) string {
	rest := string(rs[i:])
	for _, op := range operators {
		if strings.HasPrefix(rest, op) {
			return op
		}
	}
	return ""
}

// dollarTag returns the $tag$ delimiter starting at i, or "" if there is none.
func dollarTag(rs []rune, i int) string {
	if rs[i] != '$' {
		return ""
	}
	j := i + 1
	for j < len(rs) && rs[j] != '$' {
		if !isIdentRune(rs[j]) || unicode.IsDigit(rs[j]) && j == i+1 {
			return ""
		}
		j++
	}
	if j >= len(rs) {
		return ""
	}
	return string(rs[i : j+1])
}

// ---------------------------------------------------------------------------
// joining tokens back into a single line
// ---------------------------------------------------------------------------

func joinTokens(toks []token) string {
	var b strings.Builder
	for i, t := range toks {
		if i > 0 && needsSpace(toks[i-1], t) {
			b.WriteByte(' ')
		}
		b.WriteString(t.text)
	}
	return b.String()
}

func needsSpace(prev, cur token) bool {
	glue := false

	switch cur.text {
	case ",", ")", ";", ".", "::":
		glue = true
	}
	switch prev.text {
	case "(", ".", "::":
		glue = true
	}
	// "users (name, email)" keeps its space; "NOW()" does not. Preserve
	// whatever the input did, which is what pgFormatter does too.
	if cur.text == "(" {
		glue = !cur.space
	}

	if !glue {
		return true
	}
	// Only glue when doing so is reversible. Writing two adjacent punctuation
	// tokens with no space between them can produce a longer operator that
	// re-tokenizes differently — ":" followed by "::" would render as ":::",
	// which reads back as "::" followed by ":". Keeping the space makes
	// formatting idempotent for input like that.
	if !gluesCleanly(prev.text, cur.text) {
		return true
	}
	return false
}

// gluesCleanly reports whether concatenating a and b tokenizes back into
// exactly a and b.
func gluesCleanly(a, b string) bool {
	toks := tokenize(a + b)
	return len(toks) == 2 && toks[0].text == a && toks[1].text == b
}

// ---------------------------------------------------------------------------
// splitting helpers, all paren-aware
// ---------------------------------------------------------------------------

// splitTopLevel splits toks on commas that sit outside any parentheses.
func splitTopLevel(toks []token) [][]token {
	var out [][]token
	depth, start := 0, 0
	for i, t := range toks {
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
		case ",":
			if depth == 0 {
				out = append(out, toks[start:i])
				start = i + 1
			}
		}
	}
	if start < len(toks) {
		out = append(out, toks[start:])
	}
	return out
}

// splitBefore splits toks immediately before any top-level token for which
// match reports true. The matching token begins the following group.
func splitBefore(toks []token, match func(i int, t token) bool) [][]token {
	var out [][]token
	depth, start := 0, 0
	for i, t := range toks {
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
		}
		if depth == 0 && i > start && match(i, t) {
			out = append(out, toks[start:i])
			start = i
		}
	}
	if start < len(toks) {
		out = append(out, toks[start:])
	}
	return out
}

func isBoolOp(t token) bool { return t.isWord("AND", "OR") }

// joinPrefixes are the words that may lead a JOIN phrase.
var joinPrefixes = []string{"INNER", "OUTER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL"}

// joinStartsClause reports whether the token at i begins a JOIN phrase. Only
// the first word of the phrase qualifies, so "LEFT OUTER JOIN" stays on one
// line rather than breaking before each of its three words.
func joinStartsClause(toks []token, i int) bool {
	t := toks[i]
	if !t.isWord("JOIN") && !t.isWord(joinPrefixes...) {
		return false
	}
	if i > 0 && toks[i-1].isWord(joinPrefixes...) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// clause segmentation
// ---------------------------------------------------------------------------

// clause is one top-level segment of a statement: its leading keyword tokens
// and the tokens that follow up to the next clause keyword.
type clause struct {
	keyword string  // normalised, e.g. "ORDER BY"
	head    []token // the keyword tokens themselves
	body    []token
}

// clauseStarters are matched longest-first so that "ORDER BY" wins over "ORDER".
var clauseStarters = [][]string{
	{"INSERT", "INTO"},
	{"DELETE", "FROM"},
	{"GROUP", "BY"},
	{"ORDER", "BY"},
	{"UNION", "ALL"},
	{"LEFT", "JOIN"},
	{"SELECT"}, {"FROM"}, {"WHERE"}, {"HAVING"}, {"LIMIT"}, {"OFFSET"},
	{"VALUES"}, {"SET"}, {"RETURNING"}, {"UPDATE"}, {"UNION"}, {"INTERSECT"},
	{"EXCEPT"}, {"ON", "CONFLICT"},
}

// matchClauseStart returns the clause keyword beginning at index i, or "".
func matchClauseStart(toks []token, i int) (string, int) {
	for _, cand := range clauseStarters {
		if i+len(cand) > len(toks) {
			continue
		}
		ok := true
		for k, w := range cand {
			if !toks[i+k].isWord(w) {
				ok = false
				break
			}
		}
		if ok {
			// "LEFT JOIN" is only a clause start in the FROM body, which is
			// handled separately; treat it as ordinary text here.
			if cand[0] == "LEFT" {
				return "", 0
			}
			return strings.Join(cand, " "), len(cand)
		}
	}
	return "", 0
}

func splitClauses(toks []token) []clause {
	var out []clause
	depth := 0
	var cur *clause

	for i := 0; i < len(toks); {
		t := toks[i]
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
		}

		if depth == 0 {
			if kw, n := matchClauseStart(toks, i); kw != "" {
				out = append(out, clause{keyword: kw, head: toks[i : i+n]})
				cur = &out[len(out)-1]
				i += n
				continue
			}
		}
		if cur == nil {
			out = append(out, clause{})
			cur = &out[len(out)-1]
		}
		cur.body = append(cur.body, t)
		i++
	}
	return out
}

// ---------------------------------------------------------------------------
// emitting
// ---------------------------------------------------------------------------

type writer struct {
	lines []string
}

func (w *writer) add(indent int, s string) {
	if s == "" {
		return
	}
	w.lines = append(w.lines, strings.Repeat(sqlIndent, indent)+s)
}

// isLineComment reports whether t is a "--" comment, which runs to end of line
// and therefore must always be the last thing written on its line.
func isLineComment(t token) bool {
	return t.kind == tokComment && strings.HasPrefix(t.text, "--")
}

// addTokens writes a run of tokens, breaking the line after any "--" comment
// that is not already last. Without the break, everything after the comment
// would be commented out — a silent loss of SQL.
func (w *writer) addTokens(indent int, toks []token) {
	start := 0
	for i, t := range toks {
		if isLineComment(t) && i < len(toks)-1 {
			w.add(indent, joinTokens(toks[start:i+1]))
			start = i + 1
		}
	}
	if start < len(toks) {
		w.add(indent, joinTokens(toks[start:]))
	}
}

// emitList writes items one per line at the given indent, with a trailing
// comma on every item but the last.
func (w *writer) emitList(indent int, items [][]token) {
	// Drop empty items first. Deciding which item is last before filtering
	// leaves a trailing comma on the final visible line, which then formats
	// differently on a second pass.
	kept := make([][]token, 0, len(items))
	for _, item := range items {
		if joinTokens(item) != "" {
			kept = append(kept, item)
		}
	}
	for i, item := range kept {
		last := i == len(kept)-1
		// Put the separating comma before a trailing "--" comment, so the
		// comma is not swallowed by it.
		if !last && len(item) > 0 && isLineComment(item[len(item)-1]) {
			head, comment := item[:len(item)-1], item[len(item)-1]
			w.addTokens(indent, append(append([]token{}, head...), token{text: ",", kind: tokPunct}, comment))
			continue
		}
		if last {
			w.addTokens(indent, item)
			continue
		}
		w.addTokens(indent, append(append([]token{}, item...), token{text: ",", kind: tokPunct}))
	}
}

// formatStatement renders one statement (possibly with leading CTEs) at indent.
func formatStatement(toks []token, indent int, w *writer) {
	toks = trimTrailingSemicolon(toks, w == nil)

	if len(toks) > 0 && toks[0].isWord("WITH") {
		if ctes, rest, ok := parseCTEs(toks); ok {
			emitCTEs(ctes, indent, w)
			if len(rest) == 0 {
				return
			}
			toks = rest
		}
		// If the WITH block does not parse — truncated input, an unclosed
		// paren — fall through and emit the tokens verbatim rather than
		// dropping them.
	}

	clauses := splitClauses(toks)
	for _, c := range clauses {
		emitClause(c, indent, w, clauses)
	}
}

func trimTrailingSemicolon(toks []token, _ bool) []token {
	for len(toks) > 0 && toks[len(toks)-1].text == ";" {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// cte is one parsed common table expression.
type cte struct {
	header       string  // "name AS (" without the leading WITH
	inner        []token // the body between the parentheses
	trailingComa bool    // another CTE follows
}

// parseCTEs parses the leading WITH block. It reports ok only when the whole
// block is well formed, so that malformed input can be emitted verbatim rather
// than silently rewritten or dropped.
func parseCTEs(toks []token) (ctes []cte, rest []token, ok bool) {
	i := 1 // skip WITH
	prefix := ""
	if i < len(toks) && toks[i].isWord("RECURSIVE") {
		prefix = "RECURSIVE "
		i++
	}

	for i < len(toks) {
		nameStart := i
		for i < len(toks) && !toks[i].isWord("AS") {
			i++
		}
		if i >= len(toks) || i == nameStart {
			return nil, nil, false // no AS, or no name before it
		}
		header := joinTokens(toks[nameStart:i])
		i++ // skip AS

		for i < len(toks) && toks[i].isWord("MATERIALIZED", "NOT") {
			header += " " + toks[i].text
			i++
		}
		if i >= len(toks) || toks[i].text != "(" {
			return nil, nil, false
		}

		open := i
		depth := 0
		closed := false
		for i < len(toks) {
			switch toks[i].text {
			case "(":
				depth++
			case ")":
				depth--
				if depth == 0 {
					closed = true
				}
			}
			if closed {
				break
			}
			i++
		}
		if !closed {
			return nil, nil, false // unbalanced parentheses
		}

		c := cte{header: prefix + header, inner: toks[open+1 : i]}
		prefix = ""
		i++ // skip ")"

		if i < len(toks) && toks[i].text == "," {
			c.trailingComa = true
			ctes = append(ctes, c)
			i++
			continue
		}
		ctes = append(ctes, c)
		break
	}

	if len(ctes) == 0 {
		return nil, nil, false
	}
	return ctes, toks[i:], true
}

// emitCTEs writes parsed CTE blocks, formatting each body one level in.
func emitCTEs(ctes []cte, indent int, w *writer) {
	for n, c := range ctes {
		lead := ""
		if n == 0 {
			lead = "WITH "
		}
		w.add(indent, lead+c.header+" AS (")
		formatStatement(c.inner, indent+1, w)
		if c.trailingComa {
			w.add(indent, "),")
			continue
		}
		w.add(indent, ")")
	}
}

func emitClause(c clause, indent int, w *writer, all []clause) {
	head := joinTokens(c.head)

	// A fragment with no clause keyword (rare; malformed input) is emitted
	// verbatim so nothing is silently dropped.
	if head == "" {
		w.addTokens(indent, c.body)
		return
	}

	switch c.keyword {

	// Keyword and argument stay on one line.
	case "LIMIT", "OFFSET":
		w.add(indent, strings.TrimSpace(head+" "+joinTokens(c.body)))

	// Target and column list stay with the keyword; VALUES indents.
	case "INSERT INTO":
		w.add(indent, strings.TrimSpace(head+" "+joinTokens(c.body)))

	case "VALUES":
		w.add(indent+1, strings.TrimSpace(head+" "+joinTokens(c.body)))

	// DELETE FROM keeps its target inline, and so does the WHERE that follows.
	case "DELETE FROM":
		w.add(indent, strings.TrimSpace(head+" "+joinTokens(c.body)))

	case "WHERE", "HAVING":
		if inlineWhere(all) {
			w.add(indent, strings.TrimSpace(head+" "+joinTokens(c.body)))
			return
		}
		w.add(indent, head)
		for _, part := range splitBefore(c.body, func(_ int, t token) bool { return isBoolOp(t) }) {
			w.add(indent+1, joinTokens(part))
		}

	// The driving tables sit at body indent; each JOIN starts its own line.
	case "FROM":
		w.add(indent, head)
		groups := splitBefore(c.body, func(i int, _ token) bool { return joinStartsClause(c.body, i) })
		for gi, g := range groups {
			if gi == 0 {
				w.emitList(indent+1, splitTopLevel(g))
				continue
			}
			w.add(indent+1, joinTokens(g))
		}

	// Comma-separated lists, one item per line.
	case "SELECT", "GROUP BY", "ORDER BY", "SET", "RETURNING", "UPDATE":
		w.add(indent, head)
		w.emitList(indent+1, splitTopLevel(c.body))

	// Set operations sit flush with the statements they join.
	case "UNION", "UNION ALL", "INTERSECT", "EXCEPT":
		w.add(indent, head)
		w.emitList(indent+1, splitTopLevel(c.body))

	default:
		w.add(indent, head)
		w.emitList(indent+1, splitTopLevel(c.body))
	}
}

// inlineWhere reports whether this statement's WHERE clause should stay on one
// line, which pgFormatter does for DELETE.
func inlineWhere(all []clause) bool {
	for _, c := range all {
		if c.keyword == "DELETE FROM" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// entry point
// ---------------------------------------------------------------------------

// formatSQLString pretty-prints a SQL string. It never returns an error: input
// it cannot make sense of is passed through with keywords normalised, matching
// pgFormatter's behaviour on malformed input.
func formatSQLString(sql string) string {
	toks := tokenize(sql)
	if len(toks) == 0 {
		return strings.TrimSpace(sql)
	}

	w := &writer{}
	for _, stmt := range splitStatements(toks) {
		if len(w.lines) > 0 {
			w.lines = append(w.lines, "")
		}
		formatStatement(stmt.toks, 0, w)
		// Re-attach the terminator so that scripts survive a round trip and
		// formatting stays idempotent. Statements the caller did not
		// terminate stay unterminated.
		if stmt.terminated && len(w.lines) > 0 {
			// A ";" appended to a line ending in a "--" comment would be
			// commented out, so give it a line of its own.
			if last := w.lines[len(w.lines)-1]; strings.Contains(last, "--") {
				w.lines = append(w.lines, ";")
			} else {
				w.lines[len(w.lines)-1] = last + ";"
			}
		}
	}
	return strings.TrimSpace(strings.Join(w.lines, "\n"))
}

// statement is one top-level statement plus whether the input terminated it
// with a semicolon.
type statement struct {
	toks       []token
	terminated bool
}

// splitStatements divides tokens on top-level semicolons.
func splitStatements(toks []token) []statement {
	var out []statement
	depth, start := 0, 0
	for i, t := range toks {
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
		case ";":
			if depth == 0 {
				if i > start {
					out = append(out, statement{toks: toks[start:i], terminated: true})
				}
				start = i + 1
			}
		}
	}
	if start < len(toks) {
		out = append(out, statement{toks: toks[start:]})
	}
	return out
}
