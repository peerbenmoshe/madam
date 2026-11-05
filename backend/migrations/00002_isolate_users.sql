-- Isolation for users: the application role, Row Level Security, and the
-- revocation of Supabase's API-role grants. Reasoning: docs/data-model.md §9.
--
-- Tables are written public.users on purpose: on Supabase, auth.users exists
-- too, and a policy or grant on the wrong one would be a silent mistake.

-- +goose Up

-- The role the app connects as. It owns nothing, so table ownership can't
-- exempt it from RLS, and NOBYPASSRLS means nothing else can either. Every
-- attribute is spelled out for review, even where it is the default.
--
-- No password here: the repository is public. It is set out of band, with
-- psql's \password on Supabase and by the Makefile locally and in CI
-- (docs/tech-stack.md §5, Database roles). Without one it cannot log in.
CREATE ROLE madam_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

GRANT USAGE ON SCHEMA public TO madam_app;

-- The four statements the app needs on users, and nothing else: TRUNCATE
-- ignores RLS, and REFERENCES and TRIGGER aren't needed. Each new table
-- grants explicitly in its own migration; there are deliberately no default
-- privileges for madam_app, so a forgotten grant fails loudly.
GRANT SELECT, INSERT, UPDATE, DELETE ON public.users TO madam_app;

-- FORCE also subjects the table's owner to the policies. It doesn't reach a
-- superuser or a BYPASSRLS role, which skip RLS always: on Supabase the owner
-- is postgres, with BYPASSRLS, and locally it's the superuser madam. That is
-- why the app needs madam_app; FORCE is the second lock, for any owner
-- without either.
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.users FORCE ROW LEVEL SECURITY;

-- A row is visible and writable only when its id is the request's user id,
-- which the transaction wrapper sets with set_config('madam.user_id', ..., true).
--   USING decides which existing rows a statement can see, update or delete.
--   WITH CHECK decides which rows it may write, so no one can insert or
--   update a row into someone else's id.
-- current_setting(..., true) is NULL when the setting was never made on this
-- connection, and '' once it was made in an earlier, finished transaction.
-- NULLIF turns '' into NULL too, so both give zero rows instead of an error.
-- The (SELECT ...) makes Postgres evaluate it once per statement, not per row.
CREATE POLICY users_own_row ON public.users
    FOR ALL
    TO madam_app
    USING (id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid))
    WITH CHECK (id = (SELECT NULLIF(current_setting('madam.user_id', true), '')::uuid));

-- Supabase's API roles (anon, authenticated, service_role) are granted every
-- privilege on new objects in public, TRUNCATE included, through default
-- privileges (measured on madam-dev in 0d). They're dormant while the Data
-- API is off; this makes sure switching it on can't expose a table. Auth
-- doesn't use these roles (it connects as supabase_auth_admin).
--
-- The roles exist only on Supabase, so locally and in CI this does nothing.
-- If only some of them exist, something is wrong: stop.
-- +goose StatementBegin
DO $$
DECLARE
    found int;
BEGIN
    SELECT count(*) INTO found FROM pg_roles
    WHERE rolname IN ('anon', 'authenticated', 'service_role');

    IF found = 0 THEN
        RETURN;
    ELSIF found <> 3 THEN
        RAISE EXCEPTION 'expected all or none of anon, authenticated, service_role; found %', found;
    END IF;

    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM anon, authenticated, service_role;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated, service_role;
    REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM anon, authenticated, service_role;

    ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
        REVOKE ALL ON TABLES FROM anon, authenticated, service_role;
    ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
        REVOKE ALL ON SEQUENCES FROM anon, authenticated, service_role;
    ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
        REVOKE ALL ON FUNCTIONS FROM anon, authenticated, service_role;
END
$$;
-- +goose StatementEnd

-- For local use only. Nothing runs down migrations against production. It
-- doesn't restore Supabase's grants: those roles don't exist locally.
-- +goose Down
DROP POLICY users_own_row ON public.users;
ALTER TABLE public.users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE public.users DISABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.users FROM madam_app;
REVOKE USAGE ON SCHEMA public FROM madam_app;
DROP ROLE madam_app;
