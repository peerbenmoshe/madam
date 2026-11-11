#!/bin/sh
# SPIKE (spike/invite-hook, never merged): calls madam-dev's Supabase Auth the
# way supabase-js does, to measure the "before user created" hook.
# Run through `make spike-auth ACTION=… EMAIL=… CODE=… PASSWORD=…`, which
# exports MADAM_DEV_SUPABASE_URL and MADAM_DEV_PUBLISHABLE_KEY from .env.
#
#   signup  POST /auth/v1/signup  {email, password, data: {invite_code}}   (supabase-js signUp)
#   nocode  POST /auth/v1/signup  {email, password, data: {}}              (signUp without options.data)
#   otp     POST /auth/v1/otp     {email, data: {}, create_user: true}     (signInWithOtp's default)
#   anon    POST /auth/v1/signup  {data: {}}                               (signInAnonymously)
#   token   POST /auth/v1/token?grant_type=password, then print the access
#           token's header and claims (never the token itself)
set -eu

: "${MADAM_DEV_SUPABASE_URL:?add MADAM_DEV_SUPABASE_URL to .env}"
: "${MADAM_DEV_PUBLISHABLE_KEY:?add MADAM_DEV_PUBLISHABLE_KEY to .env}"
ACTION=${ACTION:?ACTION=signup|nocode|otp|anon|token}
EMAIL=${EMAIL:-}
CODE=${CODE:-}
PASSWORD=${PASSWORD:-spike-Password-123}

auth=${MADAM_DEV_SUPABASE_URL%/}/auth/v1

# post PATH JSON: prints the HTTP status, the time taken, then the body.
post() {
  curl -sS -w '\n--- HTTP %{http_code} in %{time_total}s\n' \
    -X POST "$auth$1" \
    -H "apikey: $MADAM_DEV_PUBLISHABLE_KEY" \
    -H 'Content-Type: application/json' \
    --data "$2"
}

# Base64url segment of a JWT to text (adds the padding basenc needs).
b64url() {
  s=$1
  case $(( ${#s} % 4 )) in 2) s="$s==" ;; 3) s="$s=" ;; esac
  printf %s "$s" | basenc --base64url -d
}

case $ACTION in
  signup) post /signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"data\":{\"invite_code\":\"$CODE\"}}" ;;
  nocode) post /signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"data\":{}}" ;;
  otp)    post /otp "{\"email\":\"$EMAIL\",\"data\":{},\"create_user\":true}" ;;
  anon)   post /signup '{"data":{}}' ;;
  token)
    out=$(post '/token?grant_type=password' "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
    jwt=$(printf %s "$out" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
    if [ -z "$jwt" ]; then printf '%s\n' "$out"; exit 1; fi
    echo "header: $(b64url "$(printf %s "$jwt" | cut -d. -f1)")"
    echo "claims: $(b64url "$(printf %s "$jwt" | cut -d. -f2)")"
    ;;
  *) echo "unknown ACTION=$ACTION" >&2; exit 2 ;;
esac
