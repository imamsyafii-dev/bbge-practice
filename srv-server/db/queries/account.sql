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

-- name: CreateAccount :one
INSERT INTO account (
    user_id,
    name
) VALUES (
    $1,
    $2
)
RETURNING
    id,
    user_id,
    name,
    status,
    version,
    created_at,
    updated_at;
