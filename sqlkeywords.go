package expect

import "strings"

// reservedWords are upper-cased by the formatter. The list covers PostgreSQL's
// reserved and non-reserved key words that appear in statement structure. It
// deliberately excludes function names and type names, which are left exactly
// as the caller wrote them.
var reservedWords = map[string]struct{}{}

func init() {
	words := []string{
		// statement and clause structure
		"SELECT", "FROM", "WHERE", "GROUP", "BY", "HAVING", "ORDER", "LIMIT",
		"OFFSET", "FETCH", "NEXT", "ROWS", "ONLY", "INSERT", "INTO", "VALUES",
		"UPDATE", "SET", "DELETE", "RETURNING", "WITH", "RECURSIVE", "AS",
		"DISTINCT", "ALL", "UNION", "INTERSECT", "EXCEPT", "CONFLICT",
		"DO", "NOTHING", "MATERIALIZED", "LATERAL", "WINDOW", "OVER",
		"PARTITION", "FILTER", "WITHIN", "GROUPING", "SETS", "CUBE", "ROLLUP",

		// joins
		"JOIN", "INNER", "OUTER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL",
		"ON", "USING",

		// predicates and operators
		"AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE", "ILIKE",
		"SIMILAR", "TO", "IS", "NULL", "TRUE", "FALSE", "UNKNOWN", "ANY",
		"SOME", "ASC", "DESC", "NULLS", "FIRST", "LAST", "CASE", "WHEN",
		"THEN", "ELSE", "END", "CAST",

		// DDL and transactions, so that mixed scripts normalise consistently
		"CREATE", "TABLE", "VIEW", "INDEX", "SEQUENCE", "SCHEMA", "DATABASE",
		"ALTER", "DROP", "TRUNCATE", "ADD", "COLUMN", "CONSTRAINT", "PRIMARY",
		"FOREIGN", "KEY", "REFERENCES", "UNIQUE", "CHECK", "DEFAULT",
		"CASCADE", "RESTRICT", "IF", "REPLACE", "TEMPORARY", "TEMP",
		"BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION", "SAVEPOINT",
		"GRANT", "REVOKE", "ON",
	}
	for _, w := range words {
		reservedWords[w] = struct{}{}
	}
}

func isReservedWord(s string) bool {
	_, ok := reservedWords[strings.ToUpper(s)]
	return ok
}
