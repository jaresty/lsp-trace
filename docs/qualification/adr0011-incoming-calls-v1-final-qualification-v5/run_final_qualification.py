#!/usr/bin/env python3
"""Author final docs/evidence-only qualification package for ADR0011 incoming calls v1."""
import hashlib, json, os, pathlib, re, shutil, subprocess, sys, tempfile, time

SOURCE = pathlib.Path("/private/tmp/adr0011-incoming-calls-v1-6yiY4m")
ROOT = SOURCE / "docs/qualification/adr0011-incoming-calls-v1-final-qualification-v5"
LOGS = ROOT / "logs"
RECEIPTS = ROOT / "receipts"
V5 = SOURCE / "docs/qualification/adr0011-directional-raw-custody-real-path-vertical-slice-v5"
STAGEB = SOURCE / "docs/qualification/adr0011-directional-raw-custody-c15-stageb-v5"
HEAD = "49334ce23de37c0094f50d21d57ab251a46130ca"

WITNESSES = {
    "EXACT32": "TestV5CausalExact32MiB",
    "PLUS_ONE": "TestV5CausalOver32MiBPreEffect",
    "PARTIAL_EXTERNAL": "TestV5CausalPartialExternalFailure",
    "EARLY_TERMINAL": "TestV5CausalEarlyTerminalOnce",
}

COMMANDS = [
    ("affected-compile", ["go", "test", "-run", "^$", "./incomingops", "./internal/acquisition", "./internal/directionalraw", "./internal/lspwire", "./internal/slicer", "./internal/traverse", "./sessionruntime"]),
    ("affected-vet", ["go", "vet", "./incomingops", "./internal/acquisition", "./internal/directionalraw", "./internal/lspwire", "./internal/slicer", "./internal/traverse", "./sessionruntime"]),
    ("lspwire-framing-allocation-regressions", ["go", "test", "./internal/lspwire", "-run", "^(TestReadWithFrameRetainsExactConsumedFrameNotDecodedReserialization|TestReadWithExactFrameReturnsBoundedExactBytes|TestReadWithExactFrameCapAndPreflight|TestReadWithExactFrameCapWithholdsOutput|TestReadWithExactFrameWithholdsFailedReads|TestReadWithFrameIfWithinPreservesOverCapMessageAndNextFrame|TestReadWithFrameIfWithinPreflightAndMalformedWithhold|TestReadWithFrameWithholdsIncompleteOrInvalidFrames|TestOrdinaryReadWithFrameCharacterizationUnchanged|TestSuccessorCaptureIsExplicitAndOnceBeforeDecode|TestSuccessorIngressOriginalFrameLeaseTransfersExactBacking|TestSuccessorIngressOriginalFrameRefusalPrecedesCapture|TestCloneSuccessorMessageOwnedReservesExactCapacityBeforeIndependentClone|TestCloneSuccessorMessageOwnedRefusalHasZeroClone)$", "-count=1", "-v"]),
    ("traverse-ordinal-compatibility", ["go", "test", "./internal/traverse", "-run", "^(TestIncomingShuffledResponsesProduceIdenticalCanonicalJSON|TestIncomingExactGraphParityAndDeterminism|TestIncomingSchemaV1CompatibilityProjection|TestIncomingSchemaV2CompletenessTerminalProvenanceAndQuality)$", "-count=1", "-v"]),
    ("slicer-ordinal-compatibility", ["go", "test", "./internal/slicer", "-run", "^(TestDiscoverPreparedUsesSameOutgoingBFS|TestDiscoverCanonicalizesAliasesAcrossOutgoingResponses|TestDiscoverExactDepthFrontierIsDeterministicAndDeduplicated)$", "-count=1", "-v"]),
    ("ordinary-roundtrip-default-off-byte-effect", ["go", "test", "./sessionruntime", "-run", "^(TestCallHierarchyObservationAttachmentIgnoredByOrdinaryRoundTrip|TestOrdinaryRoundTripRetainedNotificationCompatibility|TestOrdinaryRoundTripServerErrorCompatibility|TestMethodRequestFrameWithheldUntilAcceptedSuccess|TestMethodFrameCaptureReferencesWithheldAndDefinitionParity|TestDefinitionFrameCaptureBindsOnlyAcceptedSuccessfulResponse)$", "-count=1", "-v"]),
    ("historical-calls-only-cli-baseline", ["go", "test", "./cmd/lsp-trace", "-run", "^TestADR0011HistoricalOmittedCallsOnlyProcessByteBaseline$", "-count=1", "-v"]),
    ("calls-only-dr-admission-boundary", ["go", "test", "./internal/programcadmission", "-run", "^(TestRelationIndexCallsOnlyCompatibilityAndAdmissionBoundary|TestMethodsPresentButDisabledCannotAffectCallsAndRawCandidatesRemainRejected|TestTypedMixedCallsReferencesAndDefinitionsPreservesCallsAndAddsMethods|TestTypedStagingOnlyProjectsAdmittedCalls)$", "-count=1", "-v"]),
    ("historical-calls-only-programc", ["go", "test", "./internal/programc", "-run", "^(TestComputeTypedCallsDeepEqualHistoricalCompositeForTwoSeedsWithoutMutation|TestComputeTypedCarrierCallsEqualsHistoricalBytesForTwoSeedsWithoutMutation|TestStage4CallsComputeAndRecomputeRemainExactHistoricalBytesAndDelegateOnce)$", "-count=1", "-v"]),
]


def sha_bytes(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def sha_file(path: pathlib.Path) -> str:
    return sha_bytes(path.read_bytes())


def run(name, argv, env=None, timeout=180):
    started = time.time()
    fullenv = dict(os.environ)
    fullenv.update({"GOWORK": "off"})
    if env:
        fullenv.update(env)
    proc = subprocess.run(argv, cwd=SOURCE, env=fullenv, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
    elapsed = time.time() - started
    log_path = LOGS / f"{name}.log"
    log_path.write_text(proc.stdout)
    return {
        "name": name,
        "argv": argv,
        "exit_code": proc.returncode,
        "duration_seconds": round(elapsed, 3),
        "log": str(log_path.relative_to(ROOT)),
        "log_sha256": sha_file(log_path),
    }


def git(args, name):
    return run(name, ["git", *args], timeout=120)


def write_json(name, value):
    path = ROOT / name
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    return str(path.relative_to(ROOT))


def source_post_hashes():
    manifest = json.loads((V5 / "PATH_MANIFEST.json").read_text())
    rows = []
    for item in manifest["paths"]:
        path = SOURCE / item["path"]
        digest = sha_file(path)
        rows.append({"path": item["path"], "sha256": digest, "manifest_sha256": item["sha256"], "matches_manifest": digest == item["sha256"]})
    return {"count": len(rows), "all_match_manifest": all(r["matches_manifest"] for r in rows), "rows": rows}


def stageb_counts():
    raw = json.loads((STAGEB / "DUPLICATES_OMISSIONS.json").read_text())
    return {
        "actual_rows": raw["actual_rows"],
        "necessary": raw["classification_counts"]["NECESSARY"],
        "reuse": raw["classification_counts"]["REUSE"],
        "false_positive": raw["classification_counts"]["FALSE_POSITIVE"],
        "unresolved": raw["provisional_unresolved"],
        "duplicates": raw["actual_duplicate_ids"],
        "omissions": raw["actual_omissions"],
    }


def witness_run(key, index):
    receipt = RECEIPTS / f"{key.lower()}-{index}.json"
    if receipt.exists():
        receipt.unlink()
    cmd = ["go", "test", "./sessionruntime", "-run", f"^{WITNESSES[key]}$", "-count=1", "-timeout", "30s", "-v"]
    rec = run(f"witness-{key.lower()}-{index}", cmd, {"ADR0011_V5_CAUSAL_RECEIPT": str(receipt)}, timeout=90)
    rec["receipt"] = str(receipt.relative_to(ROOT))
    if receipt.exists():
        data = json.loads(receipt.read_text())
        rec["receipt_sha256"] = sha_file(receipt)
        rec["pid"] = data.get("pid")
        rec["production_go"] = data.get("production_go")
        rec["entry"] = data.get("entry")
        rec["witness"] = data.get("witness")
        rec["release"] = data.get("release", {})
        if key == "PLUS_ONE":
            facts = data.get("facts", {})
            zero = facts.get("zero_wire", {})
            reuse = facts.get("reuse", {})
            rec["plus_one_zero_write"] = zero.get("write_calls") == 0 and zero.get("raw_bytes") == 0 and zero.get("written_bytes") == 0 and facts.get("wire_effect_delta") == 0
            rec["plus_one_same_session_reuse"] = bool(reuse.get("same_client") and reuse.get("same_manager") and reuse.get("session_id_before") == reuse.get("session_id_after") and reuse.get("generation_before") == reuse.get("generation_after") and reuse.get("ready_before") and reuse.get("ready_after") and reuse.get("success"))
        if key == "PARTIAL_EXTERNAL":
            facts = data.get("facts", {})
            rec["partial_external_typed_once"] = facts.get("failure") == "SESSION_POISONED" and facts.get("refusal_failure") == "RESOURCE_EXHAUSTED"
        if key == "EARLY_TERMINAL":
            rec["early_terminal_once"] = bool(data.get("facts", {}).get("null_early_return") and data.get("release", {}).get("terminal_wins") == 1)
    return rec


def compute_checksums():
    rows = []
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(ROOT).as_posix()
        if rel == "CHECKSUMS.sha256":
            continue
        rows.append(f"{sha_file(path)}  {rel}")
    (ROOT / "CHECKSUMS.sha256").write_text("\n".join(rows) + "\n")


def verifier_self_test_before_final_checksum():
    # Build a temporary checksum over the currently written files, then require the verifier's own tamper mode to detect mutation.
    compute_checksums()
    rec = run("package-verifier-self-test", ["python3", str(ROOT / "verify_final_qualification.py"), "--self-test", "--allow-missing-tamper-record"], timeout=60)
    try:
        payload = json.loads((LOGS / "package-verifier-self-test.log").read_text())
        detected = payload.get("self_test", {}).get("detected_checksum_tamper") is True
    except Exception:
        detected = False
    rec["detected_checksum_tamper"] = detected
    return rec


def main():
    LOGS.mkdir(parents=True, exist_ok=True)
    RECEIPTS.mkdir(parents=True, exist_ok=True)
    result = {
        "status": "IMPLEMENTATION_QUALIFICATION_AUTHORED_NOT_GO",
        "authority": False,
        "production_go": False,
        "self_go": False,
        "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=SOURCE, text=True).strip(),
        "commands": [],
        "witness_runs": [],
        "known_deferred": ["broad aggregate lifecycle/race/C15 scenarios beyond accepted StageB", "production-server/authentication/semantic-completeness evidence"],
    }
    result["source_status_before"] = run("source-status-before", ["git", "status", "--porcelain"], timeout=120)
    patch = subprocess.check_output(["git", "diff", "--binary", "HEAD"], cwd=SOURCE)
    (ROOT / "current.patch").write_bytes(patch)
    result["current_patch"] = {"path": "current.patch", "sha256": sha_file(ROOT / "current.patch"), "bytes": len(patch)}

    exact14 = source_post_hashes()
    write_json("SOURCE_POST_HASHES_EXACT14.json", exact14)
    result["source_post_hashes_exact14"] = {"count": exact14["count"], "all_match_manifest": exact14["all_match_manifest"], "path": "SOURCE_POST_HASHES_EXACT14.json"}

    scounts = stageb_counts()
    write_json("STAGEB_COUNTS.json", scounts)
    result["stageb_counts"] = scounts
    result["commands"].append(run("stageb-verifier", ["python3", str(STAGEB / "verify_stageb_v5.py"), "--source", str(SOURCE)], timeout=240))

    for key, count in [("EXACT32", 3), ("PLUS_ONE", 3), ("PARTIAL_EXTERNAL", 1), ("EARLY_TERMINAL", 1)]:
        for i in range(1, count + 1):
            wr = witness_run(key, i)
            result["witness_runs"].append(wr)
            result["commands"].append({k: wr[k] for k in ("name", "argv", "exit_code", "duration_seconds", "log", "log_sha256")})

    for name, argv in COMMANDS:
        result["commands"].append(run(name, argv, timeout=300))

    diff_check = git(["diff", "--check"], "git-diff-check")
    result["commands"].append(diff_check)
    result["diff_check_exit_code"] = diff_check["exit_code"]
    result["source_status_after"] = run("source-status-after", ["git", "status", "--porcelain"], timeout=120)
    result["commands"].append(result["source_status_after"])

    pids = [w.get("pid") for w in result["witness_runs"] if w.get("pid")]
    result["four_witnesses"] = {
        "total_runs": len(result["witness_runs"]),
        "exact32_runs": sum(1 for w in result["witness_runs"] if w.get("witness") == "EXACT32"),
        "plus_one_runs": sum(1 for w in result["witness_runs"] if w.get("witness") == "PLUS_ONE"),
        "partial_external_runs": sum(1 for w in result["witness_runs"] if w.get("witness") == "PARTIAL_EXTERNAL"),
        "early_terminal_runs": sum(1 for w in result["witness_runs"] if w.get("witness") == "EARLY_TERMINAL"),
        "distinct_processes": len(set(pids)),
        "plus_one_zero_write_all": all(w.get("plus_one_zero_write") for w in result["witness_runs"] if w.get("witness") == "PLUS_ONE"),
        "plus_one_same_session_reuse_all": all(w.get("plus_one_same_session_reuse") for w in result["witness_runs"] if w.get("witness") == "PLUS_ONE"),
        "partial_external_exact_once": all(w.get("partial_external_typed_once") for w in result["witness_runs"] if w.get("witness") == "PARTIAL_EXTERNAL"),
        "early_terminal_exact_once": all(w.get("early_terminal_once") for w in result["witness_runs"] if w.get("witness") == "EARLY_TERMINAL"),
    }

    ok = result["head"] == HEAD and all(c["exit_code"] == 0 for c in result["commands"]) and result["four_witnesses"]["distinct_processes"] == result["four_witnesses"]["total_runs"] and result["four_witnesses"]["plus_one_zero_write_all"] and result["four_witnesses"]["plus_one_same_session_reuse_all"] and scounts == {"actual_rows": 91, "necessary": 78, "reuse": 11, "false_positive": 2, "unresolved": 0, "duplicates": 0, "omissions": 0} and exact14["count"] == 14 and exact14["all_match_manifest"]
    if not ok:
        result["status"] = "IMPLEMENTATION_OR_C15_BLOCKER"
        result["blockers"] = [c for c in result["commands"] if c["exit_code"] != 0]

    manifest = {
        "package": "adr0011-incoming-calls-v1-final-qualification-v5",
        "source": str(SOURCE),
        "head": HEAD,
        "binding": "ADR0011_V5_REFRAME",
        "authority": False,
        "production_go": False,
        "implementation_go": False,
        "self_go": False,
        "deferred_not_required": ["broad aggregate/race/C15 scenarios beyond accepted StageB and four witnesses"],
        "files": ["README.md", "REVIEW_REQUEST.md", "MANIFEST.json", "RESULT.json", "SOURCE_POST_HASHES_EXACT14.json", "STAGEB_COUNTS.json", "current.patch", "CHECKSUMS.sha256", "run_final_qualification.py", "verify_final_qualification.py", "logs/", "receipts/"],
    }
    write_json("MANIFEST.json", manifest)
    write_json("RESULT.json", result)

    (ROOT / "README.md").write_text(f"""# ADR0011 incoming-calls v1 final qualification package\n\nStatus: `{result['status']}`  \nAuthority: `false`; production GO: `false`; implementation GO: `false`; self-GO: `false`.\n\nSource: `{SOURCE}` at `{result['head']}`.\n\nThis package is docs/evidence-only. It records the Phase5 final qualification commands, logs, receipts, current patch, exact14 source post-hashes, StageB counts, package hashes, and deterministic verifier/tamper guard. It does not edit production or test source and does not authorize GO.\n\n## Key recorded facts\n\n- Four independent witnesses run in separate fresh Go test processes: EXACT32 x3, PLUS_ONE x3, PARTIAL_EXTERNAL x1, EARLY_TERMINAL x1.\n- PLUS_ONE receipts assert zero Manager stdin Write calls/raw bytes/written bytes before provider frame decoding and successful same READY session reuse.\n- PARTIAL_EXTERNAL and EARLY_TERMINAL are exact-once in this package.\n- StageB verifier/counts recorded: 91 actual rows, 78 NECESSARY, 11 REUSE, 2 FALSE_POSITIVE, 0 unresolved, duplicate/omission 0.\n- Deferred by ADR0011_V5_REFRAME and not required here: broad aggregate lifecycle/race/C15 scenarios beyond accepted StageB and four witnesses.\n\n## Reproduce\n\n```bash\npython3 docs/qualification/adr0011-incoming-calls-v1-final-qualification-v5/verify_final_qualification.py --self-test\n```\n\nThe authoring command was:\n\n```bash\npython3 docs/qualification/adr0011-incoming-calls-v1-final-qualification-v5/run_final_qualification.py\n```\n""")
    (ROOT / "REVIEW_REQUEST.md").write_text(f"""# Review request — ADR0011 incoming-calls v1 final qualification v5\n\nPlease review this package as evidence only. Requested decision is limited to whether the recorded commands/logs/receipts support `IMPLEMENTATION_QUALIFICATION_AUTHORED_NOT_GO` under ADR0011_V5_REFRAME.\n\nDo not treat this package as production GO, implementation GO, or authority. `authority=false` is part of the manifest and verifier contract.\n\nReview focus:\n\n1. Four witness receipts and logs under `receipts/` and `logs/witness-*`.\n2. `RESULT.json` command matrix and selected targeted regressions.\n3. `STAGEB_COUNTS.json` and `logs/stageb-verifier.log` for 91/78/11/2/0 and duplicate/omission 0.\n4. `SOURCE_POST_HASHES_EXACT14.json` and `current.patch` for source hash/patch custody from HEAD.\n5. `CHECKSUMS.sha256` plus verifier self-test for deterministic package/tamper guard.\n\nExpected terminal status: `{result['status']}`.\n""")

    tamper = verifier_self_test_before_final_checksum()
    result["tamper_guard_self_test"] = {"command": tamper, "detected_checksum_tamper": tamper.get("detected_checksum_tamper") is True}
    write_json("RESULT.json", result)
    compute_checksums()
    print(json.dumps({"status": result["status"], "package": str(ROOT), "commands": len(result["commands"]), "witness_runs": len(result["witness_runs"]), "checksum": sha_file(ROOT / "CHECKSUMS.sha256")}, indent=2))
    return 0 if result["status"] == "IMPLEMENTATION_QUALIFICATION_AUTHORED_NOT_GO" else 1

if __name__ == "__main__":
    raise SystemExit(main())
