#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/lsp-trace-status.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
cd "$root"
go build -trimpath -o "$work/mcp" ./cmd/lsp-trace-mcp
go build -trimpath -o "$work/lsp" ./cmd/fake-lsp
mkdir -p "$work/dist"
package=$(cd providers/ember-glint && npm pack --silent --pack-destination "$work/dist")
npm install --offline --ignore-scripts --prefix "$work/install" "$work/dist/$package"
STATUS_MCP="$work/mcp" STATUS_LSP="$work/lsp" \
STATUS_PROVIDER="$work/install/node_modules/@lsp-trace/ember-glint-provider/bin/ember-glint.mjs" \
node --test providers/ember-glint/test/status-propagation.test.mjs providers/ember-glint/test/status-contract.test.mjs
