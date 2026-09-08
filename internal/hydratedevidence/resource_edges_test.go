package hydratedevidence

import (
	"strings"
	"testing"
)

func TestResourceEdges(t *testing.T) {
	input, r, _ := mixedFixture(t)
	r.Policy.MaxSpans = 0
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range b.Origins {
		if o.Status != "SPAN_BUDGET" {
			t.Fatal("zero span limit bypassed")
		}
	}
	if b.Complete || len(b.Spans) != 0 {
		t.Fatal("zero span limit false complete")
	}
	r.Policy = DefaultPolicy()
	r.Policy.IncludeBodies = true
	r.Policy.MaxOutputBytes = 1
	if _, e = Hydrate(input, r); e == nil {
		t.Fatal("output byte budget bypassed")
	}
	r.Policy = DefaultPolicy()
	r.Policy.IncludeBodies = true
	r.Policy.MaxOrigins = 2
	if _, e = Hydrate(input, r); e == nil {
		t.Fatal("origin input ceiling bypassed")
	}
	r.Policy = DefaultPolicy()
	r.Policy.IncludeBodies = true
	r.Policy.MaxPageBytes = 4096
	r.Policy.MaxPages = 1
	b, e = Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewSnapshot(input, r, b); e == nil {
		t.Fatal("global page count bypassed")
	}
	r.Policy = DefaultPolicy()
	r.Policy.IncludeBodies = true
	r.Selections[0].SourceID = "missing"
	b, e = Hydrate(input, r)
	if e != nil || b.Origins[0].Status != "UNKNOWN_SOURCE" || len(b.Origins) != len(r.Selections) {
		t.Fatal("unknown source origin dropped")
	}
	r.Selections = []Selection{}
	r.Policy.MaxOrigins = 0
	r.Policy.MaxSpans = 0
	r.Policy.MaxBodyBytes = 0
	r.Policy.MaxWork = 0
	b, e = Hydrate(input, r)
	if e != nil || b.Complete || b.TotalOrigins != 0 || b.TotalSpans != 0 {
		t.Fatal("empty zero-budget false completeness")
	}
}
func TestOpaqueSourceContentAndEmptyUnion(t *testing.T) {
	input, r, _ := mixedFixture(t)
	content := []byte(strings.Repeat("[", 100) + "opaque-not-JSON" + strings.Repeat("]", 100))
	hash := Digest(content)
	side := Sidecar{SchemaVersion: SidecarVersion, ArtifactDigest: Digest(input.Artifact), Authority: Caller, Qualification: NonAuthoritative, Sources: []AssertedSource{{ID: "opaque", ReceiptReference: "receipt", VersionReference: "version", URI: "file:///not-read", Content: &content, ContentHash: &hash, SourceEncoding: "utf-8", State: "RETAINED_BYTES"}}, Records: []AssertedRecord{{ID: "opaque", Kind: "caller-fragment", SourceIDs: []string{"opaque"}}}}
	input.Sidecars = [][]byte{encoded(t, side)}
	prefix := "sidecar:" + Digest(input.Sidecars[0]) + ":"
	r.Selections = []Selection{{ID: "opaque", RecordID: prefix + "opaque", SourceID: prefix + "opaque", Mode: "WHOLE_FILE"}}
	b, e := Hydrate(input, r)
	if e != nil || len(b.Spans) != 1 || string(b.Spans[0].Content) != string(content) {
		t.Fatal("source content interpreted recursively")
	}
	iv := Interval{2, 2}
	origins := []Origin{{Selection: Selection{ID: "a", SourceID: "s", Encoding: "utf-8"}, Status: "PENDING", Bytes: &iv}, {Selection: Selection{ID: "b", SourceID: "s", Encoding: "utf-8"}, Status: "PENDING", Bytes: &iv}}
	for _, gs := range [][]group{groups(origins), sweepGroups(origins)} {
		if len(gs) != 1 || gs[0].interval != iv || len(gs[0].indices) != 2 {
			t.Fatal("empty interval origin union")
		}
	}
}
