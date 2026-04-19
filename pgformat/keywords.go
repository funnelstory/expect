package pgformat

import "strings"

// pgFormatterClauseWords are SQL clause / structural keywords (pgFormatter-style).
var pgFormatterClauseWords map[string]struct{}

// pgFormatterTypeWords are common PostgreSQL type names for TypeCase handling.
var pgFormatterTypeWords map[string]struct{}

func init() {
	clause := []string{
		"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "NULL", "TRUE", "FALSE",
		"DISTINCT", "AS", "JOIN", "LEFT", "RIGHT", "INNER", "FULL", "CROSS", "OUTER",
		"ON", "USING", "LATERAL", "WITH", "UNION", "ALL", "INTERSECT", "EXCEPT",
		"GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET", "FETCH", "NEXT", "ROWS", "ONLY", "FIRST",
		"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE", "RETURNING",
		"EXISTS", "IN", "IS", "CASE", "WHEN", "THEN", "ELSE", "END", "ELSIF",
		"BETWEEN", "LIKE", "ILIKE", "SIMILAR", "ESCAPE", "SOME", "ANY",
		"OVER", "PARTITION", "FILTER", "WITHIN", "WINDOW",
		"DO", "CONFLICT", "USER",
	}
	pgFormatterClauseWords = make(map[string]struct{}, len(clause))
	for _, w := range clause {
		pgFormatterClauseWords[w] = struct{}{}
	}

	types := []string{
		"INT", "INTEGER", "BIGINT", "SMALLINT", "NUMERIC", "DECIMAL", "REAL", "FLOAT", "DOUBLE", "PRECISION",
		"BOOLEAN", "BOOL", "TEXT", "VARCHAR", "CHAR", "BPCHAR", "NAME",
		"DATE", "TIME", "TIMESTAMP", "TIMESTAMPTZ", "INTERVAL",
		"JSON", "JSONB", "UUID", "BYTEA", "XML", "MONEY",
		"SERIAL", "BIGSERIAL", "SMALLSERIAL",
		"OID", "REGCLASS", "REGPROC", "REGTYPE",
	}
	pgFormatterTypeWords = make(map[string]struct{}, len(types))
	for _, w := range types {
		pgFormatterTypeWords[w] = struct{}{}
	}
}

func applyWordCase(lit string, c WordCase) string {
	switch c {
	case UpperCase:
		return strings.ToUpper(lit)
	case LowerCase:
		return strings.ToLower(lit)
	default:
		return lit
	}
}

// normalizeWord applies Options-based casing for a single bare word token.
func normalizeWord(lit string, o Options) string {
	u := strings.ToUpper(lit)
	if _, ok := pgFormatterClauseWords[u]; ok {
		return applyWordCase(lit, o.KeywordCase)
	}
	if _, ok := pgFormatterTypeWords[u]; ok {
		return applyWordCase(lit, o.TypeCase)
	}
	if isBuiltinFunc(u) {
		return applyWordCase(lit, o.FunctionCase)
	}
	return applyWordCase(lit, o.IdentifierCase)
}
