#!/usr/bin/env python3
import json
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[1]
workflow = (root / ".github/workflows/ci.yml").read_text()
manifest = json.loads((root / "scripts/ci-shards.json").read_text())
checks = {
    "linux job": "  test-linux:" in workflow,
    "darwin job": "  test-darwin:" in workflow,
    "linux shards": all(f"id: linux-{i}" in workflow for i in range(1, 6)),
    "darwin shards": all(f"id: darwin-{i}" in workflow for i in range(1, 5)),
    "bounded test": "go test -json -timeout=10m" in workflow,
    "raw artifact": "go-test.json" in workflow,
    "summary artifact": "go-test-summary.json" in workflow,
    "conditional gopls": "matrix.shard.needs_gopls" in workflow and "gopls@v0.23.0" in workflow,
    "unix vet build": "  unix-vet-build:" in workflow,
    "windows vet build": "  windows:" in workflow,
    "aggregate": "  repository-qualified:" in workflow,
    "explicit aggregate needs": "needs: [quality, elixir-companion, test-linux, test-darwin, unix-vet-build, windows]" in workflow,
    "no serial xargs": "xargs go test" not in workflow,
    "no monolithic tests": "go test ./..." not in workflow,
}
checks["manifest topology"] = len(manifest["platforms"]["linux"]["shards"]) == 5 and len(manifest["platforms"]["darwin"]["shards"]) == 4
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS " if ok else "FAIL ") + name)
sys.exit(bool(failed))
