# Local development entry points. See CLAUDE.md "Commands".
#
# Targets appear phase by phase (docs/build-order.md); the frontend joins dev,
# build and lint in 0e.
#
# Recipes write $$DATABASE_URL (expanded by the shell), never $(DATABASE_URL)
# (expanded by make): make prints each recipe line before running it, so the
# second form would print the password.

# Local settings and secrets. Optional; see .env.example for the names.
# `export` with no arguments passes every variable to the commands below.
-include .env
export

# Which development database: DB=local (the container, default) or DB=dev
# (the madam-dev Supabase project). There is deliberately no option for
# production. The app itself only ever reads DATABASE_URL.
DB ?= local
LOCAL_DATABASE_URL := postgres://madam:madam@127.0.0.1:5432/madam?sslmode=disable

# Supabase signs its certificates with its own root CA, which isn't in the
# system trust store. With sslmode=verify-full in a Supabase URL, pgx (in the
# app and in goose) and psql check the server against the file named by
# PGSSLROOTCERT. It's set only for Supabase: pgx reads that file even when
# sslmode=disable, so the local container shouldn't depend on it.
SUPABASE_CA := $(CURDIR)/backend/certs/supabase-root-2021-ca.crt

ifeq ($(DB),local)
  DATABASE_URL := $(LOCAL_DATABASE_URL)
else ifeq ($(DB),dev)
  ifeq ($(MADAM_DEV_DATABASE_URL),)
    $(error DB=dev needs MADAM_DEV_DATABASE_URL in .env)
  endif
  DATABASE_URL := $(MADAM_DEV_DATABASE_URL)
  PGSSLROOTCERT := $(SUPABASE_CA)
else
  $(error DB must be "local" or "dev", got "$(DB)")
endif

# goose and sqlc are pinned in tools/go.mod and run with `go tool`. `-C`
# (which must be go's first flag) runs go inside tools/, hence absolute paths.
GOOSE := go -C $(CURDIR)/tools tool goose -dir $(CURDIR)/backend/migrations
SQLC := go -C $(CURDIR)/tools tool sqlc -f $(CURDIR)/backend/sqlc.yaml
export GOOSE_DRIVER := postgres

# golangci-lint is pinned as a binary, the way its docs recommend (not with
# `go tool`). Keep this version in sync with .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT := $(CURDIR)/bin/golangci-lint-$(GOLANGCI_LINT_VERSION)

# Cloud Run deployment. Why each setting: docs/tech-stack.md §7.
GCP_PROJECT := madam-512309
GCP_REGION := europe-west1
IMAGE := $(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/madam/server
RUN_SERVICE_ACCOUNT := madam-run@$(GCP_PROJECT).iam.gserviceaccount.com
# `=`, not `:=`: git runs only when a target uses TAG.
TAG = $(shell git rev-parse --short HEAD)

# Secret Manager secrets holding production connection URLs. The app's is
# pinned to a version, so changing it is a commit (docs/tech-stack.md §7).
DATABASE_URL_SECRET := madam-database-url:2
MIGRATE_URL_SECRET := madam-migrate-url

.PHONY: dev test test-integration lint build db-up db-down db-reset image \
  docker-run deploy sqlc migrate-up migrate-status migrate-new migrate-prod psql

ifeq ($(DB),local)
dev: db-up
endif
dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test -race ./...

# Also runs the tests that need a database, against the local container only.
# They skip themselves when MADAM_TEST_DATABASE_URL is unset (plain `make test`).
# -count=1 because Go's test cache can't see the database.
test-integration:
	$(MAKE) db-up migrate-up DB=local
	cd backend && MADAM_TEST_DATABASE_URL="$(LOCAL_DATABASE_URL)" go test -race -count=1 ./...

lint: $(GOLANGCI_LINT)
	cd backend && $(GOLANGCI_LINT) run

build:
	cd backend && go build -o ../bin/server ./cmd/server

# The production image, built and run locally for testing.
image:
	docker build -t madam-server:dev .

# With DB=local the container joins the Compose network and reaches Postgres
# by its service name. With DB=dev, `-e DATABASE_URL` copies the value from
# the environment, so it never appears on the command line.
ifeq ($(DB),local)
docker-run: image db-up
	docker run --rm -p 8080:8080 --network madam_default \
	  -e DATABASE_URL="postgres://madam:madam@postgres:5432/madam?sslmode=disable" madam-server:dev
else
docker-run: image
	docker run --rm -p 8080:8080 -e DATABASE_URL madam-server:dev
endif

# Builds, pushes and deploys the latest commit on main. CI runs this on every
# merge (.github/workflows/ci.yml); from the laptop it's the fallback for when
# Actions doesn't run (docs/tech-stack.md §7). It refuses uncommitted or
# untracked files and anything but the latest main, so the image tag (the
# commit hash) names exactly what runs, and that is what main holds.
#
# Public access is deliberately not a flag: it's the allUsers -> run.invoker
# binding on the service, set once. --allow-unauthenticated would re-set that
# policy on every deploy, which CI's deployer isn't allowed to do.
deploy:
	@test -z "$$(git status --porcelain)" || { echo "deploy: commit or stash your changes first" >&2; exit 1; }
	@git fetch --quiet origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse FETCH_HEAD)" || { \
	  echo "deploy: only the latest commit on main is deployed (HEAD $$(git rev-parse --short HEAD), main $$(git rev-parse --short FETCH_HEAD))" >&2; exit 1; }
	docker build -t $(IMAGE):$(TAG) .
	docker push $(IMAGE):$(TAG)
	gcloud run deploy madam --project=$(GCP_PROJECT) --region=$(GCP_REGION) \
	  --image=$(IMAGE):$(TAG) \
	  --service-account=$(RUN_SERVICE_ACCOUNT) \
	  --set-secrets=DATABASE_URL=$(DATABASE_URL_SECRET) \
	  --min-instances=0 --max-instances=2 \
	  --cpu=1 --memory=512Mi --concurrency=80 --timeout=30s \
	  --no-cpu-boost

# Starts the local Postgres and returns only once its healthcheck
# (pg_isready, in compose.yaml) passes. Never a fixed sleep.
db-up:
	docker compose up --detach --wait --wait-timeout 60

# Stops the container; the data volume is kept.
db-down:
	docker compose down

# Stops the container and deletes its data volume.
db-reset:
	docker compose down --volumes

# An interactive psql session on the selected database. Also reads SQL from
# stdin: `echo 'SELECT 1' | make psql DB=dev`.
psql:
	@psql "$$DATABASE_URL"

# Regenerates backend/internal/db from the migrations and the .sql queries.
# Run after touching either; never edit the generated files.
sqlc:
	$(SQLC) generate

migrate-up:
	@GOOSE_DBSTRING="$$DATABASE_URL" $(GOOSE) up

migrate-status:
	@GOOSE_DBSTRING="$$DATABASE_URL" $(GOOSE) status

# New numbered migration: make migrate-new NAME=create_entries
# -s numbers sequentially (00002) instead of by timestamp.
migrate-new:
	@test -n "$(NAME)" || { echo "usage: make migrate-new NAME=what_it_does" >&2; exit 1; }
	$(GOOSE) -s create $(NAME) sql

# Production migrations, from the laptop. Deliberately not a DB= value: it
# reads the owner URL from Secret Manager with your own gcloud login, keeps it
# in this one shell's memory, shows the status and asks before changing
# anything. Run it before `make deploy` when a deploy needs a new migration.
migrate-prod:
	@export PGSSLROOTCERT="$(SUPABASE_CA)"; \
	url="$$(gcloud secrets versions access latest --secret=$(MIGRATE_URL_SECRET) --project=$(GCP_PROJECT))" || exit 1; \
	GOOSE_DBSTRING="$$url" $(GOOSE) status || exit 1; \
	printf 'Apply pending migrations to PRODUCTION? Type madam-prod to continue: '; \
	read -r answer; \
	test "$$answer" = madam-prod || { echo "migrate-prod: cancelled" >&2; exit 1; }; \
	GOOSE_DBSTRING="$$url" $(GOOSE) up

# The version is in the file name, so bumping GOLANGCI_LINT_VERSION installs
# the new binary on the next `make lint`.
$(GOLANGCI_LINT):
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(CURDIR)/bin/tmp $(GOLANGCI_LINT_VERSION)
	mv $(CURDIR)/bin/tmp/golangci-lint $@
	rmdir $(CURDIR)/bin/tmp
