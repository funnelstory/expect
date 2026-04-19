package pgformat

import (
	"os/exec"
	"strings"
	"testing"
)

// Image must match expect.SQL / CI (FunnelStory pgFormatter wrapper).
const pgformatterDockerImage = "ghcr.io/funnelstory/pgformatter@sha256:98d352d8ffa7bf7c90c49459969f54007db39d65b501646ab570c8c6642cd435"

func dockerPgFormat(t *testing.T, sql string) string {
	t.Helper()
	cmd := exec.Command("docker", "run", "--rm", "-i", pgformatterDockerImage, "-")
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("docker pgformatter: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestFormatMatchesDockerPgFormatter compares Format output to the upstream
// docker image when docker is available; otherwise subtests skip.
func TestFormatMatchesDockerPgFormatter(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			name: "cte_window_exists_order",
			sql:  `WITH a AS (SELECT id, sum(x) FILTER (WHERE y > 0) AS s FROM t GROUP BY id), b AS (SELECT * FROM a WHERE s > 1) SELECT b.*, row_number() OVER (PARTITION BY id ORDER BY s DESC) AS rn FROM b JOIN c ON c.id = b.id WHERE EXISTS (SELECT 1 FROM d WHERE d.id = b.id) ORDER BY rn NULLS FIRST`,
		},
		{
			name: "insert_select_join",
			sql:  `INSERT INTO archive (user_id, payload, created_at) SELECT u.id, j.doc, now() FROM users u INNER JOIN jsonb_each(u.settings) AS j ON true WHERE u.active AND coalesce(j.value->>'keep', '0') = '1'`,
		},
		{
			name: "update_where_returning",
			sql:  `UPDATE accounts SET balance = balance - 100, updated_at = now() WHERE id IN (SELECT account_id FROM holds WHERE released IS NULL) AND status = 'open' RETURNING id, balance`,
		},
		{
			name: "delete_using_subquery",
			sql:  `DELETE FROM orders o USING clients c WHERE o.client_id = c.id AND c.tier = 'trial' AND o.placed_at < (SELECT min(started_at) FROM subscriptions s WHERE s.client_id = c.id)`,
		},
		{
			name: "lateral_json_grouping",
			sql:  `SELECT e.org_id, t.k, sum((t.v->>'amt')::numeric) AS total FROM events e LEFT JOIN LATERAL jsonb_each(e.props) AS t ON true WHERE e.kind = 'purchase' GROUP BY e.org_id, t.k HAVING sum((t.v->>'amt')::numeric) > 0 ORDER BY e.org_id, total DESC`,
		},
		{
			name: "union_all_order_limit",
			sql:  `SELECT region, count(*) AS n FROM sales WHERE year = 2024 GROUP BY region UNION ALL SELECT 'ALL' AS region, count(*) AS n FROM sales WHERE year = 2024 ORDER BY n DESC LIMIT 10`,
		},
		{
			name: "nested_cte_case_full_join",
			sql:  `WITH outer_q AS (WITH inner_q AS (SELECT a.id, a.name FROM authors a) SELECT * FROM inner_q) SELECT coalesce(x.id, y.id) AS id, CASE WHEN x.name IS NULL THEN y.name ELSE x.name END AS name FROM outer_q x FULL OUTER JOIN editors y ON x.id = y.id WHERE x.id IS NOT NULL OR y.id IS NOT NULL`,
		},
		{
			name: "on_conflict",
			sql:  `INSERT INTO counters (k, v) VALUES ('hits', 1) ON CONFLICT (k) DO UPDATE SET v = counters.v + excluded.v, touched = now() WHERE counters.archived IS DISTINCT FROM true`,
		},
		{
			name: "having_subquery_offset",
			sql:  `SELECT department, avg(salary) AS a FROM staff GROUP BY department HAVING avg(salary) > (SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY salary) FROM staff) ORDER BY a OFFSET 5 ROWS FETCH NEXT 20 ROWS ONLY`,
		},
		{
			name: "multi_join_where_and",
			sql:  `SELECT p.id FROM products p LEFT JOIN inventory i ON i.product_id = p.id RIGHT JOIN warehouses w ON w.id = i.warehouse_id CROSS JOIN regions r WHERE p.discontinued = false AND (i.qty > 0 OR i.qty IS NULL) AND r.code = 'US-WEST' AND w.active AND p.sku LIKE 'X-%'`,
		},
		{
			name: "deep_nested_subqueries",
			sql:  `SELECT a, (SELECT max(b) FROM t2 WHERE t2.a = t1.a AND t2.c IN (SELECT c FROM t3 WHERE t3.d = t1.d)) AS m FROM t1 WHERE t1.k = 'x'`,
		},
		{
			name: "case_in_where_and_group",
			sql:  `SELECT region, sum(amount) FROM sales WHERE CASE WHEN currency = 'USD' THEN amount ELSE amount * fx END > 1000 GROUP BY region HAVING count(*) > CASE WHEN region = 'US' THEN 100 ELSE 10 END ORDER BY region`,
		},
		{
			name: "multi_cte_chain",
			sql:  `WITH a AS (SELECT 1 AS x), b AS (SELECT x + 1 AS x FROM a), c AS (SELECT x * 2 AS x FROM b) SELECT * FROM c JOIN a ON a.x < c.x`,
		},
		{
			name: "recursive_cte",
			sql:  `WITH RECURSIVE t(n) AS (VALUES (1) UNION ALL SELECT n + 1 FROM t WHERE n < 100) SELECT sum(n) FROM t`,
		},
		{
			name: "window_with_frame",
			sql:  `SELECT id, sum(amount) OVER (PARTITION BY account ORDER BY ts ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running FROM ledger ORDER BY id`,
		},
		{
			name: "insert_select_cte",
			sql:  `WITH src AS (SELECT id, name FROM staging WHERE valid) INSERT INTO target (id, name) SELECT id, upper(name) FROM src ON CONFLICT (id) DO NOTHING`,
		},
		{
			name: "update_from_join",
			sql:  `UPDATE accounts a SET balance = a.balance + p.amount FROM payments p JOIN clients c ON c.id = p.client_id WHERE p.account_id = a.id AND c.tier = 'gold' RETURNING a.id, a.balance`,
		},
		{
			name: "array_jsonb_ops",
			sql:  `SELECT id, tags && ARRAY['hot','new']::text[] AS matches, props->'meta'->>'src' AS src FROM items WHERE props @> '{"active":true}'::jsonb AND tags <@ ARRAY['hot','new','sale']::text[]`,
		},
		{
			name: "between_and_in",
			sql:  `SELECT id FROM events WHERE ts BETWEEN '2024-01-01' AND '2024-12-31' AND kind IN ('click','view','submit') AND user_id NOT IN (SELECT user_id FROM banned)`,
		},
		{
			name: "grouping_sets_rollup",
			sql:  `SELECT region, product, sum(qty) FROM sales GROUP BY GROUPING SETS ((region, product), (region), ()) ORDER BY region NULLS LAST, product NULLS LAST`,
		},
		{
			name: "lateral_join",
			sql:  `SELECT u.id, p.title FROM users u JOIN LATERAL (SELECT title FROM posts p WHERE p.user_id = u.id ORDER BY created_at DESC LIMIT 3) p ON true WHERE u.active`,
		},
		{
			name: "filter_within_group",
			sql:  `SELECT dept, count(*) FILTER (WHERE active) AS active_count, percentile_cont(0.5) WITHIN GROUP (ORDER BY salary) AS median FROM employees GROUP BY dept`,
		},
		{
			name: "nested_select_list_subqueries",
			sql:  `SELECT a, (SELECT b FROM t2 WHERE t2.k = (SELECT k FROM t3 WHERE t3.id = t1.id LIMIT 1)) AS v FROM t1`,
		},
		{
			name: "create_table_basic",
			sql:  `CREATE TABLE accounts (id bigserial PRIMARY KEY, email text NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), tier text CHECK (tier IN ('free','pro','enterprise')))`,
		},
		{
			name: "merge_basic",
			sql:  `MERGE INTO target t USING source s ON t.id = s.id WHEN MATCHED AND s.deleted THEN DELETE WHEN MATCHED THEN UPDATE SET name = s.name WHEN NOT MATCHED THEN INSERT (id, name) VALUES (s.id, s.name)`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := dockerPgFormat(t, tc.sql)
			got := Format(tc.sql)
			if got != want {
				t.Errorf("mismatch with docker pgFormatter\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}
