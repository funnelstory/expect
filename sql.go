package expect

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
)

// pgFormatterImage is the Docker image used to format SQL via pgFormatter.
const pgFormatterImage = "ghcr.io/funnelstory/pgformatter@sha256:98d352d8ffa7bf7c90c49459969f54007db39d65b501646ab570c8c6642cd435"

// formatSQL runs SQL through pgFormatter in Docker. If Docker or the formatter
// fails, it returns the input unchanged.
func formatSQL(sql string) string {
	cmd := exec.Command("docker", "run", "--rm", "-a", "stdin", "-a", "stdout", "-i", pgFormatterImage, "-")
	cmd.Stdin = strings.NewReader(sql)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return sql
	}
	formatted := strings.TrimSpace(out.String())
	if formatted == "" {
		return sql
	}
	return formatted
}

// SQL formats a SQL string and snapshots it for testing.
// Formatting uses pgFormatter in Docker (ghcr.io/funnelstory/pgformatter). If
// that is unavailable, the original SQL is snapshot as-is.
func SQL(t *testing.T, sql string) {
	t.Helper()
	formatted := formatSQL(sql)

	cupaloy.NewDefaultConfig().
		WithOptions(cupaloy.SnapshotFileExtension(".sql")).
		SnapshotT(t, formatted)
}
