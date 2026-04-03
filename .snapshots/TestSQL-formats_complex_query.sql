SELECT
    u.id,
    u.name,
    o.order_id,
    o.total
FROM
    users u
    JOIN orders o ON u.id = o.user_id
WHERE
    u.active = TRUE
    AND o.total > 100
ORDER BY
    o.created_at DESC
LIMIT 10
