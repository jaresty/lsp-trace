package censuschunking

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/seedformat"
)

func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func fixture(t *testing.T, n int) Input {
	t.Helper()
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: workspace}).String()
	wd, err := censusenumerationprojection.WorkspaceDigest(uri)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{GlobalProjection: ArtifactRef{"global/selector", digest([]byte("global")), 6}, RequestFingerprint: digest([]byte("request")), SessionID: "session", Generation: 7, WorkspaceURI: uri, WorkspaceDigest: wd, CanonicalOrderPolicy: "captureset-order-targets-v1", DuplicatePolicy: "reject-global-ordinal-identity-seed-bytes-digest-v1", BatchTargets: 63, DownDepth: 1, UpDepth: 1, Files: []GlobalFile{{Ordinal: 0, Path: "a.go", Disposition: "processed"}, {Ordinal: 4, Path: "b.go", Disposition: "processed"}}, GlobalProviderFailures: []string{}}
	in.Targets = make([]GlobalTarget, n)
	for i := 0; i < n; i++ {
		fileOrd, path := 0, "a.go"
		if i%2 == 1 {
			fileOrd, path = 4, "b.go"
		}
		p := uint32(i / 2)
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i*2+3), Path: path, Line: uint64(p) + 1, Column: 2}}}}, workspace)
		if err != nil {
			t.Fatal(err)
		}
		identity, _, err := censusenumerationprojection.SymbolIdentityDigest(path, censusenumerationprojection.Position{Line: p, Character: 1}, 12, "", i*2+3)
		if err != nil {
			t.Fatal(err)
		}
		in.Targets[i] = GlobalTarget{GlobalOrdinal: i*2 + 3, GlobalFileOrdinal: fileOrd, GlobalIdentity: identity, CanonicalSeedV2: string(seed), CanonicalSeedV2SHA256: digest(seed), Coordinate: SourceCoordinate{Path: path, Line: p, Character: 1}, Kind: 12, Name: fmt.Sprintf("S%06d", i), URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, path)}).String(), LanguageID: "go", Disposition: "prepared"}
	}
	return in
}

func TestPlanBoundariesAndCompatibility(t *testing.T) {
	compat := []byte("exact legacy bytes\n")
	for _, n := range []int{1, 9999, 10000} {
		got, err := Build(fixture(t, n), compat)
		if err != nil || !got.Compatibility || !bytes.Equal(got.CompatibilityBytes, compat) || len(got.Chunks) != 0 {
			t.Fatalf("ASSERT_PLAN_BOUNDARIES_AND_COMPATIBILITY n=%d got=%+v err=%v", n, got, err)
		}
	}
	for _, tc := range []struct {
		n      int
		counts []int
	}{{10001, []int{10000, 1}}, {20000, []int{10000, 10000}}, {20001, []int{10000, 10000, 1}}, {28069, []int{10000, 10000, 8069}}, {100000, []int{10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000}}} {
		got, err := Build(fixture(t, tc.n), nil)
		if err != nil {
			t.Fatalf("n=%d: %v", tc.n, err)
		}
		var counts []int
		for _, c := range got.Plan.Chunks {
			counts = append(counts, c.TargetCount)
		}
		if !reflect.DeepEqual(counts, tc.counts) {
			t.Fatalf("ASSERT_PLAN_BOUNDARIES_AND_COMPATIBILITY n=%d counts=%v", tc.n, counts)
		}
	}
	if _, err := Build(fixture(t, 0), nil); err == nil {
		t.Fatal("ASSERT_PLAN_BOUNDARIES_AND_COMPATIBILITY zero accepted")
	}
	if _, err := Build(fixture(t, 100001), nil); err == nil {
		t.Fatal("ASSERT_PLAN_BOUNDARIES_AND_COMPATIBILITY over max accepted")
	}
}

func TestGlobalDuplicateAndStaleInputRejection(t *testing.T) {
	cases := map[string]func(*Input){
		"file_path": func(in *Input) {
			in.Files = append(in.Files, GlobalFile{Ordinal: 8, Path: in.Files[0].Path, Disposition: "processed"})
		},
		"file_ordinal": func(in *Input) {
			in.Files = append(in.Files, GlobalFile{Ordinal: in.Files[0].Ordinal, Path: "other.go", Disposition: "processed"})
		},
		"global_ordinal":  func(in *Input) { in.Targets[1].GlobalOrdinal = in.Targets[0].GlobalOrdinal },
		"global_identity": func(in *Input) { in.Targets[1].GlobalIdentity = in.Targets[0].GlobalIdentity },
		"seed_bytes_and_digest": func(in *Input) {
			in.Targets[1].CanonicalSeedV2 = in.Targets[0].CanonicalSeedV2
			in.Targets[1].CanonicalSeedV2SHA256 = in.Targets[0].CanonicalSeedV2SHA256
		},
		"stale_global_identity": func(in *Input) { in.Targets[0].Coordinate.Line++ },
		"malformed_seed": func(in *Input) {
			in.Targets[0].CanonicalSeedV2 = `{"bad":true}`
			in.Targets[0].CanonicalSeedV2SHA256 = digest([]byte(in.Targets[0].CanonicalSeedV2))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := fixture(t, 10001)
			mutate(&in)
			if _, err := Build(in, nil); err == nil {
				t.Fatal("ASSERT_GLOBAL_DUPLICATE_STALE_MALFORMED_REJECTED")
			}
		})
	}
}

func TestAggregateWorkExactBoundaryPlusOneAndOverflow(t *testing.T) {
	in := fixture(t, 10001)
	got, err := Build(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	projections := make([]Projection, len(got.Chunks))
	sidecars := make([]Sidecar, len(got.Chunks))
	for i := range got.Chunks {
		projections[i] = got.Chunks[i].Projection
		sidecars[i] = got.Chunks[i].Sidecar
	}
	work, err := aggregateWork(in, got.Plan, got.Index, projections, sidecars)
	if err != nil || work == 0 || work >= uint64(MaxWork) {
		t.Fatalf("ASSERT_MAX_WORK_FORMULA work=%d err=%v", work, err)
	}
	old := maxWorkLimit
	defer func() { maxWorkLimit = old }()
	maxWorkLimit = work
	if _, err := Build(in, nil); err != nil {
		t.Fatalf("ASSERT_MAX_WORK_EXACT_BOUNDARY_REJECTED work=%d err=%v", work, err)
	}
	maxWorkLimit = work - 1
	if _, err := Build(in, nil); err == nil {
		t.Fatal("ASSERT_MAX_WORK_PLUS_ONE_ACCEPTED")
	}
	if _, ok := checkedAdd(math.MaxUint64, 1); ok {
		t.Fatal("ASSERT_MAX_WORK_UINT64_OVERFLOW_ACCEPTED")
	}
}

func decodedAggregate(t *testing.T, in Input, got Result) (Input, Plan, Index, []Projection, []Sidecar) {
	t.Helper()
	inputRaw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var independent Input
	if err := json.Unmarshal(inputRaw, &independent); err != nil {
		t.Fatal(err)
	}
	planRaw, err := EncodePlan(got.Plan)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := DecodePlan(planRaw)
	if err != nil {
		t.Fatal(err)
	}
	indexRaw, err := EncodeIndex(got.Index)
	if err != nil {
		t.Fatal(err)
	}
	index, err := DecodeIndex(indexRaw)
	if err != nil {
		t.Fatal(err)
	}
	projections := make([]Projection, len(got.Chunks))
	sidecars := make([]Sidecar, len(got.Chunks))
	for i := range got.Chunks {
		projections[i], err = DecodeProjection(got.Chunks[i].ProjectionBytes)
		if err != nil {
			t.Fatal(err)
		}
		sidecars[i], err = DecodeSidecar(got.Chunks[i].SidecarBytes)
		if err != nil {
			t.Fatal(err)
		}
	}
	return independent, plan, index, projections, sidecars
}

func TestIndependentAggregateReconciliation(t *testing.T) {
	original := fixture(t, 10001)
	got, err := Build(original, nil)
	if err != nil {
		t.Fatal(err)
	}
	in, plan, index, projections, sidecars := decodedAggregate(t, original, got)
	if err := Reconcile(in, plan, index, projections, sidecars); err != nil {
		t.Fatalf("ASSERT_AGGREGATE_RECONCILIATION_VALID_REJECTED: %v", err)
	}
	ordered := append([]GlobalTarget(nil), in.Targets...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CanonicalSeedV2 != ordered[j].CanonicalSeedV2 {
			return ordered[i].CanonicalSeedV2 < ordered[j].CanonicalSeedV2
		}
		return ordered[i].GlobalOrdinal < ordered[j].GlobalOrdinal
	})
	var concatenated []int
	for _, sidecar := range sidecars {
		for _, mapping := range sidecar.Mappings {
			concatenated = append(concatenated, mapping.GlobalOrdinal)
		}
	}
	for i := range ordered {
		if concatenated[i] != ordered[i].GlobalOrdinal {
			t.Fatal("ASSERT_AGGREGATE_FULL_CROSS_CHUNK_CONCAT")
		}
	}

	cases := map[string]func(*Input, *Plan, *Index, []Projection, []Sidecar){
		"plan_binding": func(_ *Input, plan *Plan, _ *Index, _ []Projection, _ []Sidecar) {
			plan.RequestFingerprint = digest([]byte("foreign"))
			plan.LogicalDigest, plan.ImmutableSelector = "", ""
		},
		"index_binding": func(_ *Input, _ *Plan, index *Index, _ []Projection, _ []Sidecar) {
			index.PlanDigest = digest([]byte("foreign"))
			index.LogicalDigest, index.ImmutableSelector = "", ""
		},
		"descriptor_ref": func(_ *Input, plan *Plan, _ *Index, _ []Projection, _ []Sidecar) {
			plan.Chunks[0].Projection = plan.Chunks[1].Projection
			plan.LogicalDigest, plan.ImmutableSelector = "", ""
		},
		"projection_order": func(_ *Input, _ *Plan, _ *Index, projections []Projection, _ []Sidecar) {
			projections[0], projections[1] = projections[1], projections[0]
		},
		"sidecar_order": func(_ *Input, _ *Plan, _ *Index, _ []Projection, sidecars []Sidecar) {
			sidecars[0], sidecars[1] = sidecars[1], sidecars[0]
		},
		"malformed_projection": func(_ *Input, _ *Plan, _ *Index, projections []Projection, _ []Sidecar) {
			projections[0] = Projection{}
		},
		"global_fabrication": func(_ *Input, _ *Plan, _ *Index, _ []Projection, sidecars []Sidecar) {
			sidecars[1].Mappings[0].GlobalOrdinal += 1000000
			sidecars[1].LogicalDigest, sidecars[1].ImmutableSelector = "", ""
		},
		"cross_chunk_duplicate": func(_ *Input, _ *Plan, _ *Index, _ []Projection, sidecars []Sidecar) {
			sidecars[1].Mappings[0].GlobalOrdinal = sidecars[0].Mappings[0].GlobalOrdinal
			sidecars[1].Mappings[0].GlobalIdentity = sidecars[0].Mappings[0].GlobalIdentity
			sidecars[1].Mappings[0].GlobalCanonicalSeedDigest = sidecars[0].Mappings[0].GlobalCanonicalSeedDigest
			sidecars[1].LogicalDigest, sidecars[1].ImmutableSelector = "", ""
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in, plan, index, projections, sidecars := decodedAggregate(t, original, got)
			mutate(&in, &plan, &index, projections, sidecars)
			if err := Reconcile(in, plan, index, projections, sidecars); err == nil {
				t.Fatal("ASSERT_AGGREGATE_RECONCILIATION_MISMATCH_ACCEPTED")
			}
		})
	}
	for name, reconcile := range map[string]func(Input, Plan, Index, []Projection, []Sidecar) error{
		"projection_omission": func(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) error {
			return Reconcile(in, plan, index, projections[:len(projections)-1], sidecars)
		},
		"projection_fabrication": func(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) error {
			return Reconcile(in, plan, index, append(projections, projections[0]), sidecars)
		},
		"sidecar_omission": func(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) error {
			return Reconcile(in, plan, index, projections, sidecars[:len(sidecars)-1])
		},
		"sidecar_fabrication": func(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) error {
			return Reconcile(in, plan, index, projections, append(sidecars, sidecars[0]))
		},
	} {
		t.Run(name, func(t *testing.T) {
			in, plan, index, projections, sidecars := decodedAggregate(t, original, got)
			if err := reconcile(in, plan, index, projections, sidecars); err == nil {
				t.Fatal("ASSERT_AGGREGATE_OMISSION_FABRICATION_ACCEPTED")
			}
		})
	}
}

func TestPermutationProjectionAndIdentity(t *testing.T) {
	in := fixture(t, 10001)
	a, err := Build(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(in.Targets)-1; i < j; i, j = i+1, j-1 {
		in.Targets[i], in.Targets[j] = in.Targets[j], in.Targets[i]
	}
	b, err := Build(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := EncodePlan(a.Plan)
	bb, _ := EncodePlan(b.Plan)
	if !bytes.Equal(ab, bb) {
		t.Fatal("ASSERT_PERMUTATION_AND_CONCAT_GLOBAL_ORDER")
	}
	for _, ch := range a.Chunks {
		if len(ch.Projection.Targets) != ch.Descriptor.TargetCount || len(ch.Sidecar.Mappings) != ch.Descriptor.TargetCount {
			t.Fatal("ASSERT_LOCAL_LEDGERS_DENSE_AND_CLOSED")
		}
		localFiles := map[int]bool{}
		for _, file := range ch.Projection.Files {
			if localFiles[file.GlobalOrdinal] {
				t.Fatal("ASSERT_TARGET_BEARING_FILE_APPEARS_ONCE_LOCALLY")
			}
			localFiles[file.GlobalOrdinal] = true
		}
		targetFiles := map[int]bool{}
		for j, m := range ch.Sidecar.Mappings {
			targetFiles[m.GlobalFileOrdinal] = true
			if m.LocalOrdinal != j || ch.Projection.Symbols[j].Ordinal != j || ch.Projection.Targets[j].CensusOrdinal != j || !strings.Contains(ch.Projection.Targets[j].CanonicalSeedV2, fmt.Sprintf("census-%06d", j)) {
				t.Fatal("ASSERT_LOCAL_IDENTITY_SEED_AND_SIDECAR_MAPPING")
			}
		}
		if !reflect.DeepEqual(localFiles, targetFiles) {
			t.Fatal("ASSERT_TARGET_BEARING_FILE_APPEARS_ONCE_LOCALLY")
		}
	}
}

func TestValidationAndCanonicalJSON(t *testing.T) {
	got, err := Build(fixture(t, 10001), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodePlan(got.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodePlan(raw); err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		t.Fatal("json")
	}
	obj["unknown"] = true
	bad, _ := json.Marshal(obj)
	if _, err = DecodePlan(bad); err == nil {
		t.Fatal("ASSERT_STRICT_CANONICAL_JSON unknown")
	}
	if _, err = DecodePlan(append(raw, ' ')); err == nil {
		t.Fatal("ASSERT_STRICT_CANONICAL_JSON trailing")
	}
	p := got.Chunks[0].Projection
	p.ProviderFailures = []string{"global"}
	if _, err = EncodeProjection(p); err == nil {
		t.Fatal("ASSERT_GLOBAL_FAILURES_EXCLUDED_FROM_LOCAL_COMPLETE_PROJECTION")
	}
	for name, mutate := range map[string]func(string) string{
		"unknown": func(seed string) string { return strings.Replace(seed, `}`, `,"unknown":true}`, 1) },
		"coordinate": func(seed string) string {
			return strings.Replace(seed, `"column":2`, `"column":3`, 1)
		},
		"private_path": func(seed string) string {
			return strings.Replace(seed, `"path":"a.go"`, `"path":"/secret/a.go"`, 1)
		},
	} {
		t.Run("local_seed_"+name, func(t *testing.T) {
			p := got.Chunks[0].Projection
			p.Targets = append([]LocalTarget(nil), p.Targets...)
			p.Targets[0].CanonicalSeedV2 = mutate(p.Targets[0].CanonicalSeedV2)
			p.Targets[0].CanonicalSeedV2SHA256 = digest([]byte(p.Targets[0].CanonicalSeedV2))
			p.LogicalDigest, p.ImmutableSelector = "", ""
			if _, err := EncodeProjection(p); err == nil {
				t.Fatal("ASSERT_LOCAL_SEED_CANONICAL_IDENTITY_PRIVACY_RECONCILIATION")
			}
		})
	}
	s := got.Chunks[0].Sidecar
	s.Mappings = append([]Mapping(nil), s.Mappings...)
	s.Mappings[0].GlobalOrdinal = s.Mappings[1].GlobalOrdinal
	if _, err = EncodeSidecar(s); err == nil {
		t.Fatal("ASSERT_MAPPING_DUPLICATE_GAP_OVERLAP_FABRICATION_REJECTED")
	}
	s = got.Chunks[0].Sidecar
	s.Mappings = append([]Mapping(nil), s.Mappings...)
	s.Mappings[1].LocalOrdinal = s.Mappings[0].LocalOrdinal
	s.LogicalDigest, s.ImmutableSelector = "", ""
	if _, err = EncodeSidecar(s); err == nil {
		t.Fatal("ASSERT_DUPLICATE_LOCAL_ORDINAL_REJECTED")
	}
	p = got.Chunks[0].Projection
	p.Symbols = append([]LocalSymbol(nil), p.Symbols...)
	p.Symbols[1].Ordinal = p.Symbols[0].Ordinal
	p.LogicalDigest, p.ImmutableSelector = "", ""
	if _, err = EncodeProjection(p); err == nil {
		t.Fatal("ASSERT_DUPLICATE_LOCAL_ORDINAL_REJECTED")
	}
	index := got.Index
	index.Chunks = append([]ChunkDescriptor(nil), index.Chunks...)
	index.Chunks[1].TargetStart++
	index.LogicalDigest, index.ImmutableSelector = "", ""
	if _, err = EncodeIndex(index); err == nil {
		t.Fatal("ASSERT_MAPPING_DUPLICATE_GAP_OVERLAP_FABRICATION_REJECTED index gap")
	}
	plan := got.Plan
	plan.Chunks = append([]ChunkDescriptor(nil), plan.Chunks...)
	plan.Chunks[1].TargetStart = int(^uint(0) >> 1)
	plan.LogicalDigest, plan.ImmutableSelector = "", ""
	if _, err = EncodePlan(plan); err == nil {
		t.Fatal("ASSERT_MAX_INTEGER_RANGE_REJECTED")
	}
	got.Plan.Accepted = true
	if _, err = EncodePlan(got.Plan); err == nil {
		t.Fatal("ASSERT_PLAN_BINDINGS_IDENTITY_AND_CONSERVATIVE_POSTURE")
	}
}

func TestDigestSensitivityAndNilLists(t *testing.T) {
	got, err := Build(fixture(t, 10001), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := EncodePlan(got.Plan)
	got.Plan.RequestFingerprint = digest([]byte("changed"))
	got.Plan.LogicalDigest = ""
	got.Plan.ImmutableSelector = ""
	b, err := EncodePlan(got.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("ASSERT_PLAN_BINDINGS_IDENTITY_AND_CONSERVATIVE_POSTURE digest insensitive")
	}
	got.Plan.Chunks = nil
	got.Plan.LogicalDigest = ""
	got.Plan.ImmutableSelector = ""
	if _, err = EncodePlan(got.Plan); err == nil {
		t.Fatal("ASSERT_STRICT_CANONICAL_JSON null list accepted")
	}
}
