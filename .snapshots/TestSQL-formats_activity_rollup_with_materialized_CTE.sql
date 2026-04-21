WITH c AS MATERIALIZED (
    (
        SELECT
            w,
            a,
            LEAST(COALESCE(ch, '2000-01-01T00:00:00Z'), '2000-01-01T00:00:00Z') AS cap
        FROM
            dim_d
        WHERE
            (
                w = '00000000-0000-0000-0000-000000000001' AND p >= COALESCE((SELECT t FROM mark_m WHERE w = '00000000-0000-0000-0000-000000000001' AND k = 'a'), '2000-01-01T00:00:00Z')
            )
        ORDER BY
            a ASC
    )
)
SELECT
    f.a,
    f.e,
    SUM(f.n) AS s
FROM
    fact_f f
    INNER JOIN c ON ((f.w = c.w) AND (f.j ->> 'a' = c.a))
WHERE
    (
        f.t >= (c.cap - interval '6 months') AND f.t < c.cap
    )
GROUP BY
    f.a,
    f.e
