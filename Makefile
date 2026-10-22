# Local development entry points. See CLAUDE.md "Commands".
#
# Targets appear phase by phase (docs/build-order.md): migrate-up and sqlc
# arrive in 0c; the frontend joins dev, build and lint in 0e.

# Local settings and secrets. Optional; see .env.example for the names.
# `export` with no arguments passes every variable to the commands below.
-include .env
export

# Which development database: DB=local (the container, default) or DB=dev
# (the madam-dev Supabase project). There is deliberately no option for
# production. The app itself only ever reads DATABASE_URL.
DB ?= local
LOCAL_DATABASE_URL := postgres://madam:madam@127.0.0.1:5432/madam?sslmode=disable

ifeq ($(DB),local)
  DATABASE_URL := $(LOCAL_DATABASE_URL)
else ifeq ($(DB),dev)
  ifeq ($(MADAM_DEV_DATABASE_URL),)
    $(error DB=dev needs MADAM_DEV_DATABASE_URL in .env)
  endif
  DATABASE_URL := $(MADAM_DEV_DATABASE_URL)
else
  $(error DB must be "local" or "dev", got "$(DB)")
endif

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

.PHONY: dev test lint build db-up db-down db-reset image docker-run deploy

ifeq ($(DB),local)
dev: db-up
endif
dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test -race ./...

lint: $(GOLANGCI_LINT)
	cd backend && $(GOLANGCI_LINT) run

build:
	cd backend && go build -o ../bin/server ./cmd/server

# The production image, built and run locally for testing.
image:
	docker build -t madam-server:dev .

docker-run: image
	docker run --rm -p 8080:8080 madam-server:dev

# Builds, pushes and deploys the committed code. Refuses uncommitted or
# untracked files, so the image tag (the commit hash) names exactly what runs.
deploy:
	@test -z "$$(git status --porcelain)" || { echo "deploy: commit or stash your changes first" >&2; exit 1; }
	docker build -t $(IMAGE):$(TAG) .
	docker push $(IMAGE):$(TAG)
	gcloud run deploy madam --project=$(GCP_PROJECT) --region=$(GCP_REGION) \
	  --image=$(IMAGE):$(TAG) \
	  --service-account=$(RUN_SERVICE_ACCOUNT) \
	  --allow-unauthenticated \
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

# The version is in the file name, so bumping GOLANGCI_LINT_VERSION installs
# the new binary on the next `make lint`.
$(GOLANGCI_LINT):
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(CURDIR)/bin/tmp $(GOLANGCI_LINT_VERSION)
	mv $(CURDIR)/bin/tmp/golangci-lint $@
	rmdir $(CURDIR)/bin/tmp
