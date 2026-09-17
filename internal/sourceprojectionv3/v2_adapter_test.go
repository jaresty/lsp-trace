package sourceprojectionv3

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
)

type adapterCustody struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

func adapterFixture(mode string) sourceprojectionv2.WireResult[adapterCustody] {
	r := sourceprojection.Range{Start: sourceprojection.Position{Line: 1, Character: 2}, End: sourceprojection.Position{Line: 1, Character: 5}}
	return sourceprojectionv2.WireResult[adapterCustody]{
		SchemaVersion: sourceprojectionv2.SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0,
		CustodyMode: mode, CustodyBinding: adapterCustody{Mode: mode, ID: "binding-1"}, PhysicalProjectionID: "sha256:physical", RequestPolicyID: "policy", Status: "COMPLETE",
		DocumentSelection:  sourceprojectionv2.DocumentSelection{Ordering: "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC", TargetURI: "file:///a.go", SelectedURIs: []string{"file:///a.go"}},
		DocumentBindings:   []sourceprojectionv2.DocumentBinding{{URI: "file:///a.go", Role: "TARGET", Ordinal: 0, Status: "ACQUIRED", PositionEncoding: "utf-16", SourceDigest: "sha256:source", SourceByteLength: 12}},
		DocumentAccounting: sourceprojectionv2.DocumentAccounting{Candidates: 1, Selected: 1, Acquired: 1, TotalAcquiredBytes: 12},
		Units:              []sourceprojectionv2.Unit{{UnitID: "u1", Role: "ENDPOINT", GraphSubjectID: "g1", LogicalSourceID: "file:///a.go", EvidenceRange: r, DisplayRange: r, ItemRange: &r, SelectionRange: &r, PositionEncoding: "utf-16", SourceDigest: "sha256:source", SourceByteLength: 12, SelectionDisposition: "SELECTED", BodyDisposition: "INCLUDED", PrivacyClassification: "PUBLIC", Body: "abc"}},
		Citations:          []sourceprojectionv2.Citation{{CitationID: "c1", UnitID: "u1", Role: "ENDPOINT", SubjectID: "g1", EvidenceRange: r, DisplayRange: r}},
		EmittedSpans:       []sourceprojection.Span{{UnitIDs: []string{"u1"}, LogicalSourceID: "file:///a.go", Range: r, SourceDigest: "sha256:source", ByteLength: 3, Body: "abc"}},
		Accounting:         sourceprojection.Accounting{Candidates: 1, Selected: 1, Evaluated: 1, Terminal: 1},
		Omissions:          []sourceprojection.Omission{}, PrivacySummary: sourceprojection.PrivacySummary{},
	}
}

func adapterLimits() Limits {
	return Limits{MaxPageBytes: 1200, MaxPages: 20, MaxResponseBytes: 100000, MaxObjects: 100, MaxRanges: 100, MaxSourceBytes: 100, MaxWork: 100}
}

func TestPaginateV2CanonicalRecordsAndInputImmutability(t *testing.T) {
	source := adapterFixture("LIVE")
	before, _ := json.Marshal(source)
	var kinds []string
	cursor := ""
	for pages := 0; ; pages++ {
		page, err := PaginateV2(source, "sha256:request", adapterLimits(), cursor)
		if err != nil {
			t.Fatalf("ASSERT_V3_V2_ADAPTER_PAGE_%d: %v", pages, err)
		}
		if page.SchemaVersion != SchemaVersion || page.Authority != 0 || page.SourceGraphComplete != "UNKNOWN" || page.GraphFactsAdded != 0 || page.CustodyMode != "LIVE" {
			t.Fatalf("ASSERT_V3_HEADER_BOUNDARY: %+v", page.Header)
		}
		for _, record := range page.Records {
			kinds = append(kinds, record.Kind)
		}
		if page.Complete {
			break
		}
		if page.NextCursor == "" {
			t.Fatal("ASSERT_V3_CONTINUATION_CURSOR_REQUIRED")
		}
		cursor = page.NextCursor
	}
	want := []string{"CUSTODY_BINDING", "DOCUMENT_SELECTION", "DOCUMENT_BINDING", "DOCUMENT_ACCOUNTING", "UNIT", "CITATION", "EMITTED_SPAN", "ACCOUNTING", "PRIVACY_SUMMARY"}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("ASSERT_V3_CANONICAL_RECORD_ORDER: got=%v want=%v", kinds, want)
	}
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("ASSERT_V3_ADAPTER_DOES_NOT_MUTATE_V2")
	}
}

func TestPaginateV2CursorBindsRequestAndCustody(t *testing.T) {
	live := adapterFixture("LIVE")
	first, err := PaginateV2(live, "sha256:request", adapterLimits(), "")
	if err != nil || first.Complete {
		t.Fatalf("ASSERT_V3_ADAPTER_CURSOR_SETUP: page=%+v err=%v", first, err)
	}
	if _, err := PaginateV2(live, "sha256:other", adapterLimits(), first.NextCursor); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("ASSERT_V3_ADAPTER_REQUEST_BOUND: %v", err)
	}
	retained := adapterFixture("RETAINED")
	if _, err := PaginateV2(retained, "sha256:request", adapterLimits(), first.NextCursor); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("ASSERT_V3_ADAPTER_CUSTODY_BOUND: %v", err)
	}
}

func TestPaginateV2RejectsInvalidV2Boundary(t *testing.T) {
	invalid := adapterFixture("LIVE")
	invalid.Authority = 1
	if _, err := PaginateV2(invalid, "sha256:request", adapterLimits(), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ASSERT_V3_ADAPTER_AUTHORITY_CLOSED: %v", err)
	}
	invalid = adapterFixture("LIVE")
	invalid.DocumentBindings[0].SourceByteLength = -1
	if _, err := PaginateV2(invalid, "sha256:request", adapterLimits(), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ASSERT_V3_ADAPTER_NEGATIVE_BYTES_CLOSED: %v", err)
	}
}
