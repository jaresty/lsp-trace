package censuschunking

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
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
	in := Input{GlobalProjection: ArtifactRef{"global/selector", digest([]byte("global")), 6}, RequestFingerprint: digest([]byte("request")), SessionID: "session", Generation: 7, WorkspaceURI: uri, WorkspaceDigest: wd, CanonicalOrderPolicy: "captureset-order-targets-v1", DuplicatePolicy: "reject-global-ordinal-identity-seed-bytes-digest-v1", BatchTargets: 63, DownDepth: 1, UpDepth: 1, Files: []GlobalFile{{Ordinal: 0, Path: "a.go", Disposition: "processed"}, {Ordinal: 4, Path: "b.go", Disposition: "processed"}}}
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
	for _, n := range []int{1, 10000} {
		got, err := Build(fixture(t, n), compat)
		if err != nil || !got.Compatibility || !bytes.Equal(got.CompatibilityBytes, compat) || len(got.Chunks) != 0 {
			t.Fatalf("ASSERT_PLAN_BOUNDARIES_AND_COMPATIBILITY n=%d got=%+v err=%v", n, got, err)
		}
	}
	for _, tc := range []struct {
		n      int
		counts []int
	}{{10001, []int{10000, 1}}, {28069, []int{10000, 10000, 8069}}, {100000, []int{10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000}}} {
		got, err := Build(fixture(t, tc.n), nil)
		if err != nil {
			t.Fatal(err)
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
		for j, m := range ch.Sidecar.Mappings {
			if m.LocalOrdinal != j || ch.Projection.Symbols[j].Ordinal != j || ch.Projection.Targets[j].CensusOrdinal != j || !strings.Contains(ch.Projection.Targets[j].CanonicalSeedV2, fmt.Sprintf("census-%06d", j)) {
				t.Fatal("ASSERT_LOCAL_IDENTITY_SEED_AND_SIDECAR_MAPPING")
			}
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
	s := got.Chunks[0].Sidecar
	s.Mappings[0].GlobalOrdinal = s.Mappings[1].GlobalOrdinal
	if _, err = EncodeSidecar(s); err == nil {
		t.Fatal("ASSERT_MAPPING_DUPLICATE_GAP_OVERLAP_FABRICATION_REJECTED")
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
