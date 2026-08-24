package expect

import (
	"strings"
	"testing"
)

// TestFormatSQLString covers the formatter directly, without snapshotting, so
// that layout rules are asserted in one readable place.
func TestFormatSQLString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "lower case key words are normalised",
			in:   "select id from users where active = true",
			want: "SELECT\n    id\nFROM\n    users\nWHERE\n    active = TRUE",
		},
		{
			name: "identifiers keep their case",
			in:   "SELECT userName FROM MyTable",
			want: "SELECT\n    userName\nFROM\n    MyTable",
		},
		{
			name: "quoted identifiers are left intact",
			in:   `SELECT "Mixed Case" FROM "Table"`,
			want: "SELECT\n    \"Mixed Case\"\nFROM\n    \"Table\"",
		},
		{
			name: "string literals are never touched",
			in:   "SELECT * FROM t WHERE s = 'select from where'",
			want: "SELECT\n    *\nFROM\n    t\nWHERE\n    s = 'select from where'",
		},
		{
			name: "escaped quotes inside literals survive",
			in:   "SELECT * FROM t WHERE s = 'it''s here'",
			want: "SELECT\n    *\nFROM\n    t\nWHERE\n    s = 'it''s here'",
		},
		{
			name: "commas in function arguments do not split lines",
			in:   "SELECT coalesce(a, b), c FROM t",
			want: "SELECT\n    coalesce(a, b),\n    c\nFROM\n    t",
		},
		{
			name: "boolean operators start new lines",
			in:   "SELECT * FROM t WHERE a = 1 AND b = 2 OR c = 3",
			want: "SELECT\n    *\nFROM\n    t\nWHERE\n    a = 1\n    AND b = 2\n    OR c = 3",
		},
		{
			name: "joins each get a line",
			in:   "SELECT * FROM a JOIN b ON a.id = b.id LEFT JOIN c ON c.id = a.id",
			want: "SELECT\n    *\nFROM\n    a\n    JOIN b ON a.id = b.id\n    LEFT JOIN c ON c.id = a.id",
		},
		{
			name: "empty input stays empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace only input stays empty",
			in:   "   \n\t ",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatSQLString(tc.in); got != tc.want {
				t.Errorf("formatSQLString(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFormatSQLStringIdempotent checks that formatting formatted output is a
// no-op. Without this a snapshot can flip between two stable renderings.
func TestFormatSQLStringIdempotent(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"SELECT * FROM users WHERE id = 1",
		"SELECT u.id, u.name FROM users u JOIN orders o ON u.id = o.user_id WHERE u.active = true ORDER BY u.id DESC LIMIT 10",
		"WITH a AS (SELECT 1) SELECT * FROM a",
		"INSERT INTO users (name, email) VALUES ('a', 'b')",
		"UPDATE users SET name = 'x' WHERE id = 1",
		"DELETE FROM users WHERE id = 1",
		"SELECT FROM WHERE",
	}

	for _, in := range inputs {
		once := formatSQLString(in)
		twice := formatSQLString(once)
		if once != twice {
			t.Errorf("not idempotent for %q\nfirst:  %q\nsecond: %q", in, once, twice)
		}
	}
}

// TestFormatSQLStringPreservesContent checks that no identifier or literal is
// lost, whatever the layout ends up being. This is the invariant that matters
// most: a formatter may lay bytes out differently, but it must never drop them.
func TestFormatSQLStringPreservesContent(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"SELECT a, b, c FROM t WHERE x = 1 AND y = 2",
		"SELECT * FROM t -- trailing comment",
		"SELECT /* inline */ a FROM t",
		"SELECT $$dollar quoted$$ FROM t",
		"SELECT * FROM a; SELECT * FROM b",
		"SELECT (SELECT max(id) FROM inner_t) AS m FROM outer_t",
		"((((",
		"'unterminated",
	}

	for _, in := range inputs {
		got := formatSQLString(in)
		for _, word := range []string{"a", "b", "c", "inner_t", "outer_t", "dollar"} {
			if strings.Contains(in, word) && !strings.Contains(got, word) {
				t.Errorf("formatSQLString(%q) dropped %q\ngot: %q", in, word, got)
			}
		}
	}
}

// TestFormatSQLStringDoesNotPanic guards the malformed-input path. The
// formatter is called from tests, so a panic here would fail unrelated suites.
func TestFormatSQLStringDoesNotPanic(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"", " ", ";", ";;;", "(", ")", "()", "'", `"`, "$$", "$tag$",
		"--", "/*", "/* nested /* deeper */ */", "SELECT", "FROM", "WHERE",
		"SELECT FROM WHERE", "WITH", "WITH a", "WITH a AS", "WITH a AS (",
		"INSERT INTO", "VALUES", "UPDATE", "SET", "DELETE FROM",
		"SELECT * FROM t WHERE a IN (1, 2, 3)",
		strings.Repeat("SELECT * FROM t UNION ", 50) + "SELECT 1",
	}

	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("formatSQLString(%q) panicked: %v", in, r)
				}
			}()
			_ = formatSQLString(in)
		}()
	}
}
