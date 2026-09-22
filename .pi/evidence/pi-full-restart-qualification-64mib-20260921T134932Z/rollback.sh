#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
install -m 600 "$ROOT/backups/mcp.json" /Users/schwa/dev/lsp-trace/.mcp.json
install -m 600 "$ROOT/backups/bootstrap.json" /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json
install -m 755 "$ROOT/backups/lsp-trace" /Users/schwa/.local/bin/lsp-trace
install -m 755 "$ROOT/backups/lsp-trace-mcp" /Users/schwa/.local/bin/lsp-trace-mcp
printf '%s\n' ROLLBACK_COMPLETE
