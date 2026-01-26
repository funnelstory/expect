package expect

import (
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/mjibson/sqlfmt"
)

// SQL formats a SQL string and snapshots it for testing.
// The SQL is formatted using CockroachDB's SQL formatter with consistent
// formatting (2-space indentation, 180 character line width) before snapshotting.
func SQL(t *testing.T, sql string) {
	t.Helper()
	cfg := tree.DefaultPrettyCfg()
	cfg.UseTabs = false
	cfg.TabWidth = 2
	cfg.LineWidth = 180

	formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
	if err != nil {
		t.Fatal(err)
	}

	cupaloy.NewDefaultConfig().
		WithOptions(cupaloy.SnapshotFileExtension(".sql")).
		SnapshotT(t, formatted)
}
