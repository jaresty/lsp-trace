package hydratedevidence

import (
	"bytes"
	"testing"
)

func TestImmutablePaging(t *testing.T) {
	input, r, _ := mixedFixture(t)
	r.Policy.MaxPageBytes = 4096
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewSnapshot(input, r, b)
	if e != nil {
		t.Fatalf("immutable paging: %v", e)
	}
	pages := []Page{}
	cursor := ""
	for {
		p, e := s.Page(cursor)
		if e != nil {
			t.Fatal(e)
		}
		again, e := s.Page(cursor)
		if e != nil || !bytes.Equal(encoded(t, p), encoded(t, again)) {
			t.Fatal("page replay unstable")
		}
		if len(encoded(t, p)) > r.Policy.MaxPageBytes {
			t.Fatal("page byte ceiling")
		}
		pages = append(pages, p)
		if p.Next == "" {
			break
		}
		cursor = p.Next
		if len(pages) > r.Policy.MaxPages {
			t.Fatal("paging never terminates")
		}
	}
	if len(pages) < 2 {
		t.Fatal("fixture did not cross page boundary")
	}
	rebuilt, e := Reassemble(input, r, pages)
	if e != nil || !bytes.Equal(encoded(t, b), encoded(t, rebuilt)) {
		t.Fatalf("page reconstruction: %v", e)
	}
	dup := append([]Page{}, pages...)
	dup = append(dup, pages[0])
	if _, e := Reassemble(input, r, dup); e == nil {
		t.Fatal("duplicate page accepted")
	}
	missing := pages[:len(pages)-1]
	if _, e := Reassemble(input, r, missing); e == nil {
		t.Fatal("missing page accepted")
	}
	changed := r
	changed.Policy.MaxBodyBytes--
	other, e := Hydrate(input, changed)
	if e != nil {
		t.Fatal(e)
	}
	next, e := NewSnapshot(input, changed, other)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := next.Page(pages[0].Next); e == nil {
		t.Fatal("stale continuation accepted")
	}
	pages[0].Entries = nil
	if _, e := Reassemble(input, r, pages); e == nil {
		t.Fatal("page entry tamper accepted")
	}
}
func TestOverPageWholeSpan(t *testing.T) {
	input, r, _ := mixedFixture(t)
	side := Sidecar{SchemaVersion: SidecarVersion, ArtifactDigest: Digest(input.Artifact), Authority: Caller, Qualification: NonAuthoritative}
	content := bytes.Repeat([]byte("x"), 5000)
	hash := Digest(content)
	side.Sources = []AssertedSource{{ID: "large", ReceiptReference: "caller-receipt", VersionReference: "v1", URI: "file:///not-read", Content: &content, ContentHash: &hash, SourceEncoding: "utf-8", State: "RETAINED_BYTES"}}
	side.Records = []AssertedRecord{{ID: "large", Kind: "claimed", SourceIDs: []string{"large"}}}
	input.Sidecars = [][]byte{encoded(t, side)}
	prefix := "sidecar:" + Digest(input.Sidecars[0]) + ":"
	r.Policy.MaxPageBytes = 4096
	r.Selections = []Selection{{ID: "large", RecordID: prefix + "large", SourceID: prefix + "large", Mode: "WHOLE_FILE"}}
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	if b.Origins[0].Status != "PAGE_BUDGET" || len(b.Spans) != 0 {
		t.Fatal("overpage span fragmented")
	}
	s, e := NewSnapshot(input, r, b)
	if e != nil {
		t.Fatalf("overpage omission paging: %v", e)
	}
	if _, e = s.Page(""); e != nil {
		t.Fatal(e)
	}
}
