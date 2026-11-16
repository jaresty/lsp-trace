package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
)

func serialization114Assert(t *testing.T, id string, ok bool, detail any) {
	t.Helper()
	if !ok {
		t.Errorf("ASSERT114 %s: FAIL %v", id, detail)
	} else {
		t.Logf("ASSERT114 %s: PASS", id)
	}
}

// Self-contained standing fixture: no external evidence, native funding, file
// discovery, provider or HOST. The complete one-member caller universe is data.
func serialization114Standing(t *testing.T) censusInventoryHeader {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	pin := "sha256:" + strings.Repeat("a", 64)
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "guard114", ApplicationRoots: []string{"."}, Includes: []string{"a.go"}, Excludes: []string{}, SourceID: "guard114", ConfigDigest: pin, PolicyID: inventoryscope.PolicyConsensusV1}, Candidates: []inventoryscope.CandidateV1{}, Observations: []inventoryscope.ObservationV1{}, Resolutions: []inventoryscope.ResolutionV1{}}
	id, e := inventoryscope.CandidateIDV1("guard114", "a.go")
	if e != nil {
		t.Fatal(e)
	}
	in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: 0, CanonicalRelativePath: "a.go", CandidateID: id, LanguageID: "go", DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "self-contained114", Digest: pin}})
	m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	policy := pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 2, MaxContextFiles: 1, MaxMembers: 3, FileBytes: 4096, TotalBytes: 8192, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 2, MaxSymbols: 8, MaxDepth: 8, MaxStringBytes: 4096}, MaxTargets: 4, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 4}, MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 4096, MaxPathWork: 1000, TimeoutMS: 30000, RequestTimeoutMS: 1000, MaxResponseBytes: 4096, MaxMessages: 8}
	env := censusQualifiedCLIEnvelopePrivate{Family: censusQualifiedCLIFamilyPrivate, Universe: censusQualifiedUniversePrivate{Complete: true, Claim: "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED", ListMethod: "self-contained nonoperative guard universe", Provenance: inventoryscope.ArtifactRefV1{ArtifactID: "self-contained114", Digest: pin}, OriginalDenominatorCount: 1, OriginalDenominatorDigest: m.OriginalDenominatorDigest}, Originals: []censusacquisition.SourceFile{{Path: "a.go", URI: "file://" + root + "/a.go", LanguageID: "go"}}, Qualification: in, Selection: inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "guard114"}, Workspace: root, PartitionSize: 16, Preparation: censusApplicationQualifiedOptionsPrivate{RevisionAssertion: censusInventorySourceRevision, Policy: policy, MaxArtifactBytes: 1 << 20, MaxMetadataBytes: 48000}, TimeoutMS: 30000, OutputDirectory: root, OutputName: "never-published114"}
	q, e := validateCensusQualifiedCLIPrivate(env)
	if e != nil {
		t.Fatal(e)
	}
	_, identity, e := censusQualifiedCLIIdentityPrivate(env, q)
	if e != nil {
		t.Fatal(e)
	}
	return censusInventoryHeader{Schema: censusInventoryQualifiedHeaderSchema, SourceRevision: censusInventorySourceRevision, Workspace: root, PlanIdentity: q.QualifiedDigest, Workflow: "standing114", Scope: m.ScopeDigest, Provenance: env.Universe.Claim, Plan: q.AnalysisPlan, UntouchedGaps: []string{}, Exclusions: []string{}, QualifiedAdmission: &censusInventoryQualifiedAdmission{Qualified: q, Envelope: env, PreparationIdentity: identity}}
}
func serialization114Real(t testing.TB) censusInventoryHeader {
	t.Helper()
	path := os.Getenv("PREPARE114_REAL_HEADER")
	if path == "" {
		t.Skip("opt-in pinned real fixture absent; self-contained standing canonical/store guards still run")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(b)
	if len(b) != 4074630 || hex.EncodeToString(sum[:]) != "323517c6cbb39c03ae5b03993010fa6de79036e2d9ddbd20afff1466b56a1517" {
		t.Fatal("pinned real fixture integrity, NOT RED")
	}
	var h censusInventoryHeader
	if e = censusInventoryDecode(b, &h, 4<<20); e != nil {
		t.Fatal("typed fixture decode, NOT RED", e)
	}
	q := h.QualifiedAdmission.Qualified
	if len(q.Originals) != 1099 || q.Scope.IncludedCount != 1015 || q.Scope.ExcludedCount != 0 || q.Scope.UnresolvedCount != 84 {
		t.Fatal("original denominator fixture, NOT RED")
	}
	return h
}
func serialization114Store(t *testing.T, h censusInventoryHeader) censusInventoryCursorStore {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s, _, _, closeRoots, e := openCensusInventoryCursor(root, h.Workflow)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := closeRoots(); e != nil {
			t.Error("returned cursor cleanup", e)
		}
	})
	return s
}
func serialization114Canonical(t *testing.T, h censusInventoryHeader, id string) {
	t.Helper()
	want, e := json.Marshal(h)
	if e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	_, got, e := canonicalCensusInventoryHeader(h)
	serialization114Assert(t, id, e == nil && bytes.Equal(got, want), e)
	t.Logf("BODY114 bytes=%d ceiling=4194304 elapsed=%s", len(want), time.Since(start))
}
func serialization114Put(t *testing.T, h censusInventoryHeader, id string) {
	t.Helper()
	want, e := json.Marshal(h)
	if e != nil {
		t.Fatal(e)
	}
	s := serialization114Store(t, h)
	digest, e := s.put(context.Background(), h)
	if e != nil {
		serialization114Assert(t, id, false, e)
		return
	}
	got, e := s.objects.Get(context.Background(), digest)
	serialization114Assert(t, id, e == nil && bytes.Equal(got, want) && digest == censusInventoryDigest(want), e)
}
func TestQualifiedSerialization114SmallControls(t *testing.T) {
	h := serialization114Standing(t)
	_, raw, e := canonicalCensusInventoryHeader(h)
	serialization114Assert(t, "small matching canonical", e == nil && len(raw) > 0, e)
	h.SourceRevision = strings.Repeat("0", 40)
	_, _, e = canonicalCensusInventoryHeader(h)
	serialization114Assert(t, "wrong SOURCE rejected", e != nil, e)
}
func TestQualifiedSerialization114CanonicalStanding(t *testing.T) {
	h := serialization114Standing(t)
	h.UntouchedGaps = []string{strings.Repeat("x", 800000)}
	serialization114Canonical(t, h, "standing canonical exact bytes")
}
func TestQualifiedSerialization114StoreStanding(t *testing.T) {
	h := serialization114Standing(t)
	h.UntouchedGaps = []string{strings.Repeat("x", 800000)}
	serialization114Put(t, h, "standing direct store exact bytes")
}
func TestQualifiedSerialization114RealCanonical(t *testing.T) {
	serialization114Canonical(t, serialization114Real(t), "real canonical exact bytes")
}
func TestQualifiedSerialization114RealStore(t *testing.T) {
	serialization114Put(t, serialization114Real(t), "real direct store exact bytes")
}
func TestQualifiedSerialization114RealInitialize(t *testing.T) {
	h := serialization114Real(t)
	s := serialization114Store(t, h)
	c, e := s.initialize(context.Background(), h)
	if e != nil {
		serialization114Assert(t, "real initialize full custody", false, e)
		return
	}
	loaded, e := s.load(context.Background())
	got, readErr := s.objects.Get(context.Background(), c.Header)
	want, _ := json.Marshal(h)
	serialization114Assert(t, "real initialize full custody", e == nil && readErr == nil && loaded.Header == c.Header && c.Generation == 1 && bytes.Equal(got, want) && c.Header == censusInventoryDigest(want), e)
}
func TestQualifiedSerialization114InspectClosedDomain(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem())
		case reflect.Struct:
			t.Logf("DOMAIN114 %s.%s fields=%d methods=%d", typ.PkgPath(), typ.Name(), typ.NumField(), typ.NumMethod())
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				t.Logf("FIELD114 %s %s %s tag=%q embedded=%t", typ.Name(), f.Name, f.Type, f.Tag.Get("json"), f.Anonymous)
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeOf(censusInventoryHeader{}))
}

func TestQualifiedSerialization114Escapes(t *testing.T) {
	corpus := []string{"", "plain ASCII /", "\x00\b\f\n\r\t\"\\<>&", "\u2028\u2029", "é", "e\u0301", "😀�", "\xff\xc0\xaf\xe2\x82"}
	for i, text := range corpus {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			h := censusInventoryHeader{QualifiedAdmission: &censusInventoryQualifiedAdmission{}, UntouchedGaps: []string{text}, Exclusions: []string{}}
			h.QualifiedAdmission.Envelope.Qualification.Candidates = []inventoryscope.CandidateV1{{LanguageID: text}}
			h.Plan.Authority = 255
			h.Plan.Partitions = []censussourceplan.Partition{{Ordinal: math.MinInt}, {Ordinal: math.MaxInt}}
			want, _ := json.Marshal(h)
			n, e := censusInventoryQualifiedSize(h, 4<<20)
			got, err := censusInventoryQualifiedMarshal(h, 4<<20)
			after, _ := json.Marshal(h)
			serialization114Assert(t, "finite escape/integers/input-preservation", e == nil && err == nil && n == len(want) && bytes.Equal(got, want) && bytes.Equal(after, want), []any{n, len(want), e, err})
		})
	}
	// Nil/empty slices and nil QualifiedAdmission (omitempty) are observably
	// different JSON forms. The counter must match each, not normalize them.
	for _, h := range []censusInventoryHeader{{}, {UntouchedGaps: []string{}, Exclusions: []string{}}, {QualifiedAdmission: &censusInventoryQualifiedAdmission{}}} {
		want, _ := json.Marshal(h)
		n, e := censusInventoryQualifiedSize(h, 4<<20)
		serialization114Assert(t, "nil empty and omission exact", e == nil && n == len(want), e)
	}
}

func TestQualifiedSerialization114CapAndAllocation(t *testing.T) {
	h := serialization114Standing(t)
	h.UntouchedGaps = []string{""}
	base, _ := json.Marshal(h)
	h.UntouchedGaps[0] = strings.Repeat("x", (4<<20)-len(base))
	t.Run("exact", func(t *testing.T) {
		n, e := censusInventoryQualifiedSize(h, 4<<20)
		raw, err := censusInventoryQualifiedMarshal(h, 4<<20)
		serialization114Assert(t, "exact cap accepted", e == nil && err == nil && n == 4<<20 && len(raw) == 4<<20, []any{n, e, err})
	})
	h.UntouchedGaps[0] += "x"
	t.Run("overcap", func(t *testing.T) {
		_, e := censusInventoryQualifiedSize(h, 4<<20)
		raw, err := censusInventoryQualifiedMarshal(h, 4<<20)
		serialization114Assert(t, "cap plus one refused", e != nil && err != nil && raw == nil, []any{e, err})
	})
	t.Run("routes", func(t *testing.T) {
		_, _, e := canonicalCensusInventoryHeader(h)
		s := serialization114Store(t, h)
		digest, err := s.put(context.Background(), h)
		serialization114Assert(t, "overcap canonical and store refuse", e != nil && err != nil && digest == "", []any{e, err})
	})
	t.Run("allocation", func(t *testing.T) {
		// Warm the local reflection schema before measuring, outside the interval.
		// One refused whole body is >4MiB; two calls may allocate <1MiB TOTAL.
		// This asserts a finite allocation witness, not a universal runtime theorem.
		_, _ = censusInventoryQualifiedMarshal(h, 4<<20)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for i := 0; i < 2; i++ {
			_, _ = censusInventoryQualifiedMarshal(h, 4<<20)
		}
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		serialization114Assert(t, "refusal before whole-body allocation", allocated < 1<<20, allocated)
		t.Logf("ALLOCATION114 overcap_body=%d calls=2 allocated_bytes=%d bound=1048576", (4<<20)+1, allocated)
	})
}

func TestQualifiedSerialization114DomainBounds(t *testing.T) {
	fields, ok := censusInventoryQualifiedSchema()
	serialization114Assert(t, "known acyclic schema", ok, len(fields))
	for _, depth := range []int{32, 33} {
		s := censusInventoryQualifiedSizer{left: 4 << 20, fields: fields}
		ok := s.value(reflect.ValueOf("x"), depth)
		serialization114Assert(t, "depth32 accepted depth33 refused", ok == (depth == 32), depth)
	}
	for _, count := range []int{100000, 100001} {
		v := make([]string, count)
		n, e := censusInventoryQualifiedSize(v, 4<<20)
		want, _ := json.Marshal(v)
		serialization114Assert(t, "fanout100000 accepted 100001 refused", (count == 100000 && e == nil && n == len(want)) || (count == 100001 && e != nil), []any{count, n, e})
	}
	cyclic := make([]any, 1)
	cyclic[0] = cyclic
	values := []any{nil, map[string]string{}, []byte("x"), json.RawMessage(`"custom"`), time.Time{}, cyclic, (*censusInventoryHeader)(nil), struct{ X string }{"unsupported"}, 1.5, make(chan int)}
	for i, v := range values {
		_, e := censusInventoryQualifiedSize(v, 4<<20)
		serialization114Assert(t, "unsupported custom cyclic refused", e != nil, []any{i, e})
	}
	for _, limit := range []int{0, -1, (4 << 20) + 1} {
		_, e := censusInventoryQualifiedSize("x", limit)
		serialization114Assert(t, "no ceiling enlargement", e != nil, limit)
	}
}

func TestQualifiedSerialization114LegacyDispatch(t *testing.T) {
	h := serialization114Standing(t)
	legacy := h
	legacy.QualifiedAdmission = nil
	legacy.Schema = "private-connected-inventory-header.v1"
	legacy.PlanIdentity = legacy.Plan.Identity
	want, e := censusInventoryMarshal(legacy, 4<<20)
	_, got, err := canonicalCensusInventoryHeader(legacy)
	serialization114Assert(t, "legacy canonical unchanged", e == nil && err == nil && bytes.Equal(got, want), []any{e, err})
	values := []any{legacy, &h, censusInventoryCursor{}, struct{ Plain string }{"legacy generic control"}}
	for i, v := range values {
		s := serialization114Store(t, h)
		want, e := censusInventoryMarshal(v, 4<<20)
		digest, err := s.put(context.Background(), v)
		got, readErr := s.objects.Get(context.Background(), digest)
		serialization114Assert(t, "legacy nonqualified put unchanged", e == nil && err == nil && readErr == nil && bytes.Equal(got, want), []any{i, e, err, readErr})
	}
	h.UntouchedGaps = []string{strings.Repeat("x", 800000)}
	s := serialization114Store(t, h)
	_, oldErr := censusInventoryMarshal(&h, 4<<20)
	_, err = s.put(context.Background(), &h)
	serialization114Assert(t, "pointer header NOT newly dispatched", oldErr != nil && err != nil, err)
}

func BenchmarkQualifiedSerialization114RealHeader(b *testing.B) {
	h := serialization114Real(b)
	want, _ := json.Marshal(h)
	b.Run("preflight", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			n, e := censusInventoryQualifiedSize(h, 4<<20)
			if e != nil || n != len(want) {
				b.Fatal(n, e)
			}
		}
	})
	b.Run("canonical", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for i := 0; i < b.N; i++ {
			_, raw, e := canonicalCensusInventoryHeader(h)
			if e != nil || !bytes.Equal(raw, want) {
				b.Fatal(e)
			}
		}
	})
}
