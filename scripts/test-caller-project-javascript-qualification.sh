#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
for path in qualification/caller-project-javascript/cases.v1.json qualification/caller-project-javascript/qualify.mjs qualification/caller-project-javascript/guard.test.mjs qualification/retained/caller-project-javascript/evidence.v1.json; do
  [ -s "$path" ] || { printf 'FAIL ASSERT_CALLER_JS_TRACKED_ARTIFACT %s\n' "$path" >&2; exit 1; }
done
node --test qualification/caller-project-javascript/guard.test.mjs
printf 'PASS ASSERT_CALLER_JS_RELEASE_ADMISSION\n'
