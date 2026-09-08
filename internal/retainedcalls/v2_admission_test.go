package retainedcalls

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
)

func TestRetainedV2SingleAdmission(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	raw, err := ExportV2(input)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		calls := 0
		version, err := validateForV2Observed(raw, func() { calls++ })
		if err != nil || version != VersionV2 {
			t.Fatalf("valid export: %s %v", version, err)
		}
		if calls != 1 {
			t.Fatalf("ASSERT_SINGLE_ADMISSION: got %d full admissions, want 1", calls)
		}
	}
	if _, err := ValidateFor(raw, Family, "v2"); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_SINGLE_ADMISSION: PASS")
}

func TestRetainedV2AdmissionExactOwnedBytes(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	calls := 0
	a := envelopeAdmissionV2{observe: func() { calls++ }}
	owned := append([]byte(nil), input...)
	if err := a.admit(owned); err != nil {
		t.Fatal(err)
	}
	if err := a.admit(append([]byte(nil), input...)); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("ASSERT_ADMISSION_EXACT_COPY: got %d calls, want 1", calls)
	}
	// Equal semantic content is not equal input bytes.
	if err := a.admit(append([]byte(" \n"), input...)); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("ASSERT_ADMISSION_BYTE_SUBSTITUTION: got %d calls, want 2", calls)
	}
	// Re-admit the original, then mutate the caller's same backing array. A
	// borrowed slice or pointer-identity cache would silently admit the change.
	if err := a.admit(owned); err != nil {
		t.Fatal(err)
	}
	at := bytes.Index(owned, []byte(graphprovenance.Unverified))
	if at < 0 {
		t.Fatal("fixture lacks ceiling")
	}
	owned[at] = 'X'
	if err := a.admit(owned); err == nil {
		t.Fatal("ASSERT_ADMISSION_OWNED_BYTES: mutated caller bytes admitted")
	}
	if err := a.admit(owned); err == nil {
		t.Fatal("ASSERT_ADMISSION_FAILURE_NOT_REUSED: rejected bytes became admitted")
	}
	t.Log("ASSERT_ADMISSION_EXACT_COPY: PASS")
	t.Log("ASSERT_ADMISSION_BYTE_SUBSTITUTION: PASS")
	t.Log("ASSERT_ADMISSION_OWNED_BYTES: PASS")
	t.Log("ASSERT_ADMISSION_FAILURE_NOT_REUSED: PASS")
}

func TestRetainedV2AdmissionSemanticAndOrdering(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	var inner graphprovenance.EvidenceV2
	if err := json.Unmarshal(input, &inner); err != nil {
		t.Fatal(err)
	}
	inner.CaptureBudget.ChargedBytes++
	badInput, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	id := digest(VersionV2+":input", badInput)
	tables, err := extractTablesV2(inner, id)
	if err != nil {
		t.Fatal(err)
	}
	// All projected rows and digests agree. Only mandatory semantic admission
	// rejects this forged accounting: it is not a table-comparison negative.
	bad, err := json.Marshal(EvidenceV2{VersionV2, PolicyV2, badInput, id, tables})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{bad, append([]byte(`{"input_bytes":"`+base64.StdEncoding.EncodeToString(badInput)+`",`), []byte(`"unknown":1}`)...)} {
		_, err := ValidateFor(raw, Family, "v2")
		if err == nil || err.Error() != "V2 capture budget accounting mismatch" {
			t.Fatalf("ASSERT_ADMISSION_SEMANTIC_ORDER: %v", err)
		}
	}
	validCarrier := `"input_bytes":"` + base64.StdEncoding.EncodeToString(input) + `"`
	for _, tc := range []struct{ raw, want string }{
		{`{"unknown":1,` + validCarrier + `}`, `json: unknown field "unknown"`},
		{`{` + validCarrier + `,"n":1,"n":2}`, "V2 duplicate member"},
		{`{"n":1,"n":2,` + validCarrier + `}`, "V2 duplicate member"},
		{`{` + validCarrier + `,"extra":{"input_bytes":"e30="}}`, "schema_version: missing or not a string"},
	} {
		_, err := ValidateFor([]byte(tc.raw), Family, "v2")
		if err == nil || err.Error() != tc.want {
			t.Fatalf("ASSERT_ADMISSION_OUTER_ORDER: want %q, got %v", tc.want, err)
		}
	}
	if _, err := ReconstructV2(tables); err == nil || err.Error() != "V2 capture budget accounting mismatch" {
		t.Fatalf("ASSERT_ADMISSION_INDEPENDENT_RECONSTRUCTION: %v", err)
	}
	t.Log("ASSERT_ADMISSION_SEMANTIC_ORDER: PASS")
	t.Log("ASSERT_ADMISSION_OUTER_ORDER: PASS")
	t.Log("ASSERT_ADMISSION_INDEPENDENT_RECONSTRUCTION: PASS")
}

func TestRetainedV2AdmissionPreflight(t *testing.T) {
	for _, tc := range []struct{ name, inner, want string }{
		{"depth", strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65), "V2 known-carrier depth limit"},
		{"duplicate", `{"n":1,"n":2}`, "V2 duplicate member"},
		{"utf8", string([]byte{'"', 0xff, '"'}), "V2 byte/UTF-8 limit"},
		{"number", `{"n":1e1025}`, "V2 numeric exponent limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"input_bytes":"` + base64.StdEncoding.EncodeToString([]byte(tc.inner)) + `"}`)
			_, err := ValidateFor(raw, Family, "v2")
			if err == nil || err.Error() != tc.want {
				t.Fatalf("ASSERT_ADMISSION_PREFLIGHT: want %q, got %v", tc.want, err)
			}
			t.Log("ASSERT_ADMISSION_PREFLIGHT: PASS")
		})
	}
}
