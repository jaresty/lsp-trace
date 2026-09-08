package hydratedevidence

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
)

func TestIndependentIntervalOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	for trial := 0; trial < 100; trial++ {
		origins := []Origin{}
		coverage := make([]bool, 64)
		for i := 0; i < 20; i++ {
			lo := rng.Intn(63)
			hi := lo + 1 + rng.Intn(64-lo)
			iv := Interval{lo, hi}
			origins = append(origins, Origin{Selection: Selection{ID: string(rune('a' + i)), SourceID: "source", Encoding: "utf-8"}, Status: "PENDING", Bytes: &iv})
			for j := lo; j < hi; j++ {
				coverage[j] = true
			}
		}
		want := []Interval{}
		for i := 0; i < len(coverage); {
			if !coverage[i] {
				i++
				continue
			}
			lo := i
			for i < len(coverage) && coverage[i] {
				i++
			}
			want = append(want, Interval{lo, i})
		}
		for _, got := range [][]group{groups(origins), sweepGroups(origins)} {
			ivs := []Interval{}
			seen := map[int]int{}
			for _, g := range got {
				ivs = append(ivs, g.interval)
				for _, i := range g.indices {
					seen[i]++
				}
			}
			if !same(ivs, want) {
				t.Fatalf("independent interval oracle: %v != %v", ivs, want)
			}
			for i := range origins {
				if seen[i] != 1 {
					t.Fatal("origin union coverage")
				}
			}
		}
	}
}
func TestV2NativeAdmission(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	if e := os.WriteFile(path, []byte("A😀xyz\n"), 0600); e != nil {
		t.Fatal(e)
	}
	line, char := uint32(0), uint32(0)
	req := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "ctx", SessionID: "s", Generation: 1, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &line, Character: &char}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 20, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	client := acquisition.NewWireClient(func(_ context.Context, r acquisition.WireRequest) (json.RawMessage, error) {
		if r.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Character: 3}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}, Data: json.RawMessage(`{"uri":"file:///opaque-ignored","range":{"start":0}}`)}})
		}
		return json.RawMessage(`[]`), nil
	})
	result, e := acquisition.Acquire(context.Background(), client, req)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := graphprovenance.CaptureV2(context.Background(), result, root)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.RemoveAll(root); e != nil {
		t.Fatal(e)
	}
	input := Input{Artifact: raw}
	p := DefaultPolicy()
	p.IncludeBodies = true
	c, e := Inspect(input, p)
	if e != nil {
		t.Fatalf("native V2 admission: %v", e)
	}
	r := Request{Policy: p, Selections: []Selection{{ID: "v2", RecordID: "native:/graph/nodes/0/range", SourceID: c.Sources[0].ID, Mode: "RETAINED_RANGE"}}}
	b, e := Hydrate(input, r)
	if e != nil || len(b.Spans) != 1 || string(b.Spans[0].Content) != "A😀" {
		t.Fatalf("native V2 byte coordinates: %v %+v", e, b)
	}
	if e = Validate(input, r, b); e != nil {
		t.Fatal(e)
	}
	for _, record := range c.Records {
		if record.Pointer == "file:///opaque-ignored" {
			t.Fatal("opaque data interpreted as source")
		}
	}
}
func TestCallerBoundaryAuthority(t *testing.T) {
	input, r, _ := mixedFixture(t)
	s := &r.Selections[0]
	s.Mode = "BOUNDARY"
	s.Range = nil
	c, e := Inspect(input, r.Policy)
	if e != nil {
		t.Fatal(e)
	}
	var hash string
	for _, src := range c.Sources {
		if src.ID == s.SourceID {
			hash = *src.ContentHash
		}
	}
	s.Boundary = &Boundary{Kind: "FUNCTION", Reference: "caller-boundary-1", Authority: Caller, Qualification: NonAuthoritative, SourceID: s.SourceID, ContentHash: hash, Range: Range{Position{0, 0}, Position{1, 0}}}
	r.Selections = r.Selections[:1]
	b, e := Hydrate(input, r)
	if e != nil || b.Origins[0].Status != "UNKNOWN_BOUNDARY" {
		t.Fatal("boundary opt-in ignored")
	}
	r.Policy.AllowCallerBoundaries = true
	b, e = Hydrate(input, r)
	if e != nil || b.Origins[0].Status != "EXPORTED" || b.Origins[0].Selection.Boundary.Authority != Caller {
		t.Fatal("attributable caller boundary")
	}
	r.Selections[0].Boundary.Authority = Native
	b, e = Hydrate(input, r)
	if e != nil || b.Origins[0].Status != "UNKNOWN_BOUNDARY" {
		t.Fatal("forged native boundary accepted")
	}
}
