package expect

import (
	"testing"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/sem/tree"
	"github.com/mjibson/sqlfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQL(t *testing.T) {
	t.Parallel()

	t.Run("formats simple query", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = 1"

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "SELECT")
		assert.Contains(t, formatted, "FROM users")
	})

	t.Run("formats complex query", func(t *testing.T) {
		sql := `
			SELECT u.id, u.name, o.order_id, o.total
			FROM users u
			JOIN orders o ON u.id = o.user_id
			WHERE u.active = true AND o.total > 100
			ORDER BY o.created_at DESC
			LIMIT 10
		`

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "SELECT")
		assert.Contains(t, formatted, "JOIN")
		assert.Contains(t, formatted, "WHERE")
		assert.Contains(t, formatted, "ORDER BY")
		assert.Contains(t, formatted, "LIMIT")
	})

	t.Run("formats query with CTE", func(t *testing.T) {
		sql := `
			WITH active_users AS (
				SELECT id, name FROM users WHERE active = true
			)
			SELECT * FROM active_users WHERE name LIKE 'A%'
		`

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "WITH")
	})

	t.Run("formats INSERT statement", func(t *testing.T) {
		sql := "INSERT INTO users (name, email) VALUES ('John Doe', 'john@example.com')"

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "INSERT INTO")
	})

	t.Run("formats UPDATE statement", func(t *testing.T) {
		sql := "UPDATE users SET name = 'Jane Doe', updated_at = NOW() WHERE id = 123"

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "UPDATE")
	})

	t.Run("formats DELETE statement", func(t *testing.T) {
		sql := "DELETE FROM users WHERE created_at < '2020-01-01'"

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		formatted, err := sqlfmt.FmtSQL(cfg, []string{sql})
		require.NoError(t, err)
		assert.NotEmpty(t, formatted)
		assert.Contains(t, formatted, "DELETE")
	})

	t.Run("handles invalid SQL", func(t *testing.T) {
		sql := "SELECT FROM WHERE" // Invalid SQL

		cfg := tree.DefaultPrettyCfg()
		cfg.UseTabs = false
		cfg.TabWidth = 2
		cfg.LineWidth = 180

		_, err := sqlfmt.FmtSQL(cfg, []string{sql})
		assert.Error(t, err)
	})
}
