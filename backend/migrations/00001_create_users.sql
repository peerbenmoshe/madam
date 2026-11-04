-- The users table only. Row Level Security and its policy arrive in 00002
-- (Phase 0d). Columns and reasoning: docs/data-model.md §4.1.

-- +goose Up
CREATE TYPE temperature_unit AS ENUM ('celsius', 'fahrenheit');

-- id is the UUID that Supabase Auth issues. There is deliberately no foreign
-- key to auth.users, which doesn't exist in the local container
-- (docs/data-model.md §8.1).
CREATE TABLE users (
    id               UUID             PRIMARY KEY,
    temperature_unit temperature_unit NOT NULL DEFAULT 'celsius',
    onboarded_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ      NOT NULL DEFAULT now()
);

-- For local use only. Nothing runs down migrations against production.
-- +goose Down
DROP TABLE users;
DROP TYPE temperature_unit;
