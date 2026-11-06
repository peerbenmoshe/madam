# The production image. Built from the repository root (`make image`,
# `make deploy`); .dockerignore limits what the build can see.

# Frontend stage: type-check and build the React app into /src/dist. Exactly
# the Node in .nvmrc (CI fails if the two differ); Debian, like the Go stage,
# because the lockfile's native packages are the glibc ones. Nothing from this
# stage reaches the runtime image except the built files.
FROM node:26.13.0-trixie-slim AS frontend
WORKDIR /src

# Dependencies first, cached until the lockfile changes. .npmrc turns off
# install scripts.
COPY frontend/package.json frontend/package-lock.json frontend/.npmrc ./
RUN npm ci

COPY frontend/ ./
RUN npm run typecheck && npm run build

# Build stage: compile a static binary. Pinned to the exact Go patch release,
# like the rest of the toolchain; Dependabot proposes updates.
FROM golang:1.27.3-trixie AS build
WORKDIR /src

# Dependencies first, so this layer stays cached until go.mod or go.sum changes.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# The built frontend, where go:embed expects it (internal/web).
COPY --from=frontend /src/dist/ ./internal/web/assets/
# CGO_ENABLED=0: fully static, so the runtime image needs no C library.
# -trimpath: no laptop paths in the binary. -s -w: no debug symbol tables
# (stack traces still work).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage: no shell and no package manager. Includes the public CA
# certificates, timezone data, and the non-root user 65532 that the :nonroot
# tag runs as.
FROM gcr.io/distroless/static-debian13:nonroot

# Supabase's own root CA, which the public set doesn't include. pgx checks the
# database's certificate against it when the URL says sslmode=verify-full.
COPY backend/certs/supabase-root-2021-ca.crt /etc/ssl/madam/supabase-root-2021-ca.crt
ENV PGSSLROOTCERT=/etc/ssl/madam/supabase-root-2021-ca.crt

COPY --from=build /out/server /server
ENTRYPOINT ["/server"]
