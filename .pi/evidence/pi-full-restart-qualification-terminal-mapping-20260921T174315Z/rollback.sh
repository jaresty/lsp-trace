#!/bin/sh
set -eu
root='/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-terminal-mapping-20260921T174315Z'
cp "$root/backups/mcp.json" /Users/schwa/dev/lsp-trace/.mcp.json
cp "$root/backups/bootstrap.json" /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json
install -m 700 "$root/backups/lsp-trace" /Users/schwa/.local/bin/lsp-trace
install -m 700 "$root/backups/lsp-trace-mcp" /Users/schwa/.local/bin/lsp-trace-mcp
chmod 600 /Users/schwa/dev/lsp-trace/.mcp.json /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json
printf 'Rollback staged; fully restart Pi.\n'
