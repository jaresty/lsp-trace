package censusacquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/lsp"
)

type discoveryFunc func(context.Context, SessionIdentity) (Discovery, error)

func (f discoveryFunc) Discover(c context.Context, s SessionIdentity) (Discovery, error) {
	return f(c, s)
}

type acquirerFunc func(context.Context, BatchRequest) (AcquiredV5, error)

func (f acquirerFunc) AcquireV5(c context.Context, b BatchRequest) (AcquiredV5, error) {
	return f(c, b)
}

func authority() captureset.ExactBytesAuthority {
	return captureset.ExactBytesAuthority{AdmitGraphProvenanceV5: func(b []byte) (string, error) {
		if !strings.HasPrefix(string(b), "v5:") {
			return "", errors.New("not v5")
		}
		s := sha256.Sum256(b)
		return "native:" + hex.EncodeToString(s[:]), nil
	}}
}
func target(i int) PreparedTarget {
	return PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: []byte(fmt.Sprintf("seed-%03d", i)), URI: fmt.Sprintf("file:///w/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}, End: lsp.Position{Line: uint32(i + 20)}}}
}
func discovery(n int) Discovery {
	d := Discovery{Session: SessionIdentity{"s", 7}, Complete: true}
	for i := 0; i < n; i++ {
		d.Targets = append(d.Targets, target(i))
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: fmt.Sprintf("sym-%d", i), Disposition: "prepared"})
	}
	d.Accounting.SymbolDenominator = n
	d.SymbolLedger.Denominator = n
	d.Accounting.Files = []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}
	d.Accounting.FileDenominator = 1
	d.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "f", Disposition: "processed"}}}
	return d
}
func runWith(d Discovery, af acquirerFunc) (Plan, error) {
	return (Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil }), Acquirer: af, Authority: authority()}).Run(context.Background(), SessionIdentity{"s", 7})
}
func goodAcquirer(_ context.Context, b BatchRequest) (AcquiredV5, error) {
	return AcquiredV5{Session: b.Session, Raw: []byte("v5:" + b.BatchID)}, nil
}

func TestFiltersExclusionsWin(t *testing.T) {
	f := Filters{Includes: []string{"*.go"}, Excludes: []string{"x.go"}}
	match := func(p, s string) bool { return p == "*.go" && strings.HasSuffix(s, ".go") || p == s }
	if f.Select("x.go", match) || !f.Select("y.go", match) {
		t.Fatal("exclusion did not win")
	}
}
func TestExactSelectionRangeStartInManifest(t *testing.T) {
	p, e := runWith(discovery(1), goodAcquirer)
	if e != nil {
		t.Fatal(e)
	}
	m := p.Batches[0].AcquisitionManifest(testLimits())
	if *m.Root.Locator.Line != 10 || *m.Root.Locator.Character != 3 {
		t.Fatalf("got %d:%d", *m.Root.Locator.Line, *m.Root.Locator.Character)
	}
}
func TestZeroFilesSymbolsAndPartialFailBeforeAcquisition(t *testing.T) {
	cases := []Discovery{{Session: SessionIdentity{"s", 7}, Complete: true}, {Session: SessionIdentity{"s", 7}, Complete: false}, discovery(0)}
	for i, d := range cases {
		calls := 0
		_, e := runWith(d, func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })
		if e == nil || calls != 0 {
			t.Fatalf("case %d err=%v calls=%d", i, e, calls)
		}
	}
}
func TestDuplicateTargetAndOrdinalRejected(t *testing.T) {
	for _, mutate := range []func(*Discovery){func(d *Discovery) { d.Targets[1].CanonicalSeedV2 = d.Targets[0].CanonicalSeedV2 }, func(d *Discovery) { d.Targets[1].CensusOrdinal = d.Targets[0].CensusOrdinal }} {
		d := discovery(2)
		mutate(&d)
		calls := 0
		_, e := runWith(d, func(context.Context, BatchRequest) (AcquiredV5, error) { calls++; return AcquiredV5{}, nil })
		if e == nil || calls != 0 {
			t.Fatalf("err=%v calls=%d", e, calls)
		}
	}
}
func TestOver63BatchesExactOnceAndDefaults(t *testing.T) {
	d := discovery(130)
	var got []BatchRequest
	p, e := runWith(d, func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
		got = append(got, b)
		return goodAcquirer(nil, b)
	})
	if e != nil {
		t.Fatal(e)
	}
	if x := []int{len(got[0].Targets), len(got[1].Targets), len(got[2].Targets)}; !reflect.DeepEqual(x, []int{63, 63, 4}) {
		t.Fatal(x)
	}
	seen := map[int]bool{}
	for i, b := range got {
		if b.Ordinal != i || b.DownDepth != 1 || b.UpDepth != 0 || b.CensusID != p.CensusID || b.BatchID == "" {
			t.Fatal("batch invariant")
		}
		for _, x := range b.Targets {
			if seen[x.CensusOrdinal] {
				t.Fatal("duplicate")
			}
			seen[x.CensusOrdinal] = true
		}
	}
	if len(seen) != 130 {
		t.Fatal(len(seen))
	}
}
func TestPerturbationOrderDeterministic(t *testing.T) {
	a := discovery(70)
	b := discovery(70)
	sort.Slice(b.Targets, func(i, j int) bool { return i > j })
	p1, e := runWith(a, goodAcquirer)
	if e != nil {
		t.Fatal(e)
	}
	p2, e := runWith(b, goodAcquirer)
	if e != nil {
		t.Fatal(e)
	}
	if p1.CensusID != p2.CensusID || !reflect.DeepEqual(p1.ManifestBytes, p2.ManifestBytes) {
		t.Fatal("nondeterministic")
	}
	for i := range p1.Constituents {
		if !reflect.DeepEqual(p1.Constituents[i], p2.Constituents[i]) {
			t.Fatal("constituents differ")
		}
	}
}
func TestDiscoveryPreparationAcquisitionAccountingAndDriftFailures(t *testing.T) {
	base := discovery(2)
	cases := []struct {
		name        string
		d           Discovery
		discoverErr error
		af          acquirerFunc
	}{
		{"discovery", base, errors.New("partial"), goodAcquirer},
		{"accounting", func() Discovery { x := base; x.Accounting.SymbolDenominator++; return x }(), nil, goodAcquirer},
		{"ledger", func() Discovery { x := base; x.SymbolLedger.Denominator++; return x }(), nil, goodAcquirer},
		{"acquisition", base, nil, func(context.Context, BatchRequest) (AcquiredV5, error) { return AcquiredV5{}, errors.New("partial") }},
		{"discovery-drift", func() Discovery { x := base; x.Session.Generation++; return x }(), nil, goodAcquirer},
		{"acquisition-drift", base, nil, func(_ context.Context, b BatchRequest) (AcquiredV5, error) {
			b.Session.Generation++
			return AcquiredV5{Session: b.Session, Raw: []byte("v5:x")}, nil
		}},
		{"preparation", func() Discovery { x := base; x.Targets[0].CanonicalSeedV2 = nil; return x }(), nil, goodAcquirer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acq := 0
			core := Core{Discoverer: discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return tc.d, tc.discoverErr }), Acquirer: acquirerFunc(func(c context.Context, b BatchRequest) (AcquiredV5, error) { acq++; return tc.af(c, b) }), Authority: authority()}
			if _, e := core.Run(context.Background(), SessionIdentity{"s", 7}); e == nil {
				t.Fatal("success")
			}
			if (tc.name == "discovery" || tc.name == "accounting" || tc.name == "ledger" || tc.name == "discovery-drift" || tc.name == "preparation") && acq != 0 {
				t.Fatal("acquired before validation")
			}
		})
	}
}
func TestSuccessPlanExactConstituentsAndPrivateCeilings(t *testing.T) {
	p, e := runWith(discovery(64), goodAcquirer)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Constituents) != 2 || len(p.Batches) != 2 {
		t.Fatal("cardinality")
	}
	for i, c := range p.Constituents {
		if c.Ordinal != i || c.BatchID != p.Batches[i].BatchID || string(c.Raw) != "v5:"+c.BatchID {
			t.Fatal("constituent mismatch")
		}
	}
	m := p.Manifest
	if m.Authority != 0 || m.SourceGraphComplete != "UNKNOWN" || m.NativeSingleCaptureCustody || len(m.CrossCaptureCalls) != 0 || m.LeidenAdmissible || m.Disclosure != "PRIVATE" {
		t.Fatal("authority ceiling")
	}
	if e := p.Validate(authority()); e != nil {
		t.Fatal(e)
	}
}
func TestNoPublicationCapabilityAndFailuresReturnZeroPlan(t *testing.T) {
	typ := reflect.TypeOf(Core{})
	if _, ok := typ.FieldByName("Publisher"); ok {
		t.Fatal("core must not have publisher")
	}
	p, e := runWith(discovery(1), func(context.Context, BatchRequest) (AcquiredV5, error) { return AcquiredV5{}, errors.New("x") })
	if e == nil || !reflect.DeepEqual(p, Plan{}) {
		t.Fatal("failure leaked assembly")
	}
}
func TestPlanValidationRejectsDuplicateBatchOrdinal(t *testing.T) {
	p, e := runWith(discovery(64), goodAcquirer)
	if e != nil {
		t.Fatal(e)
	}
	p.Batches[1].Ordinal = 0
	if e := p.Validate(authority()); e == nil {
		t.Fatal("accepted duplicate ordinal")
	}
}
func testLimits() acquisitionops.Limits {
	n := func(x int) *int { return &x }
	return acquisitionops.Limits{MaxNodes: n(100), MaxRequests: n(1000), MaxEvidenceBytes: n(1 << 20), MaxPathWork: n(10000), TimeoutMS: n(1000), RequestTimeoutMS: n(1000), MaxResponseBytes: n(1 << 20), MaxMessages: n(64)}
}
