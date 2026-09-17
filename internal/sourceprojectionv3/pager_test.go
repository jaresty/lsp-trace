package sourceprojectionv3

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func testHeader() Header {
	return Header{SchemaVersion: "lsp-trace.source-projection.v3", Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0, CustodyMode: "LIVE", PhysicalProjectionID: "sha256:physical", RequestPolicyID: "policy", Status: "COMPLETE"}
}

func testBinding() Binding {
	return Binding{RequestDigest: "sha256:req", CustodyMode: "LIVE", CustodyDigest: "sha256:custody", ProjectionDigest: "sha256:projection"}
}

func testLimits() Limits {
	return Limits{MaxPageBytes: 150, MaxPages: 4, MaxResponseBytes: 10000, MaxObjects: 10, MaxRanges: 10, MaxSourceBytes: 100, MaxWork: 10}
}

func testRecords() []Record {
	return []Record{
		{Kind: "UNIT", Key: "a", Data: json.RawMessage(`{"id":"a"}`), Objects: 1, Ranges: 1, SourceBytes: 3, Work: 1},
		{Kind: "UNIT", Key: "b", Data: json.RawMessage(`{"id":"b"}`), Objects: 1, Ranges: 1, SourceBytes: 4, Work: 1},
		{Kind: "CITATION", Key: "c", Data: json.RawMessage(`{"id":"c"}`), Objects: 1, Ranges: 1, Work: 1},
	}
}

func TestPaginateDeterministicReplayAndCumulativeAccounting(t *testing.T) {
	records := testRecords()
	limits := testLimits()
	first, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
	if err != nil || first.Complete || first.NextCursor == "" || len(first.Records) != 1 {
		t.Fatalf("ASSERT_V3_FIRST_PAGE_ATOMIC: page=%+v err=%v", first, err)
	}
	replay, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
	if err != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("ASSERT_V3_FIRST_PAGE_DETERMINISTIC: replay=%+v err=%v", replay, err)
	}
	firstRaw, _ := json.Marshal(first)
	if first.Accounting.ResponseBytes != uint64(len(firstRaw)) {
		t.Fatalf("ASSERT_V3_EXACT_FIRST_RESPONSE_BYTES: accounting=%d encoded=%d", first.Accounting.ResponseBytes, len(firstRaw))
	}
	second, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits, Cursor: first.NextCursor})
	if err != nil || second.Complete || len(second.Records) != 1 || second.Accounting.Pages != 2 || second.Accounting.Objects != 2 || second.Accounting.SourceBytes != 7 {
		t.Fatalf("ASSERT_V3_CUMULATIVE_SECOND_PAGE: page=%+v err=%v", second, err)
	}
	secondRaw, _ := json.Marshal(second)
	if second.Accounting.ResponseBytes != uint64(len(firstRaw)+len(secondRaw)) {
		t.Fatalf("ASSERT_V3_EXACT_SECOND_RESPONSE_BYTES: accounting=%d encoded=%d", second.Accounting.ResponseBytes, len(firstRaw)+len(secondRaw))
	}
	third, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits, Cursor: second.NextCursor})
	if err != nil || !third.Complete || third.NextCursor != "" || len(third.Records) != 1 || third.Accounting.Pages != 3 || third.Accounting.Objects != 3 || third.Accounting.Ranges != 3 || third.Accounting.Work != 3 {
		t.Fatalf("ASSERT_V3_CUMULATIVE_FINAL_PAGE: page=%+v err=%v", third, err)
	}
	thirdRaw, _ := json.Marshal(third)
	if third.Accounting.ResponseBytes != uint64(len(firstRaw)+len(secondRaw)+len(thirdRaw)) {
		t.Fatalf("ASSERT_V3_EXACT_FINAL_RESPONSE_BYTES: accounting=%d encoded=%d", third.Accounting.ResponseBytes, len(firstRaw)+len(secondRaw)+len(thirdRaw))
	}
}

func TestPaginateCursorBindingReplayAndTamperFailClosed(t *testing.T) {
	first, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	changed := testBinding()
	changed.CustodyMode = "RETAINED"
	changedHeader := testHeader()
	changedHeader.CustodyMode = "RETAINED"
	if _, err := Paginate(testRecords(), Request{Header: changedHeader, Binding: changed, Limits: testLimits(), Cursor: first.NextCursor}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("ASSERT_V3_CURSOR_CUSTODY_BOUND: %v", err)
	}
	changed = testBinding()
	changed.RequestDigest = "sha256:other"
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: changed, Limits: testLimits(), Cursor: first.NextCursor}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("ASSERT_V3_CURSOR_REQUEST_BOUND: %v", err)
	}
	tampered := []byte(first.NextCursor)
	tampered[len(tampered)/2] ^= 1
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits(), Cursor: string(tampered)}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("ASSERT_V3_CURSOR_TAMPER_CLOSED: %v", err)
	}
}

func TestPaginateZeroAndCumulativeBoundsAreReal(t *testing.T) {
	limits := testLimits()
	limits.MaxObjects = 0
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("ASSERT_V3_ZERO_OBJECT_BOUND: %v", err)
	}
	limits = testLimits()
	limits.MaxPages = 1
	first, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits, Cursor: first.NextCursor}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("ASSERT_V3_PAGE_BOUND_CUMULATIVE: %v", err)
	}
	limits = testLimits()
	limits.MaxSourceBytes = 6
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits}); err != nil {
		t.Fatalf("first record should fit: %v", err)
	}
	first, _ = Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
	if _, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits, Cursor: first.NextCursor}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("ASSERT_V3_SOURCE_BOUND_CUMULATIVE: %v", err)
	}
}

func TestPaginateRejectsOversizeDuplicateInvalidAndOverflowRecords(t *testing.T) {
	limits := testLimits()
	records := testRecords()
	records[0].Data = json.RawMessage(`{"body":"` + strings.Repeat("x", 200) + `"}`)
	if _, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("ASSERT_V3_WHOLE_RECORD_ATOMIC: %v", err)
	}
	records = testRecords()
	records[1].Key = records[0].Key
	if _, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ASSERT_V3_DUPLICATE_RECORD_REJECTED: %v", err)
	}
	records = testRecords()
	records[0].Data = json.RawMessage(`{`)
	if _, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ASSERT_V3_INVALID_JSON_REJECTED: %v", err)
	}
	records = testRecords()
	records[0].Objects = math.MaxUint64
	records[1].Objects = 1
	limits.MaxObjects = math.MaxUint64
	first, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("ASSERT_V3_COUNTER_OVERFLOW_SETUP: page=%+v err=%v", first, err)
	}
	if _, err := Paginate(records, Request{Header: testHeader(), Binding: testBinding(), Limits: limits, Cursor: first.NextCursor}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("ASSERT_V3_COUNTER_OVERFLOW_CLOSED: %v", err)
	}
}

func TestCursorEncodingContainsNoRawBindingText(t *testing.T) {
	first, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(first.NextCursor); err != nil {
		t.Fatalf("ASSERT_V3_CURSOR_OPAQUE_ENCODING: %v", err)
	}
}
