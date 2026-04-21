WITH r AS (
    SELECT
        1 AS c0,
        2 AS c1
)
SELECT
    r.c0,
    (
        SELECT
            COUNT(*)
        FROM
            r u
        WHERE
            u.c0 = r.c0) AS cnt,
    COALESCE((SELECT c1 FROM r u2 WHERE u2.c0 = r.c0 LIMIT 1), 0) AS val
FROM
    r
