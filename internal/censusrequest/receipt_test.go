package censusrequest

import (
	"bytes"
	"testing"
)

func TestNormalizedReceiptRegressionAndStrictReplay(t *testing.T) {
	descriptor := []byte(`{"session_id":"s","generation":7,"sources":["pipeline.go","capture.go"],"down_depth":1,"up_depth":0,"max_nodes":10000,"batch_targets":16,"timeout_ms":60000,"request_timeout_ms":30000,"stop_after":"describe-requests"}`)
	oneSource := []byte(`{"session_id":"s","generation":7,"sources":["pipeline.go"]}`)

	first, err := Decode(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Decode(oneSource)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatal("different semantic requests have identical fingerprints")
	}
	if first.Refresh.SessionID != "s" || first.Refresh.Generation != 7 {
		t.Fatalf("refresh coordinate = %+v", first.Refresh)
	}
	if first.Semantic.BatchTargets != 16 || first.Semantic.StopAfter != "describe-requests" {
		t.Fatalf("descriptor semantics = %+v", first.Semantic)
	}
	if second.Semantic.DownDepth != 1 || second.Semantic.UpDepth != 0 || second.Semantic.MaxNodes != 10000 || second.Semantic.BatchTargets != 63 || second.Semantic.TimeoutMS != 60000 || second.Semantic.RequestTimeoutMS != 30000 {
		t.Fatalf("defaults not normalized: %+v", second.Semantic)
	}
	replayed, err := Decode(first.CanonicalJSON)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Fingerprint != first.Fingerprint || !bytes.Equal(replayed.CanonicalJSON, first.CanonicalJSON) {
		t.Fatalf("normalized replay changed receipt: first=%s %s replay=%s %s", first.Fingerprint, first.CanonicalJSON, replayed.Fingerprint, replayed.CanonicalJSON)
	}
}

func TestDecodeRejectsDuplicateUnknownAndTrailingJSON(t *testing.T) {
	for name, raw := range map[string]string{
		"duplicate": `{"session_id":"s","session_id":"t","generation":1,"sources":["."]}`,
		"unknown":   `{"session_id":"s","generation":1,"sources":["."],"max_bytes":1}`,
		"trailing":  `{"session_id":"s","generation":1,"sources":["."]} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(raw)); err == nil {
				t.Fatal("accepted invalid request")
			}
		})
	}
}
