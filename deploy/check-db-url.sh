#!/bin/sh
# Refuses a database URL that wouldn't verify the server's certificate.
# Used by `make migrate-prod` before it connects to production.
#
# The URL comes on standard input, so it never appears in a command line, the
# process list or a log; the messages below never print it.
#
# The rule is the server's (backend/cmd/server/db.go): a postgres:// URL with
# sslmode=verify-full (docs/tech-stack.md §5). One addition: sslmode must
# appear exactly once, because with two, which one wins is up to the library.
#
#   printf %s "$url" | sh deploy/check-db-url.sh    # exit 0: acceptable
set -eu

url=$(cat)

fail() {
	echo "check-db-url: $1. It must be a postgres:// URL with sslmode=verify-full, once (docs/tech-stack.md §5)" >&2
	exit 1
}

case "$url" in
postgres://* | postgresql://*) ;;
*) fail "not a postgres:// URL" ;;
esac

# Everything after the first "?": the parameters.
query=${url#*\?}
[ "$query" != "$url" ] || fail "no parameters, so no sslmode"

# Wrapped in "&" so the parameter must be whole: not "sslmode=verify-fullx".
case "&$query&" in
*"&sslmode=verify-full&"*) ;;
*) fail "no sslmode=verify-full" ;;
esac

case "${query#*sslmode=}" in
*sslmode=*) fail "sslmode appears more than once" ;;
esac
