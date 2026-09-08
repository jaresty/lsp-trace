package hydratedevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func nativeFixture(t *testing.T) (Input, string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	if err := os.WriteFile(path, []byte("B😀abcd\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	n := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	g := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: []graph.Node{n}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	g.Invocation.Target = graph.Target{URI: uri}
	g.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: uri + ":0:0", ResolvedURI: uri}}
	g.Seeds = []graph.SeedResult{{Label: "start", Requested: g.Invocation.Target, PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: []string{n.ID}}}
	g.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: uri, StartingNodeIDs: []string{n.ID}, Layers: []graph.SliceLayer{{NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID}, UpwardStartNodeIDs: []string{n.ID}, TraversalComplete: true}
	g.Canonicalize()
	a := []byte("A😀abcd\r\n")
	params := encoded(t, map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": 1, "text": string(a)}})
	supply := &sessionruntime.DocumentSupply{Classification: graphprovenance.Supplied, SessionID: "s", Generation: 1, URI: uri, DocumentVersion: 1, Method: "textDocument/didOpen", Content: a, Params: params}
	raw, err := graphprovenance.Capture(context.Background(), encoded(t, g), root, uri, "s", 1, supply)
	if err != nil {
		t.Fatal(err)
	}
	var e graphprovenance.Evidence
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return Input{Artifact: raw}, "native:" + e.Supply.ID, path
}
func mixedFixture(t *testing.T) (Input, Request, string) {
	t.Helper()
	input, sid, path := nativeFixture(t)
	var e graphprovenance.Evidence
	json.Unmarshal(input.Artifact, &e)
	side := Sidecar{SchemaVersion: SidecarVersion, ArtifactDigest: Digest(input.Artifact), Authority: Caller, Qualification: NonAuthoritative, Sources: []AssertedSource{}, Records: []AssertedRecord{{ID: "claim", Kind: "CALLS", SourceIDs: []string{sid}, RelationshipReferences: []string{"asserted-callback"}}}}
	input.Sidecars = [][]byte{encoded(t, side)}
	p := DefaultPolicy()
	p.IncludeBodies = true
	request := Request{Policy: p, Selections: []Selection{
		{ID: "native", RecordID: "native:/nodes/0/range", SourceID: sid, Mode: "SPAN", Encoding: "utf-8", Range: &Range{Position{0, 0}, Position{0, 6}}},
		{ID: "side", RecordID: "sidecar:" + Digest(input.Sidecars[0]) + ":claim", SourceID: sid, Mode: "SPAN", Encoding: "utf-8", Range: &Range{Position{0, 5}, Position{0, 8}}},
		{ID: "nested", RecordID: "native:/nodes/0/range", SourceID: sid, Mode: "SPAN", Encoding: "utf-8", Range: &Range{Position{0, 5}, Position{0, 6}}},
	}}
	return input, request, path
}
func TestMixedOfflineUnion(t *testing.T) {
	input, r, path := mixedFixture(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	b, err := Hydrate(input, r)
	if err != nil {
		t.Fatalf("mixed offline origin union: %v", err)
	}
	if len(b.Origins) != 3 || len(b.Spans) != 1 || !bytes.Equal(b.Spans[0].Content, []byte("A😀abc")) || len(b.Spans[0].OriginIDs) != 3 {
		t.Fatalf("mixed offline origin union: %+v", b)
	}
	if b.Origins[0].Record.Authority != Native || b.Origins[1].Record.Authority != Caller || b.Origins[1].Record.Qualification != NonAuthoritative {
		t.Fatal("sidecar authority upgraded")
	}
	if err := os.WriteFile(path, []byte("newer checkout"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Hydrate(input, r)
	if err != nil || !bytes.Equal(encoded(t, b), encoded(t, c)) {
		t.Fatal("filesystem fallback")
	}
	if err := Validate(input, r, b); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}
}
func TestPrivacyAndDispositions(t *testing.T) {
	input, r, _ := mixedFixture(t)
	r.Policy.IncludeBodies = false
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatalf("privacy accounting: %v", e)
	}
	if len(b.Spans) != 0 || len(b.Origins) != 3 || b.Complete {
		t.Fatal("privacy source body exposure")
	}
	for _, o := range b.Origins {
		if o.Status != "PRIVACY_EXCLUDED" {
			t.Fatal("privacy disposition")
		}
	}
	r.Policy.IncludeBodies = true
	r.Selections[0].Mode = "BOUNDARY"
	r.Selections[0].Range = nil
	r.Selections[1].Range = &Range{Position{0, 2}, Position{0, 3}}
	r.Selections[2].RecordID = "unknown"
	b, e = Hydrate(input, r)
	if e != nil {
		t.Fatalf("typed dispositions: %v", e)
	}
	for i, w := range []string{"UNKNOWN_BOUNDARY", "INVALID_COORDINATES", "UNKNOWN_RECORD"} {
		if b.Origins[i].Status != w {
			t.Fatalf("typed dispositions %d: %s want %s", i, b.Origins[i].Status, w)
		}
	}
}
func TestSourceVersionAndWholeFile(t *testing.T) {
	input, r, _ := mixedFixture(t)
	c, err := Inspect(input, r.Policy)
	if err != nil {
		t.Fatalf("source version catalog: %v", err)
	}
	if len(c.Sources) != 2 || c.Sources[0].ID == c.Sources[1].ID || c.Sources[0].URI != c.Sources[1].URI {
		t.Fatal("same URI versions collapsed")
	}
	r.Selections = []Selection{}
	for i, s := range c.Sources {
		r.Selections = append(r.Selections, Selection{ID: []string{"a", "b"}[i], RecordID: "receipt:" + s.ID, SourceID: s.ID, Mode: "WHOLE_FILE"})
	}
	b, err := Hydrate(input, r)
	if err != nil || len(b.Spans) != 2 {
		t.Fatalf("whole-file versions: %v %+v", err, b)
	}
	for _, o := range b.Origins {
		if o.OriginalRange != nil || o.OriginalCoordinatesStatus != "UNAVAILABLE" {
			t.Fatal("invented original whole-file coordinates")
		}
	}
}
func TestSidecarStatesAndUnsupported(t *testing.T) {
	input, r, _ := mixedFixture(t)
	content := []byte{}
	h := Digest(content)
	side := Sidecar{SchemaVersion: SidecarVersion, ArtifactDigest: Digest(input.Artifact), Authority: Caller, Qualification: NonAuthoritative, Sources: []AssertedSource{}, Records: []AssertedRecord{}}
	for _, state := range []string{"RETAINED_BYTES", "REFERENCE_ONLY", "MISSING", "TRUNCATED_INPUT"} {
		s := AssertedSource{ID: state, ReceiptReference: "asserted-receipt/" + state, VersionReference: "asserted-v1", URI: "file:///same", SourceEncoding: "utf-8", State: state}
		if state == "RETAINED_BYTES" || state == "TRUNCATED_INPUT" {
			s.Content = &content
			s.ContentHash = &h
		}
		side.Sources = append(side.Sources, s)
		side.Records = append(side.Records, AssertedRecord{ID: state, Kind: "declaration", SourceIDs: []string{state}, RelationshipReferences: []string{}})
	}
	input.Sidecars = [][]byte{encoded(t, side)}
	prefix := "sidecar:" + Digest(input.Sidecars[0]) + ":"
	r.Selections = []Selection{}
	for _, s := range side.Sources {
		r.Selections = append(r.Selections, Selection{ID: s.ID, RecordID: prefix + s.ID, SourceID: prefix + s.ID, Mode: "WHOLE_FILE"})
	}
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatalf("source-state accounting: %v", e)
	}
	for i, w := range []string{"EXPORTED", "REFERENCE_ONLY", "MISSING_BYTES", "TRUNCATED_INPUT"} {
		if b.Origins[i].Status != w {
			t.Fatalf("source-state accounting: %s != %s", b.Origins[i].Status, w)
		}
	}
	if len(b.Spans) != 1 || len(b.Spans[0].Content) != 0 || b.Complete {
		t.Fatal("empty readable confused with missing")
	}
	input.Artifact = []byte(`{"schema_version":"unsupported.provider.v1"}`)
	if _, e := Hydrate(input, r); e == nil {
		t.Fatal("unknown family silently accepted")
	}
}
func TestGlobalBudgets(t *testing.T) {
	input, r, _ := mixedFixture(t)
	r.Policy.MaxBodyBytes = 7
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatalf("whole union budget: %v", e)
	}
	if len(b.Spans) != 0 || b.Complete {
		t.Fatal("whole union budget silently truncated")
	}
	for _, o := range b.Origins {
		if o.Status != "BODY_BUDGET" {
			t.Fatalf("whole union budget: %s", o.Status)
		}
	}
	r.Policy.MaxBodyBytes = 8
	b, e = Hydrate(input, r)
	if e != nil || len(b.Spans) != 1 {
		t.Fatalf("exact budget edge: %v", e)
	}
	r.Policy.MaxWork = 0
	b, e = Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range b.Origins {
		if o.Status != "WORK_BUDGET" {
			t.Fatal("work budget reset")
		}
	}
	r.Policy.MaxInputBytes = 1
	if _, e = Hydrate(input, r); e == nil {
		t.Fatal("input budget ignored")
	}
}
func TestValidatorRejectsTamperWithoutProducer(t *testing.T) {
	input, r, _ := mixedFixture(t)
	// A forged but structurally present bundle must not be accepted by a stub validator.
	if Validate(input, r, Bundle{SchemaVersion: Version, Complete: true}) == nil {
		t.Fatal("coherent omission tamper accepted")
	}
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatalf("validator source fixture: %v", e)
	}
	for _, tc := range []struct {
		name   string
		change func(*Bundle)
	}{
		{"hash", func(b *Bundle) {
			b.Spans[0].Content = []byte("forged")
			b.Spans[0].ContentHash = Digest(b.Spans[0].Content)
		}},
		{"authority", func(b *Bundle) { b.Origins[1].Record.Authority = Native }},
		{"span", func(b *Bundle) { b.Spans[0].Bytes.End-- }},
		{"origin", func(b *Bundle) {
			b.Origins = b.Origins[:2]
			b.TotalOrigins = 2
			b.Spans[0].OriginIDs = b.Spans[0].OriginIDs[:2]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var copy Bundle
			json.Unmarshal(encoded(t, b), &copy)
			tc.change(&copy)
			copy.Digest = ""
			copy.Digest = Digest(encoded(t, copy))
			if Validate(input, r, copy) == nil {
				t.Fatalf("coherent %s tamper accepted", tc.name)
			}
		})
	}
}
