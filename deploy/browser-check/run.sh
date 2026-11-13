#!/bin/sh
# Runs both browser checks against a running server, from WSL, using the
# Chrome and Node installed on the Windows side. See README.md.
#
#   sh deploy/browser-check/run.sh                       # http://localhost:8080
#   sh deploy/browser-check/run.sh https://example.app   # another server
#
# Screenshots end up in deploy/browser-check/screenshots/ (gitignored).
set -eu

base=${1:-http://localhost:8080}
here=$(cd "$(dirname "$0")" && pwd)

# Chrome's debugging port listens on Windows' own localhost, which WSL can't
# reach, so the scripts must run under Windows' Node, from a Windows folder.
wintemp=$(cmd.exe /c "echo %TEMP%" 2>/dev/null | tr -d '\r')
work="$(wslpath -u "$wintemp")/madam-browser-check"
rm -rf "$work"
mkdir -p "$work"
cp "$here"/*.mjs "$work"/

status=0
for script in auth-screens.mjs auth-first-use.mjs; do
	echo "=== $script"
	# cmd.exe needs a Windows folder as its working directory. Its exit
	# status doesn't survive the pipe, so the last line decides.
	out=$(cd "$work" && cmd.exe /c "node $script . $base" 2>&1 | tr -d '\r')
	printf '%s\n' "$out"
	printf '%s\n' "$out" | grep -q "ALL CHECKS PASSED" || status=1
done

mkdir -p "$here/screenshots"
cp "$work"/*.png "$here/screenshots"/ 2>/dev/null || true
# The scripts' Chrome profiles are left in the same Windows temp folder.
rm -rf "$work" "$(wslpath -u "$wintemp")"/madam-cdp-* 2>/dev/null || true
exit $status
