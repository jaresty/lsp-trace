#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
exec python3 - "$repo" <<'PY'
import hashlib
import io
import json
import os
import pathlib
import sys
import tarfile

repo = pathlib.Path(sys.argv[1])
root = repo / "qualification" / "compatibility-release"
paths = {
    "matrix": root / "matrix.v1.json",
    "historical": root / "historical-native-graph.v1.json",
    "normalized": root / "normalized-retained-export.v1.json",
}
raw = {name: path.read_bytes() for name, path in paths.items()}
data = {name: json.loads(value) for name, value in raw.items()}
transition = data["matrix"]["transitions"][0]
failures = []

def require(assertion, condition, detail):
    if not condition:
        print(f"FAIL {assertion}: {detail}")
        failures.append(assertion)
    else:
        print(f"PASS {assertion}")

require(
    "ASSERT_FR22_VERSIONED_PROVENANCE",
    data["matrix"]["schema_id"] == "lsp-trace.compatibility-matrix.v1"
    and transition["input"]["sha256"]
    and transition["output"]["sha256"]
    and transition["producer"]["schema_id"] == data["historical"]["schema_id"]
    and transition["consumer"]["schema_id"] == data["normalized"]["schema_id"],
    "matrix version or provenance coordinate is incomplete",
)

historical_digest = hashlib.sha256(raw["historical"]).hexdigest()
normalized_digest = hashlib.sha256(raw["normalized"]).hexdigest()
require(
    "ASSERT_FR22_HISTORICAL_BYTES_IMMUTABLE",
    historical_digest == transition["input"]["sha256"]
    and normalized_digest == transition["output"]["sha256"],
    "retained fixture bytes differ from pinned provenance",
)

require(
    "ASSERT_FR22_PRODUCER_CONSUMER_DIAGNOSTICS",
    transition["producer"]["status"] == "HISTORICAL_RETAINED"
    and transition["consumer"]["status"] == "SOURCE_IMPLEMENTED"
    and transition["diagnostics"] == {
        "unsupported_transition": "EXPLICIT_FAILURE",
        "failure_is_empty": False,
    },
    "transition surface or explicit diagnostics are incomplete",
)

require(
    "ASSERT_FR22_NULL_EMPTY_DISTINCT",
    data["historical"]["nodes"] is None
    and data["historical"]["edges"] is None
    and data["historical"]["failure"] is None
    and data["normalized"]["nodes"] == []
    and data["normalized"]["edges"] == []
    and data["normalized"]["failure"] is None
    and data["normalized"]["unresolved_seeds"] == [],
    "native null, normalized empty, failure, or unresolved states collapsed",
)

require(
    "ASSERT_FR22_PUBLICATION_OFFLINE_VALID",
    data["matrix"]["publication"] == {
        "kind": "LOCAL_IMMUTABLE_EVIDENCE",
        "offline_validatable": True,
        "deployment_claim": False,
    },
    "publication is not immutable, offline-validatable, and non-deployment",
)

converted = {
    "schema_id": "lsp-trace.retained-export.v1",
    "source_schema_id": data["historical"]["schema_id"],
    "nodes": [] if data["historical"]["nodes"] is None else data["historical"]["nodes"],
    "edges": [] if data["historical"]["edges"] is None else data["historical"]["edges"],
    "failure": data["historical"]["failure"],
    "unresolved_seeds": data["historical"]["unresolved_seeds"],
}
replay_one = (json.dumps(converted, separators=(",", ":")) + "\n").encode()
replay_two = (json.dumps(converted, separators=(",", ":")) + "\n").encode()
require(
    "ASSERT_FR22_REPLAY_BYTE_EXACT",
    replay_one == replay_two == raw["normalized"],
    "offline hydration replay differs from retained normalized bytes",
)

archive_buffer = io.BytesIO()
with tarfile.open(fileobj=archive_buffer, mode="w:gz") as archive:
    archive_names = ("historical", "normalized") if os.environ.get("LSP_TRACE_FR22_OMIT_ARCHIVE_MATRIX") == "1" else ("matrix", "historical", "normalized")
    for name in archive_names:
        info = tarfile.TarInfo(f"compatibility-release/{paths[name].name}")
        info.size = len(raw[name])
        info.mtime = 0
        archive.addfile(info, io.BytesIO(raw[name]))
archive_buffer.seek(0)
with tarfile.open(fileobj=archive_buffer, mode="r:gz") as archive:
    entries = set(archive.getnames())
required = {f"compatibility-release/{path.name}" for path in paths.values()}
require(
    "ASSERT_FR22_RELEASE_ARCHIVE_EVIDENCE",
    entries == required,
    "temporary release archive does not contain the exact declared evidence set",
)

require(
    "ASSERT_FR22_SOURCE_READY_DEPLOYMENT_UNKNOWN",
    data["matrix"]["source_ready"] is True
    and data["matrix"]["deployment"] == {
        "status": "UNKNOWN",
        "reason": "NO_LIVE_INSTANCE_INSPECTED",
    }
    and data["matrix"]["publication"]["deployment_claim"] is False,
    "source readiness was conflated with deployed availability",
)
if failures:
    raise SystemExit(1)
PY
