#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mode=${1:---required}
case "$mode" in
  --required|--unit) ;;
  *) printf 'FAIL ASSERT_ADAPTER_CANARY_MODE: expected --required or --unit\n' >&2; exit 2 ;;
esac

adapter_root=${PI_MCP_ADAPTER_ROOT:-"$HOME/.pi/agent/npm/node_modules/pi-mcp-adapter"}
bun_bin=${BUN_BIN:-"$HOME/.bun/bin/bun"}
missing=
[ -f "$adapter_root/package.json" ] || missing="$adapter_root/package.json"
[ -f "$adapter_root/direct-tools.ts" ] || missing="$adapter_root/direct-tools.ts"
[ -f "$adapter_root/server-manager.ts" ] || missing="$adapter_root/server-manager.ts"
[ -x "$bun_bin" ] || missing="$bun_bin"
if [ -n "$missing" ]; then
  if [ "$mode" = --unit ] && [ "${LSP_TRACE_ALLOW_MISSING_PI_MCP_ADAPTER:-}" = 1 ]; then
    printf 'SKIP ASSERT_ADAPTER_CANARY_EXPLICIT_UNIT_GATE: unavailable=%s\n' "$missing"
    exit 0
  fi
  printf 'FAIL ASSERT_ADAPTER_CANARY_REQUIRED: installed pi-mcp-adapter v2.32.1 runtime unavailable: %s\n' "$missing" >&2
  exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/lsp-trace-adapter-canary.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
mkdir -p "$work/node_modules"
cp -R "$adapter_root" "$work/node_modules/pi-mcp-adapter"
for dependency in "$HOME/.pi/agent/npm/node_modules"/*; do
  name=$(basename "$dependency")
  [ "$name" = pi-mcp-adapter ] || ln -s "$dependency" "$work/node_modules/$name"
done
pi_agent_root=${PI_CODING_AGENT_ROOT:-/opt/homebrew/lib/node_modules/@earendil-works/pi-coding-agent}
if [ ! -e "$work/node_modules/typebox" ]; then
  ln -s "$pi_agent_root/node_modules/typebox" "$work/node_modules/typebox"
fi
if [ ! -e "$work/node_modules/@earendil-works" ]; then
  mkdir -p "$work/node_modules/@earendil-works"
  ln -s "$pi_agent_root/node_modules/@earendil-works/pi-ai" "$work/node_modules/@earendil-works/pi-ai"
  ln -s "$pi_agent_root" "$work/node_modules/@earendil-works/pi-coding-agent"
fi
for source in direct-tools.ts server-manager.ts metadata-cache.ts ts-shape.ts package.json; do
  installed_digest=$(shasum -a 256 "$adapter_root/$source" | cut -d ' ' -f 1)
  staged_digest=$(shasum -a 256 "$work/node_modules/pi-mcp-adapter/$source" | cut -d ' ' -f 1)
  if [ "$installed_digest" != "$staged_digest" ]; then
    printf 'FAIL ASSERT_ADAPTER_UNMODIFIED: %s installed=%s staged=%s\n' "$source" "$installed_digest" "$staged_digest" >&2
    exit 1
  fi
done
printf 'PASS ASSERT_ADAPTER_UNMODIFIED: staged source bytes match installed pi-mcp-adapter v2.32.1\n'
go build -trimpath -o "$work/lsp-trace-mcp" ./cmd/lsp-trace-mcp
PI_MCP_ADAPTER_ROOT="$work/node_modules/pi-mcp-adapter" LSP_TRACE_MCP_BINARY="$work/lsp-trace-mcp" "$bun_bin" "$root/scripts/installed-pi-direct-tools-program-b-d01-canary.ts"
PI_MCP_ADAPTER_ROOT="$work/node_modules/pi-mcp-adapter" LSP_TRACE_MCP_BINARY="$work/lsp-trace-mcp" "$bun_bin" "$root/scripts/installed-pi-direct-tools-slice-selector-canary.ts"
