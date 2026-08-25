package expect

import (
	"strings"
	"testing"
)

// sqlCorpus is the body of SQL the formatter is held to. The committed .sql
// snapshots pin exact output for seven queries; this corpus is much wider and
// pins the invariants that must hold for anything a caller might pass in.
var sqlCorpus = []string{
	// projection and ordering
	"SELECT * FROM users WHERE id = 1",
	"SELECT a, b, c FROM t",
	"SELECT DISTINCT a FROM t",
	"SELECT DISTINCT ON (a) a, b FROM t",
	"SELECT count(*) FROM t",
	"SELECT count(*) AS n, max(x) FROM t GROUP BY y HAVING count(*) > 2",
	"SELECT a FROM t ORDER BY a ASC, b DESC NULLS LAST",
	"SELECT a FROM t LIMIT 10 OFFSET 20",
	"SELECT t.* FROM t",
	"SELECT 1.5, 2, -3 FROM t",
	"SELECT schema.tbl.col FROM schema.tbl",

	// joins
	"SELECT * FROM a JOIN b ON a.id = b.id",
	"SELECT * FROM a LEFT JOIN b ON a.id = b.id",
	"SELECT * FROM a LEFT OUTER JOIN b ON a.id = b.id",
	"SELECT * FROM a FULL OUTER JOIN b USING (id)",
	"SELECT * FROM a CROSS JOIN b",
	"SELECT * FROM a NATURAL JOIN b",
	"SELECT * FROM a, b, c WHERE a.id = b.id",

	// predicates
	"SELECT * FROM t WHERE a = 1 AND b = 2 AND c = 3",
	"SELECT * FROM t WHERE (a = 1 OR b = 2) AND c = 3",
	"SELECT * FROM t WHERE a IN (1, 2, 3)",
	"SELECT * FROM t WHERE a IN (SELECT id FROM u)",
	"SELECT * FROM t WHERE EXISTS (SELECT 1 FROM u WHERE u.id = t.id)",
	"SELECT * FROM t WHERE a BETWEEN 1 AND 10",
	"SELECT * FROM t WHERE a IS NULL",
	"SELECT * FROM t WHERE a IS NOT NULL",
	"SELECT * FROM t WHERE name LIKE 'A%'",
	"SELECT * FROM t WHERE name ILIKE '%a%'",
	"SELECT * FROM t WHERE a <> 1",
	"SELECT * FROM t WHERE a != 1",
	"SELECT * FROM t WHERE a <= 1 AND b >= 2",

	// expressions
	"SELECT (SELECT max(id) FROM inner_t) AS m FROM outer_t",
	"SELECT CASE WHEN a = 1 THEN 'one' WHEN a = 2 THEN 'two' ELSE 'other' END FROM t",
	"SELECT row_number() OVER (PARTITION BY a ORDER BY b) FROM t",
	"SELECT sum(x) OVER w FROM t WINDOW w AS (PARTITION BY y)",
	"SELECT a::text FROM t",
	"SELECT CAST(a AS text) FROM t",
	"SELECT a || b FROM t",
	"SELECT data->>'key' FROM t",
	"SELECT data->'key' FROM t",
	"SELECT array_agg(a) FROM t",
	"SELECT * FROM generate_series(1, 10)",

	// CTEs
	"WITH a AS (SELECT 1) SELECT * FROM a",
	"WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a, b",
	"WITH RECURSIVE t AS (SELECT 1 UNION ALL SELECT n + 1 FROM t) SELECT * FROM t",

	// writes
	"INSERT INTO t (a, b) VALUES (1, 2)",
	"INSERT INTO t (a, b) VALUES (1, 2), (3, 4)",
	"INSERT INTO t SELECT * FROM u",
	"INSERT INTO t (a) VALUES (1) RETURNING id",
	"INSERT INTO t (a) VALUES (1) ON CONFLICT DO NOTHING",
	"UPDATE t SET a = 1",
	"UPDATE t SET a = 1, b = 2 WHERE id = 3",
	"UPDATE t SET a = 1 FROM u WHERE t.id = u.id",
	"UPDATE t SET a = 1 RETURNING *",
	"DELETE FROM t",
	"DELETE FROM t WHERE id = 1",
	"DELETE FROM t USING u WHERE t.id = u.id",

	// set operations
	"SELECT 1 UNION SELECT 2",
	"SELECT 1 UNION ALL SELECT 2",
	"SELECT 1 INTERSECT SELECT 2",
	"SELECT 1 EXCEPT SELECT 2",

	// lexical edge cases
	"SELECT * FROM t -- trailing comment",
	"SELECT /* inline */ a FROM t",
	"SELECT /* nested /* deeper */ still */ a FROM t",
	"SELECT $$dollar$$ FROM t",
	"SELECT $tag$body$tag$ FROM t",
	"SELECT 'it''s' FROM t",
	`SELECT "Quoted Ident" FROM "Quoted Table"`,
	"SELECT 'select from where' FROM t",
	"SELECT * FROM t WHERE a = 'multi\nline'",
	"select * from users where id = 1",

	// scripts and DDL
	"SELECT * FROM a; SELECT * FROM b",
	"BEGIN; SELECT 1; COMMIT",
	"CREATE TABLE t (id int PRIMARY KEY, name text)",
	"ALTER TABLE t ADD COLUMN x int",
	"DROP TABLE IF EXISTS t",

	// malformed or partial input
	"SELECT FROM WHERE",
	"", "   ", ";", ";;;", "(", ")", "()", "'", `"`, "$$", "$tag$",
	"--", "/*", "SELECT", "WITH", "WITH a AS (", "INSERT INTO", "VALUES",
	"'unterminated",
}

// tokenTexts is the tokenizer's view of a string, used to compare the content
// of input and output independently of layout.
func tokenTexts(s string) []string {
	toks := tokenize(s)
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, t.text)
	}
	return out
}

// assertPreservesContent checks that every token in the input survives into the
// output. Layout may change freely; content may not. Semicolons are exempt
// because terminators are re-attached per statement rather than carried
// through as tokens.
func assertPreservesContent(t *testing.T, in, out string) {
	t.Helper()

	have := map[string]int{}
	for _, tok := range tokenTexts(out) {
		have[tok]++
	}
	for _, tok := range tokenTexts(in) {
		if tok == ";" {
			continue
		}
		if have[tok] == 0 {
			t.Errorf("formatSQLString dropped %q\n  in:  %q\n  out: %q", tok, in, out)
			return
		}
		have[tok]--
	}
}

// TestCorpusInvariants runs every corpus entry through the three properties
// that must hold for all input: it must not panic, formatting must be
// idempotent, and no content may be lost.
func TestCorpusInvariants(t *testing.T) {
	t.Parallel()

	for _, q := range sqlCorpus {
		t.Run(shortName(q), func(t *testing.T) {
			t.Parallel()

			out := formatSQLString(q)

			if twice := formatSQLString(out); twice != out {
				t.Errorf("not idempotent\n  in:    %q\n  once:  %q\n  twice: %q", q, out, twice)
			}
			assertPreservesContent(t, q, out)
		})
	}
}

// TestCorpusNoTrailingWhitespace guards a class of diff noise that is easy to
// introduce and annoying to review.
func TestCorpusNoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	for _, q := range sqlCorpus {
		out := formatSQLString(q)
		for i, line := range strings.Split(out, "\n") {
			if line != strings.TrimRight(line, " \t") {
				t.Errorf("trailing whitespace on line %d of %q:\n%q", i+1, q, line)
			}
		}
		if strings.HasPrefix(out, "\n") || strings.HasSuffix(out, "\n") {
			t.Errorf("output is not trimmed for %q: %q", q, out)
		}
	}
}

// TestLiteralsAreNeverReformatted checks the property that matters most for
// correctness: whatever is inside a string literal, a quoted identifier, a
// dollar-quoted body or a comment comes out byte-for-byte unchanged, even when
// it contains text that looks like SQL.
func TestLiteralsAreNeverReformatted(t *testing.T) {
	t.Parallel()

	cases := []string{
		"'select * from t'",
		"'SELECT   spaced   keywords'",
		"'it''s'",
		`"select from where"`,
		"$$select * from t$$",
		"$tag$ select 1 $tag$",
	}

	for _, lit := range cases {
		out := formatSQLString("SELECT " + lit + " FROM t")
		if !strings.Contains(out, lit) {
			t.Errorf("literal %q was altered\n  got: %q", lit, out)
		}
	}
}

// FuzzFormatSQLString asserts the same invariants over arbitrary bytes. The
// formatter runs inside test suites, so a panic here would take down unrelated
// tests; content loss would silently corrupt a snapshot.
func FuzzFormatSQLString(f *testing.F) {
	for _, q := range sqlCorpus {
		f.Add(q)
	}

	f.Fuzz(func(t *testing.T, in string) {
		out := formatSQLString(in) // must not panic

		if twice := formatSQLString(out); twice != out {
			t.Fatalf("not idempotent\n  in:    %q\n  once:  %q\n  twice: %q", in, out, twice)
		}
		if strings.TrimSpace(out) != out {
			t.Fatalf("output not trimmed: %q", out)
		}
	})
}

func BenchmarkFormatSQLString(b *testing.B) {
	const query = `
		SELECT u.id, u.name, o.order_id, o.total
		FROM users u
		JOIN orders o ON u.id = o.user_id
		WHERE u.active = true AND o.total > 100
		ORDER BY o.created_at DESC
		LIMIT 10
	`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = formatSQLString(query)
	}
}

// shortName makes a readable subtest name out of a query.
func shortName(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return "empty"
	}
	if len(q) > 48 {
		q = q[:48]
	}
	return q
}
