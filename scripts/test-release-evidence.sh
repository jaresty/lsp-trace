#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
release_check=$root/scripts/release-check.sh
skill=$root/cmd/lsp-trace/SKILL.md
releasing=$root/docs/RELEASING.md
readme=$root/README.md

assert_contains() {
  id=$1
  path=$2
  text=$3
  if grep -F "$text" "$path" >/dev/null; then
    printf 'PASS %s\n' "$id"
  else
    printf 'FAIL %s: %s must contain %s\n' "$id" "$path" "$text"
    return 1
  fi
}

assert_contains ASSERT_RELEASE_DOC_TWELVE "$skill" 'The default surface advertises twelve canonical tools:'
assert_contains ASSERT_RELEASE_BOOTSTRAP_GUARD "$release_check" 'bootstrap_process_test.go'
assert_contains ASSERT_RELEASE_TWELVE_TOOL_GUARD "$release_check" 'TestLifecycleExecutorFamilyIsEnabledAndAdvertisedByDefault'
assert_contains ASSERT_RELEASE_STDIO_CHANNEL_GUARD "$release_check" 'TestRunStdioOnly'
assert_contains ASSERT_RELEASE_GUIDE_BOOTSTRAP "$releasing" 'production bootstrap'
assert_contains ASSERT_RELEASE_GUIDE_CHANNELS "$releasing" 'trusted-local warning on stderr and protocol-clean MCP stdout'
assert_contains ASSERT_RELEASE_BOOTSTRAP_USAGE "$readme" 'lsp-trace-mcp --bootstrap-config /absolute/path/bootstrap.json'
assert_contains ASSERT_RELEASE_BOOTSTRAP_HOST_AUTHORITY "$readme" 'the host—not the MCP caller—provisions trusted sessions'
