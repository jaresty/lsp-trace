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
	"reflect"
	"sort"
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
	return []byte(fmt.Sprintf(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[{"type":"position","label":"s-%03d","path":"f%03d.go","line":%d,"column":%d}]}`, i, i, i+11, i+4))
}
func target(i int) PreparedTarget {
	return PreparedTarget{i, seed(i), fmt.Sprintf("file:///w/f%03d.go", i), lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}}}
}
func discovery(n int) Discovery {
	d := Discovery{Session: SessionIdentity{"s", 7}, Complete: true}
	for i := 0; i < n; i++ {
		d.Targets = append(d.Targets, target(i))
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: fmt.Sprint(i), Disposition: "prepared"})
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
func run(t *testing.T, d Discovery, mut func(*[]byte, BatchRequest)) (Assembly, error) {
	t.Helper()
	return (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		r := v5(t, b)
		if mut != nil {
			mut(&r, b)
		}
		return AcquiredV5{b.Session, r}, nil
	})}).Run(context.Background(), SessionIdentity{"s", 7})
}
func projection(t *testing.T, a Assembly) Projection {
	t.Helper()
	p, e := a.Inspect()
	if e != nil {
		t.Fatal(e)
	}
	return p
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
func TestOpaqueCapabilityZeroReplayAndPublicationAtomicity(t *testing.T) {
	if _, e := (Assembly{}).Inspect(); e == nil {
		t.Fatal("zero assembly")
	}
	if e := Publish(context.Background(), PublicationCapability{}, publisherFunc(func(context.Context, PublicationCapability) error { return nil })); e == nil {
		t.Fatal("zero capability")
	}
	a, e := run(t, discovery(1), nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := a.PublicationCapability()
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	p := publisherFunc(func(context.Context, PublicationCapability) error { calls++; return nil })
	if e = Publish(context.Background(), c, p); e != nil || calls != 1 {
		t.Fatal(e, calls)
	}
	if e = Publish(context.Background(), c, p); e == nil || calls != 1 {
		t.Fatal("replay")
	}
	late := discovery(64)
	acq := 0
	_, e = (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return late, nil }), Acquirer: acquirerFunc(func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		acq++
		if acq == 2 {
			return AcquiredV5{}, errors.New("late")
		}
		return AcquiredV5{b.Session, v5(t, b)}, nil
	})}).Run(context.Background(), SessionIdentity{"s", 7})
	if e == nil || calls != 1 {
		t.Fatal("failure published")
	}
}

type publisherFunc func(context.Context, PublicationCapability) error

func (f publisherFunc) Publish(c context.Context, p PublicationCapability) error { return f(c, p) }
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
