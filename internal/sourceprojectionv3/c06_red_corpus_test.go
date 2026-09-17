package sourceprojectionv3

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestC06PagerMalformedAndMixedPageCorpusFailsTerminalZero(t *testing.T) {
	first, err := Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits()})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("ASSERT_C06_SETUP_VALID_FIRST_PAGE: page=%+v err=%v", first, err)
	}

	unknownEnvelopeRaw, err := base64.RawURLEncoding.DecodeString(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	unknownEnvelopeRaw = []byte(strings.Replace(string(unknownEnvelopeRaw), `{"payload":`, `{"unknown":1,"payload":`, 1))
	duplicateEnvelopeRaw := []byte(strings.Replace(string(mustDecodeCursorBytes(t, first.NextCursor)), `"digest":`, `"digest":"sha256:duplicate","digest":`, 1))

	var original cursorEnvelope
	if err := json.Unmarshal(mustDecodeCursorBytes(t, first.NextCursor), &original); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(original.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["unknown"] = true
	unknownPayloadRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	unknownPayloadCursor := cursorForRawPayload(t, unknownPayloadRaw)
	duplicatePayloadRaw := []byte(strings.Replace(string(original.Payload), `"version":`, `"version":1,"version":`, 1))
	duplicatePayloadCursor := cursorForRawPayload(t, duplicatePayloadRaw)

	tests := []struct {
		name string
		run  func() (Page, error)
		want error
	}{
		{name: "unknown-envelope-field", want: ErrInvalidCursor, run: func() (Page, error) {
			return Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits(), Cursor: base64.RawURLEncoding.EncodeToString(unknownEnvelopeRaw)})
		}},
		{name: "duplicate-envelope-field", want: ErrInvalidCursor, run: func() (Page, error) {
			return Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits(), Cursor: base64.RawURLEncoding.EncodeToString(duplicateEnvelopeRaw)})
		}},
		{name: "unknown-payload-field", want: ErrInvalidCursor, run: func() (Page, error) {
			return Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits(), Cursor: unknownPayloadCursor})
		}},
		{name: "duplicate-payload-field", want: ErrInvalidCursor, run: func() (Page, error) {
			return Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits(), Cursor: duplicatePayloadCursor})
		}},
		{name: "missing-identity", want: ErrInvalidRequest, run: func() (Page, error) {
			r := Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits()}
			r.Binding.RequestDigest = ""
			return Paginate(testRecords(), r)
		}},
		{name: "invalid-status", want: ErrInvalidRequest, run: func() (Page, error) {
			r := Request{Header: testHeader(), Binding: testBinding(), Limits: testLimits()}
			r.Header.Status = ""
			return Paginate(testRecords(), r)
		}},
		{name: "invalid-limit", want: ErrResourceLimit, run: func() (Page, error) {
			limits := testLimits()
			limits.MaxPageBytes = 0
			return Paginate(testRecords(), Request{Header: testHeader(), Binding: testBinding(), Limits: limits})
		}},
		{name: "reordered-page", want: ErrInvalidCursor, run: func() (Page, error) {
			reordered := testRecords()
			reordered[0], reordered[1] = reordered[1], reordered[0]
			binding := testBinding()
			binding.ProjectionDigest, _ = RecordDigest(reordered)
			return Paginate(reordered, Request{Header: testHeader(), Binding: binding, Limits: testLimits(), Cursor: first.NextCursor})
		}},
		{name: "mixed-page", want: ErrInvalidCursor, run: func() (Page, error) {
			mixed := append(testRecords(), Record{Kind: "UNIT", Key: "mixed", Data: json.RawMessage(`{"id":"mixed"}`), Objects: 1})
			binding := testBinding()
			binding.ProjectionDigest, _ = RecordDigest(mixed)
			return Paginate(mixed, Request{Header: testHeader(), Binding: binding, Limits: testLimits(), Cursor: first.NextCursor})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := tc.run()
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(page, Page{}) {
				t.Fatalf("ASSERT_C06_%s_TERMINAL_ZERO: page=%+v err=%v want=%v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), page, err, tc.want)
			}
		})
	}
}

func mustDecodeCursorBytes(t *testing.T, cursor string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cursorForRawPayload(t *testing.T, payload []byte) string {
	t.Helper()
	digest := sha256.Sum256(payload)
	raw, err := json.Marshal(cursorEnvelope{Payload: payload, Digest: "sha256:" + hex.EncodeToString(digest[:])})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
