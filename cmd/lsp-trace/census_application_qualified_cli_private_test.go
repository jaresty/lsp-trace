//go:build census_qualified_cli_isolated

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/inventoryscope"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func qualifiedCLIFixturePrivate(t *testing.T) censusQualifiedCLIEnvelopePrivate {
	t.Helper()
	root := t.TempDir()
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	workspace := filepath.Join(root, "synthetic")
	output := filepath.Join(root, "output")
	for _, p := range []string{workspace, output} {
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(workspace, "A.ts"), []byte("export const controlled = 1;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "synthetic-only", ApplicationRoots: []string{}, Includes: []string{"A.ts", ".hidden/gap.go"}, Excludes: []string{"a.ts"}, SourceID: "caller-universe", ConfigDigest: "sha256:" + strings.Repeat("a", 64), PolicyID: inventoryscope.PolicyConsensusV1}, Candidates: []inventoryscope.CandidateV1{}, Observations: []inventoryscope.ObservationV1{}, Resolutions: []inventoryscope.ResolutionV1{}}
	files := []censusacquisition.SourceFile{{Path: "A.ts", LanguageID: "typescript"}, {Path: "a.ts", LanguageID: "typescript"}, {Path: ".hidden/gap.go", LanguageID: "go", Disposition: census.FileUnsupported}, {Path: "conflict.ts", LanguageID: "typescript"}, {Path: "missing.ts", LanguageID: "typescript"}}
	for i := range files {
		f := &files[i]
		f.URI = "file://" + workspace + "/" + f.Path
		id, e := inventoryscope.CandidateIDV1(in.Declaration.SourceID, f.Path)
		if e != nil {
			t.Fatal(e)
		}
		in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: f.Path, CandidateID: id, LanguageID: f.LanguageID, DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "explicit-unpruned-list", Digest: in.Declaration.ConfigDigest}})
	}
	for i, outcome := range []inventoryscope.MembershipV1{inventoryscope.IncludedV1, inventoryscope.ExcludedV1} {
		in.Observations = append(in.Observations, inventoryscope.ObservationV1{ObservationID: []string{"include-conflict", "exclude-conflict"}[i], CandidateID: in.Candidates[3].CandidateID, Assertion: outcome, Kind: inventoryscope.MembershipFactV1, ProducerIdentity: "controlled", ProducerVersion: "v1", Artifact: inventoryscope.ArtifactRefV1{ArtifactID: "controlled-evidence", Digest: in.Declaration.ConfigDigest}, SourceID: in.Declaration.SourceID, ConfigDigest: in.Declaration.ConfigDigest})
	}
	for i := range in.Observations {
		id, e := inventoryscope.ObservationIDV1(in.Observations[i], inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		in.Observations[i].ObservationID = id
	}
	m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	policy := pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 2, MaxContextFiles: 1, MaxMembers: 3, FileBytes: 4096, TotalBytes: 8192, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 2, MaxSymbols: 8, MaxDepth: 8, MaxStringBytes: 4096}, MaxTargets: 4, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 4}, MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 4096, MaxPathWork: 1000, TimeoutMS: 30000, RequestTimeoutMS: 1000, MaxResponseBytes: 4096, MaxMessages: 8}
	return censusQualifiedCLIEnvelopePrivate{Family: censusQualifiedCLIFamilyPrivate, Universe: censusQualifiedUniversePrivate{Complete: true, Claim: "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED", ListMethod: "caller-authored synthetic unpruned explicit list", Provenance: inventoryscope.ArtifactRefV1{ArtifactID: "controlled-list", Digest: in.Declaration.ConfigDigest}, OriginalDenominatorCount: m.OriginalDenominatorCount, OriginalDenominatorDigest: m.OriginalDenominatorDigest}, Originals: files, Qualification: in, Selection: inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "controlled-selection"}, Workspace: workspace, PartitionSize: 2, Preparation: censusApplicationQualifiedOptionsPrivate{RevisionAssertion: strings.Repeat("a", 40), Policy: policy, MaxArtifactBytes: 1 << 20, MaxMetadataBytes: 48000}, TimeoutMS: 30000, OutputDirectory: output, OutputName: "prepared"}
}
func qualifiedCLIRebindPrivate(t *testing.T, in *censusQualifiedCLIEnvelopePrivate) {
	t.Helper()
	m, e := inventoryscope.QualifyV1(in.Qualification, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	in.Selection.ManifestDigest = m.ScopeDigest
	in.Universe.OriginalDenominatorCount = m.OriginalDenominatorCount
	in.Universe.OriginalDenominatorDigest = m.OriginalDenominatorDigest
}
func qualifiedCLIRunPrivate(t *testing.T, in censusQualifiedCLIEnvelopePrivate) (int, []byte, []byte) {
	t.Helper()
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	return qualifiedCLIRunRawPrivate(t, in, b)
}
func qualifiedCLIRunRawPrivate(t *testing.T, in censusQualifiedCLIEnvelopePrivate, b []byte) (int, []byte, []byte) {
	t.Helper()
	path := filepath.Join(filepath.Dir(in.Workspace), "input.json")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	refuse := func() { calls++; t.Error("ASSERT preparation runtime effects zero: FAIL") }
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) { refuse(); return nil, errors.New("spy refusal") }, newManager: func(sessionruntime.Config) (*sessionruntime.Manager, error) {
		refuse()
		return nil, errors.New("spy refusal")
	}, abs: func(string) (string, error) { refuse(); return "", errors.New("spy refusal") }, lookPath: func(string) (string, error) { refuse(); return "", errors.New("spy refusal") }, openRoot: func(string) (*publication.Root, error) { refuse(); return nil, errors.New("spy refusal") }, enumeratePreflight: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
		refuse()
		return nil, errors.New("spy refusal")
	}}
	before := qualifiedProfileCallsPrivate
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies([]string{"--machine", "--private-inventory-qualified-prepare-config", path}, &stdout, &stderr, deps)
	if calls != 0 || before != qualifiedProfileCallsPrivate {
		t.Fatal("ASSERT preparation runtime effects zero: FAIL")
	}
	t.Log("ASSERT preparation runtime effects zero: PASS")
	return code, stdout.Bytes(), stderr.Bytes()
}
func TestQualifiedCLIConnectedPrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	encoded, _ := json.Marshal(in)
	if _, e := decodeCensusQualifiedCLIPrivate(encoded, inventoryscope.LimitsV1{}); e != nil {
		t.Fatal("fixture strict decode", e)
	}
	qualified, e := validateCensusQualifiedCLIPrivate(in)
	if e != nil {
		t.Fatal("fixture qualified validation", e)
	}
	if _, _, e := buildCensusApplicationQualifiedFullPlanPrivate(context.Background(), qualified, in.Preparation, inventoryscope.LimitsV1{}); e != nil {
		t.Fatal("fixture actual freeze", e)
	}
	code, raw, errout := qualifiedCLIRunPrivate(t, in)
	if code != 0 {
		t.Fatalf("ASSERT real qualified preparation route succeeds: FAIL code=%d diagnostic=%s", code, errout)
	}
	t.Log("ASSERT real qualified preparation route succeeds: PASS")
	var r censusQualifiedCLIResultPrivate
	if e := json.Unmarshal(raw, &r); e != nil {
		t.Fatal(e)
	}
	if !r.Prepared || r.Authority != 0 || r.Accepted || r.ExecutionDisposition != "UNFUNDED_CONNECTED_EXECUTION_PENDING" || r.FeatureIdentity != "UNRESOLVED" || r.AbsenceInference != "PROHIBITED" {
		t.Fatal("ASSERT preparation remains unfunded unresolved: FAIL")
	}
	t.Log("ASSERT preparation remains unfunded unresolved: PASS")
	if len(raw) > 48000 || len(raw) > in.Preparation.MaxMetadataBytes || bytes.Contains(raw, []byte("Artifacts")) || r.Returned+r.Omitted != r.Retained {
		t.Fatal("ASSERT compact bounded metadata: FAIL")
	}
	t.Log("ASSERT compact bounded metadata: PASS")
	output := filepath.Join(in.OutputDirectory, in.OutputName)
	if st, e := os.Stat(output); e != nil || st.Mode().Perm() != 0700 {
		t.Fatal("private output directory")
	}
	b, e := os.ReadFile(filepath.Join(output, r.Index.Name))
	if e != nil || censusApplicationQualifiedArtifactDigestPrivate(b) != r.Index.Digest {
		t.Fatal("ASSERT durable referenced publication: FAIL")
	}
	var refs []censusQualifiedCLIRefPrivate
	if e = json.Unmarshal(b, &refs); e != nil {
		t.Fatal(e)
	}
	for _, ref := range refs {
		path := filepath.Join(output, ref.Name)
		b, e := os.ReadFile(path)
		st, se := os.Lstat(path)
		if e != nil || se != nil || st.Mode().Perm() != 0600 || len(b) != ref.Bytes || censusApplicationQualifiedArtifactDigestPrivate(b) != ref.Digest {
			t.Fatal("ASSERT durable referenced publication: FAIL")
		}
	}
	t.Log("ASSERT durable referenced publication: PASS")
	plan, e := os.ReadFile(filepath.Join(output, "qualified-source-plan.v1.json"))
	if e != nil {
		t.Fatal(e)
	}
	var q struct {
		Originals        []censusacquisition.SourceFile
		Scope            inventoryscope.ManifestV1
		OriginalOrdinals []int
	}
	if e = json.Unmarshal(plan, &q); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(in.Originals, q.Originals) || len(q.Scope.Entries) != 5 || q.Scope.IncludedCount != 2 || q.Scope.ExcludedCount != 1 || q.Scope.UnresolvedCount != 2 || !reflect.DeepEqual(q.OriginalOrdinals, []int{2, 0}) {
		t.Fatal("ASSERT full original mixed ledger preserved: FAIL")
	}
	t.Log("ASSERT full original mixed ledger preserved: PASS")
	ledger := map[string]inventoryscope.EntryV1{}
	for _, v := range q.Scope.Entries {
		if _, ok := ledger[v.CandidateID]; ok {
			t.Fatal("duplicate ledger")
		}
		ledger[v.CandidateID] = v
	}
	if ledger[in.Qualification.Candidates[3].CandidateID].Reason != inventoryscope.ReasonConflictV1 || ledger[in.Qualification.Candidates[4].CandidateID].Reason != inventoryscope.ReasonMissingV1 {
		t.Fatal("missing/conflicting evidence not unresolved")
	}
}
func TestQualifiedCLIRefusalsPrivate(t *testing.T) {
	cases := []struct {
		name   string
		change func(*censusQualifiedCLIEnvelopePrivate)
	}{
		{"missing-complete", func(in *censusQualifiedCLIEnvelopePrivate) { in.Universe.Complete = false }},
		{"count", func(in *censusQualifiedCLIEnvelopePrivate) { in.Universe.OriginalDenominatorCount++ }},
		{"digest", func(in *censusQualifiedCLIEnvelopePrivate) {
			in.Universe.OriginalDenominatorDigest = in.Qualification.Declaration.ConfigDigest
		}},
		{"unselected", func(in *censusQualifiedCLIEnvelopePrivate) {
			in.Selection.State = inventoryscope.ProposedV1
			in.Selection.CallerActionRef = ""
		}},
		{"selection-substitution", func(in *censusQualifiedCLIEnvelopePrivate) {
			in.Selection.ManifestDigest = in.Qualification.Declaration.ConfigDigest
		}},
		{"source-binding", func(in *censusQualifiedCLIEnvelopePrivate) { in.Originals[1].URI = in.Originals[0].URI }},
		{"policy-extreme", func(in *censusQualifiedCLIEnvelopePrivate) { in.Preparation.Policy.MaxNominationFiles = 9 }},
		{"metadata-bound", func(in *censusQualifiedCLIEnvelopePrivate) { in.Preparation.MaxMetadataBytes = 1 }},
		{"artifact-bound", func(in *censusQualifiedCLIEnvelopePrivate) { in.Preparation.MaxArtifactBytes = 1 }},
		{"publication-exists", func(in *censusQualifiedCLIEnvelopePrivate) {
			if e := os.Mkdir(filepath.Join(in.OutputDirectory, in.OutputName), 0700); e != nil {
				t.Fatal(e)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := qualifiedCLIFixturePrivate(t)
			tc.change(&in)
			code, raw, errout := qualifiedCLIRunPrivate(t, in)
			if code == 0 || len(raw) != 0 || len(errout) == 0 {
				t.Fatal("ASSERT invalid preparation has no Prepared result: FAIL")
			}
			t.Log("ASSERT invalid preparation has no Prepared result: PASS")
		})
	}
}
func TestQualifiedCLINoWorkPrivate(t *testing.T) {
	for _, mode := range []string{"zero-included", "all-unsupported"} {
		t.Run(mode, func(t *testing.T) {
			in := qualifiedCLIFixturePrivate(t)
			in.Qualification.Declaration.Includes = []string{}
			in.Qualification.Observations = []inventoryscope.ObservationV1{}
			if mode == "all-unsupported" {
				in.Qualification.Declaration.Excludes = []string{}
				for i := range in.Originals {
					in.Originals[i].Disposition = census.FileUnsupported
					in.Qualification.Declaration.Includes = append(in.Qualification.Declaration.Includes, in.Originals[i].Path)
				}
			}
			qualifiedCLIRebindPrivate(t, &in)
			code, raw, errout := qualifiedCLIRunPrivate(t, in)
			if code != 0 {
				t.Fatalf("ASSERT no-work and unsupported preparation legal: FAIL %s", errout)
			}
			var r censusQualifiedCLIResultPrivate
			_ = json.Unmarshal(raw, &r)
			if !r.Prepared {
				t.Fatal("not prepared")
			}
			t.Log("ASSERT no-work and unsupported preparation legal: PASS")
		})
	}
}
func TestQualifiedCLIIdentityPrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	q, e := validateCensusQualifiedCLIPrivate(in)
	if e != nil {
		t.Fatal(e)
	}
	base, id, e := censusQualifiedCLIIdentityPrivate(in, q)
	if e != nil {
		t.Fatal(e)
	}
	tests := []struct {
		name   string
		change func(*censusQualifiedCLIEnvelopePrivate)
	}{
		{"list-method", func(v *censusQualifiedCLIEnvelopePrivate) { v.Universe.ListMethod += " changed" }},
		{"provenance", func(v *censusQualifiedCLIEnvelopePrivate) { v.Universe.Provenance.ArtifactID += "-changed" }},
		{"selection-action", func(v *censusQualifiedCLIEnvelopePrivate) { v.Selection.CallerActionRef = "other-action" }},
		{"revision", func(v *censusQualifiedCLIEnvelopePrivate) { v.Preparation.RevisionAssertion = strings.Repeat("b", 40) }},
		{"policy", func(v *censusQualifiedCLIEnvelopePrivate) { v.Preparation.Policy.MaxNodes++ }},
		{"budget", func(v *censusQualifiedCLIEnvelopePrivate) { v.Preparation.MaxArtifactBytes++ }},
		{"metadata-budget", func(v *censusQualifiedCLIEnvelopePrivate) { v.Preparation.MaxMetadataBytes-- }},
		{"partition", func(v *censusQualifiedCLIEnvelopePrivate) { v.PartitionSize++ }},
		{"timeout", func(v *censusQualifiedCLIEnvelopePrivate) { v.TimeoutMS-- }},
		{"destination", func(v *censusQualifiedCLIEnvelopePrivate) { v.OutputName += "-other" }},
		{"qualification-config", func(v *censusQualifiedCLIEnvelopePrivate) {
			v.Qualification.Declaration.ConfigDigest = "sha256:" + strings.Repeat("b", 64)
			for i := range v.Qualification.Observations {
				v.Qualification.Observations[i].ConfigDigest = v.Qualification.Declaration.ConfigDigest
				id, e := inventoryscope.ObservationIDV1(v.Qualification.Observations[i], inventoryscope.LimitsV1{})
				if e != nil {
					t.Fatal(e)
				}
				v.Qualification.Observations[i].ObservationID = id
			}
			qualifiedCLIRebindPrivate(t, v)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(in)
			var v censusQualifiedCLIEnvelopePrivate
			_ = json.Unmarshal(b, &v)
			tc.change(&v)
			q, e := validateCensusQualifiedCLIPrivate(v)
			if e != nil {
				t.Fatal(e)
			}
			payload, changed, e := censusQualifiedCLIIdentityPrivate(v, q)
			if e != nil || bytes.Equal(base, payload) || id == changed {
				t.Fatal("ASSERT changed bound input changes preparation identity payload: FAIL")
			}
			t.Log("ASSERT changed bound input changes preparation identity payload: PASS")
		})
	}
}
func TestQualifiedCLIStrictDecodePrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	raw, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = decodeCensusQualifiedCLIPrivate(raw, inventoryscope.LimitsV1{}); e != nil {
		t.Fatal(e)
	}
	variants := map[string][]byte{
		"outer-null": []byte("null"), "leading": append([]byte(" "), raw...), "trailing": append(append([]byte{}, raw...), ' '), "two-values": append(append([]byte{}, raw...), raw...),
		"duplicate":        bytes.Replace(raw, []byte(`"complete":true`), []byte(`"complete":true,"complete":true`), 1),
		"unknown":          bytes.Replace(raw, []byte(`"complete":true`), []byte(`"complete":true,"unknown":1`), 1),
		"case":             bytes.Replace(raw, []byte(`"complete":true`), []byte(`"Complete":true`), 1),
		"nested-null":      bytes.Replace(raw, []byte(`"resolutions":[]`), []byte(`"resolutions":null`), 1),
		"nested-case":      bytes.Replace(raw, []byte(`"source_id":`), []byte(`"Source_ID":`), 1),
		"nested-duplicate": bytes.Replace(raw, []byte(`"original_ordinal":0`), []byte(`"original_ordinal":0,"original_ordinal":0`), 1),
		"missing":          bytes.Replace(raw, []byte(`"complete":true,`), nil, 1),
		"utf8":             append([]byte{'{', '"', 0xff}, raw...),
	}
	for name, b := range variants {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeCensusQualifiedCLIPrivate(b, inventoryscope.LimitsV1{}); e == nil {
				t.Fatal("ASSERT strict typed preflight refuses malformed envelope: FAIL")
			}
			code, out, _ := qualifiedCLIRunRawPrivate(t, in, b)
			if code == 0 || len(out) != 0 {
				t.Fatal("malformed input prepared")
			}
			t.Log("ASSERT strict typed preflight refuses malformed envelope: PASS")
		})
	}
	for _, l := range []inventoryscope.LimitsV1{{MaxCandidates: 4}, {MaxObservations: 1}, {MaxWork: 10}, {MaxUTF8Bytes: 10}, {MaxInputBytes: int64(len(raw) - 1)}} {
		if _, e := decodeCensusQualifiedCLIPrivate(raw, l); e == nil {
			t.Fatal("ASSERT preallocation limits enforced: FAIL")
		}
	}
	t.Log("ASSERT preallocation limits enforced: PASS")
}
func TestQualifiedCLISyntaxPrivate(t *testing.T) {
	base := []string{"--machine", "--private-inventory-qualified-prepare-config", "/controlled/input.json"}
	if _, e := parseCensusCLIOptions(base); e != nil {
		t.Fatal(e)
	}
	for _, arg := range []string{"--workspace=/x", "--server=x", "--profile=x", "--config=/x", "--resume=x", "--catalog", "--enumeration-preflight", "--private-inventory-config=/x", "--private-inventory-bridge-config=/x", "--timeout=1s", "--help", "--source=x", "--publication-root=/x", "--private-enumeration-projection", "--private-provider-probe-path=/x"} {
		if _, e := parseCensusCLIOptions(append(append([]string{}, base...), arg)); e == nil {
			t.Fatalf("ordinary flag admitted: %s", arg)
		}
	}
	for _, args := range [][]string{{"--private-inventory-qualified-prepare-config", "/x"}, {"--machine", "--private-inventory-qualified-prepare-config", "relative"}, {"--machine", "--private-inventory-qualified-prepare-config", ""}, append(base, "--private-inventory-qualified-prepare-config=/other")} {
		if _, e := parseCensusCLIOptions(args); e == nil {
			t.Fatal("invalid syntax admitted")
		}
	}
	for _, args := range [][]string{{"--machine", "-private-inventory-qualified-prepare-config=/x", "--workspace=/ordinary", "--server=forbidden", "--publication-root=/ordinary"}, {"-machine", "--machine", "--private-inventory-qualified-prepare-config=/x"}, {"-machine", "-private-inventory-qualified-prepare-config="}} {
		if _, e := parseCensusCLIOptions(args); e == nil {
			t.Fatal("single-hyphen or duplicate controls admitted")
		}
	}
	t.Log("ASSERT qualified CLI isolated from ordinary controls: PASS")
}
func TestQualifiedCLIPublicationRefusalPrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	q, e := validateCensusQualifiedCLIPrivate(in)
	if e != nil {
		t.Fatal(e)
	}
	p, metadata, e := buildCensusApplicationQualifiedFullPlanPrivate(context.Background(), q, in.Preparation, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	refuse := func(s *censusApplicationStorePrivate, name string, b []byte, limit int) error {
		calls++
		if calls == 2 {
			return errors.New("controlled artifact write refusal")
		}
		return s.write(name, b, limit)
	}
	raw, e := publishCensusQualifiedCLIWithWritePrivate(context.Background(), in, q, p, metadata, refuse)
	if e == nil || len(raw) != 0 || calls != 2 {
		t.Fatal("ASSERT failed bounded artifact write cannot be Prepared: FAIL")
	}
	if _, e = os.Lstat(filepath.Join(in.OutputDirectory, in.OutputName)); !os.IsNotExist(e) {
		t.Fatal("owned failed publication not cleaned")
	}
	t.Log("ASSERT failed bounded artifact write cannot be Prepared: PASS")
	in.Preparation.MaxMetadataBytes = 100
	if raw, e = publishCensusQualifiedCLIPrivate(context.Background(), in, q, p, metadata); e == nil || len(raw) != 0 {
		t.Fatal("ASSERT final metadata bound enforced at publication: FAIL")
	}
	t.Log("ASSERT final metadata bound enforced at publication: PASS")
	in.Preparation.MaxMetadataBytes = 48000
	in.OutputName = "a"
	measured, e := publishCensusQualifiedCLIPrivate(context.Background(), in, q, p, metadata)
	if e != nil {
		t.Fatal(e)
	}
	in.OutputName = "b"
	in.Preparation.MaxMetadataBytes = len(measured) + 1
	equal, e := publishCensusQualifiedCLIPrivate(context.Background(), in, q, p, metadata)
	if e != nil || len(equal)+1 != in.Preparation.MaxMetadataBytes {
		t.Fatal("ASSERT metadata equality accepted and one-byte overflow refused: FAIL")
	}
	in.OutputName = "c"
	in.Preparation.MaxMetadataBytes--
	if raw, e = publishCensusQualifiedCLIPrivate(context.Background(), in, q, p, metadata); e == nil || len(raw) != 0 {
		t.Fatal("ASSERT metadata equality accepted and one-byte overflow refused: FAIL")
	}
	t.Log("ASSERT metadata equality accepted and one-byte overflow refused: PASS")
}

func TestQualifiedCLIFailureEvidencePrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	for _, kind := range []inventoryscope.ObservationKindV1{inventoryscope.UnsupportedLanguageV1, inventoryscope.UnreadableV1, inventoryscope.AnalysisFailureV1, inventoryscope.ResourceGapV1} {
		o := inventoryscope.ObservationV1{CandidateID: in.Qualification.Candidates[4].CandidateID, Assertion: inventoryscope.UnresolvedV1, Kind: kind, ProducerIdentity: "controlled", ProducerVersion: "v1", Artifact: in.Universe.Provenance, SourceID: in.Qualification.Declaration.SourceID, ConfigDigest: in.Qualification.Declaration.ConfigDigest}
		id, e := inventoryscope.ObservationIDV1(o, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		o.ObservationID = id
		in.Qualification.Observations = append(in.Qualification.Observations, o)
	}
	qualifiedCLIRebindPrivate(t, &in)
	code, b, errout := qualifiedCLIRunPrivate(t, in)
	if code != 0 {
		t.Fatalf("failure ledger preparation refused: %s", errout)
	}
	var r censusQualifiedCLIResultPrivate
	_ = json.Unmarshal(b, &r)
	if r.Included != 2 || r.Excluded != 1 || r.Unresolved != 2 {
		t.Fatal("ASSERT analysis failure evidence never proves exclusion: FAIL")
	}
	t.Log("ASSERT analysis failure evidence never proves exclusion: PASS")
}

func TestQualifiedCLIOrdinaryBoundaryPrivate(t *testing.T) {
	before := qualifiedProfileCallsPrivate
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies([]string{"--machine", "--workspace=/controlled", "--server=never-run", "--publication-root=/controlled", "--server-arg", "--private-inventory-qualified-prepare-config"}, &stdout, &stderr, censusRunnerDependencies{})
	if code != 1 || qualifiedProfileCallsPrivate != before+1 || len(stdout.Bytes()) != 0 {
		t.Fatal("ASSERT ordinary dispatch retains preexisting profile boundary: FAIL")
	}
	t.Log("ASSERT ordinary dispatch retains preexisting profile boundary: PASS")
}

func TestQualifiedCLIInputIOPrivate(t *testing.T) {
	in := qualifiedCLIFixturePrivate(t)
	parent := filepath.Dir(in.Workspace)
	path := filepath.Join(parent, "bounded.json")
	if e := os.WriteFile(path, []byte("12345"), 0600); e != nil {
		t.Fatal(e)
	}
	if b, e := readCensusQualifiedCLIPrivate(context.Background(), path, 5); e != nil || len(b) != 5 {
		t.Fatal("equality read boundary")
	}
	if _, e := readCensusQualifiedCLIPrivate(context.Background(), path, 4); e == nil {
		t.Fatal("max+1 not refused")
	}
	link := filepath.Join(parent, "link.json")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e := readCensusQualifiedCLIPrivate(context.Background(), link, 10); e == nil {
		t.Fatal("symlink followed")
	}
	alias := filepath.Join(in.OutputDirectory, "alias")
	if e := os.Symlink(parent, alias); e != nil {
		t.Fatal(e)
	}
	if _, e := readCensusQualifiedCLIPrivate(context.Background(), filepath.Join(alias, "bounded.json"), 10); e == nil {
		t.Fatal("ancestor alias followed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := readCensusQualifiedCLIPrivate(ctx, path, 10); e == nil {
		t.Fatal("cancellation ignored")
	}
	t.Log("ASSERT bounded nofollow input and canceled context: PASS")
}

// Relative same-parent links distinguish leaf refusal from Root confinement.
func TestQualifiedCLILeafSymlinkPrivate(t *testing.T) {
	for _, kind := range []string{"relative", "chain", "dangling"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if e := os.Chmod(root, 0700); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(root, "private")
			if e := os.WriteFile(path, []byte("abc"), 0600); e != nil {
				t.Fatal(e)
			}
			if b, e := readCensusQualifiedCLIPrivate(context.Background(), path, 3); e != nil || string(b) != "abc" {
				t.Fatalf("R97_REGULAR_CONTROL FAIL err=%v bytes=%d", e, len(b))
			}
			target := "private"
			if kind == "chain" {
				if e := os.Symlink(target, filepath.Join(root, "hop")); e != nil {
					t.Fatal(e)
				}
				target = "hop"
			} else if kind == "dangling" {
				target = "absent"
			}
			link := filepath.Join(root, "leaf")
			if e := os.Symlink(target, link); e != nil {
				t.Fatal(e)
			}
			b, e := readCensusQualifiedCLIPrivate(context.Background(), link, 3)
			if e != errCensusQualifiedCLIPrivate || len(b) != 0 {
				t.Fatalf("R97_LEAF_SYMLINK FAIL %s err=%v bytes=%d", kind, e, len(b))
			}
			t.Logf("R97_LEAF_SYMLINK PASS %s", kind)
		})
	}
}

func TestQualifiedCLILeafSymlinkRoutePrivate(t *testing.T) {
	for _, link := range []bool{false, true} {
		name := "regular-control"
		if link {
			name = "relative-leaf"
		}
		t.Run(name, func(t *testing.T) {
			in := qualifiedCLIFixturePrivate(t)
			raw, e := json.Marshal(in)
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(filepath.Dir(in.Workspace), "valid-envelope.json")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			if link {
				alias := filepath.Join(filepath.Dir(path), "relative-envelope.json")
				if e := os.Symlink(filepath.Base(path), alias); e != nil {
					t.Fatal(e)
				}
				path = alias
			}
			calls := 0
			deps := censusRunnerDependencies{
				newStarter: func() (sessionruntime.Starter, error) { calls++; return nil, errors.New("inert refusal") },
				newManager: func(sessionruntime.Config) (*sessionruntime.Manager, error) {
					calls++
					return nil, errors.New("inert refusal")
				},
			}
			before := qualifiedProfileCallsPrivate
			var stdout, stderr bytes.Buffer
			code := runCensusWithDependencies([]string{"--machine", "--private-inventory-qualified-prepare-config", path}, &stdout, &stderr, deps)
			if calls != 0 || qualifiedProfileCallsPrivate != before {
				t.Fatal("R97_RUNTIME_ZERO FAIL")
			}
			t.Log("R97_RUNTIME_ZERO PASS profile=0 Starter=0 Manager=0")
			if !link {
				var result censusQualifiedCLIResultPrivate
				if e := json.Unmarshal(stdout.Bytes(), &result); e != nil || code != 0 || !result.Prepared || stderr.Len() != 0 {
					t.Fatalf("R97_ROUTE_CONTROL FAIL code=%d err=%v", code, e)
				}
				t.Log("R97_ROUTE_CONTROL PASS valid-envelope")
				return
			}
			if code != 1 || stdout.Len() != 0 || stderr.Len() == 0 || bytes.Contains(stderr.Bytes(), []byte(path)) || bytes.Contains(stderr.Bytes(), []byte(in.Workspace)) {
				t.Fatalf("R97_LEAF_ROUTE FAIL code=%d stdout=%d stderr=%d", code, stdout.Len(), stderr.Len())
			}
			if _, e := os.Lstat(filepath.Join(in.OutputDirectory, in.OutputName)); !os.IsNotExist(e) {
				t.Fatal("R97_LEAF_ROUTE FAIL publication reached")
			}
			t.Log("R97_LEAF_ROUTE PASS valid-envelope refused before publication")
		})
	}
}

// Persistent changes at bounded publication boundaries, not a chmod-race lease.
// The caller-write seam invokes the real writer before changing only this child.
func TestQualifiedCLIChildDirectoryPrivate(t *testing.T) {
	for _, tc := range []struct {
		name, after string
		mode        os.FileMode
	}{
		{"control0700", "", 0700},
		{"post-first-write0755", "first", 0755},
		{"post-index0755", "artifact-index.v1.json", 0755},
		{"post-result0755", "result.v1.json", 0755},
		{"helper-private0500", "empty-child", 0500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := qualifiedCLIFixturePrivate(t)
			if tc.after == "empty-child" {
				// Exact0700, not merely private: no populated unwritable directory
				// and no permission restoration are needed for fixture cleanup.
				path := filepath.Join(in.OutputDirectory, in.OutputName)
				if e := os.Mkdir(path, 0700); e != nil {
					t.Fatal(e)
				}
				s, e := openCensusApplicationStorePrivate(context.Background(), path, in.Preparation.MaxArtifactBytes)
				if e != nil {
					t.Fatal(e)
				}
				if e := os.Chmod(path, tc.mode); e != nil {
					_ = s.close()
					t.Fatal(e)
				}
				e = checkCensusQualifiedChildDirectoryPrivate(s)
				closeErr := s.close()
				if e != errCensusQualifiedCLIPrivate || closeErr != nil {
					t.Fatalf("R95_CHILD_DIRECTORY FAIL %s err=%v close=%v", tc.name, e, closeErr)
				}
				t.Logf("R95_CHILD_DIRECTORY PASS %s empty-held-child", tc.name)
				return
			}
			q, e := validateCensusQualifiedCLIPrivate(in)
			if e != nil {
				t.Fatal(e)
			}
			p, metadata, e := buildCensusApplicationQualifiedFullPlanPrivate(context.Background(), q, in.Preparation, inventoryscope.LimitsV1{})
			if e != nil {
				t.Fatal(e)
			}
			writes, flips := 0, 0
			raw, e := publishCensusQualifiedCLIWithWritePrivate(context.Background(), in, q, p, metadata, func(s *censusApplicationStorePrivate, name string, b []byte, limit int) error {
				writes++
				if e := s.write(name, b, limit); e != nil {
					return e
				}
				if name == tc.after || tc.after == "first" && writes == 1 {
					if e := os.Chmod(s.path, tc.mode); e != nil {
						return e
					}
					flips++
				}
				return nil
			})
			var result censusQualifiedCLIResultPrivate
			_ = json.Unmarshal(raw, &result)
			if tc.after == "" {
				info, statErr := os.Stat(filepath.Join(in.OutputDirectory, in.OutputName))
				if e != nil || !result.Prepared || statErr != nil || info.Mode().Perm() != 0700 || flips != 0 {
					t.Fatalf("R95_CHILD_DIRECTORY FAIL %s err=%v prepared=%v", tc.name, e, result.Prepared)
				}
			} else if flips != 1 || e != errCensusQualifiedCLIPrivate || len(raw) != 0 || result.Prepared {
				t.Fatalf("R95_CHILD_DIRECTORY FAIL %s flips=%d err=%v prepared=%v bytes=%d", tc.name, flips, e, result.Prepared, len(raw))
			}
			t.Logf("R95_CHILD_DIRECTORY PASS %s writes=%d flips=%d", tc.name, writes, flips)
		})
	}
}
