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
assert_contains ASSERT_RELEASE_B05_PROVIDER_BUILD "$release_check" 'go build -trimpath -o "$release_tmp/lsp-trace-provider-ember-glint" ./cmd/lsp-trace-provider-ember-glint'
assert_contains ASSERT_RELEASE_B05_PROVIDER_BINARY "$release_check" 'LSP_TRACE_EMBER_PROVIDER_BINARY="$release_tmp/lsp-trace-provider-ember-glint"'
assert_contains ASSERT_RELEASE_B05_LIFECYCLE_TEST "$release_check" 'TestProductionEmberProviderCompletesManagedB05Lifecycle'
assert_contains ASSERT_RELEASE_B05_RETAINED_QUALIFICATION "$release_check" 'qualification/retained/ember-glint/b05-production-lifecycle.json'
assert_contains ASSERT_RELEASE_B05_GORELEASER_MAIN .goreleaser.yaml 'main: ./cmd/lsp-trace-provider-ember-glint'
assert_contains ASSERT_RELEASE_B05_GORELEASER_BINARY .goreleaser.yaml 'binary: lsp-trace-provider-ember-glint'
