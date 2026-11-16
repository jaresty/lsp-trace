package inventorysuccessor

import (
	"encoding/json"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
	"testing"
)

// Pure carrier only; neither Execute nor any kernel/lifecycle method is invoked.
func TestContext80OperationCompleteSnapshot(t *testing.T) {
	at := sessionruntime.PrivateDiscoveryEventTime{Available: true, ElapsedNS: 7, RemainingNS: 11, WithinDeadline: true}
	read := sessionruntime.PrivateDiscoveryReadStages{FrameComplete: true, Received: at, DecodeAttempted: true, Decoded: at, Outcome: sessionruntime.PrivateDecodeSucceeded}
	f := lspwire.ResponseFacts{State: lspwire.ResponseValidJSON, Envelope: lspwire.ResponseObject, Result: lspwire.ResponseArray, ID: lspwire.ResponseNumber, Association: lspwire.ResponseIDMatched}
	d := sessionruntime.PrivateDiscoveryResponseFacts{Available: true, Count: 1, Context: sessionruntime.PrivateDiscoverySessionContext{Encoding: sessionruntime.PrivateEncodingUTF16, EncodingOrigin: sessionruntime.PrivateEncodingProtocolDefault, Fingerprints: sessionruntime.PrivateDiscoveryFingerprints{Preverified: true, VersionAvailable: true, Major: 1}}}
	d.Records[0] = sessionruntime.PrivateDiscoveryResponseRecord{Ordinal: 1, ReadAttempts: 1, LastDisposition: sessionruntime.PrivateResponseSelected, LastFrame: f, Selected: f, Correlation: sessionruntime.PrivateDiscoveryCorrelation{Method: sessionruntime.PrivateMethodPrepareCallHierarchy, FileAvailable: true, FileOrdinal: 0, TargetAvailable: true, TargetOrdinal: 7, PositionAvailable: true, Line: 4, Character: 9, ContentAvailable: true, ContentSHA256: [32]byte{3}}, Sent: true, SentAt: at, LastRead: read, SelectedRead: read, Classification: sessionruntime.PrivateDiscoveryClassification{Observed: true, At: at, Status: sessionruntime.PrivateClassificationSuccess}}
	if !d.Valid() {
		t.Fatal("fixture invalid")
	}
	slot := new(privateDiscoveryOperationResponseFailureV2)
	e := slot.capture(sessionruntime.PrivateDiscoveryRefusal{}, false, d)
	got, ok := PrivateDiscoveryOperationResponseFactsV2(e)
	if !ok || got != d {
		t.Fatal("ASSERT80_OPERATION_FULL_HANDOFF")
	}
	op := PrivateDiscoveryOperationV2{state: &privateDiscoveryOperationV2{observation: sessionruntime.PrivateDiscoveryObservation{ResponseFacts: d}}}
	before, _ := json.Marshal(got)
	got.Records[0].Correlation.TargetOrdinal = 99
	got.Context.Fingerprints.Major = 9
	again, _ := PrivateDiscoveryOperationResponseFactsV2(e)
	after, _ := json.Marshal(again)
	if string(before) != string(after) || op.ResponseFacts() != d {
		t.Fatal("ASSERT80_OPERATION_INDEPENDENT_VALUES")
	}
}
