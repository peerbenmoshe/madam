-- name: GetEntry :one
-- GetEntry reads the user's entry for one date; no row means the day has no
-- entry. The columns are named, never *, so a later column can't change
-- what a prepared statement returns (docs/tech-stack.md §6). Run it through
-- Store.InUserTx: Row Level Security hides every other user's rows even if
-- the user_id asked for is someone else's.
SELECT
    user_id,
    entry_date,
    temperature_milli_c,
    measured_at,
    bleeding,
    secretion_category,
    sensation,
    secretion_amount,
    secretion_description,
    cervix_firmness,
    cervix_position,
    disturbances,
    symptoms,
    intercourse,
    notes,
    created_at,
    updated_at
FROM entries
WHERE user_id = sqlc.arg(user_id)
  AND entry_date = sqlc.arg(entry_date);

-- name: UpsertEntry :one
-- UpsertEntry saves the user's entry for a date, whether or not one exists:
-- the primary key (user_id, entry_date) allows one per date, so a second
-- save replaces the first (AC-4; docs/fam-rules.md §12.7, I2). It replaces
-- every column a person can fill in. created_at keeps the first save's
-- time, and updated_at is set by the table's trigger, which is why neither
-- is named here.
--
-- The row it changes is always the user's own: the conflict is on
-- (user_id, entry_date), so only a row with this user_id can conflict. Run
-- it through Store.InUserTx: the policy's WITH CHECK refuses any user_id
-- but the request's.
INSERT INTO entries (
    user_id,
    entry_date,
    temperature_milli_c,
    measured_at,
    bleeding,
    secretion_category,
    sensation,
    secretion_amount,
    secretion_description,
    cervix_firmness,
    cervix_position,
    disturbances,
    symptoms,
    intercourse,
    notes
)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(entry_date),
    sqlc.arg(temperature_milli_c),
    sqlc.arg(measured_at),
    sqlc.arg(bleeding),
    sqlc.arg(secretion_category),
    sqlc.arg(sensation),
    sqlc.arg(secretion_amount),
    sqlc.arg(secretion_description),
    sqlc.arg(cervix_firmness),
    sqlc.arg(cervix_position),
    sqlc.arg(disturbances),
    sqlc.arg(symptoms),
    sqlc.arg(intercourse),
    sqlc.arg(notes)
)
ON CONFLICT (user_id, entry_date) DO UPDATE
SET temperature_milli_c   = EXCLUDED.temperature_milli_c,
    measured_at           = EXCLUDED.measured_at,
    bleeding              = EXCLUDED.bleeding,
    secretion_category    = EXCLUDED.secretion_category,
    sensation             = EXCLUDED.sensation,
    secretion_amount      = EXCLUDED.secretion_amount,
    secretion_description = EXCLUDED.secretion_description,
    cervix_firmness       = EXCLUDED.cervix_firmness,
    cervix_position       = EXCLUDED.cervix_position,
    disturbances          = EXCLUDED.disturbances,
    symptoms              = EXCLUDED.symptoms,
    intercourse           = EXCLUDED.intercourse,
    notes                 = EXCLUDED.notes
RETURNING
    user_id,
    entry_date,
    temperature_milli_c,
    measured_at,
    bleeding,
    secretion_category,
    sensation,
    secretion_amount,
    secretion_description,
    cervix_firmness,
    cervix_position,
    disturbances,
    symptoms,
    intercourse,
    notes,
    created_at,
    updated_at;

-- name: DeleteEntry :exec
-- DeleteEntry removes the user's entry for one date. Deleting a date that
-- has none changes nothing and is not an error. Run it through
-- Store.InUserTx: Row Level Security lets a request delete no row but its
-- own.
DELETE FROM entries
WHERE user_id = sqlc.arg(user_id)
  AND entry_date = sqlc.arg(entry_date);
