package censussourceplan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/inventoryscope"
)

const QualifiedFamilyV1 = "census-qualified-source-plan/v1"

// QualifiedInputV1 is preparation data, not a native input, grant or source
// claim. AnalysisPlan.Identity is an internal legacy representation identity;
// only QualifiedDigest binds the new qualification family.
type QualifiedInputV1 struct {
	InputFamily   string
	Input         inventoryscope.InputV1
	Scope         inventoryscope.ManifestV1
	Selection     inventoryscope.SelectionV1
	Originals     []censusacquisition.SourceFile
	Workspace     string
	PartitionSize int
	AnalysisPlan  Plan
	// OriginalOrdinals is parallel to AnalysisPlan.Files, not a renumbering.
	OriginalOrdinals []int
	NoWork           bool
	QualifiedDigest  string
}

func BuildQualifiedV1(originals []censusacquisition.SourceFile, in inventoryscope.InputV1, m inventoryscope.ManifestV1, s inventoryscope.SelectionV1, workspace string, partitionSize int, limits inventoryscope.LimitsV1) (QualifiedInputV1, error) {
	var q QualifiedInputV1
	l, e := limits.EffectiveV1()
	if e != nil {
		return q, e
	}
	if len(originals) > l.MaxCandidates || len(originals) != len(in.Candidates) || len(workspace) > 16384 || strings.ContainsRune(workspace, 0) || !utf8.ValidString(workspace) || partitionSize < 1 || partitionSize > MaxPartitionSize {
		return q, ErrInvalidIdentity
	}
	if _, e = qualifiedWorkspaceURIPathV1(workspace); e != nil {
		return q, e
	}
	if e = inventoryscope.ValidateV1(in, m, l); e != nil {
		return q, e
	}
	if e = inventoryscope.ValidateSelectionV1(s, m); e != nil {
		return q, e
	}
	if s.State != inventoryscope.CallerSelectedV1 {
		return q, inventoryscope.ErrSelectionV1
	}
	remaining := l.MaxUTF8Bytes
	for _, f := range originals {
		for _, v := range []string{f.Path, f.URI, f.LanguageID, string(f.Disposition)} {
			if len(v) > remaining || !utf8.ValidString(v) {
				return q, ErrInvalidIdentity
			}
			remaining -= len(v)
		}
	}
	// Validate every original source binding, including non-included members.
	// Never route qualified identity through the legacy platform case policy.
	policy := qualifiedPathIdentityV1()
	if len(originals) > 0 {
		if _, e = buildWithPathIdentity(originals, workspace, partitionSize, policy); e != nil {
			return q, e
		}
	}
	ordinals := make(map[string]int, len(originals))
	for _, c := range in.Candidates {
		if c.OriginalOrdinal < 0 || c.OriginalOrdinal >= len(originals) {
			return q, ErrInvalidIdentity
		}
		f := originals[c.OriginalOrdinal]
		if f.Path != c.CanonicalRelativePath || f.LanguageID != c.LanguageID {
			return q, ErrInvalidIdentity
		}
		ordinals[c.CandidateID] = c.OriginalOrdinal
	}
	members := []censusacquisition.SourceFile{}
	for _, entry := range m.Entries {
		if entry.Membership == inventoryscope.IncludedV1 {
			members = append(members, originals[ordinals[entry.CandidateID]])
		}
	}
	plan := Plan{Files: []censusacquisition.SourceFile{}, Partitions: []Partition{}}
	if len(members) > 0 {
		plan, e = buildWithPathIdentity(members, workspace, partitionSize, policy)
		if e != nil {
			return q, e
		}
	}
	mapping := []int{}
	byPath := make(map[string]int, len(originals))
	for i, f := range originals {
		byPath[f.Path] = i
	}
	for _, f := range plan.Files {
		mapping = append(mapping, byPath[f.Path])
	}
	// Deep-copy the validated caller-owned evidence; no retained aliases.
	encoded, e := json.Marshal(in)
	if e != nil || int64(len(encoded)) > l.MaxInputBytes {
		return q, inventoryscope.ErrLimitV1
	}
	var input inventoryscope.InputV1
	if json.Unmarshal(encoded, &input) != nil {
		return q, ErrInvalidIdentity
	}
	// The evidence ledger is set-like except for original ordinal identity.
	// Canonicalize only this owned copy, preserving every original ordinal.
	if input.Declaration.ApplicationRoots == nil {
		input.Declaration.ApplicationRoots = []string{}
	}
	if input.Declaration.Includes == nil {
		input.Declaration.Includes = []string{}
	}
	if input.Declaration.Excludes == nil {
		input.Declaration.Excludes = []string{}
	}
	if input.Candidates == nil {
		input.Candidates = []inventoryscope.CandidateV1{}
	}
	if input.Observations == nil {
		input.Observations = []inventoryscope.ObservationV1{}
	}
	if input.Resolutions == nil {
		input.Resolutions = []inventoryscope.ResolutionV1{}
	}
	sort.Strings(input.Declaration.ApplicationRoots)
	sort.Strings(input.Declaration.Includes)
	sort.Strings(input.Declaration.Excludes)
	sort.Slice(input.Candidates, func(i, j int) bool { return input.Candidates[i].OriginalOrdinal < input.Candidates[j].OriginalOrdinal })
	sort.Slice(input.Observations, func(i, j int) bool { return input.Observations[i].ObservationID < input.Observations[j].ObservationID })
	for i := range input.Resolutions {
		sort.Strings(input.Resolutions[i].ObservationRefs)
	}
	sort.Slice(input.Resolutions, func(i, j int) bool { return input.Resolutions[i].ResolutionID < input.Resolutions[j].ResolutionID })
	scope, e := inventoryscope.QualifyV1(input, l)
	if e != nil {
		return q, e
	}
	q = QualifiedInputV1{InputFamily: QualifiedFamilyV1, Input: input, Scope: scope, Selection: s, Originals: append([]censusacquisition.SourceFile{}, originals...), Workspace: workspace, PartitionSize: partitionSize, AnalysisPlan: plan, OriginalOrdinals: mapping, NoWork: len(members) == 0}
	q.QualifiedDigest = qualifiedDigestV1(q)
	return q, nil
}

func qualifiedPathIdentityV1() pathIdentityPolicy {
	return pathIdentityPolicy{
		key:      func(value string) string { return value },
		relative: qualifiedWorkspaceRelativePathV1,
	}
}

// qualifiedWorkspaceURIPathV1 interprets an explicitly supplied lexical root;
// it does not select one or query host filesystem semantics. Drive roots accept
// either consistent slash spelling. The original workspace string remains in
// the qualified digest, so no identity is transferred across root spellings.
func qualifiedWorkspaceURIPathV1(workspace string) (string, error) {
	if workspace == "" || len(workspace) > 16384 || !utf8.ValidString(workspace) || strings.ContainsRune(workspace, 0) {
		return "", ErrInvalidIdentity
	}
	root := workspace
	drive := len(root) >= 3 && (root[0] >= 'A' && root[0] <= 'Z' || root[0] >= 'a' && root[0] <= 'z') && root[1] == ':' && (root[2] == '/' || root[2] == '\\')
	if drive {
		if root[2] == '/' && strings.Contains(root[3:], "\\") || root[2] == '\\' && strings.Contains(root[3:], "/") {
			return "", ErrInvalidIdentity
		}
		root = "/" + strings.ReplaceAll(root, "\\", "/")
	}
	driveRoot := drive && len(root) == 4 // /C:/ is the canonical URI drive root.
	if !path.IsAbs(root) || strings.Contains(root, "\\") || path.Clean(root) != root && !driveRoot {
		return "", ErrInvalidIdentity
	}
	return root, nil
}

func qualifiedWorkspaceRelativePathV1(workspace, rawURI string) (string, error) {
	root, err := qualifiedWorkspaceURIPathV1(workspace)
	if err != nil {
		return "", err
	}
	uriPath, err := canonicalFileURIPath(rawURI)
	prefix := strings.TrimSuffix(root, "/") + "/"
	if err != nil || !strings.HasPrefix(uriPath, prefix) {
		return "", ErrInvalidIdentity
	}
	relative := strings.TrimPrefix(uriPath, prefix)
	if !validRelativePath(relative) {
		return "", ErrInvalidIdentity
	}
	return relative, nil
}

func qualifiedDigestV1(q QualifiedInputV1) string {
	// Input's complete facts are already bound through Scope. Do not include the
	// self-digest; the old AnalysisPlan identity alone never validates this value.
	b, _ := json.Marshal(struct {
		Family           string
		Scope            inventoryscope.ManifestV1
		Selection        inventoryscope.SelectionV1
		Originals        []censusacquisition.SourceFile
		Workspace        string
		PartitionSize    int
		AnalysisPlan     Plan
		OriginalOrdinals []int
		NoWork           bool
	}{q.InputFamily, q.Scope, q.Selection, q.Originals, q.Workspace, q.PartitionSize, q.AnalysisPlan, q.OriginalOrdinals, q.NoWork})
	h := sha256.New()
	h.Write([]byte("lsp-trace:census-qualified-source-plan/v1\n"))
	h.Write(b)
	h.Write([]byte{'\n'})
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func ValidateQualifiedV1(q QualifiedInputV1, l inventoryscope.LimitsV1) error {
	if q.InputFamily != QualifiedFamilyV1 {
		return ErrInvalidIdentity
	}
	effective, e := l.EffectiveV1()
	if e != nil {
		return e
	}
	// Bound all externally mutable plan collections before recomputation/copy.
	if len(q.AnalysisPlan.Files) > effective.MaxCandidates || len(q.AnalysisPlan.Partitions) > effective.MaxCandidates || len(q.OriginalOrdinals) > effective.MaxCandidates {
		return inventoryscope.ErrLimitV1
	}
	members := 0
	chars := effective.MaxUTF8Bytes
	for _, p := range q.AnalysisPlan.Partitions {
		if len(p.Members) > effective.MaxCandidates-members {
			return inventoryscope.ErrLimitV1
		}
		members += len(p.Members)
		for _, f := range p.Members {
			for _, s := range []string{f.Path, f.URI, f.LanguageID, string(f.Disposition)} {
				if len(s) > chars {
					return inventoryscope.ErrLimitV1
				}
				chars -= len(s)
			}
		}
	}
	for _, f := range q.AnalysisPlan.Files {
		for _, s := range []string{f.Path, f.URI, f.LanguageID, string(f.Disposition)} {
			if len(s) > chars {
				return inventoryscope.ErrLimitV1
			}
			chars -= len(s)
		}
	}
	if len(q.QualifiedDigest) > 71 || len(q.AnalysisPlan.Identity) > 71 {
		return ErrInvalidIdentity
	}
	want, e := BuildQualifiedV1(q.Originals, q.Input, q.Scope, q.Selection, q.Workspace, q.PartitionSize, l)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(q)
	b, _ := json.Marshal(want)
	if !bytes.Equal(a, b) {
		return ErrInvalidIdentity
	}
	return nil
}
