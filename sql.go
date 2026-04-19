package expect

import (
	"strings"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/funnelstory/expect/pgformat"
)

// formatSQL formats SQL for snapshots (pgFormatter-compatible layout).
func formatSQL(sql string) string {
	out := strings.TrimSpace(pgformat.Format(sql))
	if out == "" {
		return strings.TrimSpace(sql)
	}
	return out
}

// SQL formats a SQL string and snapshots it for testing.
func SQL(t *testing.T, sql string) {
	t.Helper()
	formatted := formatSQL(sql)

	cupaloy.NewDefaultConfig().
		WithOptions(cupaloy.SnapshotFileExtension(".sql")).
		SnapshotT(t, formatted)
}
