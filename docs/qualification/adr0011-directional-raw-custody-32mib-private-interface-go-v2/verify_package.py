#!/usr/bin/env python3
import hashlib
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
V1 = HERE.parent / "adr0011-directional-raw-custody-32mib-private-interface-go-v1"
EXPECTED_V1 = "d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c"
EXPECTED_HEAD = "f03672416bbe1342c06e7cbb33bf51127909ce8a"


def require(assertion, condition):
    if not condition:
        print(f"FAIL {assertion}")
        raise SystemExit(1)
    print(f"PASS {assertion}")


def text(name):
    return (HERE / name).read_text()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

require("P1_V1_EXACT_MANIFEST", digest(V1 / "SHA256SUMS") == EXPECTED_V1)
for line in (V1 / "SHA256SUMS").read_text().splitlines():
    expected, name = line.split(None, 1)
    require(f"P1_V1_FILE_{name}", digest(V1 / name) == expected)

deps = text("DEPENDENCIES.sha256")
require("P2_AMENDMENT_PREDECESSOR", "REVIEWED_GENERIC_AMENDMENT_PREDECESSOR_PIN" in deps)
require("P2_V1_PREDECESSOR", f"{EXPECTED_V1}  IMMUTABLE_PRIVATE_INTERFACE_V1_PREDECESSOR_PIN" in deps)
require("P2_NONCIRCULAR", "neither dependency names v2" in deps and "private-interface-go-v2" not in "\n".join(deps.splitlines()[2:]))

go = text("sessionruntime_private.go.txt")
require("P3_CANONICAL_DIGEST_PREFIX", 'privateCallsLSPProfileDigestConstructorDomainV1             = "lsp-trace.adr0011.calls.profile-identity.digest.v1\\x00"' in go)
require("P3_LENGTH_PREFIXED_DOMAIN_ID", go.count("binary.BigEndian.PutUint32") == 2 and "sha256.Sum256(preimage)" in go)
require("P3_BOTH_CONSTRUCTORS_POPULATE_DIGEST", go.count("Digest: privateCallHierarchyProfileDigestV1(") == 2)
require("P4_BOTH_DIRECTIONS", go.count("compilePlannedCallSitesForDirection(") == 3 and "privateCallHierarchyDirectionIncomingV1" in go and "privateCallHierarchyDirectionOutgoingV1" in go)
for marker in ["MeasurementMaxBytesV1+1", "var nilManager *Manager", "privateCallHierarchyDirectionInvalidV1", "dispositions :=", "terminals :=", "refusals :=", "lifetimes :=", "owner.requestTerminal()", "m.releaseCallHierarchyV1(lease, r)", "m.releaseCallHierarchyTerminalV1(owner, r)"]:
    require(f"P4_CALL_SHAPE_{marker}", marker in go)
require("P5_ATTEMPT_OUTCOME_RECORD", "type privateCallHierarchyAttemptOutcomeV1 struct" in go and "AttemptOutcomes" in go)
require("P5_ORDERED_UNATTEMPTED_FRONTIER", "UnattemptedOrdinals: []uint64{2}" in go and "AttemptOutcomes: []privateCallHierarchyAttemptOutcomeV1" in go)
require("P6_NO_RELEASED_DISPOSITION", "privateCallHierarchyDispositionReleasedV1" not in go)
require("P6_SEPARATE_RELEASE", "ReleaseID" in go and "privateCallHierarchyLifetimeReleasedV1" in go)
interface = text("INTERFACE.md")
state = text("STATE_AND_OWNERSHIP.md")
readme = text("README.md")
require("P7_PRESERVED_BOUNDARIES", all(s in interface + state + readme for s in ["sole generic owner", "Manager.mu -> callHierarchyLayer1GenericOwner.mu", "No Go/gopls semantics"]))

manifest = text("SHA256SUMS")
for line in manifest.splitlines():
    expected, name = line.split(None, 1)
    require(f"P8_MANIFEST_{name}", digest(HERE / name) == expected)
require("P8_AUTHORED_NOT_GO", all("AUTHORED_NOT_GO" in text(name) for name in ["README.md", "INTERFACE.md", "STATE_AND_OWNERSHIP.md", "WRITESET.md", "HARNESS_EVIDENCE.md", "REVIEW_REQUEST.md"]))
head = subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip()
require("P9_HEAD_UNCHANGED", head == EXPECTED_HEAD)
allowed = ("docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1/", "docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v2/")
status = subprocess.check_output(["git", "-C", str(ROOT), "status", "--porcelain"], text=True).splitlines()
require("P9_WRITE_SCOPE", all(any(line[3:].startswith(prefix) for prefix in allowed) for line in status))
print("V2_PACKAGE_VERIFY_OK")
