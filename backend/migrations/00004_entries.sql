-- What a day's entry holds (Slice 4): six enum types, the entries table, and
-- the trigger that keeps updated_at. Columns and reasoning:
-- docs/data-model.md §3 and §4.2; isolation: §9.
--
-- One file on purpose: goose runs it in one transaction, so a database ends
-- with all of it or none (docs/tech-stack.md §6).
--
-- Safe with the code already running, which matters because this reaches
-- production before the code that uses it: it changes no existing table, and
-- that code never mentions entries.

-- +goose Up

-- The closed sets (docs/data-model.md §3). An enum makes the database refuse
-- a value outside the set, and sqlc generates a Go constant for each.
CREATE TYPE public.bleeding_level     AS ENUM ('none', 'spotting', 'bleeding');
CREATE TYPE public.secretion_category AS ENUM ('clear_stretchy', 'milky', 'sticky');
CREATE TYPE public.sensation          AS ENUM ('dry', 'creamy', 'wet');
CREATE TYPE public.cervix_firmness    AS ENUM ('firm', 'soft');
CREATE TYPE public.cervix_position    AS ENUM ('high_open', 'low_closed');
CREATE TYPE public.intercourse_record AS ENUM ('none', 'protected', 'unprotected');

-- One row per user per date: the primary key is the "one entry per date"
-- rule (AC-4). Nothing derived is stored: no cycle day, no adjusted value.
--
-- The CHECKs, each named so an error says which rule refused:
--   reading_has_a_temperature_and_a_time  both or neither. A NULL = NULL
--       comparison would be NULL, which a CHECK lets pass, so it compares
--       the two IS NULL results, which are never NULL.
--   temperature_is_possible  20.00 to 50.00 °C, in thousandths. It catches a
--       unit bug in our own code, not a typo: the API refuses first, with a
--       readable message (AC-17).
--   measured_at_is_a_minute_of_the_day  a Postgres TIME could also hold
--       24:00:00, seconds and microseconds. The twin of
--       measurement_time_periods' constraint in 00003.
--   secretion_amount_is_1_to_3
--   disturbances_are_known, symptoms_are_known  <@ is "is contained by":
--       every element must be in the list. They are TEXT[] with a CHECK, not
--       enum arrays, because a CHECK can be changed in one migration and
--       removing an enum value can't.
-- The three on a column that may be empty (temperature, time, amount) let
-- NULL pass, as they must.
CREATE TABLE public.entries (
    user_id               UUID                      NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    entry_date            DATE                      NOT NULL,
    temperature_milli_c   INTEGER,
    measured_at           TIME,
    bleeding              public.bleeding_level     NOT NULL DEFAULT 'none',
    secretion_category    public.secretion_category,
    sensation             public.sensation,
    secretion_amount      SMALLINT,
    secretion_description TEXT,
    cervix_firmness       public.cervix_firmness,
    cervix_position       public.cervix_position,
    disturbances          TEXT[]                    NOT NULL DEFAULT '{}',
    symptoms              TEXT[]                    NOT NULL DEFAULT '{}',
    intercourse           public.intercourse_record,
    notes                 TEXT,
    created_at            TIMESTAMPTZ               NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ               NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, entry_date),
    CONSTRAINT reading_has_a_temperature_and_a_time CHECK (
        (temperature_milli_c IS NULL) = (measured_at IS NULL)
    ),
    CONSTRAINT temperature_is_possible CHECK (
        temperature_milli_c BETWEEN 20000 AND 50000
    ),
    CONSTRAINT measured_at_is_a_minute_of_the_day CHECK (
        measured_at < TIME '24:00'
        AND EXTRACT(SECOND FROM measured_at) = 0
    ),
    CONSTRAINT secretion_amount_is_1_to_3 CHECK (
        secretion_amount BETWEEN 1 AND 3
    ),
    CONSTRAINT disturbances_are_known CHECK (
        disturbances <@ ARRAY['alcohol', 'poor_sleep', 'illness', 'medication',
            'travel', 'vacation', 'new_thermometer', 'uti', 'yeast_infection']::TEXT[]
    ),
    CONSTRAINT symptoms_are_known CHECK (
        symptoms <@ ARRAY['breast_tenderness', 'bloating', 'pms']::TEXT[]
    )
);

-- updated_at is set here, by the database, on every UPDATE of a row. No
-- query names the column, so none can forget it (docs/data-model.md §4.2).
-- That includes the DO UPDATE branch of an INSERT ... ON CONFLICT.
--
-- now() is the time the transaction began. search_path is emptied for the
-- function so that nothing but the built-in now() can ever be found.
-- +goose StatementBegin
CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = ''
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END
$$;
-- +goose StatementEnd

-- A new function can be run by everyone (PUBLIC) by default. Nothing needs
-- to: a trigger function can't be called directly, and firing a trigger
-- doesn't ask for this privilege (see the tests).
REVOKE ALL ON FUNCTION public.set_updated_at() FROM PUBLIC;

CREATE TRIGGER entries_set_updated_at
    BEFORE UPDATE ON public.entries
    FOR EACH ROW
    EXECUTE FUNCTION public.set_updated_at();

-- An entry is written, changed and deleted by its user: all four. Never
-- TRUNCATE, which ignores Row Level Security. There are no default
-- privileges for madam_app, so these four are everything it may do here.
GRANT SELECT, INSERT, UPDATE, DELETE ON public.entries TO madam_app;

ALTER TABLE public.entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.entries FORCE ROW LEVEL SECURITY;

-- The same rule as users_own_row in 00002, on user_id: a row is visible and
-- writable only to the user the transaction wrapper named. USING covers the
-- rows a statement can see, update or delete; WITH CHECK the rows it may
-- write.
CREATE POLICY entries_own_rows ON public.entries
    FOR ALL
    TO madam_app
    USING (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid))
    WITH CHECK (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid));

-- For local use only. Nothing runs down migrations against production.
-- Dropping the table drops its trigger, its policy and its grants with it.
-- +goose Down
DROP TABLE public.entries;
DROP FUNCTION public.set_updated_at();
DROP TYPE public.intercourse_record;
DROP TYPE public.cervix_position;
DROP TYPE public.cervix_firmness;
DROP TYPE public.sensation;
DROP TYPE public.secretion_category;
DROP TYPE public.bleeding_level;
