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

	t.Run("formats materialized double-paren CTE", func(t *testing.T) {
		t.Parallel()
		SQL(t, `WITH m AS MATERIALIZED ((SELECT 1)) SELECT 1`)
	})

	t.Run("formats select list bounded subquery", func(t *testing.T) {
		t.Parallel()
		SQL(t, `WITH r AS (SELECT 1 AS c0, 2 AS c1)
SELECT
    r.c0,
    (SELECT COUNT(*) FROM r u WHERE u.c0 = r.c0) AS cnt,
    COALESCE((SELECT c1 FROM r u2 WHERE u2.c0 = r.c0 LIMIT 1), 0) AS val
FROM r`)
	})

	t.Run("formats activity rollup with materialized CTE", func(t *testing.T) {
		t.Parallel()
		SQL(t, `WITH c AS MATERIALIZED ((
        SELECT
            w,
            a,
            LEAST(COALESCE(ch, '2000-01-01T00:00:00Z'), '2000-01-01T00:00:00Z') AS cap
        FROM
            dim_d
        WHERE (
            w = '00000000-0000-0000-0000-000000000001'
            AND p >= COALESCE((
                    SELECT t FROM mark_m
                    WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'a'
), '2000-01-01T00:00:00Z')
)
    ORDER BY
        a ASC
))
SELECT
    f.a,
    f.e,
    SUM(f.n) AS s
FROM
    fact_f f
    INNER JOIN c ON ((f.w = c.w) AND (f.j ->> 'a' = c.a))
WHERE (f.t >= (c.cap - INTERVAL '6 months') AND f.t < c.cap)
GROUP BY
    f.a,
    f.e`)
	})

	t.Run("formats union of derived tables with nested materialized CTEs", func(t *testing.T) {
		t.Parallel()
		SQL(t, `SELECT
    *
FROM ( WITH q0 AS (
        WITH c_ids AS MATERIALIZED ((
                SELECT
                    a
                FROM
                    dim_d
                WHERE (
                    w = '00000000-0000-0000-0000-000000000001'
                    AND p >= COALESCE((
                            SELECT t FROM mark_m
                            WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'x'
), '2000-01-01T00:00:00Z')
)
            ORDER BY
                a ASC
)
)
        SELECT
            *,
            (
                SELECT d FROM ver_m WHERE (w, mid, h) = (f.w, f.mid, f.h0)
            LIMIT 1
) r0,
        (
            SELECT d FROM ver_m WHERE (w, mid, h) = (f.w, f.mid, f.h1)
        LIMIT 1
) r1
FROM
    fact_a f
WHERE ((
        (e IN ('00000000-0000-0000-0000-000000000002'))
        AND (t > '2000-01-01T00:00:00Z')
        AND (w = '00000000-0000-0000-0000-000000000001')
)
    AND j ->> 'a' IN (SELECT a FROM c_ids)
        AND j ->> 'a' IN ('id_1', 'id_2', 'id_3')
))
SELECT
    t,
    q0.h0 AS eid,
    j ->> 'a' AS aid,
    'u' AS ty,
    row_to_json(q0) AS d
FROM
    q0
ORDER BY
    t ASC,
    q0.h0 ASC) AS t1
UNION ALL (
    SELECT
        *
    FROM ( WITH c_ids AS MATERIALIZED ((
                SELECT a FROM dim_d
                WHERE (
                    w = '00000000-0000-0000-0000-000000000001'
                    AND p >= COALESCE((
                            SELECT t FROM mark_m
                        WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'x'
), '2000-01-01T00:00:00Z')
)
            ORDER BY a ASC
))
        SELECT t, id::text AS eid, a, 'v' AS ty, row_to_json(s) AS d
        FROM sig_s s
        WHERE (((r IN ('00000000-0000-0000-0000-000000000003'))
                AND (t > '2000-01-01T00:00:00Z')
                AND (w = '00000000-0000-0000-0000-000000000001'))
            AND a IN (SELECT a FROM c_ids) AND a IN ('id_1', 'id_2', 'id_3'))
        ORDER BY t ASC, id ASC) AS t1)
ORDER BY t ASC
LIMIT 100`)
	})
}
