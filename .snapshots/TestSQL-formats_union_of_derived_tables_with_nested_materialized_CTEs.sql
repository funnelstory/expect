SELECT
    *
FROM
    (
        WITH q0 AS (
            WITH c_ids AS MATERIALIZED (
                (
                    SELECT
                        a
                    FROM
                        dim_d
                    WHERE
                        (
                            w = '00000000-0000-0000-0000-000000000001' AND p >= COALESCE((SELECT t FROM mark_m WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'x'), '2000-01-01T00:00:00Z')
                        )
                    ORDER BY
                        a ASC
                )
            )
            SELECT
                *,
                (
                    SELECT
                        d
                    FROM
                        ver_m
                    WHERE
                        (
                            w, mid, h
                        ) = (f.w, f.mid, f.h0)
                    LIMIT 1) r0,
                (
                    SELECT
                        d
                    FROM
                        ver_m
                    WHERE
                        (
                            w, mid, h
                        ) = (f.w, f.mid, f.h1)
                    LIMIT 1) r1
            FROM
                fact_a f
            WHERE
                (
                    ((e IN ('00000000-0000-0000-0000-000000000002')) AND (t > '2000-01-01T00:00:00Z') AND (w = '00000000-0000-0000-0000-000000000001')) AND j ->> 'a' IN (SELECT a FROM c_ids) AND j ->> 'a' IN ('id_1', 'id_2', 'id_3')
                )
        )
        SELECT
            t,
            q0.h0 AS eid,
            j ->> 'a' AS aid,
            'u' AS ty,
            row_to_json (q0) AS d
        FROM
            q0
        ORDER BY
            t ASC,
            q0.h0 ASC
    ) AS t1
UNION ALL
(
    SELECT
        *
    FROM
        (
            WITH c_ids AS MATERIALIZED (
                (
                    SELECT
                        a
                    FROM
                        dim_d
                    WHERE
                        (
                            w = '00000000-0000-0000-0000-000000000001' AND p >= COALESCE((SELECT t FROM mark_m WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'x'), '2000-01-01T00:00:00Z')
                        )
                    ORDER BY
                        a ASC
                )
            )
            SELECT
                t,
                id::text AS eid,
                a,
                'v' AS ty,
                row_to_json (s) AS d
            FROM
                sig_s s
            WHERE
                (
                    ((r IN ('00000000-0000-0000-0000-000000000003')) AND (t > '2000-01-01T00:00:00Z') AND (w = '00000000-0000-0000-0000-000000000001')) AND a IN (SELECT a FROM c_ids) AND a IN ('id_1', 'id_2', 'id_3')
                )
            ORDER BY
                t ASC,
                id ASC
        ) AS t1
)
ORDER BY
    t ASC
LIMIT 100
