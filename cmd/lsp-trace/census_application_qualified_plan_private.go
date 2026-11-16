//go:build darwin || linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
)

const censusApplicationQualifiedFamilyPrivate = "census-qualified-application-preparation/v1"

var errCensusApplicationQualifiedPrivate = errors.New("invalid qualified application preparation")

type censusApplicationQualifiedOptionsPrivate struct {
	RevisionAssertion string
	Policy            pilot.ScopeEffectivePolicyPrivateV2
	// MaxArtifactBytes bounds the combined returned artifacts; MaxMetadataBytes
	// bounds the compact reference envelope. Neither raises a native quota.
	MaxArtifactBytes, MaxMetadataBytes int
}
type censusApplicationQualifiedRemainderPrivate struct {
	OriginalOrdinal int
	File            censusacquisition.SourceFile
	Entry           inventoryscope.EntryV1
}
type censusApplicationQualifiedGapPrivate struct {
	OriginalOrdinal int
	Reason          string
}
type censusApplicationQualifiedOriginPrivate struct {
	OriginalOrdinals []int
	ManifestRef      inventoryscope.ArtifactRefV1
}
type censusApplicationQualifiedFullPlanPrivate struct {
	InputFamily               string
	QualifiedPlan             censussourceplan.QualifiedInputV1
	ScopeLedger               inventoryscope.ManifestV1
	ScopeLedgerRef            inventoryscope.ArtifactRefV1
	OriginalDenominatorCount  int
	OriginalDenominatorDigest string
	AnalysisEligible          []int
	AnalysisGaps              []censusApplicationQualifiedGapPrivate
	UntouchedRemainder        []censusApplicationQualifiedRemainderPrivate
	Origins                   []censusApplicationQualifiedOriginPrivate
	Artifacts                 map[string][]byte
	// This fingerprint is a fresh requirement for FUTURE source admission. It
	// is not an authenticated source claim, permission, grant or reusable owner.
	SourceAuthorizationRequirementFingerprint string
	ExecutionDisposition                      string
}

type censusApplicationQualifiedFreezePrivate func(context.Context, string, string, []censusacquisition.SourceFile, pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error)

func censusApplicationQualifiedFreezeActualPrivate(ctx context.Context, workspace, revision string, files []censusacquisition.SourceFile, policy pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
	f, e := pilot.FreezeCensusScopePrivateV2(ctx, workspace, revision, files, policy)
	if e != nil {
		return nil, errCensusApplicationQualifiedPrivate
	}
	return f.ManifestBytes(), nil // Validation-only owner discarded; no native use.
}

// buildCensusApplicationQualifiedFullPlanPrivate performs real preparation via
// the existing freeze helper. It neither constructs an old inventory header nor
// calls git, Initialize, a provider, kernel admission, grants or reservations.
func buildCensusApplicationQualifiedFullPlanPrivate(ctx context.Context, q censussourceplan.QualifiedInputV1, o censusApplicationQualifiedOptionsPrivate, l inventoryscope.LimitsV1) (censusApplicationQualifiedFullPlanPrivate, []byte, error) {
	return buildCensusApplicationQualifiedFullPlanWithFreezePrivate(ctx, q, o, l, censusApplicationQualifiedFreezeActualPrivate)
}

func censusApplicationQualifiedArtifactDigestPrivate(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func buildCensusApplicationQualifiedFullPlanWithFreezePrivate(ctx context.Context, q censussourceplan.QualifiedInputV1, o censusApplicationQualifiedOptionsPrivate, l inventoryscope.LimitsV1, freeze censusApplicationQualifiedFreezePrivate) (censusApplicationQualifiedFullPlanPrivate, []byte, error) {
	var p censusApplicationQualifiedFullPlanPrivate
	if ctx == nil || freeze == nil || o.Policy.MaxNominationFiles < 1 || o.Policy.MaxNominationFiles > 8 || o.MaxArtifactBytes < 1 || o.MaxArtifactBytes > 32<<20 || o.MaxMetadataBytes < 1 || o.MaxMetadataBytes > 48000 || len(o.RevisionAssertion) == 0 || len(o.RevisionAssertion) > 256 {
		return p, nil, errCensusApplicationQualifiedPrivate
	}
	if e := ctx.Err(); e != nil {
		return p, nil, e
	}
	if e := censussourceplan.ValidateQualifiedV1(q, l); e != nil {
		return p, nil, e
	}
	// Rebuild owns a deep copy before callbacks can observe or alter caller data.
	owned, e := censussourceplan.BuildQualifiedV1(q.Originals, q.Input, q.Scope, q.Selection, q.Workspace, q.PartitionSize, l)
	if e != nil {
		return p, nil, e
	}
	q = owned
	p = censusApplicationQualifiedFullPlanPrivate{InputFamily: censusApplicationQualifiedFamilyPrivate, QualifiedPlan: q, ScopeLedger: q.Scope, OriginalDenominatorCount: q.Scope.OriginalDenominatorCount, OriginalDenominatorDigest: q.Scope.OriginalDenominatorDigest, AnalysisEligible: []int{}, AnalysisGaps: []censusApplicationQualifiedGapPrivate{}, UntouchedRemainder: []censusApplicationQualifiedRemainderPrivate{}, Origins: []censusApplicationQualifiedOriginPrivate{}, Artifacts: map[string][]byte{}, ExecutionDisposition: "UNFUNDED_CONNECTED_EXECUTION_PENDING"}
	used := 0
	addArtifact := func(id string, b []byte) (inventoryscope.ArtifactRefV1, error) {
		if len(b) > o.MaxArtifactBytes-used {
			return inventoryscope.ArtifactRefV1{}, errCensusApplicationQualifiedPrivate
		}
		used += len(b)
		p.Artifacts[id] = append([]byte(nil), b...)
		return inventoryscope.ArtifactRefV1{ArtifactID: id, Digest: censusApplicationQualifiedArtifactDigestPrivate(b)}, nil
	}
	scopeBytes, e := inventoryscope.CanonicalManifestV1(q.Scope, l)
	if e != nil {
		return p, nil, e
	}
	p.ScopeLedgerRef, e = addArtifact("qualification-ledger.v1.json", scopeBytes)
	if e != nil {
		return p, nil, e
	}
	planBytes, e := json.Marshal(q)
	if e != nil {
		return p, nil, errCensusApplicationQualifiedPrivate
	}
	planRef, e := addArtifact("qualified-source-plan.v1.json", planBytes)
	if e != nil {
		return p, nil, e
	}
	byID := map[string]inventoryscope.EntryV1{}
	for _, v := range q.Scope.Entries {
		byID[v.CandidateID] = v
	}
	byOrdinal := make([]inventoryscope.EntryV1, len(q.Originals))
	for _, v := range q.Input.Candidates {
		byOrdinal[v.OriginalOrdinal] = byID[v.CandidateID]
	}
	for ordinal, file := range q.Originals {
		entry := byOrdinal[ordinal]
		if entry.Membership != inventoryscope.IncludedV1 {
			p.UntouchedRemainder = append(p.UntouchedRemainder, censusApplicationQualifiedRemainderPrivate{ordinal, file, entry})
		}
	}
	// Partitions remain the qualified plan's actual partitions. Analysis gaps
	// stay INCLUDED in ScopeLedger and are not passed to the old freeze helper.
	index := 0
	for _, partition := range q.AnalysisPlan.Partitions {
		files := []censusacquisition.SourceFile{}
		ordinals := []int{}
		for _, file := range partition.Members {
			ordinal := q.OriginalOrdinals[index]
			index++
			if file.Disposition != "" || file.LanguageID == "" {
				p.AnalysisGaps = append(p.AnalysisGaps, censusApplicationQualifiedGapPrivate{ordinal, "ANALYSIS_UNAVAILABLE_NOT_MEMBERSHIP_EXCLUSION"})
				continue
			}
			files = append(files, file)
			ordinals = append(ordinals, ordinal)
			p.AnalysisEligible = append(p.AnalysisEligible, ordinal)
		}
		for start := 0; start < len(files); start += o.Policy.MaxNominationFiles {
			end := start + o.Policy.MaxNominationFiles
			if end > len(files) {
				end = len(files)
			}
			if e = ctx.Err(); e != nil {
				return p, nil, e
			}
			manifest, e := freeze(ctx, q.Workspace, o.RevisionAssertion, files[start:end], o.Policy)
			if e != nil {
				return p, nil, errCensusApplicationQualifiedPrivate
			}
			if len(manifest) == 0 || len(manifest) > o.MaxArtifactBytes-used || !json.Valid(manifest) {
				return p, nil, errCensusApplicationQualifiedPrivate
			}
			// Content-addressed metadata artifacts, never serialized live owners.
			id := "frozen-origin-" + censusApplicationQualifiedArtifactDigestPrivate(manifest)[7:] + ".json"
			ref, e := addArtifact(id, manifest)
			if e != nil {
				return p, nil, e
			}
			p.Origins = append(p.Origins, censusApplicationQualifiedOriginPrivate{append([]int(nil), ordinals[start:end]...), ref})
		}
	}
	fingerprintBytes, _ := json.Marshal(struct {
		Family, QualifiedDigest string
		Scope, Plan             inventoryscope.ArtifactRefV1
		Options                 censusApplicationQualifiedOptionsPrivate
		Origins                 []censusApplicationQualifiedOriginPrivate
	}{p.InputFamily, q.QualifiedDigest, p.ScopeLedgerRef, planRef, o, p.Origins})
	p.SourceAuthorizationRequirementFingerprint = censusApplicationQualifiedArtifactDigestPrivate(append([]byte("lsp-trace:qualified-source-authorization-requirement/v1\n"), fingerprintBytes...))
	// Compact metadata refers to bounded artifacts rather than inlining the
	// entire 1,099-row historical denominator or inventing a new actual scope.
	raw, e := json.Marshal(struct {
		InputFamily, ExecutionDisposition                                       string
		Authority                                                               int
		Accepted                                                                bool
		FeatureIdentity, AbsenceInference                                       string
		OriginalDenominatorCount                                                int
		OriginalDenominatorDigest, QualifiedDigest                              string
		ScopeLedger, QualifiedPlan                                              inventoryscope.ArtifactRefV1
		Included, Excluded, Unresolved, AnalysisEligible, AnalysisGaps, Origins int
		SourceAuthorizationRequirementFingerprint                               string
	}{p.InputFamily, p.ExecutionDisposition, 0, false, "UNRESOLVED", "PROHIBITED", p.OriginalDenominatorCount, p.OriginalDenominatorDigest, q.QualifiedDigest, p.ScopeLedgerRef, planRef, q.Scope.IncludedCount, q.Scope.ExcludedCount, q.Scope.UnresolvedCount, len(p.AnalysisEligible), len(p.AnalysisGaps), len(p.Origins), p.SourceAuthorizationRequirementFingerprint})
	if e != nil || len(raw) > o.MaxMetadataBytes {
		return p, nil, errCensusApplicationQualifiedPrivate
	}
	return p, raw, ctx.Err()
}
