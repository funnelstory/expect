package pgformat

import "strings"

// sqlBuiltinFuncs — word before '(' with no space (common PostgreSQL builtins).
var sqlBuiltinFuncs = map[string]struct{}{
	"AVG": {}, "CAST": {}, "COALESCE": {}, "COUNT": {}, "CUME_DIST": {}, "DATE_TRUNC": {},
	"DENSE_RANK": {}, "EXTRACT": {}, "FIRST_VALUE": {}, "GREATEST": {}, "JSONB_EACH": {}, "LAG": {}, "LAST_VALUE": {},
	"LEAD": {}, "LEAST": {}, "LOWER": {}, "MAX": {}, "MIN": {}, "NOW": {}, "NTH_VALUE": {},
	"NTILE": {}, "NULLIF": {}, "OVERLAY": {}, "PERCENT_RANK": {}, "PERCENTILE_CONT": {}, "PERCENTILE_DISC": {},
	"POSITION": {}, "RANK": {},
	"REGEXP_MATCH": {}, "REGEXP_REPLACE": {}, "REGEXP_SPLIT_TO_ARRAY": {}, "ROUND": {}, "ROW_NUMBER": {},
	"SUBSTRING": {}, "SUM": {}, "TO_CHAR": {}, "TO_DATE": {}, "TO_TIMESTAMP": {}, "TRIM": {},
	"TRUNC": {}, "UPPER": {},
}

func isBuiltinFunc(w string) bool {
	_, ok := sqlBuiltinFuncs[strings.ToUpper(w)]
	return ok
}
