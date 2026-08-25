package expect

import (
	"testing"

	"github.com/bradleyjkemp/cupaloy"
)

// formatSQL pretty-prints SQL for snapshotting.
//
// Formatting runs in-process (see sqlformat.go). It needs no Docker, no
// pgFormatter binary and nothing outside the standard library. Input that
// cannot be parsed as a statement is passed through with its key words
// normalised rather than being dropped, so this never fails and needs no
// fallback path — a missing formatter can no longer cause unformatted SQL to
// be snapshot silently.
func formatSQL(sql string) string {
	return formatSQLString(sql)
}

// SQL formats a SQL string and snapshots it for testing.
// Snapshots are written as .sql files under .snapshots/.
func SQL(t *testing.T, sql string) {
	t.Helper()
	formatted := formatSQL(sql)

	cupaloy.NewDefaultConfig().
		WithOptions(cupaloy.SnapshotFileExtension(".sql")).
		SnapshotT(t, formatted)
}
