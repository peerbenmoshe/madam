-- name: SetUserID :exec
-- SetUserID tells Row Level Security whose request this is, for the rest of
-- the current transaction only: set_config's third argument (is_local) makes
-- the setting end at COMMIT or ROLLBACK, so it can't leak into the next
-- request on the same pooled connection. It's a function, not SET LOCAL,
-- because SET can't take a parameter: the id travels as a value, never as SQL
-- text. Call it only through Store.InUserTx. It touches no table.
SELECT set_config('madam.user_id', sqlc.arg(user_id)::uuid::text, true);

-- name: CurrentRole :one
-- CurrentRole reports which role this connection runs as and whether that
-- role skips Row Level Security. The isolation test refuses to run as one
-- that does. It reads only the role catalog, no table of ours.
SELECT
    rolname::text AS role_name,
    rolsuper AS is_superuser,
    rolbypassrls AS bypasses_rls
FROM pg_catalog.pg_roles
WHERE rolname = current_user;
