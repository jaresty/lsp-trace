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

assert_contains ASSERT_RELEASE_DOC_THIRTEEN "$skill" 'The default surface advertises thirteen canonical tools:'
assert_contains ASSERT_RELEASE_BOOTSTRAP_GUARD "$release_check" 'bootstrap_process_test.go'
assert_contains ASSERT_RELEASE_THIRTEEN_TOOL_GUARD "$release_check" 'TestLifecycleExecutorFamilyIsEnabledAndAdvertisedByDefault'
assert_contains ASSERT_RELEASE_STDIO_CHANNEL_GUARD "$release_check" 'TestRunStdioOnly'
assert_contains ASSERT_RELEASE_GUIDE_BOOTSTRAP "$releasing" 'production bootstrap'
assert_contains ASSERT_RELEASE_GUIDE_CHANNELS "$releasing" 'trusted-local warning on stderr and protocol-clean MCP stdout'
assert_contains ASSERT_RELEASE_BOOTSTRAP_USAGE "$readme" 'lsp-trace-mcp --bootstrap-config /absolute/path/bootstrap.json'
assert_contains ASSERT_RELEASE_BOOTSTRAP_HOST_AUTHORITY "$readme" 'The host—not the MCP caller—provisions trusted sessions'
assert_contains ASSERT_RELEASE_EXTERNAL_PROVIDER_QUALIFIER "$release_check" 'scripts/qualify-external-provider.sh'
assert_contains ASSERT_RELEASE_RETAINED_EXTERNAL_QUALIFICATION "$release_check" 'qualification/retained/external-provider/ember-glint.json'
assert_contains ASSERT_RELEASE_REAL_EXTERNAL_PROVIDER_QUALIFICATION "$release_check" 'ASSERT_RELEASE_REAL_EXTERNAL_PROVIDER_QUALIFICATION'
assert_contains ASSERT_RELEASE_CORE_ARCHIVE_GUARD "$release_check" 'ASSERT_CORE_ARCHIVES_EXCLUDE_PROVIDER_ASSETS'
assert_contains ASSERT_RELEASE_CORE_ARCHIVE_IDS .goreleaser.yaml 'ids: [lsp-trace, lsp-trace-mcp]'
if grep -F 'lsp-trace-provider-ember-glint' .goreleaser.yaml >/dev/null; then
  printf 'FAIL ASSERT_RELEASE_CORE_ARCHIVES_NO_PROVIDER_ASSETS\n'
  exit 1
fi
printf 'PASS ASSERT_RELEASE_CORE_ARCHIVES_NO_PROVIDER_ASSETS\n'
