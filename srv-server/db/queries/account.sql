-- name: GetAccountByUserID :one
SELECT
    id,
    user_id,
    name,
    status,
    version,
    created_at,
    updated_at
FROM account
WHERE user_id = $1
LIMIT 1;
