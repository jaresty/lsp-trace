package censuscontinuation

import (
	"bytes"
	"strings"
	"testing"

	"lsp-trace/internal/describeworker"
)

func TestWorkerRecordsV2OpaqueExactReplay(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	host := describeworker.ResponseHostV2{RequestRecordID: "request", MessageID: "message", AttemptID: "attempt", Consumer: describeworker.ConsumerIdentityV2{Resolution: describeworker.ConsumerUnresolvedV2, AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: describeworker.HostPinsV2{WorkerSHA256: digest, ModelSHA256: digest, GrammarSHA256: digest, PromptSHA256: digest}, Provenance: describeworker.HostProvenanceV2{PacketID: "packet", RequestLineageIdentity: "lineage"}, Custody: describeworker.HostCustodyV2{GraphDigest: digest, CaptureID: "capture"}}
	semantic := []byte(`{"verdict":"COMPLETE","target_role":"target","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`)
	record, err := describeworker.NewResponseRecordV2(semantic, host)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := record.Bytes()
	encoded, err := encodeWorkerRecordsV2([]describeworker.ResponseRecordV2{record})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWorkerRecordsV2(encoded)
	if err != nil || len(decoded) != 1 {
		t.Fatal(err)
	}
	after, _ := decoded[0].Bytes()
	if decoded[0].ID() != record.ID() || !bytes.Equal(before, after) {
		t.Fatal("ASSERT_V2_OPAQUE_RETRY_EXACT_REPLAY")
	}
	tampered := bytes.Replace(encoded, []byte("OUTWARD_CONSUMER_UNRESOLVED"), []byte("OUTWARD_CONSUMER_SUBSTITUTED"), 1)
	if _, err := decodeWorkerRecordsV2(tampered); err == nil {
		t.Fatal("ASSERT_V2_OPAQUE_COORDINATED_SUBSTITUTION_REJECTED")
	}
}
