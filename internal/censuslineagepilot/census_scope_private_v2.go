package censuslineagepilot

import (
	"context"
	"lsp-trace/internal/censusacquisition"
)

// FreezeCensusScopePrivateV2 derives members from the Census enumerator, not a
// caller-authored feature/target inventory. It is a fresh original-source owner,
// never an importer of serialized discovery or previously captured authority.
// The caller retains the entire enumeration denominator, including entries it
// cannot pass here. Existing V2 source, alias, byte and metadata guards apply.
func FreezeCensusScopePrivateV2(ctx context.Context, root, revision string, files []censusacquisition.SourceFile, policy ScopeEffectivePolicyPrivateV2) (FrozenScopePrivateV2, error) {
	if ctx == nil || policy.validate() != nil || len(files) < 1 || len(files) > policy.MaxNominationFiles || !featureScopeRevision.MatchString(revision) {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	if _, err := NewSourceSelection(root, files); err != nil {
		return FrozenScopePrivateV2{}, err
	}
	descriptors := make([]scopeReadDescriptorPrivate, len(files))
	for i, f := range files {
		if f.Disposition != "" || f.LanguageID == "" {
			return FrozenScopePrivateV2{}, ErrInvalid
		}
		descriptors[i] = scopeReadDescriptorPrivate{f.Path, "NOMINATION"}
	}
	observed, _, err := readScopeDescriptorsPrivate(ctx, root, descriptors, scopeReadBoundsPrivate{policy.FileBytes, policy.TotalBytes, policy.MaxPathDepth, true}, scopeReadOps{})
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	members := make([]ScopeMemberPrivateV2, len(files))
	for i, f := range files {
		members[i] = ScopeMemberPrivateV2{CanonicalRelativePath: f.Path, Role: "NOMINATION", LanguageID: f.LanguageID, ExpectedSHA256: observed[i].SHA256, ByteLength: observed[i].ByteLength}
	}
	// Freeze independently reopens and compares the declared bytes. A change
	// in bytes between observation and freeze fails instead of changing the scope.
	return FreezeScopePrivateV2(ctx, ScopeSpecPrivateV2{Root: root, RevisionAssertion: revision, Members: members, PartitionSize: len(files), EffectivePolicy: policy})
}
