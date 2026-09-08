package hydratedevidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSidecarStructureBeforeSemantics(t *testing.T) {
	input, r, _ := mixedFixture(t)
	var side map[string]any
	json.Unmarshal(input.Sidecars[0], &side)
	delete(side, "sources")
	input.Sidecars = [][]byte{encoded(t, side)}
	if _, e := Inspect(input, r.Policy); e == nil {
		t.Fatal("sidecar missing required table accepted")
	}
}

func TestCoordinateAuthoritySeparateFromRecord(t *testing.T) {
	input, r, _ := mixedFixture(t)
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	if b.Origins[0].Record.Authority != Native || b.Origins[0].CoordinateAuthority != Caller {
		t.Fatal("caller span promoted to native coordinate authority")
	}
}

func TestSidecarRecordPointer(t *testing.T) {
	input, r, _ := mixedFixture(t)
	c, e := Inspect(input, r.Policy)
	if e != nil {
		t.Fatal(e)
	}
	var raw any
	json.Unmarshal(input.Sidecars[0], &raw)
	for _, record := range c.Records {
		if record.Authority == Caller && record.Kind == "CALLS" {
			v, ok := pointer(raw, record.Pointer).(map[string]any)
			if !ok || v["id"] != "claim" {
				t.Fatalf("sidecar pointer does not address retained record: %s", record.Pointer)
			}
		}
	}
}

func TestAdmissionAndSelectionAdversaries(t *testing.T) {
	input, r, _ := mixedFixture(t)
	for _, tc := range []struct {
		name   string
		change func(*Sidecar)
	}{
		{"digest", func(s *Sidecar) { s.ArtifactDigest = "sha256:wrong" }},
		{"authority", func(s *Sidecar) { s.Authority = Native }},
		{"qualification", func(s *Sidecar) { s.Qualification = "AUTHENTICATED" }},
		{"source-join", func(s *Sidecar) { s.Records[0].SourceIDs = []string{"absent"} }},
		{"unknown-family", func(s *Sidecar) { s.SchemaVersion = "some.provider.v1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var side Sidecar
			json.Unmarshal(input.Sidecars[0], &side)
			tc.change(&side)
			bad := Input{Artifact: input.Artifact, Sidecars: [][]byte{encoded(t, side)}}
			if _, e := Hydrate(bad, r); e == nil {
				t.Fatalf("invalid sidecar %s admitted", tc.name)
			}
		})
	}
	duplicate := Input{Artifact: input.Artifact, Sidecars: [][]byte{input.Sidecars[0], input.Sidecars[0]}}
	if _, e := Hydrate(duplicate, r); e == nil {
		t.Fatal("duplicate sidecar accepted")
	}
	conflicting := r
	conflicting.Selections = append([]Selection{}, r.Selections...)
	conflicting.Selections[0].Bytes = &Interval{1, 6}
	b, e := Hydrate(input, conflicting)
	if e != nil || b.Origins[0].Status != "INVALID_COORDINATES" {
		t.Fatal("mismatched byte coordinates accepted")
	}
	r.Selections = []Selection{}
	b, e = Hydrate(input, r)
	if e != nil || b.Complete || b.Origins == nil || b.Spans == nil {
		t.Fatal("empty selection claims completeness or null tables")
	}
	if e := preflight([]byte(`{"data":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}`)); e == nil {
		t.Fatal("recursive carrier limit ignored")
	}
	if e := preflight([]byte(`{"x":1,"x":2}`)); e == nil {
		t.Fatal("duplicate JSON accepted")
	}
}
func TestDistinctAssertedReceiptsDoNotMerge(t *testing.T) {
	input, r, _ := mixedFixture(t)
	data := []byte("identical")
	h := Digest(data)
	s := Sidecar{SchemaVersion: SidecarVersion, ArtifactDigest: Digest(input.Artifact), Authority: Caller, Qualification: NonAuthoritative, Sources: []AssertedSource{}, Records: []AssertedRecord{}}
	for _, id := range []string{"a", "b"} {
		s.Sources = append(s.Sources, AssertedSource{ID: id, ReceiptReference: "receipt-" + id, VersionReference: "version-" + id, URI: "file:///same", Content: &data, ContentHash: &h, SourceEncoding: "utf-8", State: "RETAINED_BYTES"})
		s.Records = append(s.Records, AssertedRecord{ID: id, Kind: "caller-fragment", SourceIDs: []string{id}})
	}
	input.Sidecars = [][]byte{encoded(t, s)}
	prefix := "sidecar:" + Digest(input.Sidecars[0]) + ":"
	r.Selections = []Selection{}
	for _, id := range []string{"a", "b"} {
		r.Selections = append(r.Selections, Selection{ID: id, RecordID: prefix + id, SourceID: prefix + id, Mode: "WHOLE_FILE"})
	}
	b, e := Hydrate(input, r)
	if e != nil || len(b.Spans) != 2 || b.Spans[0].SourceID == b.Spans[1].SourceID {
		t.Fatal("equal content erased distinct receipt provenance")
	}
	s.Sources[0].ContentHash = new(string)
	input.Sidecars = [][]byte{encoded(t, s)}
	if _, e := Hydrate(input, r); e == nil {
		t.Fatal("tampered source hash accepted")
	}
}
