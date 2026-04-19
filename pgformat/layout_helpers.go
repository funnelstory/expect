package pgformat

import "strings"

func deleteHasUsingAtDepth0(toks []snapToken, start, whereIdx int) bool {
	d := 0
	for k := start; k < whereIdx; k++ {
		switch toks[k].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
		if d == 0 && snapKw(toks, k, "USING") {
			return true
		}
	}
	return false
}

// Heuristic: docker indents the outer FROM / WHERE / … one extra level when a
// select-list item is a parenthesized subquery that contains `IN (` (see
// docker_pgformatter_hard_test deep_nested_subqueries). Scalar subqueries
// using `= (` do not trigger the bump (nested_select_list_subqueries).
func parenListSubqueryBumpsOuterFromClause(toks []snapToken, lparen int, itemEnd int) bool {
	close := findMatchingRParenBetween(toks, lparen, itemEnd)
	if close < 0 {
		return false
	}
	for j := lparen + 1; j < close; j++ {
		if snapKw(toks, j, "IN") && j+1 < len(toks) && toks[j+1].kind == snapTokLParen {
			return true
		}
	}
	return false
}

func findThenKeywordAtOuterDepth(toks []snapToken, scanFrom, limit, anchorIdx int) int {
	if anchorIdx < 0 || anchorIdx >= len(toks) {
		return -1
	}
	target := snapParenDepthBefore(toks, anchorIdx)
	for j := scanFrom; j < limit; j++ {
		if snapParenDepthBefore(toks, j) != target {
			continue
		}
		if snapKw(toks, j, "THEN") {
			return j
		}
	}
	return -1
}

func findKeywordAtDepth0(toks []snapToken, start, to int, kw string, caseFrom int) int {
	for j := start; j < to; j++ {
		if snapParenDepthBetween(toks, caseFrom, j) != 0 {
			continue
		}
		if snapKw(toks, j, kw) {
			return j
		}
	}
	return -1
}

func findCaseBranchEnd(toks []snapToken, caseFrom, start, to int) int {
	cd := 0
	for j := start; j < to; j++ {
		if snapParenDepthBetween(toks, caseFrom, j) != 0 {
			continue
		}
		if snapKw(toks, j, "CASE") {
			cd++
			continue
		}
		if snapKw(toks, j, "END") {
			if cd > 0 {
				cd--
				continue
			}
			return j
		}
		if cd == 0 && (snapKw(toks, j, "WHEN") || snapKw(toks, j, "ELSE")) {
			return j
		}
	}
	return to
}

func joinRunAppendLateral(toks []snapToken, i, stop, n int) int {
	if i+n < stop && snapKw(toks, i+n, "LATERAL") {
		return n + 1
	}
	return n
}

func snapJoinRunLength(toks []snapToken, i, stop int) (int, bool) {
	if i >= stop {
		return 0, false
	}
	if snapKw(toks, i, "INNER") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "LEFT") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "RIGHT") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "CROSS") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "FULL") && i+1 < stop && snapPeek(toks, i+1, "OUTER") && i+2 < stop && snapPeek(toks, i+2, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 3), true
	}
	if snapKw(toks, i, "FULL") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "NATURAL") && i+1 < stop && snapPeek(toks, i+1, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 2), true
	}
	if snapKw(toks, i, "JOIN") {
		return joinRunAppendLateral(toks, i, stop, 1), true
	}
	return 0, false
}

func fromClauseHasOuterJoin(toks []snapToken, i, stop int) bool {
	cur := i
	for cur < stop {
		if w, ok := snapJoinRunLength(toks, cur, stop); ok {
			if snapKw(toks, cur, "LEFT") || snapKw(toks, cur, "RIGHT") || snapKw(toks, cur, "FULL") {
				return true
			}
			cur += w
			continue
		}
		cur++
	}
	return false
}

func findMatchingRParenBetween(toks []snapToken, lparen, stop int) int {
	if lparen >= len(toks) || toks[lparen].kind != snapTokLParen {
		return -1
	}
	depth := 0
	for i := lparen; i < stop && i < len(toks); i++ {
		switch toks[i].kind {
		case snapTokLParen:
			depth++
		case snapTokRParen:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func snapKw(toks []snapToken, i int, s string) bool {
	return i < len(toks) && toks[i].kind == snapTokWord && strings.EqualFold(toks[i].lit, s)
}

func snapPeek(toks []snapToken, i int, s string) bool {
	return i < len(toks) && toks[i].kind == snapTokWord && strings.EqualFold(toks[i].lit, s)
}

// findSnapAtDepth0 finds the first `want` keyword at the same parenthesis depth
// as `start`. Scanning stops (returns -1) if the depth drops below that starting
// depth—so a subquery cannot pick up FROM / SET / etc. from an outer statement.
func findSnapAtDepth0(toks []snapToken, start int, want string) int {
	d := snapParenDepthBefore(toks, start)
	target := d
	for i := start; i < len(toks); i++ {
		switch toks[i].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
			if d < target {
				return -1
			}
		}
		if d != target {
			continue
		}
		if snapKw(toks, i, want) {
			return i
		}
	}
	return -1
}

func snapParenDepthBetween(toks []snapToken, lineStart, pos int) int {
	d := 0
	for i := lineStart; i < pos && i < len(toks); i++ {
		switch toks[i].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
	}
	return d
}

func snapBracketDepthBetween(toks []snapToken, lineStart, pos int) int {
	b := 0
	for i := lineStart; i < pos && i < len(toks); i++ {
		switch toks[i].kind {
		case snapTokLBracket:
			b++
		case snapTokRBracket:
			b--
		}
	}
	return b
}

func snapParenDepthBefore(toks []snapToken, i int) int {
	d := 0
	for k := 0; k < i && k < len(toks); k++ {
		switch toks[k].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
	}
	return d
}

// nextSnapClauseBoundary returns the index of the first token after the clause
// that starts at i: either a closing paren that pops below the depth at i, or
// a top-level clause keyword (WHERE, GROUP, …) at the same nesting depth.
func nextSnapClauseBoundary(toks []snapToken, i int) int {
	startDepth := snapParenDepthBefore(toks, i)
	d := startDepth
	for j := i; j < len(toks); j++ {
		switch toks[j].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
		if d < startDepth {
			return j
		}
		if d != startDepth {
			continue
		}
		if toks[j].kind != snapTokWord {
			continue
		}
		u := strings.ToUpper(toks[j].lit)
		if u == "ON" && snapPeek(toks, j+1, "CONFLICT") {
			return j
		}
		switch u {
		case "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "INTERSECT", "EXCEPT":
			return j
		}
	}
	return len(toks)
}

func isSnapClauseStart(toks []snapToken, i int) bool {
	if i >= len(toks) || toks[i].kind != snapTokWord {
		return false
	}
	u := strings.ToUpper(toks[i].lit)
	switch u {
	case "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "INTERSECT", "EXCEPT":
		return true
	default:
		return false
	}
}

func isWithBodyStmtStart(toks []snapToken, i int) bool {
	if i >= len(toks) {
		return false
	}
	return snapKw(toks, i, "SELECT") || snapKw(toks, i, "INSERT") ||
		snapKw(toks, i, "UPDATE") || snapKw(toks, i, "DELETE")
}

func selectListEndWithoutFrom(toks []snapToken, i int) int {
	startDepth := snapParenDepthBefore(toks, i)
	d := startDepth
	for j := i; j < len(toks); j++ {
		switch toks[j].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
			if d < startDepth {
				return j
			}
		}
		if d != startDepth {
			continue
		}
		if toks[j].kind != snapTokWord {
			continue
		}
		u := strings.ToUpper(toks[j].lit)
		switch u {
		case "FROM", "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "INTERSECT", "EXCEPT":
			return j
		}
	}
	return len(toks)
}

func caseExpressionEnd(toks []snapToken, caseFrom, limit int) int {
	cd := 0
	for j := caseFrom + 1; j < limit; j++ {
		if snapParenDepthBetween(toks, caseFrom, j) != 0 {
			continue
		}
		if snapKw(toks, j, "CASE") {
			cd++
			continue
		}
		if snapKw(toks, j, "END") {
			if cd > 0 {
				cd--
				continue
			}
			return j + 1
		}
	}
	return limit
}

func andClosesBetween(toks []snapToken, lineStart, andIdx int) bool {
	d := 0
	waitingBetweenAnd := false
	for k := lineStart; k < andIdx; k++ {
		switch toks[k].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
		if d != 0 {
			continue
		}
		if snapKw(toks, k, "BETWEEN") {
			waitingBetweenAnd = true
			continue
		}
		if snapKw(toks, k, "NOT") && k+1 < len(toks) && snapKw(toks, k+1, "BETWEEN") {
			waitingBetweenAnd = true
			k++
			continue
		}
		if snapKw(toks, k, "AND") && waitingBetweenAnd {
			waitingBetweenAnd = false
		}
	}
	return waitingBetweenAnd
}

func snapGroupingSetsHead(toks []snapToken, i, stop int) bool {
	d := 0
	for j := i; j < stop; j++ {
		switch toks[j].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
		if d != 0 {
			continue
		}
		if snapKw(toks, j, "GROUPING") && j+1 < stop && snapPeek(toks, j+1, "SETS") {
			return true
		}
	}
	return false
}
