package pgformat

import (
	"strings"
	"testing"
)

func sqlMaterializedDoubleParenCTE() string {
	return `WITH m AS MATERIALIZED ((SELECT 1)) SELECT 1`
}

func sqlSelectListBoundedSubquery() string {
	return `WITH r AS (SELECT 1 AS c0, 2 AS c1)
SELECT
    r.c0,
    (SELECT COUNT(*) FROM r u WHERE u.c0 = r.c0) AS cnt,
    COALESCE((SELECT c1 FROM r u2 WHERE u2.c0 = r.c0 LIMIT 1), 0) AS val
FROM r`
}

func TestFormat(t *testing.T) {
	t.Run("matches pgFormatter style for update", func(t *testing.T) {
		out := Format("UPDATE users SET name = 'Jane Doe', updated_at = NOW() WHERE id = 123")
		if !strings.Contains(out, "NOW()") {
			t.Fatalf("got %q", out)
		}
		if strings.Contains(out, "NOW (") {
			t.Fatalf("unexpected space before paren: %q", out)
		}
	})
}

func TestFormatWithOptions(t *testing.T) {
	o := DefaultOptions()
	o.Spaces = 2
	out := FormatWithOptions("SELECT 1 FROM t", o)
	if !strings.Contains(out, "\n  1\n") {
		t.Fatalf("expected 2-space indent, got:\n%s", out)
	}
	o2 := DefaultOptions()
	o2.KeywordCase = PreserveCase
	out2 := FormatWithOptions("select 1 from t", o2)
	if !strings.HasPrefix(strings.TrimSpace(out2), "select") {
		t.Fatalf("preserve keyword case: got %q", out2)
	}
}

func TestTokenize(t *testing.T) {
	toks := tokenize("SELECT 1 + 1")
	var b strings.Builder
	for _, tok := range toks {
		b.WriteString(tok.lit)
	}
	if b.String() != "SELECT1+1" {
		t.Fatalf("concat tokens: got %q", b.String())
	}
}

func TestFormatWithMaterializedDoubleParen(t *testing.T) {
	q := sqlMaterializedDoubleParenCTE()
	out := Format(q)
	if out == strings.TrimSpace(q) {
		t.Fatalf("expected reformat, got unchanged:\n%s", out)
	}
	if !strings.Contains(out, "MATERIALIZED (\n") {
		t.Fatalf("expected newline after MATERIALIZED (:\n%s", out)
	}
}

// A parenthesized scalar subquery in the SELECT list must not let layout scan
// past its closing ")" into the next column (would glue "AS" and the alias).
func TestFormatSelectListBoundedSubquery(t *testing.T) {
	sql := sqlSelectListBoundedSubquery()
	out := Format(sql)
	for _, bad := range []string{"AScnt", "ASval"} {
		if strings.Contains(out, bad) {
			t.Fatalf("missing space before alias (pattern %q):\n%s", bad, out)
		}
	}
	if !sameTokenStream(sql, out, DefaultOptions()) {
		t.Fatalf("token stream mismatch:\n%s", out)
	}
}

func TestFormatUnsupportedReturnsOriginal(t *testing.T) {
	for _, sql := range []string{
		"CREATE INDEX idx ON accounts (lower(email))",
		"COMMENT ON TABLE t IS 'x'",
	} {
		if got := Format(sql); got != sql {
			t.Fatalf("%q: expected original, got %q", sql, got)
		}
	}
}
