#!/bin/sh
# Checks that a running server routes, caches and protects as designed
# (docs/tech-stack.md §8.4): the page and its deep links, the API, and that
# nothing under /api/ ever falls through to the page. It changes nothing: GET
# requests, and one unsigned POST that the sign-up hook must refuse. CI runs
# it against the image; run it against production by hand.
#
#   sh deploy/smoke-test.sh http://127.0.0.1:8080
#   sh deploy/smoke-test.sh https://madam-966460417479.europe-west1.run.app
#
# Prints one line per check, and exits non-zero if any failed.
set -eu

base=${1:?usage: smoke-test.sh BASE_URL}
base=${base%/}
failures=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# fetch PATH [curl options]: the body goes to $tmp/body and the headers to
# $tmp/headers; prints the status code (000 if there was no answer).
fetch() {
	path=$1
	shift
	curl -sS --max-time 15 -o "$tmp/body" -D "$tmp/headers" -w '%{http_code}' "$@" "$base$path" || true
}

# header NAME: the value of a response header from the last fetch.
header() {
	grep -i "^$1:" "$tmp/headers" | head -n 1 | cut -d: -f2- | tr -d '\r' | sed 's/^ *//'
}

pass() { echo "ok    $1"; }
fail() {
	echo "FAIL  $1" >&2
	failures=$((failures + 1))
}

is_page() { grep -q 'id="root"' "$tmp/body"; }

# The app's page, served for / and for any client-side route.
expect_page() {
	status=$(fetch "$1")
	if [ "$status" = 200 ] && is_page && header Content-Type | grep -q '^text/html' &&
		[ "$(header Cache-Control)" = no-cache ]; then
		pass "$1: the page, no-cache"
	else
		fail "$1: want 200 text/html page with no-cache; got $status, $(header Content-Type), $(header Cache-Control)"
	fi
}

# A 404 that is not the page.
expect_404() {
	status=$(fetch "$1")
	if [ "$status" = 404 ] && ! is_page; then
		pass "$1: 404"
	else
		fail "$1: want 404 without the page; got $status"
	fi
}

expect_page /
expect_page /day/2026-11-06

# The page's own script, found in the page.
fetch / >/dev/null
script=$(grep -o '/assets/[^"]*\.js' "$tmp/body" | head -n 1 || true)
if [ -z "$script" ]; then
	fail "no /assets/*.js in the page"
else
	status=$(fetch "$script")
	if [ "$status" = 200 ] && header Content-Type | grep -q '^text/javascript' &&
		header Cache-Control | grep -q immutable; then
		pass "$script: script, immutable"
	else
		fail "$script: want 200 text/javascript, immutable; got $status, $(header Content-Type), $(header Cache-Control)"
	fi
	fetch "$script" -H 'Accept-Encoding: gzip' >/dev/null
	if [ "$(header Content-Encoding)" = gzip ]; then
		pass "$script: gzip"
	else
		fail "$script: want Content-Encoding gzip; got '$(header Content-Encoding)'"
	fi
fi

status=$(fetch /api/health)
if [ "$status" = 200 ] && [ "$(cat "$tmp/body")" = ok ] && [ "$(header Cache-Control)" = no-store ]; then
	pass "/api/health: ok, no-store"
else
	fail "/api/health: want 200 ok with no-store; got $status '$(head -c 80 "$tmp/body")', $(header Cache-Control)"
fi

# User data is behind a token: without one, 401, and never cached.
status=$(fetch /api/me)
if [ "$status" = 401 ] && ! is_page && [ "$(header Cache-Control)" = no-store ]; then
	pass "/api/me without a token: 401, no-store"
else
	fail "/api/me without a token: want 401 with no-store; got $status, $(header Cache-Control)"
fi

# The sign-up hook answers only requests signed by Supabase. Without a
# signature it's 401, whatever the body says; a GET is 405.
hook=/api/hooks/before-user-created
status=$(fetch "$hook" -X POST -H 'Content-Type: application/json' \
	--data '{"user":{"user_metadata":{"invite_code":"smoke-test"}}}')
if [ "$status" = 401 ] && ! is_page && [ "$(header Cache-Control)" = no-store ]; then
	pass "$hook unsigned: 401, no-store"
else
	fail "$hook unsigned: want 401 with no-store; got $status '$(head -c 80 "$tmp/body")'"
fi
status=$(fetch "$hook")
if [ "$status" = 405 ] && ! is_page; then
	pass "$hook GET: 405"
else
	fail "$hook GET: want 405; got $status"
fi

# Nothing under /api/ may reach the page.
for path in /api /api/ /api/nope /api/health/extra /api/me/extra; do
	expect_404 "$path"
done

# Missing files are 404, not the page: an old tab asking for a deleted chunk
# must get an error, not HTML labelled as script.
for path in /assets/index-missing.js /favicon.ico /.gitkeep; do
	expect_404 "$path"
done

# Security headers, on the page and on the API.
for path in / /api/health; do
	fetch "$path" >/dev/null
	if header Content-Security-Policy | grep -q "default-src 'self'" &&
		[ "$(header X-Content-Type-Options)" = nosniff ]; then
		pass "$path: CSP and nosniff"
	else
		fail "$path: missing CSP or nosniff"
	fi
done

if [ "$failures" -gt 0 ]; then
	echo "$failures check(s) failed" >&2
	exit 1
fi
echo "all checks passed"
