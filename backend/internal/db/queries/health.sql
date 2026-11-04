-- name: Ping :one
-- Ping is the health check's database round trip. It touches no table on
-- purpose, so it needs no user id and keeps working once Row Level Security
-- is forced on every table (Phase 0d).
SELECT 1::integer AS ok;
