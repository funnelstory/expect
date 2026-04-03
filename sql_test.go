package expect

import (
	"testing"
)

func TestSQL(t *testing.T) {
	t.Parallel()

	t.Run("formats simple query", func(t *testing.T) {
		t.Parallel()
		SQL(t, "SELECT * FROM users WHERE id = 1")
	})

	t.Run("formats complex query", func(t *testing.T) {
		t.Parallel()
		SQL(t, `
			SELECT u.id, u.name, o.order_id, o.total
			FROM users u
			JOIN orders o ON u.id = o.user_id
			WHERE u.active = true AND o.total > 100
			ORDER BY o.created_at DESC
			LIMIT 10
		`)
	})

	t.Run("formats query with CTE", func(t *testing.T) {
		t.Parallel()
		SQL(t, `
			WITH active_users AS (
				SELECT id, name FROM users WHERE active = true
			)
			SELECT * FROM active_users WHERE name LIKE 'A%'
		`)
	})

	t.Run("formats INSERT statement", func(t *testing.T) {
		t.Parallel()
		SQL(t, "INSERT INTO users (name, email) VALUES ('John Doe', 'john@example.com')")
	})

	t.Run("formats UPDATE statement", func(t *testing.T) {
		t.Parallel()
		SQL(t, "UPDATE users SET name = 'Jane Doe', updated_at = NOW() WHERE id = 123")
	})

	t.Run("formats DELETE statement", func(t *testing.T) {
		t.Parallel()
		SQL(t, "DELETE FROM users WHERE created_at < '2020-01-01'")
	})

	t.Run("formats malformed select", func(t *testing.T) {
		t.Parallel()
		SQL(t, "SELECT FROM WHERE")
	})
}
