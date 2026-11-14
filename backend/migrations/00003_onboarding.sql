-- What onboarding writes (Slice 3): the stored answer on users, and the two
-- tables that hold a cycle's start and the fixed measurement time. Columns
-- and reasoning: docs/data-model.md §4.1, §4.3 and §4.4; isolation: §9.
--
-- One file on purpose: goose runs it in one transaction, so a database ends
-- with all three changes or none (docs/tech-stack.md §6).
--
-- Safe with the code already running, which matters because this reaches
-- production before the code that uses it: that code names the columns it
-- reads and writes, and never mentions the new tables.

-- +goose Up

-- The onboarding answer "I don't know" (docs/fam-rules.md §5.2). It means
-- nothing while onboarded_at is NULL. madam_app's UPDATE on users, granted
-- in 00002, already covers a new column.
ALTER TABLE public.users
    ADD COLUMN cycle_start_unknown BOOLEAN NOT NULL DEFAULT false;

-- Day 1 of each cycle. Its own table, not a flag on an entry, so a cycle can
-- start on a day that has no entry.
CREATE TABLE public.cycle_starts (
    user_id    UUID        NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    start_date DATE        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, start_date)
);

-- A start is inserted or deleted, never edited: no UPDATE. Never TRUNCATE,
-- which ignores Row Level Security. There are no default privileges for
-- madam_app, so these three are everything it may do here.
GRANT SELECT, INSERT, DELETE ON public.cycle_starts TO madam_app;

ALTER TABLE public.cycle_starts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cycle_starts FORCE ROW LEVEL SECURITY;

-- The same rule as users_own_row in 00002, on user_id: a row is visible and
-- writable only to the user the transaction wrapper named. USING covers the
-- rows a statement can see or delete; WITH CHECK the rows it may insert.
CREATE POLICY cycle_starts_own_rows ON public.cycle_starts
    FOR ALL
    TO madam_app
    USING (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid))
    WITH CHECK (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid));

-- The fixed measurement time, and from which date each one applies. There is
-- no end date: a row runs until the day before the next row's valid_from.
-- Onboarding writes the first row with valid_from = '-infinity', so every
-- date has a time in effect (docs/fam-rules.md §4).
--
-- The CHECK: a fixed time is a minute of the day, 00:00 to 23:59. A Postgres
-- TIME could also hold 24:00:00, seconds and microseconds.
CREATE TABLE public.measurement_time_periods (
    user_id          UUID        NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    valid_from       DATE        NOT NULL,
    measurement_time TIME        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, valid_from),
    CONSTRAINT measurement_time_is_a_minute_of_the_day CHECK (
        measurement_time < TIME '24:00'
        AND EXTRACT(SECOND FROM measurement_time) = 0
    )
);

-- A period is only ever added: no UPDATE and no DELETE. Deleting the user
-- still removes her rows: a foreign key's cascade doesn't run with the
-- caller's privileges.
GRANT SELECT, INSERT ON public.measurement_time_periods TO madam_app;

ALTER TABLE public.measurement_time_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.measurement_time_periods FORCE ROW LEVEL SECURITY;

CREATE POLICY measurement_time_periods_own_rows ON public.measurement_time_periods
    FOR ALL
    TO madam_app
    USING (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid))
    WITH CHECK (user_id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid));

-- For local use only. Nothing runs down migrations against production.
-- Dropping a table drops its policy and its grants with it.
-- +goose Down
DROP TABLE public.measurement_time_periods;
DROP TABLE public.cycle_starts;
ALTER TABLE public.users DROP COLUMN cycle_start_unknown;
