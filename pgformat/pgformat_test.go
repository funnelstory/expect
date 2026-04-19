package pgformat

import (
	"strings"
	"testing"
)

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
