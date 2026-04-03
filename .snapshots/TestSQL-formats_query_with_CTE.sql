WITH active_users AS (
    SELECT
        id,
        name
    FROM
        users
    WHERE
        active = TRUE
)
SELECT
    *
FROM
    active_users
WHERE
    name LIKE 'A%'
