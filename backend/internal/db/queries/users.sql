-- name: CreateUser :exec
-- CreateUser adds the request's own users row. Run it through
-- Store.InUserTx: the policy's WITH CHECK refuses any id but the request's.
INSERT INTO users (id) VALUES (sqlc.arg(id));

-- name: GetUser :one
-- GetUser reads one users row by id, which is this table's user id, so the
-- WHERE is its "filter by user_id" (CLAUDE.md). Run it through
-- Store.InUserTx: Row Level Security then hides every other user's row even
-- if the id asked for is someone else's.
SELECT id, temperature_unit, onboarded_at, created_at
FROM users
WHERE id = sqlc.arg(id);
