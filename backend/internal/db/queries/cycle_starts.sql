-- name: CreateCycleStart :exec
-- CreateCycleStart marks start_date as day 1 of a cycle for the user. Run it
-- through Store.InUserTx: the policy's WITH CHECK refuses any user_id but
-- the request's.
INSERT INTO cycle_starts (user_id, start_date)
VALUES (sqlc.arg(user_id), sqlc.arg(start_date));

-- name: ListCycleStarts :many
-- ListCycleStarts returns the user's cycle start dates, oldest first. Run it
-- through Store.InUserTx: Row Level Security hides every other user's rows
-- even if the user_id asked for is someone else's.
SELECT start_date
FROM cycle_starts
WHERE user_id = sqlc.arg(user_id)
ORDER BY start_date;
