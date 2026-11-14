-- name: CreateFirstMeasurementTime :exec
-- CreateFirstMeasurementTime writes onboarding's row: the fixed measurement
-- time that applies to every date until a later change, which is what
-- valid_from = '-infinity' means (docs/fam-rules.md §4). The literal is
-- here, in SQL, so no Go code builds an infinite date. The primary key makes
-- a second such row for the user impossible. Run it through Store.InUserTx:
-- the policy's WITH CHECK refuses any user_id but the request's.
INSERT INTO measurement_time_periods (user_id, valid_from, measurement_time)
VALUES (sqlc.arg(user_id), '-infinity', sqlc.arg(measurement_time));

-- name: ListMeasurementTimePeriods :many
-- ListMeasurementTimePeriods returns the user's fixed measurement times,
-- oldest first, so onboarding's '-infinity' row comes first. Run it through
-- Store.InUserTx: Row Level Security hides every other user's rows even if
-- the user_id asked for is someone else's.
SELECT valid_from, measurement_time
FROM measurement_time_periods
WHERE user_id = sqlc.arg(user_id)
ORDER BY valid_from;
