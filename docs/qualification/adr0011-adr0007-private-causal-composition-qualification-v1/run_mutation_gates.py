#!/usr/bin/env python3
from __future__ import annotations

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
SRC = ROOT / "internal/adr0011adr0007composition/compose.go"
PKG = "./internal/adr0011adr0007composition"

MUTATIONS = [
    {
        "name": "remove_incoming_membership_branch",
        "test": "TestWrapperIncomingOutgoingRemovalIsolation",
        "old": 'if mark.Action == ActionOmit {\n\t\t\tomit[mark.MatchID] = true\n\t\t} else {\n\t\t\tordered = append(ordered, mark)\n\t\t}',
        "new": 'if mark.Action == ActionOmit {\n\t\t\t// MUTATION: omission branch disabled\n\t\t} else {\n\t\t\tordered = append(ordered, mark)\n\t\t}',
    },
    {
        "name": "remove_order_branch",
        "test": "TestWrapperPinnedEvidenceChangesOnlySeparateCompositionCandidate",
        "old": 'if mark.Action != ActionOmit && mark.Action != ActionOrder {',
        "new": 'if mark.Action != ActionOmit {',
    },
    {
        "name": "remove_failure_preservation",
        "test": "TestWrapperDirectionFailureNonPositivePreservesPeer",
        "old": 'if failureEnvelope(envelopes) {',
        "new": 'if false && failureEnvelope(envelopes) {',
    },
    {
        "name": "remove_pin_check",
        "test": "TestWrapperPinTamperAndIdentityEscalationRefused/composition-package",
        "old": 'if env.ProfileID != want.ProfileID || env.GenericProfileID != want.GenericProfileID || env.EvidenceSHA256 != want.EvidenceSHA256 || env.ProviderPackageChecksumSHA256 != want.ProviderPackageChecksumSHA256 || env.ProviderManifestSHA256 != want.ProviderManifestSHA256 || env.CompositionPackageChecksumSHA256 != AcceptedBidirectionalPackageChecksumSHA256 || env.CompositionArtifactSHA256 != AcceptedBidirectionalCompositionArtifactSHA256 || env.OrdinalSemantics != want.OrdinalSemantics || env.OneFinalDispositionOrdinals != want.OneFinalDispositionOrdinals || env.OrdinalIdentitySHA256 != want.OrdinalIdentitySHA256 {',
        "new": 'if false && (env.ProfileID != want.ProfileID || env.GenericProfileID != want.GenericProfileID || env.EvidenceSHA256 != want.EvidenceSHA256 || env.ProviderPackageChecksumSHA256 != want.ProviderPackageChecksumSHA256 || env.ProviderManifestSHA256 != want.ProviderManifestSHA256 || env.CompositionPackageChecksumSHA256 != AcceptedBidirectionalPackageChecksumSHA256 || env.CompositionArtifactSHA256 != AcceptedBidirectionalCompositionArtifactSHA256 || env.OrdinalSemantics != want.OrdinalSemantics || env.OneFinalDispositionOrdinals != want.OneFinalDispositionOrdinals || env.OrdinalIdentitySHA256 != want.OrdinalIdentitySHA256) {',
    },
    {
        "name": "remove_ordinal_identity_check",
        "test": "TestWrapperPinTamperAndIdentityEscalationRefused/mark-ordinal-identity",
        "old": 'if mark.OneFinalDispositionOrdinals != env.OneFinalDispositionOrdinals || mark.OrdinalIdentitySHA256 != env.OrdinalIdentitySHA256 || mark.SelectedOrdinal != 0 {',
        "new": 'if false && (mark.OneFinalDispositionOrdinals != env.OneFinalDispositionOrdinals || mark.OrdinalIdentitySHA256 != env.OrdinalIdentitySHA256 || mark.SelectedOrdinal != 0) {',
    },
    {
        "name": "remove_max_references_enforcement",
        "test": "TestWrapperBoundsAndCancellation",
        "old": 'if limits.MaxReferences > 0 && next > uint64(limits.MaxReferences) {\n\t\treturn false\n\t}',
        "new": '// MUTATION: max references disabled',
    },
    {
        "name": "remove_identity_restraint",
        "test": "TestWrapperPinTamperAndIdentityEscalationRefused",
        "old": 'Authority:               0,\n\t\tAccepted:                false,\n\t\tCompleteness:            "UNKNOWN",\n\t\tFeatureIdentity:         "UNRESOLVED",',
        "new": 'Authority:               1,\n\t\tAccepted:                true,\n\t\tCompleteness:            "COMPLETE",\n\t\tFeatureIdentity:         "FEATURE_ACCEPTED",',
    },
    {
        "name": "remove_idempotence_normalization",
        "test": "TestWrapperUnorderedMarksStableAndProviderOrdinalsPreserved",
        "old": 'sort.SliceStable(marks, func(i, j int) bool { return markLess(marks[i], marks[j]) })',
        "new": '// MUTATION: mark normalization removed',
    },
    {
        "name": "remove_cancel_precheck",
        "test": "TestWrapperBoundsAndCancellation",
        "old": 'if err := ctx.Err(); err != nil {\n\t\treturn cancelledResult(req, err), err\n\t}',
        "new": '// MUTATION: pre-ADR0007 cancellation check removed',
    },
]


def run(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)


def main() -> int:
    original = SRC.read_text()
    lines: list[str] = []
    baseline = run(["go", "test", PKG, "-run", "TestWrapperBaselineNoOptionalInputExactADR0007Forwarding", "-count=1"])
    lines.append(f"BASELINE exit={baseline.returncode}")
    lines.append(baseline.stdout.strip())
    if baseline.returncode != 0:
        print("\n".join(lines))
        return 1
    for mutation in MUTATIONS:
        text = original
        if mutation["old"] not in text:
            lines.append(f"MUTATION {mutation['name']} setup=OLD_TEXT_NOT_FOUND")
            print("\n".join(lines))
            return 1
        SRC.write_text(text.replace(mutation["old"], mutation["new"], 1))
        try:
            proc = run(["go", "test", PKG, "-run", mutation["test"], "-count=1"])
            outcome = "EXPECTED_FAIL" if proc.returncode != 0 else "UNEXPECTED_PASS"
            lines.append(f"MUTATION {mutation['name']} test={mutation['test']} outcome={outcome} exit={proc.returncode}")
            lines.append(proc.stdout.strip())
            if proc.returncode == 0:
                print("\n".join(lines))
                return 1
        finally:
            SRC.write_text(original)
    final = run(["go", "test", PKG, "-run", "TestWrapperBaselineNoOptionalInputExactADR0007Forwarding", "-count=1"])
    lines.append(f"RESTORED_BASELINE exit={final.returncode}")
    lines.append(final.stdout.strip())
    if final.returncode != 0:
        print("\n".join(lines))
        return 1
    print("\n".join(lines))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
