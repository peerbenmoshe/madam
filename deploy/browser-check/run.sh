#!/bin/sh
# Runs the browser checks against a running server, from WSL, using the
# Chrome and Node installed on the Windows side. See README.md.
#
#   sh deploy/browser-check/run.sh                       # http://localhost:8080
#   sh deploy/browser-check/run.sh https://example.app   # another server
#   make browser-check                                   # also the real pass
#   sh deploy/browser-check/run.sh "" onboarding.mjs     # one script only
#
# Screenshots end up in deploy/browser-check/screenshots/ (gitignored).
set -eu

base=${1:-http://localhost:8080}
[ -n "$base" ] || base=http://localhost:8080
# One script only, by its file name; empty for all of them.
only=${2:-}
here=$(cd "$(dirname "$0")" && pwd)

# Chrome's debugging port listens on Windows' own localhost, which WSL can't
# reach, so the scripts must run under Windows' Node, from a Windows folder.
wintemp=$(cmd.exe /c "echo %TEMP%" 2>/dev/null | tr -d '\r')
work="$(wslpath -u "$wintemp")/madam-browser-check"
rm -rf "$work"
mkdir -p "$work"
cp "$here"/*.mjs "$work"/

status=0
for script in auth-screens.mjs auth-first-use.mjs onboarding.mjs day.mjs; do
	[ -z "$only" ] || [ "$only" = "$script" ] || continue
	echo "=== $script"
	# cmd.exe needs a Windows folder as its working directory. Its exit
	# status doesn't survive the pipe, so the last line decides.
	out=$(cd "$work" && cmd.exe /c "node $script . $base" 2>&1 | tr -d '\r')
	printf '%s\n' "$out"
	printf '%s\n' "$out" | grep -q "ALL CHECKS PASSED" || status=1
done

# The real passes need a test account (README.md). Its email and password
# reach Windows' Node through the environment: WSLENV names the variables
# WSL hands to a Windows program. They are never put on a command line.
# onboarding-real.mjs needs an account that hasn't onboarded, which
# `make browser-check` gives it; day-real.mjs onboards one itself if
# needed, so it can also run alone.
for script in onboarding-real.mjs day-real.mjs; do
	[ -z "$only" ] || [ "$only" = "$script" ] || continue
	if [ -z "${MADAM_CHECK_EMAIL:-}" ] || [ -z "${MADAM_CHECK_PASSWORD:-}" ]; then
		echo "=== $script: skipped (no test account in the environment; see \"The test account\" in README.md)"
		continue
	fi
	echo "=== $script"
	out=$(cd "$work" && WSLENV="MADAM_CHECK_EMAIL:MADAM_CHECK_PASSWORD${WSLENV:+:$WSLENV}" \
		cmd.exe /c "node $script . $base" 2>&1 | tr -d '\r')
	printf '%s\n' "$out"
	printf '%s\n' "$out" | grep -q "ALL CHECKS PASSED" || status=1
done

mkdir -p "$here/screenshots"
cp "$work"/*.png "$here/screenshots"/ 2>/dev/null || true
# The scripts' Chrome profiles are left in the same Windows temp folder.
rm -rf "$work" "$(wslpath -u "$wintemp")"/madam-cdp-* 2>/dev/null || true
exit $status
