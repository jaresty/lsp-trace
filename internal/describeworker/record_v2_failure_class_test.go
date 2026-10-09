package describeworker

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/describerequest"
)

func TestRecordV2OuterCanonicalityMatchesWorkerV2(t *testing.T) {
	requireDarwinPreflight(t)
	request, packet := validRunnerFixture(t)
	cfg := fixtureConfig(t)
	cfg.ResponseVersion = ResponseVersionV2
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text, err := json.Marshal(validSemanticV2())
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte(`{"status":"COMPLETE","text":` + string(text) + `,"tokens":1,"load_ms":0,"run_ms":0,"cancelled":false,"decode_status":0,"grammar":"llama_sampler_init_grammar","context_tokens":1}` + "\n")
	if _, err = runner.RecordV2(canonical, request, packet, "attempt"); err != nil {
		t.Fatalf("ASSERT_V2_OUTER_OMITTED_EMPTY_ERROR_ACCEPTED: %v", err)
	}

	rejected := map[string][]byte{
		"explicit-empty-error": []byte(strings.Replace(string(canonical), `,"grammar":`, `,"error":"","grammar":`, 1)),
		"missing-text":         []byte(strings.Replace(string(canonical), `,"text":`+string(text), "", 1)),
		"missing-status":       []byte(strings.Replace(string(canonical), `"status":"COMPLETE",`, "", 1)),
		"invalid-status":       []byte(strings.Replace(string(canonical), `"status":"COMPLETE"`, `"status":"ERROR"`, 1)),
		"duplicate":            []byte(strings.Replace(string(canonical), `{"status":`, `{"status":"COMPLETE","status":`, 1)),
		"unknown":              []byte(strings.Replace(string(canonical), `{"status":`, `{"unknown":0,"status":`, 1)),
		"trailing":             append(append([]byte(nil), canonical...), []byte("{}\n")...),
		"field-order":          []byte(strings.Replace(string(canonical), `"status":"COMPLETE","text":`+string(text), `"text":`+string(text)+`,"status":"COMPLETE"`, 1)),
		"whitespace":           []byte(strings.Replace(string(canonical), `{"status"`, `{ "status"`, 1)),
	}
	for name, raw := range rejected {
		t.Run(name, func(t *testing.T) {
			_, gotErr := runner.RecordV2(raw, request, packet, "attempt")
			var failure *Failure
			if !AsFailure(gotErr, &failure) || failure.Subcode() != subcodeOuterProtocolV2 {
				t.Fatalf("ASSERT_V2_OUTER_REJECT_%s: %v", name, gotErr)
			}
		})
	}
}

func TestRecordV2FailureClassificationIsExactAndSourceSafe(t *testing.T) {
	requireDarwinPreflight(t)
	request, packet := validRunnerFixture(t)
	cfg := fixtureConfig(t)
	cfg.ResponseVersion = ResponseVersionV2
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"verdict":"SUPPORTED","target_role":"CaptureSnapshots","nearest_outward_consumer":"resumeWithHooks","consumer_need":"need","provided_behavior":"behavior","boundary_contribution":"boundary","limitations":[],"citations":[]}`
	valueInvalid := `{"verdict":"UNKNOWN","target_role":"role","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`
	unknown := `{"verdict":"COMPLETE","target_role":"role","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[],"secret_key":"secret prose"}`
	tests := []struct {
		name   string
		raw    []byte
		mutate func(*describerequest.Record)
		want   string
	}{
		{"outer", []byte("{}\n"), nil, subcodeOuterProtocolV2},
		{"syntax", EncodeAdapterResultV2("{"), nil, subcodeSemanticSyntaxV2},
		{"schema", EncodeAdapterResultV2(legacy), nil, subcodeSemanticSchemaV2},
		{"unknown-schema", EncodeAdapterResultV2(unknown), nil, subcodeSemanticSchemaV2},
		{"value", EncodeAdapterResultV2(valueInvalid), nil, subcodeSemanticValueV2},
		{"host", EncodeAdapterResultV2(validSemanticV2()), func(r *describerequest.Record) { r.RecordID = "" }, subcodeHostBindingV2},
		{"consumer", EncodeAdapterResultV2(validSemanticV2()), func(r *describerequest.Record) { r.Lineage.ConsumerResolution = "MISMATCH" }, subcodeConsumerResolutionV2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := request
			if tc.mutate != nil {
				tc.mutate(&r)
			}
			_, gotErr := runner.RecordV2(tc.raw, r, packet, "attempt")
			var failure *Failure
			if !AsFailure(gotErr, &failure) || failure.Subcode() != tc.want {
				t.Fatalf("ASSERT_RECORD_V2_FAILURE_CLASS: want=%s err=%v", tc.want, gotErr)
			}
			if strings.Contains(failure.Subcode(), "secret") || strings.Contains(failure.Subcode(), "key") || strings.Contains(failure.Error(), "secret") {
				t.Fatalf("ASSERT_RECORD_V2_FAILURE_SOURCE_SAFE: %q %q", failure.Subcode(), failure.Error())
			}
		})
	}
	if got := recordV2Subcode(errors.New("unexpected validator failure prose")); got != subcodeInternalValidationV2 {
		t.Fatalf("ASSERT_RECORD_V2_INTERNAL_CLASS: %s", got)
	}
}
