package pgformat

import (
	"strings"
)

// layoutSQL renders SQL in pgFormatter-style layout.
func layoutSQL(sql string, o Options) string {
	toks := tokenize(sql)
	toks = normalizeTokens(toks, o)
	if len(toks) == 0 {
		return ""
	}
	if !allStatementsLayoutable(toks) {
		return ""
	}
	var b strings.Builder
	p := snapPrinter{toks: toks, b: &b, opts: o}
	p.formatStatements(0)
	return strings.TrimSpace(b.String())
}

func allStatementsLayoutable(toks []snapToken) bool {
	for _, s := range snapStatementStarts(toks) {
		if !stmtStartSupported(toks, s) {
			return false
		}
	}
	return true
}

func snapStatementStarts(toks []snapToken) []int {
	d := 0
	var starts []int
	needStart := true
	for j := 0; j < len(toks); j++ {
		if needStart && d == 0 && toks[j].kind != snapTokSemicolon {
			starts = append(starts, j)
			needStart = false
		}
		switch toks[j].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		case snapTokSemicolon:
			if d == 0 {
				needStart = true
			}
		}
	}
	return starts
}

func stmtStartSupported(toks []snapToken, i int) bool {
	if i >= len(toks) {
		return true
	}
	if snapKw(toks, i, "WITH") || snapKw(toks, i, "INSERT") || snapKw(toks, i, "UPDATE") ||
		snapKw(toks, i, "DELETE") || snapKw(toks, i, "SELECT") || snapKw(toks, i, "MERGE") {
		return true
	}
	if snapKw(toks, i, "CREATE") {
		return isCreateTableStart(toks, i)
	}
	return false
}

func isCreateTableStart(toks []snapToken, i int) bool {
	if !snapKw(toks, i, "CREATE") {
		return false
	}
	j := i + 1
	for j < len(toks) && (snapKw(toks, j, "TEMP") || snapKw(toks, j, "TEMPORARY") ||
		snapKw(toks, j, "UNLOGGED") || snapKw(toks, j, "GLOBAL") || snapKw(toks, j, "LOCAL")) {
		j++
	}
	return j < len(toks) && snapKw(toks, j, "TABLE")
}

func normalizeTokens(toks []snapToken, o Options) []snapToken {
	out := make([]snapToken, len(toks))
	copy(out, toks)
	for i := range out {
		if out[i].kind == snapTokWord {
			out[i].lit = normalizeWord(out[i].lit, o)
		}
	}
	return out
}

type snapPrinter struct {
	toks []snapToken
	b    *strings.Builder
	opts Options

	// compactFromInSelect: pgFormatter keeps "FROM tbl" on one line in scalar
	// subqueries when the statement is DELETE ... USING ... (see docker_pgformatter_test).
	compactFromInSelect bool
	// fromHadOuterJoin: set while formatting FROM if a LEFT/RIGHT/FULL join appears;
	// main-query WHERE is outdented to column 0 (pgFormatter behavior).
	fromHadOuterJoin bool
	// cteSubqueryHadNestedWith: emitCTEClause sets true when AS (…) begins with WITH;
	// formatWith uses it to indent the trailing SELECT after a nested-CTE subquery.
	cteSubqueryHadNestedWith bool
}

func (p *snapPrinter) formatStatements(i int) int {
	for i < len(p.toks) {
		i = p.skipSemicolons(i)
		if i >= len(p.toks) {
			break
		}
		if snapKw(p.toks, i, "WITH") {
			i = p.formatWith(i, 0)
			continue
		}
		if snapKw(p.toks, i, "INSERT") {
			i = p.formatInsert(i, 0)
			continue
		}
		if snapKw(p.toks, i, "UPDATE") {
			i = p.formatUpdate(i, 0)
			continue
		}
		if snapKw(p.toks, i, "DELETE") {
			i = p.formatDelete(i, 0)
			continue
		}
		if snapKw(p.toks, i, "MERGE") {
			i = p.formatMerge(i, 0)
			continue
		}
		if snapKw(p.toks, i, "CREATE") && isCreateTableStart(p.toks, i) {
			i = p.formatCreateTable(i, 0)
			continue
		}
		if snapKw(p.toks, i, "SELECT") {
			i = p.formatSelect(i, 0)
			continue
		}
		i = p.emitRawUntil(i, len(p.toks))
	}
	return i
}

func (p *snapPrinter) skipSemicolons(i int) int {
	for i < len(p.toks) && p.toks[i].kind == snapTokSemicolon {
		p.b.WriteString(";\n")
		i++
	}
	return i
}

func (p *snapPrinter) formatWith(i int, base int) int {
	p.emitIndent(base)
	p.writeTok(i)
	i++
	if i < len(p.toks) && snapKw(p.toks, i, "RECURSIVE") {
		p.b.WriteByte(' ')
		p.writeTok(i)
		i++
	}
	firstCTE := true
	bumpTrailingSelect := false
	for i < len(p.toks) && !isWithBodyStmtStart(p.toks, i) {
		p.cteSubqueryHadNestedWith = false
		i = p.emitCTEClause(i, base, !firstCTE)
		if p.cteSubqueryHadNestedWith {
			bumpTrailingSelect = true
		}
		firstCTE = false
	}
	if i >= len(p.toks) || !isWithBodyStmtStart(p.toks, i) {
		return i
	}
	p.b.WriteByte('\n')
	selectBase := base
	if base > 0 {
		selectBase = base + 1
	}
	if base == 0 && bumpTrailingSelect {
		selectBase = 1
	}
	switch {
	case snapKw(p.toks, i, "SELECT"):
		return p.formatSelect(i, selectBase)
	case snapKw(p.toks, i, "INSERT"):
		return p.formatInsert(i, selectBase)
	case snapKw(p.toks, i, "UPDATE"):
		return p.formatUpdate(i, selectBase)
	case snapKw(p.toks, i, "DELETE"):
		return p.formatDelete(i, selectBase)
	default:
		return i
	}
}

func (p *snapPrinter) emitCTEClause(i int, base int, afterComma bool) int {
	first := true
	for i < len(p.toks) && !snapKw(p.toks, i, "AS") {
		if p.toks[i].kind == snapTokLParen {
			j := i + 1
			subqHead := j < len(p.toks) &&
				(snapKw(p.toks, j, "SELECT") || snapKw(p.toks, j, "WITH") || snapKw(p.toks, j, "VALUES"))
			if !subqHead {
				if !(afterComma && first) {
					p.b.WriteByte(' ')
				}
				first = false
				lp := i
				p.writeTok(lp)
				i++
				close := findMatchingRParenBetween(p.toks, lp, len(p.toks))
				if close < 0 {
					i = p.emitBalanced(lp)
					continue
				}
				if i >= close {
					p.writeTok(close)
					i = close + 1
					continue
				}
				p.b.WriteByte('\n')
				p.emitIndent(base + 1)
				i = p.emitCommaSeparated(i, close, base+1)
				p.b.WriteByte('\n')
				p.emitIndent(base)
				p.writeTok(close)
				i = close + 1
				continue
			}
		}
		if !(afterComma && first) {
			p.b.WriteByte(' ')
		}
		first = false
		p.writeTok(i)
		i++
	}
	if i < len(p.toks) && snapKw(p.toks, i, "AS") {
		p.b.WriteByte(' ')
		p.writeTok(i)
		i++
	}
	if i < len(p.toks) && p.toks[i].kind == snapTokLParen {
		p.b.WriteByte(' ')
		p.writeTok(i)
		i++
		subqHadWith := i < len(p.toks) && snapKw(p.toks, i, "WITH")
		i = p.formatSubqueryInParen(i, base+1)
		if subqHadWith {
			p.cteSubqueryHadNestedWith = true
		}
		if i < len(p.toks) && p.toks[i].kind == snapTokRParen {
			// Heuristic: merge `)` onto the last WHERE line when the CTE body ends
			// with WHERE + one simple operand (docker insert_select_cte shape).
			if cteClosingParenMergesWithWhereLine(p.toks, i) {
				body := p.b.String()
				if strings.HasSuffix(body, "\n") {
					body = body[:len(body)-1]
					p.b.Reset()
					p.b.WriteString(body)
					if len(body) > 0 && body[len(body)-1] != ' ' {
						p.b.WriteByte(' ')
					}
				}
			} else {
				p.b.WriteByte('\n')
			}
			p.writeTok(i)
			i++
		}
	}
	if i < len(p.toks) && p.toks[i].kind == snapTokComma {
		p.b.WriteString(",\n")
		i++
	}
	return i
}

func cteClosingParenMergesWithWhereLine(toks []snapToken, rparenIdx int) bool {
	if rparenIdx < 2 || !snapKw(toks, rparenIdx-2, "WHERE") {
		return false
	}
	return isSimpleWhereOperandToken(toks[rparenIdx-1])
}

func isSimpleWhereOperandToken(t snapToken) bool {
	switch t.kind {
	case snapTokWord, snapTokIdent, snapTokNumber, snapTokString:
		return true
	default:
		return false
	}
}

func (p *snapPrinter) formatSubqueryInParen(i int, innerBase int) int {
	if i < len(p.toks) && snapKw(p.toks, i, "WITH") {
		p.b.WriteByte('\n')
		return p.formatWith(i, innerBase)
	}
	if i < len(p.toks) && snapKw(p.toks, i, "VALUES") {
		p.b.WriteByte('\n')
		return p.formatValuesSubquery(i, innerBase)
	}
	if i < len(p.toks) && snapKw(p.toks, i, "SELECT") {
		p.b.WriteByte('\n')
		return p.formatSelect(i, innerBase)
	}
	return p.emitBalanced(i)
}

func (p *snapPrinter) formatValuesSubquery(i int, base int) int {
	p.emitIndent(base)
	p.writeTok(i)
	i++
	if i < len(p.toks) && p.toks[i].kind == snapTokLParen {
		p.b.WriteByte(' ')
		i = p.emitBalanced(i)
	}
	for i < len(p.toks) && snapKw(p.toks, i, "UNION") {
		p.b.WriteByte('\n')
		p.emitIndent(base)
		p.writeTok(i)
		i++
		if i < len(p.toks) && (snapKw(p.toks, i, "ALL") || snapKw(p.toks, i, "DISTINCT")) {
			p.b.WriteByte(' ')
			p.writeTok(i)
			i++
		}
		if i < len(p.toks) && snapKw(p.toks, i, "SELECT") {
			p.b.WriteByte('\n')
			i = p.formatSelect(i, base)
		} else {
			break
		}
	}
	return i
}

func (p *snapPrinter) formatSelect(i int, base int) int {
	p.fromHadOuterJoin = false
	p.emitIndent(base)
	p.writeTok(i)
	i++
	endFrom := findSnapAtDepth0(p.toks, i, "FROM")
	listEnd := endFrom
	if listEnd < 0 {
		listEnd = selectListEndWithoutFrom(p.toks, i)
	}
	listBumpOuterClauses := false
	if i < listEnd {
		p.b.WriteByte('\n')
		p.emitIndent(base + 1)
		var bump bool
		i, bump = p.emitSelectList(i, listEnd, base+1)
		listBumpOuterClauses = bump
	}
	clauseBase := base
	if listBumpOuterClauses {
		clauseBase = base + 1
	}
	i = listEnd
	if endFrom >= 0 {
		i = endFrom
		p.b.WriteByte('\n')
		p.emitIndent(clauseBase)
		p.writeTok(i)
		i++
		stop := nextSnapClauseBoundary(p.toks, i)
		if i < stop {
			fromIndent := clauseBase
			tableIndent := clauseBase + 1
			joinIndent := tableIndent
			if clauseBase > 0 && fromClauseHasOuterJoin(p.toks, i, stop) {
				joinIndent = fromIndent
			}
			if p.compactFromInSelect {
				p.b.WriteByte(' ')
				i = p.emitFromJoins(i, stop, fromIndent, joinIndent)
			} else {
				p.b.WriteByte('\n')
				p.emitIndent(tableIndent)
				i = p.emitFromJoins(i, stop, fromIndent, joinIndent)
			}
		}
	}
	whereIndent := clauseBase
	if p.fromHadOuterJoin {
		whereIndent = 0
	}
	for i < len(p.toks) {
		if snapKw(p.toks, i, "WHERE") {
			p.b.WriteByte('\n')
			p.emitIndent(whereIndent)
			p.writeTok(i)
			i++
			nx := nextSnapClauseBoundary(p.toks, i)
			p.b.WriteByte('\n')
			p.emitIndent(whereIndent + 1)
			i = p.emitExprLines(i, nx, whereIndent+1)
			continue
		}
		if snapKw(p.toks, i, "ORDER") && snapPeek(p.toks, i+1, "BY") {
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase)
			p.writeTok(i)
			p.b.WriteByte(' ')
			p.writeTok(i + 1)
			i += 2
			nx := p.orderByClauseEnd(i)
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase + 1)
			i, _ = p.emitSelectList(i, nx, clauseBase+1)
			continue
		}
		if snapKw(p.toks, i, "GROUP") && snapPeek(p.toks, i+1, "BY") {
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase)
			p.writeTok(i)
			p.b.WriteByte(' ')
			p.writeTok(i + 1)
			i += 2
			nx := nextSnapClauseBoundary(p.toks, i)
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase + 1)
			if snapGroupingSetsHead(p.toks, i, nx) {
				i = p.emitGroupingSetsExpr(i, nx, clauseBase+1)
			} else {
				i, _ = p.emitSelectList(i, nx, clauseBase+1)
			}
			continue
		}
		if snapKw(p.toks, i, "HAVING") {
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase)
			p.writeTok(i)
			i++
			nx := nextSnapClauseBoundary(p.toks, i)
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase + 1)
			i = p.emitExprLines(i, nx, clauseBase+1)
			continue
		}
		if snapKw(p.toks, i, "LIMIT") {
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase)
			i = p.emitLimitOffset(i, clauseBase)
			break
		}
		if snapKw(p.toks, i, "OFFSET") {
			p.b.WriteByte('\n')
			p.emitIndent(clauseBase)
			i = p.emitLimitOffset(i, clauseBase)
			break
		}
		break
	}
	for i < len(p.toks) && snapKw(p.toks, i, "UNION") {
		p.b.WriteByte('\n')
		p.emitIndent(base)
		p.writeTok(i)
		i++
		if i < len(p.toks) && (snapKw(p.toks, i, "ALL") || snapKw(p.toks, i, "DISTINCT")) {
			p.b.WriteByte(' ')
			p.writeTok(i)
			i++
		}
		p.b.WriteByte('\n')
		if i < len(p.toks) && snapKw(p.toks, i, "SELECT") {
			i = p.formatSelect(i, base)
		}
	}
	return i
}

// orderByClauseEnd returns the end index (exclusive) of ORDER BY sort keys and trailing OFFSET/FETCH.
func (p *snapPrinter) orderByClauseEnd(start int) int {
	startDepth := snapParenDepthBefore(p.toks, start)
	d := startDepth
	for j := start; j < len(p.toks); j++ {
		switch p.toks[j].kind {
		case snapTokLParen:
			d++
		case snapTokRParen:
			d--
		}
		if d != startDepth {
			continue
		}
		if p.toks[j].kind != snapTokWord {
			continue
		}
		u := strings.ToUpper(p.toks[j].lit)
		switch u {
		case "UNION", "INTERSECT", "EXCEPT", "LIMIT":
			return j
		}
	}
	return len(p.toks)
}

func (p *snapPrinter) emitLimitOffset(i int, base int) int {
	kw := i
	i++
	if i >= len(p.toks) {
		p.writeTok(kw)
		return i
	}
	valStart := i
	parDepth := 0
	for i < len(p.toks) {
		switch p.toks[i].kind {
		case snapTokLParen:
			parDepth++
		case snapTokRParen:
			if parDepth == 0 {
				goto limitDone
			}
			parDepth--
		}
		if parDepth == 0 && isSnapClauseStart(p.toks, i) {
			goto limitDone
		}
		if parDepth == 0 && p.toks[i].kind == snapTokSemicolon {
			goto limitDone
		}
		i++
	}
limitDone:
	p.writeTok(kw)
	p.b.WriteByte(' ')
	p.emitRange(valStart, i)
	return i
}

func (p *snapPrinter) formatInsert(i int, base int) int {
	p.emitIndent(base)
	sel := findSnapAtDepth0(p.toks, i, "SELECT")
	vals := findSnapAtDepth0(p.toks, i, "VALUES")
	if sel >= 0 && (vals < 0 || sel < vals) {
		p.emitRange(i, sel)
		p.b.WriteByte('\n')
		i = p.formatSelect(sel, base)
		return p.formatInsertOnConflict(i, base)
	}
	if vals < 0 {
		return p.emitRawUntil(i, len(p.toks))
	}
	endInto := vals
	p.emitRange(i, endInto)
	i = endInto
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	for i < len(p.toks) && snapKw(p.toks, i, "VALUES") {
		p.writeTok(i)
		i++
		break
	}
	if i < len(p.toks) && p.toks[i].kind == snapTokLParen {
		p.b.WriteByte(' ')
		i = p.emitBalanced(i)
	}
	return p.formatInsertOnConflict(i, base)
}

func (p *snapPrinter) formatInsertOnConflict(i int, base int) int {
	if i >= len(p.toks) || !snapKw(p.toks, i, "ON") || !snapPeek(p.toks, i+1, "CONFLICT") {
		return i
	}
	p.b.WriteByte('\n')
	p.emitIndent(base)
	p.writeTok(i)
	p.b.WriteByte(' ')
	p.writeTok(i + 1)
	i += 2
	if i < len(p.toks) && p.toks[i].kind == snapTokLParen {
		p.b.WriteByte(' ')
		i = p.emitBalanced(i)
	}
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	if i >= len(p.toks) || !snapKw(p.toks, i, "DO") {
		return i
	}
	p.writeTok(i)
	i++
	if i < len(p.toks) && snapKw(p.toks, i, "NOTHING") {
		p.b.WriteByte(' ')
		p.writeTok(i)
		return i + 1
	}
	if i < len(p.toks) && snapKw(p.toks, i, "UPDATE") {
		p.b.WriteByte(' ')
		p.writeTok(i)
		i++
		p.b.WriteByte(' ')
	}
	if i >= len(p.toks) || !snapKw(p.toks, i, "SET") {
		return i
	}
	p.writeTok(i)
	i++
	p.b.WriteByte('\n')
	p.emitIndent(base + 2)
	wend := findSnapAtDepth0(p.toks, i, "WHERE")
	if wend < 0 {
		return p.emitCommaSeparated(i, len(p.toks), base+2)
	}
	i = p.emitCommaSeparated(i, wend, base+2)
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	p.writeTok(wend)
	i = wend + 1
	p.b.WriteByte('\n')
	p.emitIndent(base + 2)
	return p.emitExprLines(i, len(p.toks), base+2)
}

func (p *snapPrinter) formatUpdate(i int, base int) int {
	p.emitIndent(base)
	p.writeTok(i)
	i++
	setPos := findSnapAtDepth0(p.toks, i, "SET")
	if setPos < 0 {
		return p.emitRawUntil(i, len(p.toks))
	}
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	p.emitRange(i, setPos)
	i = setPos
	p.b.WriteByte('\n')
	p.emitIndent(base)
	p.writeTok(i)
	i++
	fromPos := findSnapAtDepth0(p.toks, i, "FROM")
	wherePos := findSnapAtDepth0(p.toks, i, "WHERE")
	retPos := findSnapAtDepth0(p.toks, i, "RETURNING")
	endSet := len(p.toks)
	if fromPos >= 0 {
		endSet = fromPos
	} else if wherePos >= 0 {
		endSet = wherePos
	} else if retPos >= 0 {
		endSet = retPos
	}
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	i = p.emitCommaSeparated(i, endSet, base+1)
	i = endSet
	if fromPos >= 0 && i == fromPos {
		p.b.WriteByte('\n')
		p.emitIndent(base)
		p.writeTok(i)
		i++
		nextBound := len(p.toks)
		if wherePos >= 0 {
			nextBound = wherePos
		} else if retPos >= 0 {
			nextBound = retPos
		}
		fromIndent := base
		tableIndent := base + 1
		joinIndent := tableIndent
		if base > 0 && fromClauseHasOuterJoin(p.toks, i, nextBound) {
			joinIndent = fromIndent
		}
		p.b.WriteByte('\n')
		p.emitIndent(tableIndent)
		i = p.emitFromJoins(i, nextBound, fromIndent, joinIndent)
		i = nextBound
	}
	if wherePos >= 0 {
		p.b.WriteByte('\n')
		p.emitIndent(base)
		p.writeTok(wherePos)
		i = wherePos + 1
		whereStop := len(p.toks)
		if retPos >= 0 {
			whereStop = retPos
		}
		p.b.WriteByte('\n')
		p.emitIndent(base + 1)
		i = p.emitExprLines(i, whereStop, base+1)
		i = whereStop
	}
	if retPos >= 0 {
		p.b.WriteByte('\n')
		p.emitIndent(base)
		p.writeTok(retPos)
		i = retPos + 1
		p.b.WriteByte('\n')
		p.emitIndent(base + 1)
		i = p.emitCommaSeparated(i, len(p.toks), base+1)
	}
	return i
}

func (p *snapPrinter) formatCreateTable(i int, base int) int {
	lparen := -1
	for j := i; j < len(p.toks); j++ {
		if p.toks[j].kind == snapTokLParen && snapParenDepthBefore(p.toks, j) == 0 {
			lparen = j
			break
		}
	}
	if lparen < 0 {
		return p.emitRawUntil(i, len(p.toks))
	}
	p.emitIndent(base)
	p.emitRange(i, lparen)
	if p.needSpace(lparen-1, lparen) {
		p.b.WriteByte(' ')
	}
	p.writeTok(lparen)
	close := findMatchingRParenBetween(p.toks, lparen, len(p.toks))
	if close < 0 {
		return p.emitBalanced(lparen)
	}
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	p.emitCommaSeparated(lparen+1, close, base+1)
	p.writeTok(close)
	return close + 1
}

func (p *snapPrinter) formatMerge(i int, base int) int {
	if i >= len(p.toks) || !snapKw(p.toks, i, "MERGE") {
		return p.emitRawUntil(i, len(p.toks))
	}
	us := findSnapAtDepth0(p.toks, i, "USING")
	if us < 0 {
		return p.emitRawUntil(i, len(p.toks))
	}
	p.emitIndent(base)
	p.emitRange(i, us)
	i = us
	p.b.WriteByte('\n')
	p.emitIndent(base + 1)
	firstWhen := findSnapAtDepth0(p.toks, i, "WHEN")
	if firstWhen < 0 {
		return p.emitRestOfStatement(i)
	}
	p.emitRange(i, firstWhen)
	i = firstWhen
	for i < len(p.toks) && snapKw(p.toks, i, "WHEN") {
		p.b.WriteByte('\n')
		p.emitIndent(base + 1)
		whenStart := i
		nextWhen := findSnapAtDepth0(p.toks, i+1, "WHEN")
		stop := len(p.toks)
		if nextWhen >= 0 {
			stop = nextWhen
		}
		thenPos := findThenKeywordAtOuterDepth(p.toks, i+1, stop, whenStart)
		if thenPos < 0 {
			p.emitRange(i, stop)
			i = stop
			break
		}
		p.emitRange(i, thenPos+1)
		i = thenPos + 1
		p.b.WriteByte('\n')
		p.emitIndent(base + 2)
		if snapKw(p.toks, i, "DELETE") {
			p.writeTok(i)
			i++
			continue
		}
		p.emitRange(i, stop)
		i = stop
	}
	return i
}

func (p *snapPrinter) formatDelete(i int, base int) int {
	p.emitIndent(base)
	end := findSnapAtDepth0(p.toks, i, "WHERE")
	if end < 0 {
		return p.emitRawUntil(i, len(p.toks))
	}
	hadUsing := deleteHasUsingAtDepth0(p.toks, i, end)
	prevCompact := p.compactFromInSelect
	if hadUsing {
		p.compactFromInSelect = true
	}
	j := i
	for j < end {
		p.writeTok(j)
		if j+1 < end {
			p.b.WriteByte(' ')
		}
		j++
	}
	i = end
	p.b.WriteByte('\n')
	p.emitIndent(base)
	p.writeTok(i)
	i++
	var n int
	if i < len(p.toks) {
		n = p.emitWhereClausePredicates(i, len(p.toks), base)
	} else {
		n = i
	}
	p.compactFromInSelect = prevCompact
	return n
}

// emitWhereClausePredicates formats DELETE (and similar) WHERE: first predicate on
// the same line as WHERE; further AND/OR predicates on following lines.
func (p *snapPrinter) emitWhereClausePredicates(i, stop, base int) int {
	p.b.WriteByte(' ')
	lineStart := i
	for j := i; j < stop; j++ {
		if (snapKw(p.toks, j, "AND") || snapKw(p.toks, j, "OR")) && snapParenDepthBetween(p.toks, lineStart, j) == 0 {
			if snapKw(p.toks, j, "AND") && andClosesBetween(p.toks, lineStart, j) {
				continue
			}
			p.emitExprSegment(lineStart, j, base+1)
			p.b.WriteByte('\n')
			p.emitIndent(base + 1)
			lineStart = j
		}
	}
	if lineStart < stop {
		p.emitExprSegment(lineStart, stop, base+1)
	}
	return stop
}

func (p *snapPrinter) emitRestOfStatement(i int) int {
	end := len(p.toks)
	for k := i; k < len(p.toks); k++ {
		if p.toks[k].kind == snapTokSemicolon {
			end = k
			break
		}
	}
	p.emitRange(i, end)
	if end < len(p.toks) && p.toks[end].kind == snapTokSemicolon {
		p.writeTok(end)
		return end + 1
	}
	return end
}

func (p *snapPrinter) emitSelectList(from, end, ind int) (int, bool) {
	if !p.opts.CommaBreak {
		_, bump := p.emitSelectItem(from, end, ind)
		return end, bump
	}
	lineStart := from
	bumpOuter := false
	for j := from; j < end; j++ {
		if p.toks[j].kind == snapTokComma && snapParenDepthBetween(p.toks, lineStart, j) == 0 &&
			snapBracketDepthBetween(p.toks, lineStart, j) == 0 {
			_, b := p.emitSelectItem(lineStart, j, ind)
			bumpOuter = bumpOuter || b
			p.b.WriteString(",\n")
			p.emitIndent(ind)
			lineStart = j + 1
		}
	}
	if lineStart < end {
		_, b := p.emitSelectItem(lineStart, end, ind)
		bumpOuter = bumpOuter || b
	}
	return end, bumpOuter
}

func (p *snapPrinter) emitSelectItem(from, to, ind int) (hadParen bool, bumpOuterFrom bool) {
	if from >= to {
		return false, false
	}
	if snapKw(p.toks, from, "CASE") {
		p.emitCaseExpression(from, to, ind)
		return false, false
	}
	if p.toks[from].kind == snapTokLParen && from+1 < to {
		j := from + 1
		if snapKw(p.toks, j, "SELECT") || snapKw(p.toks, j, "WITH") {
			p.writeTok(from)
			p.b.WriteByte('\n')
			var k int
			if snapKw(p.toks, j, "WITH") {
				k = p.formatWith(j, ind+1)
			} else {
				k = p.formatSelect(j, ind+1)
			}
			for k < to && p.toks[k].kind == snapTokRParen {
				if p.needSpace(k-1, k) {
					p.b.WriteByte(' ')
				}
				p.writeTok(k)
				k++
			}
			if k < to {
				if p.needSpace(k-1, k) {
					p.b.WriteByte(' ')
				}
				p.emitRange(k, to)
			}
			bump := false
			if snapKw(p.toks, j, "SELECT") {
				bump = parenListSubqueryBumpsOuterFromClause(p.toks, from, to)
			}
			return true, bump
		}
	}
	p.emitRange(from, to)
	return false, false
}

// emitCaseExpression formats CASE … END like pgFormatter (THEN/ELSE branches on new lines).
func (p *snapPrinter) emitCaseExpression(from, to, ind int) {
	if from >= to || !snapKw(p.toks, from, "CASE") {
		p.emitRange(from, to)
		return
	}
	i := from
	p.writeTok(i)
	i++
	firstWhen := true
	for i < to {
		if snapKw(p.toks, i, "WHEN") && snapParenDepthBetween(p.toks, from, i) == 0 {
			if firstWhen {
				p.b.WriteByte(' ')
				firstWhen = false
			} else {
				p.b.WriteByte('\n')
				p.emitIndent(ind)
			}
			thenIdx := findKeywordAtDepth0(p.toks, i+1, to, "THEN", from)
			if thenIdx < 0 {
				p.emitRange(i, to)
				return
			}
			p.emitRange(i, thenIdx+1)
			i = thenIdx + 1
			p.b.WriteByte('\n')
			p.emitIndent(ind + 1)
			nextCtl := findCaseBranchEnd(p.toks, from, i, to)
			p.emitRange(i, nextCtl)
			i = nextCtl
			continue
		}
		if snapKw(p.toks, i, "ELSE") && snapParenDepthBetween(p.toks, from, i) == 0 {
			p.b.WriteByte('\n')
			p.emitIndent(ind)
			p.writeTok(i)
			i++
			p.b.WriteByte('\n')
			p.emitIndent(ind + 1)
			nextCtl := findCaseBranchEnd(p.toks, from, i, to)
			p.emitRange(i, nextCtl)
			i = nextCtl
			continue
		}
		if snapKw(p.toks, i, "END") && snapParenDepthBetween(p.toks, from, i) == 0 {
			p.b.WriteByte('\n')
			p.emitIndent(ind)
			p.emitRange(i, to)
			return
		}
		p.emitRange(i, to)
		return
	}
}

// emitFromJoins prints the FROM clause. joinLineIndent is the indent used before
// each JOIN run; it matches the first table line for inner joins, and the FROM
// keyword line when the clause uses LEFT/RIGHT/FULL (pgFormatter / docker).
func (p *snapPrinter) emitFromJoins(i, stop, fromIndent, joinLineIndent int) int {
	_ = fromIndent
	lineStart := i
	cur := i
	for cur < stop {
		if w, ok := snapJoinRunLength(p.toks, cur, stop); ok {
			if snapKw(p.toks, cur, "LEFT") || snapKw(p.toks, cur, "RIGHT") || snapKw(p.toks, cur, "FULL") {
				p.fromHadOuterJoin = true
			}
			if cur > lineStart {
				p.emitRange(lineStart, cur)
				p.b.WriteByte('\n')
				p.emitIndent(joinLineIndent)
			}
			lineStart = cur
			cur += w
			if cur < stop && p.toks[cur].kind == snapTokLParen {
				j := cur + 1
				if j < stop && (snapKw(p.toks, j, "SELECT") || snapKw(p.toks, j, "WITH")) {
					p.emitRange(lineStart, cur)
					p.b.WriteByte(' ')
					p.writeTok(cur)
					p.b.WriteByte('\n')
					inner := joinLineIndent + 1
					var k int
					if snapKw(p.toks, j, "WITH") {
						k = p.formatWith(j, inner)
					} else {
						k = p.formatSelect(j, inner)
					}
					if k < stop && p.toks[k].kind == snapTokRParen {
						p.writeTok(k)
						k++
						if k < stop && p.needSpace(k-1, k) {
							p.b.WriteByte(' ')
						}
					}
					lineStart = k
					cur = k
					continue
				}
			}
			continue
		}
		cur++
	}
	if lineStart < stop {
		p.emitRange(lineStart, stop)
	}
	return stop
}

func (p *snapPrinter) emitExprLines(i, stop, ind int) int {
	lineStart := i
	for j := i; j < stop; j++ {
		if (snapKw(p.toks, j, "AND") || snapKw(p.toks, j, "OR")) && snapParenDepthBetween(p.toks, lineStart, j) == 0 {
			if snapKw(p.toks, j, "AND") && andClosesBetween(p.toks, lineStart, j) {
				continue
			}
			p.emitExprSegment(lineStart, j, ind)
			p.b.WriteByte('\n')
			p.emitIndent(ind)
			lineStart = j
		}
	}
	if lineStart < stop {
		p.emitExprSegment(lineStart, stop, ind)
	}
	return stop
}

// emitExprSegment formats a single predicate fragment (e.g. one side of AND).
func (p *snapPrinter) emitExprSegment(from, to, ind int) {
	if from >= to {
		return
	}
	if snapKw(p.toks, from, "AND") && from+1 < to && p.toks[from+1].kind == snapTokLParen {
		p.writeTok(from)
		p.b.WriteByte(' ')
		paren := from + 1
		endp := findMatchingRParenBetween(p.toks, paren, to)
		if endp > paren {
			p.writeTok(paren)
			p.emitExprLinesWithOR(paren+1, endp, ind+1)
			p.writeTok(endp)
			if endp+1 < to {
				if p.needSpace(endp, endp+1) {
					p.b.WriteByte(' ')
				}
				p.emitRange(endp+1, to)
			}
			return
		}
	}
	depth := 0
	for j := from; j < to; j++ {
		switch p.toks[j].kind {
		case snapTokLParen:
			if depth == 0 && j+1 < to && (snapKw(p.toks, j+1, "SELECT") || snapKw(p.toks, j+1, "WITH")) {
				p.emitRange(from, j)
				if j > from && p.needSpace(j-1, j) {
					p.b.WriteByte(' ')
				}
				p.writeTok(j)
				p.b.WriteByte('\n')
				var k int
				if snapKw(p.toks, j+1, "WITH") {
					k = p.formatWith(j+1, ind+1)
				} else {
					k = p.formatSelect(j+1, ind+1)
				}
				if k < to && p.toks[k].kind == snapTokRParen {
					p.writeTok(k)
				}
				if k+1 < to {
					if p.needSpace(k, k+1) {
						p.b.WriteByte(' ')
					}
					p.emitRange(k+1, to)
				}
				return
			}
			depth++
		case snapTokRParen:
			depth--
		}
	}
	if p.toks[from].kind == snapTokLParen {
		endp := findMatchingRParenBetween(p.toks, from, to)
		if endp > from {
			p.writeTok(from)
			p.b.WriteByte('\n')
			p.emitIndent(ind + 1)
			p.emitExprLinesWithOR(from+1, endp, ind+1)
			p.b.WriteByte('\n')
			p.emitIndent(ind)
			p.writeTok(endp)
			if endp+1 < to {
				if p.needSpace(endp, endp+1) {
					p.b.WriteByte(' ')
				}
				p.emitRange(endp+1, to)
			}
			return
		}
	}
	fromWalk := from
	for fromWalk < to {
		caseAt := -1
		for j := fromWalk; j < to; j++ {
			if snapKw(p.toks, j, "CASE") && snapParenDepthBetween(p.toks, fromWalk, j) == 0 {
				caseAt = j
				break
			}
		}
		if caseAt < 0 {
			p.emitRange(fromWalk, to)
			return
		}
		if caseAt > fromWalk {
			p.emitRange(fromWalk, caseAt)
			if p.needSpace(caseAt-1, caseAt) {
				p.b.WriteByte(' ')
			}
		}
		ce := caseExpressionEnd(p.toks, caseAt, to)
		p.emitCaseExpression(caseAt, ce, ind)
		fromWalk = ce
		if fromWalk < to && p.needSpace(fromWalk-1, fromWalk) {
			p.b.WriteByte(' ')
		}
	}
}

func (p *snapPrinter) emitExprLinesWithOR(from, to, ind int) {
	lineStart := from
	for j := from; j < to; j++ {
		if snapKw(p.toks, j, "OR") && snapParenDepthBetween(p.toks, lineStart, j) == 0 {
			p.emitRange(lineStart, j)
			p.b.WriteByte('\n')
			p.emitIndent(ind)
			lineStart = j
		}
	}
	if lineStart < to {
		p.emitRange(lineStart, to)
	}
}

func (p *snapPrinter) emitCommaSeparated(i, stop, ind int) int {
	if !p.opts.CommaBreak {
		p.emitRange(i, stop)
		return stop
	}
	lineStart := i
	for j := i; j < stop; j++ {
		if p.toks[j].kind == snapTokComma && snapParenDepthBetween(p.toks, lineStart, j) == 0 {
			p.emitRange(lineStart, j)
			p.b.WriteString(",\n")
			p.emitIndent(ind)
			lineStart = j + 1
		}
	}
	if lineStart < stop {
		p.emitRange(lineStart, stop)
	}
	return stop
}

func (p *snapPrinter) emitBalanced(i int) int {
	pdepth, bdepth := 0, 0
	start := i
	for i < len(p.toks) {
		switch p.toks[i].kind {
		case snapTokLParen:
			pdepth++
		case snapTokRParen:
			pdepth--
			if pdepth == 0 && bdepth == 0 {
				p.emitRange(start, i+1)
				return i + 1
			}
		case snapTokLBracket:
			bdepth++
		case snapTokRBracket:
			bdepth--
		}
		i++
	}
	p.emitRange(start, len(p.toks))
	return len(p.toks)
}

func (p *snapPrinter) emitRawUntil(i, end int) int {
	for i < end {
		p.writeTok(i)
		i++
	}
	return i
}

func (p *snapPrinter) emitRange(a, b int) {
	for j := a; j < b; j++ {
		p.writeTok(j)
		if j+1 < b && p.needSpace(j, j+1) {
			p.b.WriteByte(' ')
		}
	}
}

func (p *snapPrinter) needSpace(a, b int) bool {
	ta, tb := p.toks[a], p.toks[b]
	if ta.kind == snapTokComma {
		return true
	}
	if tb.kind == snapTokComma {
		return false
	}
	if tb.kind == snapTokOp && tb.lit == "::" {
		return false
	}
	if tb.kind == snapTokLBracket {
		return false
	}
	if ta.kind == snapTokLBracket {
		return false
	}
	if tb.kind == snapTokRBracket {
		return false
	}
	if ta.kind == snapTokRBracket {
		if tb.kind == snapTokWord {
			return true
		}
		return false
	}
	if ta.kind == snapTokLParen {
		return false
	}
	if ta.kind == snapTokRParen {
		if tb.kind == snapTokWord || tb.kind == snapTokIdent || tb.kind == snapTokOp {
			return true
		}
		return false
	}
	if tb.kind == snapTokRParen {
		return false
	}
	if ta.kind == snapTokWord && ta.lit == "." || tb.kind == snapTokWord && tb.lit == "." {
		return false
	}
	if tb.kind == snapTokLParen {
		if ta.kind == snapTokWord {
			u := strings.ToUpper(ta.lit)
			switch u {
			case "WHERE", "AND", "OR", "IN", "NOT", "HAVING", "WHEN", "ON", "SELECT", "OVER", "VALUES", "BY", "OFFSET", "LIMIT":
				return true
			}
			if isBuiltinFunc(u) {
				return false
			}
			return true
		}
		return true
	}
	if ta.kind == snapTokOp && tb.kind == snapTokOp {
		return false
	}
	if ta.kind == snapTokOp && ta.lit == "::" {
		return false
	}
	if ta.kind == snapTokOp {
		return true
	}
	if tb.kind == snapTokOp {
		return true
	}
	return true
}

func (p *snapPrinter) writeTok(i int) {
	if i >= len(p.toks) {
		return
	}
	t := p.toks[i]
	switch t.kind {
	case snapTokWord, snapTokString, snapTokIdent, snapTokNumber, snapTokOp, snapTokLineComment, snapTokBlockComment, snapTokDollar:
		p.b.WriteString(t.lit)
	case snapTokLParen:
		p.b.WriteByte('(')
	case snapTokRParen:
		p.b.WriteByte(')')
	case snapTokLBracket:
		p.b.WriteByte('[')
	case snapTokRBracket:
		p.b.WriteByte(']')
	case snapTokComma:
		p.b.WriteByte(',')
	case snapTokSemicolon:
		p.b.WriteByte(';')
	}
}

func (p *snapPrinter) emitIndent(level int) {
	u := p.opts.indentUnit()
	for k := 0; k < level; k++ {
		p.b.WriteString(u)
	}
}

func (p *snapPrinter) newlineIndent(level int) {
	p.b.WriteByte('\n')
	p.emitIndent(level)
}

// Heuristic: comma breaks inside GROUPING SETS ((…), …) tuned to docker output.
const (
	groupingSetsCommaBreakDepth     = 1
	groupingSetsCommaBreakDeepDepth = 2
)

func (p *snapPrinter) emitGroupingSetsExpr(i, stop, ind int) int {
	lineStart := i
	for j := i; j < stop; j++ {
		if p.toks[j].kind == snapTokComma {
			d := snapParenDepthBetween(p.toks, i, j)
			if d >= groupingSetsCommaBreakDepth {
				p.emitRange(lineStart, j)
				p.b.WriteString(",\n")
				extra := ind + 1
				if d >= groupingSetsCommaBreakDeepDepth {
					extra = ind + 2
				}
				p.emitIndent(extra)
				lineStart = j + 1
			}
		}
	}
	if lineStart < stop {
		p.emitRange(lineStart, stop)
	}
	return stop
}
