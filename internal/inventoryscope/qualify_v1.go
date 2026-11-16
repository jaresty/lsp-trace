package inventoryscope

import (
	"bytes"
	"sort"
	"strings"
)

// QualifyV1 evaluates only supplied static facts and explicit caller intent.
// No observation is synthesized from an extension, hidden directory or language.
func QualifyV1(in InputV1, limits LimitsV1) (ManifestV1, error) {
	var m ManifestV1
	l, err := limits.EffectiveV1()
	if err != nil {
		return m, err
	}
	if err = boundInputV1(in, l); err != nil {
		return m, err
	}
	d := in.Declaration
	if in.Schema != SchemaV1 || !validID(d.SourceID) || !validID(d.Subject) || !validDigest(d.ConfigDigest) || (d.PolicyID != PolicyConsensusV1 && d.PolicyID != PolicyIntentFirstV1) {
		return m, ErrInvalidV1
	}
	if d.ApplicationRoots, err = sortedSetV1(d.ApplicationRoots); err != nil {
		return m, err
	}
	if d.Includes, err = sortedSetV1(d.Includes); err != nil {
		return m, err
	}
	if d.Excludes, err = sortedSetV1(d.Excludes); err != nil {
		return m, err
	}
	if d.RootBoundary && len(d.ApplicationRoots) == 0 {
		return m, ErrInvalidV1
	}
	for _, r := range d.ApplicationRoots {
		if r != "." && !CanonicalRelativePathV1(r) {
			return m, ErrInvalidV1
		}
	}
	candidates := append([]CandidateV1{}, in.Candidates...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].OriginalOrdinal < candidates[j].OriginalOrdinal })
	ids := make(map[string]CandidateV1, len(candidates))
	paths := make(map[string]bool, len(candidates))
	for i, c := range candidates {
		id, e := CandidateIDV1(d.SourceID, c.CanonicalRelativePath)
		if e != nil || c.OriginalOrdinal != i || id != c.CandidateID || !validRef(c.DiscoveryProvenance) || len(c.LanguageID) > 256 || paths[c.CanonicalRelativePath] {
			return m, ErrInvalidV1
		}
		if _, ok := ids[id]; ok {
			return m, ErrInvalidV1
		}
		ids[id] = c
		paths[c.CanonicalRelativePath] = true
	}
	inc, exc := map[string]bool{}, map[string]bool{}
	for _, p := range d.Includes {
		if !paths[p] {
			return m, ErrInvalidV1
		}
		inc[p] = true
	}
	for _, p := range d.Excludes {
		if !paths[p] {
			return m, ErrInvalidV1
		}
		exc[p] = true
	}
	observations := append([]ObservationV1{}, in.Observations...)
	sort.Slice(observations, func(i, j int) bool { return observations[i].ObservationID < observations[j].ObservationID })
	byCandidate := make(map[string][]ObservationV1)
	for i, o := range observations {
		if _, ok := ids[o.CandidateID]; !ok {
			return m, ErrInvalidV1
		}
		id, e := ObservationIDV1(o, l)
		if e != nil || id != o.ObservationID || i > 0 && observations[i-1].ObservationID == id || o.SourceID != d.SourceID || o.ConfigDigest != d.ConfigDigest || !validID(o.ProducerIdentity) || !validID(o.ProducerVersion) || !validRef(o.Artifact) {
			return m, ErrInvalidV1
		}
		if o.Assertion != IncludedV1 && o.Assertion != ExcludedV1 && o.Assertion != UnresolvedV1 {
			return m, ErrInvalidV1
		}
		switch o.Kind {
		case MembershipFactV1, BuildMembershipV1:
		case AnalysisFailureV1, UnreadableV1, UnsupportedLanguageV1, ResourceGapV1:
			if o.Assertion != UnresolvedV1 {
				return m, ErrInvalidV1
			}
		default:
			return m, ErrInvalidV1
		}
		byCandidate[o.CandidateID] = append(byCandidate[o.CandidateID], o)
	}
	resolutions := append([]ResolutionV1{}, in.Resolutions...)
	byResolution := make(map[string]ResolutionV1, len(resolutions))
	for i, r := range resolutions {
		if _, ok := ids[r.CandidateID]; !ok {
			return m, ErrInvalidV1
		}
		if _, ok := byResolution[r.CandidateID]; ok {
			return m, ErrInvalidV1
		}
		r.ObservationRefs, err = sortedSetV1(r.ObservationRefs)
		if err != nil {
			return m, err
		}
		id, e := ResolutionIDV1(r, l)
		if e != nil || id != r.ResolutionID || !validID(r.CallerActionRef) || r.Rule != ResolutionIntentFirstV1 || d.PolicyID != PolicyIntentFirstV1 || (r.Outcome != IncludedV1 && r.Outcome != ExcludedV1) {
			return m, ErrInvalidV1
		}
		refs := []string{}
		for _, o := range byCandidate[r.CandidateID] {
			refs = append(refs, o.ObservationID)
		}
		if !equalStringsV1(refs, r.ObservationRefs) {
			return m, ErrInvalidV1
		}
		resolutions[i] = r
		byResolution[r.CandidateID] = r
	}
	sort.Slice(resolutions, func(i, j int) bool { return resolutions[i].ResolutionID < resolutions[j].ResolutionID })
	m = ManifestV1{Schema: SchemaV1, OriginalDenominatorCount: len(candidates), OriginalDenominatorDigest: digestV1("inventory-denominator/v1", struct {
		Source     string
		Candidates []CandidateV1
	}{d.SourceID, candidates}), DeclarationDigest: digestV1("inventory-declaration/v1", d), ConfigDigest: d.ConfigDigest, PolicyDigest: digestV1("inventory-policy/v1", d.PolicyID), EvidenceDigest: digestV1("inventory-evidence/v1", observations), ResolutionDigest: digestV1("inventory-resolutions/v1", resolutions), Entries: []EntryV1{}}
	for _, c := range candidates {
		yes, no := inc[c.CanonicalRelativePath], exc[c.CanonicalRelativePath]
		outside := false
		if d.RootBoundary {
			outside = true
			for _, r := range d.ApplicationRoots {
				if r == "." || c.CanonicalRelativePath == r || strings.HasPrefix(c.CanonicalRelativePath, r+"/") {
					outside = false
					break
				}
			}
			if outside {
				no = true
			}
		}
		intentYes, intentNo := yes, no
		entry := EntryV1{CandidateID: c.CandidateID, Membership: UnresolvedV1, Reason: ReasonMissingV1, EvidenceRefs: []string{}}
		if yes || no {
			entry.EvidenceRefs = append(entry.EvidenceRefs, m.DeclarationDigest)
		}
		for _, o := range byCandidate[c.CandidateID] {
			entry.EvidenceRefs = append(entry.EvidenceRefs, o.ObservationID)
			if o.Kind == MembershipFactV1 || o.Kind == BuildMembershipV1 {
				yes = yes || o.Assertion == IncludedV1
				no = no || o.Assertion == ExcludedV1
			}
		}
		switch {
		case yes && no:
			entry.Reason = ReasonConflictV1
		case yes:
			entry.Membership = IncludedV1
			entry.Reason = ReasonEvidenceIncludedV1
			if intentYes {
				entry.Reason = ReasonCallerIncludedV1
			}
		case no:
			entry.Membership = ExcludedV1
			entry.Reason = ReasonEvidenceExcludedV1
			if intentNo {
				entry.Reason = ReasonCallerExcludedV1
			}
			if outside {
				entry.Reason = ReasonOutsideRootV1
			}
		}
		if r, ok := byResolution[c.CandidateID]; ok {
			// An explicit intent-first resolution must describe an actual conflict,
			// cite every observation, and choose the unambiguous declared intent.
			if !yes || !no || intentYes == intentNo || r.Outcome == IncludedV1 && !intentYes || r.Outcome == ExcludedV1 && !intentNo {
				return ManifestV1{}, ErrInvalidV1
			}
			entry.Membership = r.Outcome
			entry.Reason = ReasonResolvedV1
			entry.ResolutionRef = r.ResolutionID
		}
		sort.Strings(entry.EvidenceRefs)
		m.Entries = append(m.Entries, entry)
		switch entry.Membership {
		case IncludedV1:
			m.IncludedCount++
		case ExcludedV1:
			m.ExcludedCount++
		default:
			m.UnresolvedCount++
		}
	}
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].CandidateID < m.Entries[j].CandidateID })
	// No circular self-hash: the scope field is empty in the digest envelope.
	m.ScopeDigest = digestV1("inventory-scope/v1", m)
	if _, err = CanonicalManifestV1(m, l); err != nil {
		return ManifestV1{}, err
	}
	return m, nil
}

func equalStringsV1(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ValidateV1 recomputes the complete manifest; encoded counts or digests alone
// are not trusted. Set-like entry/reference ordering is immaterial.
func ValidateV1(in InputV1, m ManifestV1, l LimitsV1) error {
	actual, e := CanonicalManifestV1(m, l)
	if e != nil {
		return e
	}
	want, e := QualifyV1(in, l)
	if e != nil {
		return e
	}
	expected, e := CanonicalManifestV1(want, l)
	if e != nil {
		return e
	}
	if !bytes.Equal(actual, expected) {
		return ErrInvalidV1
	}
	return nil
}
