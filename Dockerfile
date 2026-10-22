# The production image. Built from the repository root (`make image`,
# `make deploy`); .dockerignore limits what the build can see.

# Build stage: compile a static binary. Pinned to the exact Go patch release,
# like the rest of the toolchain; Dependabot proposes updates.
FROM golang:1.27.3-trixie AS build
WORKDIR /src

# Dependencies first, so this layer stays cached until go.mod or go.sum changes.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# CGO_ENABLED=0: fully static, so the runtime image needs no C library.
# -trimpath: no laptop paths in the binary. -s -w: no debug symbol tables
# (stack traces still work).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage: no shell and no package manager. Includes CA certificates
# (TLS to Supabase from 0c), timezone data, and the non-root user 65532 that
# the :nonroot tag runs as.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/server /server
ENTRYPOINT ["/server"]
