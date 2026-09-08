#!/bin/sh
set -eu

runner=./scripts/qualify-compatibility-release.sh
output=$($runner 2>&1 || true)
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
assert_result ASSERT_FR22_HISTORICAL_BYTES_IMMUTABLE
assert_result ASSERT_FR22_PRODUCER_CONSUMER_DIAGNOSTICS
assert_result ASSERT_FR22_NULL_EMPTY_DISTINCT
assert_result ASSERT_FR22_PUBLICATION_OFFLINE_VALID
assert_result ASSERT_FR22_REPLAY_BYTE_EXACT
assert_result ASSERT_FR22_RELEASE_ARCHIVE_EVIDENCE
assert_result ASSERT_FR22_SOURCE_READY_DEPLOYMENT_UNKNOWN

[ "$failed" -eq 0 ]
