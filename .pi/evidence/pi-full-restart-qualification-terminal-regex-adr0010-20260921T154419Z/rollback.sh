#!/bin/sh
set -eu
ROOT=/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-terminal-regex-adr0010-20260921T154419Z
install -m 0644 "$ROOT/backups/mcp.json" /Users/schwa/dev/lsp-trace/.mcp.json
install -m 0644 "$ROOT/backups/bootstrap.json" /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json
install -m 0755 "$ROOT/backups/lsp-trace" /Users/schwa/.local/bin/lsp-trace
install -m 0755 "$ROOT/backups/lsp-trace-mcp" /Users/schwa/.local/bin/lsp-trace-mcp
printf '%s\n' 'ROLLBACK_RESTORED; fully restart Pi before further MCP use'
