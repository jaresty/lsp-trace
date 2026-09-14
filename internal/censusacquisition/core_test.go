package censusacquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/acquisitionops"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/seedformat"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type discoveryFunc func(context.Context, SessionIdentity) (Discovery, error)

func (f discoveryFunc) Discover(c context.Context, s SessionIdentity) (Discovery, error) {
	return f(c, s)
}

type acquirerFunc func(context.Context, BatchRequest) (AcquiredV5, error)

func (f acquirerFunc) AcquireV5(c context.Context, b BatchRequest) (AcquiredV5, error) {
	return f(c, b)
}
func seed(i int) []byte {
	raw, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: fmt.Sprintf("f%03d.go", i), Line: uint64(i + 11), Column: uint64(i + 4)}}}}, "/w")
	if err != nil {
		panic(err)
	}
	return raw
}
func target(i int) PreparedTarget {
	name := fmt.Sprintf("Target%d", i)
	return PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed(i), URI: fmt.Sprintf("file:///w/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("f%03d.go#%d:%d:%d:%s:%d", i, i+10, i+3, 12, name, i)}
}
func discovery(n int) Discovery {
	d := Discovery{Session: SessionIdentity{"s", 7}, Workspace: "/w", Complete: true}
	for i := 0; i < n; i++ {
		d.Targets = append(d.Targets, target(i))
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: d.Targets[i].SymbolIdentity, Disposition: "prepared"})
	}
	d.Accounting.SymbolDenominator = n
	d.SymbolLedger.Denominator = n
	d.Accounting.Files = []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}
	d.Accounting.FileDenominator = 1
	d.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "f", Disposition: "processed"}}}
	return d
}
func v5(t *testing.T, b BatchRequest) []byte {
	t.Helper()
	n := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Seeds: []graph.InvocationSeed{}, Provenance: graph.InvocationProvenance{InvocationID: "i", SourceRevision: "r", ServerVersion: "v"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, e := json.Marshal(n)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := graphprovenance.CaptureV5WithSeedSpec(native, b.Session.SessionID, b.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, b.CanonicalSeedsV2)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func run(t *testing.T, d Discovery, mut func(*[]byte, BatchRequest)) (Projection, error) {
	t.Helper()
	return (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		r := v5(t, b)
		if mut != nil {
			mut(&r, b)
		}
		return AcquiredV5{b.Session, r}, nil
	})}).Run(context.Background(), SessionIdentity{"s", 7})
}
func projection(t *testing.T, p Projection) Projection {
	t.Helper()
	return cloneProjection(p)
}
func seedPath(path string) []byte {
	raw, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "census-000000", Path: filepath.ToSlash(path), Line: 1, Column: 1}}}}, "/")
	if err != nil {
		panic(err)
	}
	return raw
}

func TestCoreBatchFailurePreservesExactOneBasedOrdinal(t *testing.T) {
	for _, tc := range []struct {
		targets, failOrdinal, want int
	}{{1, 0, 1}, {64, 1, 2}, {127, 2, 3}} {
		t.Run(fmt.Sprintf("targets-%d", tc.targets), func(t *testing.T) {
			cause := errors.New("late acquisition")
			_, err := (Core{
				Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return discovery(tc.targets), nil }),
				Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
					if b.Ordinal == tc.failOrdinal {
						return AcquiredV5{}, fmt.Errorf("wrapped: %w", cause)
					}
					return AcquiredV5{Session: b.Session, Raw: v5(t, b)}, nil
				}),
			}).Run(context.Background(), SessionIdentity{"s", 7})
			var failure interface{ BatchOrdinal() int }
			if !errors.As(err, &failure) || failure.BatchOrdinal() != tc.want {
				t.Fatalf("ASSERT_ONE_BASED_BATCH_FAILURE got=%v want=%d err=%v", failure, tc.want, err)
			}
			if !errors.Is(err, cause) {
				t.Fatal("ASSERT_BATCH_FAILURE_WRAPPING")
			}
		})
	}
}

func TestCorePreBatchAndDiscoveryFailuresHaveNoOrdinal(t *testing.T) {
	for _, err := range []error{
		errors.New("unknown"),
		func() error {
			_, err := (Core{}).Run(context.Background(), SessionIdentity{"s", 7})
			return err
		}(),
		func() error {
			_, err := (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) {
				return Discovery{}, errors.New("discovery")
			}), Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) { return AcquiredV5{}, nil })}).Run(context.Background(), SessionIdentity{"s", 7})
			return err
		}(),
	} {
		var failure interface{ BatchOrdinal() int }
		if errors.As(err, &failure) {
			t.Fatalf("ASSERT_PRE_BATCH_NO_ORDINAL err=%v", err)
		}
	}
}

func TestReconcileSeedUsesExactCanonicalWorkspaceIdentity(t *testing.T) {
	workspace := t.TempDir()
	uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	targetFor := func(rawURI, seedRelative string) PreparedTarget {
		return PreparedTarget{CanonicalSeedV2: seedPath(seedRelative), URI: rawURI}
	}
	validRelative := filepath.Join("dir", "space é.go")
	validPath := filepath.Join(workspace, validRelative)
	cases := []struct {
		name, workspace, uri, seedPath string
		wantOK                         bool
	}{
		{"spaces-unicode", workspace, uri(validPath), validRelative, true},
		{"suffix-collision", workspace, uri(filepath.Join(filepath.Dir(workspace), "other", filepath.Base(workspace), "same.go")), "same.go", false},
		{"dot-dot", workspace, (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace) + "/dir/../same.go"}).String(), "same.go", false},
		{"encoded-separator", workspace, strings.Replace(uri(filepath.Join(workspace, "dir", "same.go")), "/dir/same.go", "/dir%2Fsame.go", 1), filepath.Join("dir", "same.go"), false},
		{"encoded-dot", workspace, strings.Replace(uri(filepath.Join(workspace, "same.go")), "/same.go", "/%2e/same.go", 1), "same.go", false},
		{"malformed-escape", workspace, "file:///bad%zz.go", "bad.go", false},
		{"query", workspace, uri(filepath.Join(workspace, "same.go")) + "?x=1", "same.go", false},
		{"fragment", workspace, uri(filepath.Join(workspace, "same.go")) + "#x", "same.go", false},
		{"authority", workspace, "file://host" + filepath.ToSlash(filepath.Join(workspace, "same.go")), "same.go", false},
		{"outside-workspace", workspace, uri(filepath.Join(filepath.Dir(workspace), "same.go")), "same.go", false},
		{"case-variant", workspace, uri(filepath.Join(workspace, "Case.go")), "case.go", runtime.GOOS == "windows"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := reconcileSeed(targetFor(tc.uri, tc.seedPath), tc.workspace)
			if (err == nil) != tc.wantOK {
				t.Fatalf("ASSERT_EXACT_CANONICAL_URI_IDENTITY: err=%v", err)
			}
		})
	}
}

func TestPlatformPathEqualUsesCanonicalSeedsV2Separators(t *testing.T) {
	cases := []struct {
		name, goos, left, right string
		want                    bool
		assertion               string
	}{
		{"windows-nested-spaces-unicode", "windows", "Dir/space é/file.go", `dir\SPACE É\FILE.GO`, true, "ASSERT_PLATFORM_PATH_EQUAL_WINDOWS_CANONICAL_SLASH"},
		{"windows-mixed-separators", "windows", `Dir\nested/file.go`, `dir/nested\FILE.GO`, true, "ASSERT_PLATFORM_PATH_EQUAL_WINDOWS_CANONICAL_SLASH"},
		{"windows-traversal-alias", "windows", "dir/file.go", `dir\nested\..\file.go`, false, "ASSERT_PLATFORM_PATH_EQUAL_REJECTS_TRAVERSAL_ALIAS"},
		{"unix-case-sensitive", "linux", "Dir/space é/file.go", "dir/space é/file.go", false, "ASSERT_PLATFORM_PATH_EQUAL_NONWINDOWS_LEXICAL"},
		{"unix-backslash-not-separator", "linux", "dir/file.go", `dir\file.go`, false, "ASSERT_PLATFORM_PATH_EQUAL_NONWINDOWS_LEXICAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := platformPathEqual(tc.goos, tc.left, tc.right); got != tc.want {
				t.Fatalf("%s: got %v want %v", tc.assertion, got, tc.want)
			}
		})
	}
}

func TestInvalidCompleteSymbolLedgerHasNoAcquisitionSideEffects(t *testing.T) {
	mutations := map[string]func(*Discovery){
		"duplicate-accounting-ordinal": func(d *Discovery) { d.Accounting.Symbols[1].Ordinal = 0 },
		"out-of-range-accounting":      func(d *Discovery) { d.Accounting.Symbols[1].Ordinal = 2 },
		"malformed-accounting":         func(d *Discovery) { d.Accounting.Symbols[1].Disposition = "bogus" },
		"duplicate-ledger-ordinal":     func(d *Discovery) { d.SymbolLedger.Entries[1].Ordinal = 0 },
		"out-of-range-ledger":          func(d *Discovery) { d.SymbolLedger.Entries[1].Ordinal = 2 },
		"duplicate-ledger-identity":    func(d *Discovery) { d.SymbolLedger.Entries[1].Identity = d.SymbolLedger.Entries[0].Identity },
		"malformed-ledger-disposition": func(d *Discovery) {
			d.Accounting.Symbols[1].Disposition = census.SymbolUnsupported
			d.SymbolLedger.Entries[1].Disposition = "bogus"
			d.Targets = d.Targets[:1]
		},
		"duplicate-target-identity": func(d *Discovery) {
			d.Targets[1] = cloneTarget(d.Targets[0])
			d.Targets[1].CensusOrdinal = 1
		},
		"duplicate-target-uri": func(d *Discovery) {
			d.Targets[1].URI = d.Targets[0].URI
			d.Targets[1].CanonicalSeedV2 = seedPath("f000.go")
			d.Targets[1].SelectionRange = lsp.Range{}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			d := discovery(2)
			mutate(&d)
			calls := 0
			_, err := (Core{
				Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }),
				Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) {
					calls++
					return AcquiredV5{}, nil
				}),
			}).Run(context.Background(), SessionIdentity{"s", 7})
			if err == nil || calls != 0 {
				t.Fatalf("ASSERT_INVALID_LEDGER_ZERO_ACQUIRER_CALLS: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestCoreRejectsOversizedDiscoveryBeforeAcquisition(t *testing.T) {
	d := Discovery{Session: SessionIdentity{"s", 7}, Workspace: "/w", Complete: true}
	d.Accounting.SymbolDenominator = captureset.MaxTargets + 1
	d.SymbolLedger.Denominator = captureset.MaxTargets + 1
	calls := 0
	_, err := (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })}).Run(context.Background(), SessionIdentity{"s", 7})
	if err == nil || calls != 0 || !strings.Contains(err.Error(), "discovery bounds exceeded") {
		t.Fatalf("ASSERT_CORE_BOUNDS_BEFORE_ITERATION: err=%v calls=%d", err, calls)
	}
}

func TestCanonicalTargetsRejectDuplicateExactCoordinatesAndNoncanonicalBytes(t *testing.T) {
	d := discovery(2)
	d.Targets[1].URI = d.Targets[0].URI
	d.Targets[1].SelectionRange = d.Targets[0].SelectionRange
	d.Targets[1].SymbolIdentity = "f000.go#10:3:12:Target1:1"
	d.SymbolLedger.Entries[1].Identity = d.Targets[1].SymbolIdentity
	raw, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "census-000001", Path: "f000.go", Line: 11, Column: 4}}}}, "/w")
	if err != nil {
		t.Fatal(err)
	}
	d.Targets[1].CanonicalSeedV2 = raw
	calls := 0
	_, err = (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })}).Run(context.Background(), SessionIdentity{"s", 7})
	if err == nil || calls != 0 || !strings.Contains(err.Error(), "duplicate exact seed coordinates") {
		t.Fatalf("ASSERT_DISCOVERY_DUPLICATE_TARGET_REJECTED: err=%v calls=%d", err, calls)
	}

	d = discovery(1)
	d.Targets[0].CanonicalSeedV2 = append(append([]byte(nil), d.Targets[0].CanonicalSeedV2...), ' ')
	d.Complete = true
	_, err = (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })}).Run(context.Background(), SessionIdentity{"s", 7})
	if err == nil || !strings.Contains(err.Error(), "noncanonical seed bytes") {
		t.Fatalf("ASSERT_COMPLETE_REQUIRES_STRICT_CANONICAL_RECONCILIATION: %v", err)
	}
}

func TestBijectionRejectsLedgerAndTargetGaps(t *testing.T) {
	for _, m := range []func(*Discovery){func(d *Discovery) { d.Targets = d.Targets[:1] }, func(d *Discovery) { d.SymbolLedger.Entries[1].Disposition = "failed" }, func(d *Discovery) { d.Accounting.Symbols[1].Disposition = census.SymbolUnsupported }, func(d *Discovery) { d.Targets[1].CensusOrdinal = 99 }, func(d *Discovery) { d.Targets[1].SelectionRange.Start.Line++ }} {
		d := discovery(2)
		m(&d)
		calls := 0
		_, e := (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })}).Run(context.Background(), SessionIdentity{"s", 7})
		if e == nil || calls != 0 {
			t.Fatalf("err=%v calls=%d", e, calls)
		}
	}
}
func TestOver63DeterminismAndDeepCopy(t *testing.T) {
	a, e := run(t, discovery(130), nil)
	if e != nil {
		t.Fatal(e)
	}
	p := projection(t, a)
	if got := []int{len(p.Batches[0].Targets), len(p.Batches[1].Targets), len(p.Batches[2].Targets)}; !reflect.DeepEqual(got, []int{63, 63, 4}) {
		t.Fatal(got)
	}
	d := discovery(130)
	sort.Slice(d.Targets, func(i, j int) bool { return i > j })
	b, e := run(t, d, nil)
	if e != nil {
		t.Fatal(e)
	}
	q := projection(t, b)
	if p.CensusID != q.CensusID || !reflect.DeepEqual(p.ManifestBytes, q.ManifestBytes) {
		t.Fatal("nondeterministic")
	}
	p.ManifestBytes[0] ^= 1
	p.Batches[0].Targets[0].CanonicalSeedV2[0] ^= 1
	r := projection(t, a)
	if p.ManifestBytes[0] == r.ManifestBytes[0] || p.Batches[0].Targets[0].CanonicalSeedV2[0] == r.Batches[0].Targets[0].CanonicalSeedV2[0] {
		t.Fatal("projection aliases authority")
	}
}
func TestExactV5AdmissionRejectsMutations(t *testing.T) {
	cases := []func(*[]byte, BatchRequest){func(r *[]byte, b BatchRequest) { *r = nil }, func(r *[]byte, b BatchRequest) { (*r)[0] ^= 1 }, func(r *[]byte, b BatchRequest) {
		var e graphprovenance.EvidenceV5
		json.Unmarshal(*r, &e)
		e.SessionID = "wrong"
		*r, _ = json.Marshal(e)
	}, func(r *[]byte, b BatchRequest) {
		var e graphprovenance.EvidenceV5
		json.Unmarshal(*r, &e)
		e.Generation++
		*r, _ = json.Marshal(e)
	}, func(r *[]byte, b BatchRequest) {
		var e graphprovenance.EvidenceV5
		json.Unmarshal(*r, &e)
		e.SeedSpec.Bytes = []byte(`{"schema_version":"lsp-trace.seeds.v2"}`)
		*r, _ = json.Marshal(e)
	}}
	for i, m := range cases {
		if _, e := run(t, discovery(1), m); e == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}
func TestCoreDoesNotReturnPartialProjectionAfterLateBatchFailure(t *testing.T) {
	late := discovery(64)
	acquired := 0
	projection, err := (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return late, nil }), Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		acquired++
		if acquired == 2 {
			return AcquiredV5{}, errors.New("late")
		}
		return AcquiredV5{b.Session, v5(t, b)}, nil
	})}).Run(context.Background(), SessionIdentity{"s", 7})
	if err == nil || projection.CensusID != "" {
		t.Fatal("partial projection returned after failure")
	}
}
func TestManifestAssociationAndRecomputedMutations(t *testing.T) {
	a, e := run(t, discovery(64), nil)
	if e != nil {
		t.Fatal(e)
	}
	p := projection(t, a)
	mutations := []func(*Projection){func(x *Projection) { x.CensusID = "x" }, func(x *Projection) { x.Batches[1].Ordinal = 0 }, func(x *Projection) { x.Batches[0].BatchID = "x" }, func(x *Projection) { x.Batches[0].DownDepth = 2 }, func(x *Projection) {
		x.Batches[0].Targets[0], x.Batches[0].Targets[1] = x.Batches[0].Targets[1], x.Batches[0].Targets[0]
	}, func(x *Projection) { x.Constituents[0], x.Constituents[1] = x.Constituents[1], x.Constituents[0] }, func(x *Projection) {
		x.Manifest.Batches[0].ConstituentIndex, x.Manifest.Batches[1].ConstituentIndex = x.Manifest.Batches[1].ConstituentIndex, x.Manifest.Batches[0].ConstituentIndex
	}, func(x *Projection) { x.ManifestBytes[0] ^= 1 }}
	for i, m := range mutations {
		x := cloneProjection(p)
		m(&x)
		if validateProjection(x) == nil {
			t.Fatalf("mutation %d accepted", i)
		}
	}
}
func TestExactSelectionRangeStartInManifest(t *testing.T) {
	a, e := run(t, discovery(1), nil)
	if e != nil {
		t.Fatal(e)
	}
	m := projection(t, a).Batches[0].AcquisitionManifest(acquisitionops.Limits{})
	if *m.Root.Locator.Line != 10 || *m.Root.Locator.Character != 3 {
		t.Fatal("wrong start")
	}
}

func TestPlanningDepthBoundsAndPropagation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		down, up int
		wantOK   bool
	}{
		{"zero", 0, 0, true}, {"maximum", 64, 64, true},
		{"negative-down", -1, 0, false}, {"excess-down", 65, 0, false},
		{"negative-up", 1, -1, false}, {"excess-up", 1, 65, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			core := Core{
				Planning:   &PlanningConfig{DownDepth: tc.down, UpDepth: tc.up},
				Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return discovery(64), nil }),
				Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
					calls++
					if b.DownDepth != tc.down || b.UpDepth != tc.up {
						t.Fatalf("depths not propagated: %+v", b)
					}
					manifest := b.AcquisitionManifest(acquisitionops.Limits{})
					for _, target := range append([]acquisitionops.Target{manifest.Root}, manifest.RequiredTargets...) {
						if target.DownDepth == nil || *target.DownDepth != tc.down || target.UpDepth == nil || *target.UpDepth != tc.up {
							t.Fatalf("manifest depths not propagated: %+v", target)
						}
					}
					return AcquiredV5{Session: b.Session, Raw: v5(t, b)}, nil
				}),
			}
			_, err := core.Run(context.Background(), SessionIdentity{"s", 7})
			if (err == nil) != tc.wantOK {
				t.Fatalf("wantOK=%v err=%v", tc.wantOK, err)
			}
			if !tc.wantOK && calls != 0 {
				t.Fatalf("invalid planning acquired %d batches", calls)
			}
		})
	}
}

func TestPlanningDefaultsRemainIdentityCompatible(t *testing.T) {
	legacy, err := run(t, discovery(64), nil)
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := runWithPlanning(t, discovery(64), &PlanningConfig{DownDepth: 1, UpDepth: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.CensusID != explicit.CensusID || !reflect.DeepEqual(legacy.ManifestBytes, explicit.ManifestBytes) {
		t.Fatal("default planning changed canonical identity")
	}
	for i := range legacy.Batches {
		if legacy.Batches[i].BatchID != explicit.Batches[i].BatchID || legacy.Batches[i].DownDepth != 1 || legacy.Batches[i].UpDepth != 0 {
			t.Fatal("default batch identity changed")
		}
	}
}

func TestDepthAwarePlanCountsRangesAndDeterministicReorder(t *testing.T) {
	for _, count := range []int{1, 62, 63, 64, 126, 10000} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			d := discovery(count)
			first, err := runWithPlanning(t, d, &PlanningConfig{DownDepth: 2, UpDepth: 3}, nil)
			if err != nil {
				t.Fatal(err)
			}
			sort.Slice(d.Targets, func(i, j int) bool { return i > j })
			second, err := runWithPlanning(t, d, &PlanningConfig{DownDepth: 2, UpDepth: 3}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if first.CensusID != second.CensusID || !reflect.DeepEqual(first.ManifestBytes, second.ManifestBytes) {
				t.Fatal("reorder changed plan identity")
			}
			next := 0
			for ordinal, batch := range first.Batches {
				if batch.Ordinal != ordinal || len(batch.Targets) < 1 || len(batch.Targets) > 63 {
					t.Fatalf("invalid batch %d size %d", ordinal, len(batch.Targets))
				}
				for _, target := range batch.Targets {
					if target.CensusOrdinal != next {
						t.Fatalf("noncontiguous range: got %d want %d", target.CensusOrdinal, next)
					}
					next++
				}
			}
			if next != count {
				t.Fatalf("planned %d targets want %d", next, count)
			}
		})
	}
}

func TestDuplicatePlanningInputsAreRejectedBeforeAcquisition(t *testing.T) {
	cases := map[string]func(*Discovery){
		"target": func(d *Discovery) {
			d.Targets[1].SymbolIdentity = d.Targets[0].SymbolIdentity
			d.SymbolLedger.Entries[1].Identity = d.Targets[0].SymbolIdentity
		},
		"coordinate": func(d *Discovery) {
			d.Targets[1].URI = d.Targets[0].URI
			d.Targets[1].SelectionRange = d.Targets[0].SelectionRange
		},
		"ordinal": func(d *Discovery) { d.Targets[1].CensusOrdinal = d.Targets[0].CensusOrdinal },
		"seed": func(d *Discovery) {
			d.Targets[1].CanonicalSeedV2 = append([]byte(nil), d.Targets[0].CanonicalSeedV2...)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := discovery(2)
			mutate(&d)
			calls := 0
			_, err := (Core{
				Planning:   &PlanningConfig{DownDepth: 2, UpDepth: 3},
				Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }),
				Acquirer: acquirerFunc(func(context.Context, BatchRequest) (AcquiredV5, error) {
					calls++
					return AcquiredV5{}, nil
				}),
			}).Run(context.Background(), SessionIdentity{"s", 7})
			if err == nil || calls != 0 {
				t.Fatalf("duplicate %s reached acquisition: err=%v calls=%d", name, err, calls)
			}
		})
	}
}

func TestAllBatchesArePlannedAndIsolatedBeforeAcquisition(t *testing.T) {
	var firstID string
	projection, err := runWithPlanning(t, discovery(64), &PlanningConfig{DownDepth: 2, UpDepth: 3}, func(b *BatchRequest) {
		if b.Ordinal == 0 {
			firstID = b.BatchID
			b.BatchID = "mutated"
			b.Ordinal = 99
			b.DownDepth = 64
			b.Targets[0].CensusOrdinal = 99
			b.CanonicalSeedsV2[0] ^= 1
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.Batches[0].BatchID != firstID || projection.Batches[0].Ordinal != 0 || projection.Batches[0].DownDepth != 2 || projection.Batches[0].Targets[0].CensusOrdinal != 0 {
		t.Fatal("acquirer mutated planned batch")
	}
}

func runWithPlanning(t *testing.T, d Discovery, planning *PlanningConfig, observe func(*BatchRequest)) (Projection, error) {
	t.Helper()
	return (Core{
		Planning:   planning,
		Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }),
		Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
			original := cloneBatchRequest(b)
			if observe != nil {
				observe(&b)
			}
			return AcquiredV5{Session: original.Session, Raw: v5(t, original)}, nil
		}),
	}).Run(context.Background(), SessionIdentity{"s", 7})
}
