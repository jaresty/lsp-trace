package projectionfailure

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDocument() Document {
	return Document{SchemaVersion: SchemaVersion, Identity: Identity{AttemptID: "attempt-1", SessionID: "session-1", Generation: 7, Operation: "slice-v3", RequestPolicySHA256: "sha256:" + strings.Repeat("a", 64)}, Failure: Failure{Stage: "GRAPH_PROVENANCE_V5_PROJECTION", Code: "OUTPUT_VALIDATION_FAILED", MismatchReason: "NO_EXACT_TOPMOST_SIBLING_RELATIONS", PublicArtifact: "UNAVAILABLE", PublicationStatus: "NOT_PUBLISHED"}, Records: []Record{{Method: "textDocument/documentSymbol", Disposition: "UNMATCHED"}}, Retention: Retention{RetainedRecords: 1, MaxRecords: MaxRecords, MaxBytes: MaxBytes, MaxStringBytes: MaxStringBytes}}
}
func encode(t *testing.T, d Document) []byte {
	t.Helper()
	b, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	return append(b, '\n')
}

func TestValidateClosedPrivateFailureCarrier(t *testing.T) {
	d := validDocument()
	if err := Validate(encode(t, d)); err != nil {
		t.Fatalf("ASSERT_V5_FAILURE_VALID: %v", err)
	}
	cases := map[string]func(*Document){
		"generation":  func(d *Document) { d.Identity.Generation = 0 },
		"policy":      func(d *Document) { d.Identity.RequestPolicySHA256 = "sha256:secret/path" },
		"stage":       func(d *Document) { d.Failure.Stage = "PUBLICATION" },
		"reason":      func(d *Document) { d.Failure.MismatchReason = "../../secret" },
		"publication": func(d *Document) { d.Failure.PublicationStatus = "PUBLISHED" },
		"disposition": func(d *Document) { d.Records[0].Disposition = "RAW_ERROR" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := validDocument()
			mutate(&x)
			if Validate(encode(t, x)) == nil {
				t.Fatal("ASSERT_V5_FAILURE_MUTATION_REJECTED")
			}
		})
	}
}
func TestValidatePrivacyAndBoundsFailClosed(t *testing.T) {
	raw := encode(t, validDocument())
	raw = append(raw[:len(raw)-2], []byte(`,"source_body":"secret","path":"/private/x"}`)...)
	raw = append(raw, '\n')
	if Validate(raw) == nil {
		t.Fatal("ASSERT_V5_FAILURE_UNKNOWN_SENSITIVE_FIELDS_REJECTED")
	}
	duplicate := []byte(`{"schema_version":"` + SchemaVersion + `","schema_version":"` + SchemaVersion + `"}`)
	if Validate(duplicate) == nil {
		t.Fatal("ASSERT_V5_FAILURE_DUPLICATE_REJECTED")
	}
	d := validDocument()
	d.Identity.AttemptID = strings.Repeat("x", MaxStringBytes+1)
	if Validate(encode(t, d)) == nil {
		t.Fatal("ASSERT_V5_FAILURE_STRING_BOUND")
	}
	d = validDocument()
	for len(d.Records) <= MaxRecords {
		d.Records = append(d.Records, d.Records[0])
	}
	d.Retention.RetainedRecords = len(d.Records)
	if Validate(encode(t, d)) == nil {
		t.Fatal("ASSERT_V5_FAILURE_RECORD_BOUND")
	}
	if Validate(make([]byte, MaxBytes+1)) == nil {
		t.Fatal("ASSERT_V5_FAILURE_BYTE_BOUND")
	}
}
