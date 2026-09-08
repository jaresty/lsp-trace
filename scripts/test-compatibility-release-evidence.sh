#!/bin/sh
set -eu

case $0 in
  /*) script=$0 ;;
  *) script=$PWD/$0 ;;
esac
if [ -L "$script" ]; then
  printf 'qualification wrapper unsafe symlink: %s\n' "$script" >&2
  exit 126
fi
script_dir=$(CDPATH= cd -P -- "$(dirname -- "$script")" 2>/dev/null && pwd) || {
  printf 'qualification wrapper location unavailable: %s\n' "$script" >&2
  exit 126
}
runner=$script_dir/qualify-compatibility-release.sh
if [ ! -f "$runner" ] || [ ! -x "$runner" ] || [ -L "$runner" ]; then
  output="qualification runner unavailable: $runner"
  runner_status=127
else
  set +e
  output=$($runner 2>&1)
  runner_status=$?
  set -e
fi

if [ "$runner_status" -ne 0 ]; then
  printf '%s\n' "$output"
fi
failed=0

assert_result() {
  assertion=$1
  expected="PASS $assertion"
  if printf '%s\n' "$output" | grep -F "$expected" >/dev/null; then
    printf '%s\n' "$expected"
  else
    printf 'FAIL %s: qualification output omitted assertion\n' "$assertion"
    failed=1
  fi
}

assert_result ASSERT_FR22_VERSIONED_PROVENANCE
assert_result ASSERT_FR22_SYNTHETIC_AUTHORITY
assert_result ASSERT_FR22_FIXTURE_SOURCE_IDENTITY
assert_result ASSERT_FR22_FIXTURE_BYTES_PINNED
assert_result ASSERT_FR22_PRODUCER_CONSUMER_DIAGNOSTICS
assert_result ASSERT_FR22_NULL_EMPTY_DISTINCT
assert_result ASSERT_FR22_PUBLICATION_OFFLINE_VALID
assert_result ASSERT_FR22_REPLAY_BYTE_EXACT
assert_result ASSERT_FR22_RELEASE_ARCHIVE_EVIDENCE
assert_result ASSERT_FR22_SOURCE_READY_DEPLOYMENT_UNKNOWN

if [ "$runner_status" -ne 0 ]; then
  printf 'FAIL QUALIFICATION_RUNNER_STATUS: runner exited %s\n' "$runner_status"
  failed=1
fi
[ "$failed" -eq 0 ]
