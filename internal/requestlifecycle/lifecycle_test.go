package requestlifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const (
	assertClosed     = "ASSERT_LIFECYCLE_CLOSED_LANGUAGE"
	assertSemantic   = "ASSERT_LIFECYCLE_SEMANTIC_CORRELATION"
	assertBinding    = "ASSERT_LIFECYCLE_PUBLIC_V3_BINDING"
	assertProjection = "ASSERT_LIFECYCLE_CERTIFIED_PROJECTION"
)

func binding(public []byte) ArtifactBinding {
	s := sha256.Sum256(public)
	return ArtifactBinding{Schema: PublicV3Schema, Length: len(public), SHA256: "sha256:" + hex.EncodeToString(s[:])}
}
func ev(seq uint64, op string, handle uint64, kind string) Event {
	return Event{Sequence: seq, OperationID: op, Handle: handle, Kind: kind}
}
func matchedEvents(seq *uint64, op string, h uint64) []Event {
	kinds := []string{"DISPATCH", "REQUEST_WRITE_COMPLETE", "RESPONSE_BODY", "RESPONSE_DECODED", "MATCHED"}
	out := make([]Event, 0, len(kinds))
	for _, k := range kinds {
		*seq++
		out = append(out, ev(*seq, op, h, k))
	}
	return out
}
func baseFixture(public []byte) DocumentModel {
	seq := uint64(0)
	events := matchedEvents(&seq, "init", 1)
	d := DocumentModel{SchemaVersion: SchemaVersion, Attempt: Attempt{"attempt-1", "manager-1", "process-1", 1}, Initialize: Initialize{"init", "MATCHED", true}, Operations: []Operation{{"init", 1, "initialize", ""}}, Events: events, Artifact: binding(public)}
	finishRetention(&d)
	return d
}
func finishRetention(d *DocumentModel) {
	d.Retention = Retention{Status: "AVAILABLE", RetainedRecords: len(d.Documents) + len(d.Operations) + len(d.Events), MaxRecords: MaxRecords, MaxBytes: MaxBytes, MaxStringBytes: MaxStringBytes}
}
func addOperation(d *DocumentModel, op Operation, kinds ...string) {
	d.Operations = append(d.Operations, op)
	seq := d.Events[len(d.Events)-1].Sequence
	for _, k := range kinds {
		seq++
		d.Events = append(d.Events, ev(seq, op.OperationID, op.Handle, k))
	}
	finishRetention(d)
}
func matchedFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	addOperation(&d, Operation{"prepare", 2, "textDocument/prepareCallHierarchy", ""}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "RESPONSE_BODY", "RESPONSE_DECODED", "MATCHED")
	return d
}
func timeoutLateFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	addOperation(&d, Operation{"incoming", 2, "callHierarchy/incomingCalls", ""}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "TERMINAL_TIMEOUT", "LATE")
	return d
}
func unmatchedFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	addOperation(&d, Operation{"outgoing", 2, "callHierarchy/outgoingCalls", ""}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "RESPONSE_BODY", "RESPONSE_DECODED", "UNMATCHED")
	return d
}
func framingFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	addOperation(&d, Operation{"prepare", 2, "textDocument/prepareCallHierarchy", ""}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "TERMINAL_FRAMING_FAILURE")
	return d
}
func writeFailureFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	addOperation(&d, Operation{"prepare", 2, "textDocument/prepareCallHierarchy", ""}, "DISPATCH", "TERMINAL_WRITE_FAILURE")
	return d
}
func processExitFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	d.Operations = append(d.Operations, Operation{"prepare", 2, "textDocument/prepareCallHierarchy", ""})
	seq := d.Events[len(d.Events)-1].Sequence
	d.Events = append(d.Events, ev(seq+1, "prepare", 2, "DISPATCH"), ev(seq+2, "prepare", 2, "REQUEST_WRITE_COMPLETE"), Event{Sequence: seq + 3, Kind: "PROCESS_EXIT"}, ev(seq+4, "prepare", 2, "TERMINAL_PROCESS_EXIT"), Event{Sequence: seq + 5, Kind: "CLEANUP_COMPLETE"})
	finishRetention(&d)
	return d
}
func emptyRetentionFixture(public []byte, status string) DocumentModel {
	d := DocumentModel{SchemaVersion: SchemaVersion, Attempt: Attempt{"attempt-1", "manager-1", "process-1", 1}, Initialize: Initialize{Status: "OMITTED"}, Artifact: binding(public)}
	d.Retention = Retention{Status: status, MaxRecords: MaxRecords, MaxBytes: MaxBytes, MaxStringBytes: MaxStringBytes}
	if status == "OMITTED" {
		d.Retention.OmittedRecords = 1
		d.Retention.Truncated = true
	} else {
		d.Retention.EvictedRecords = 1
	}
	return d
}
func seedDocumentSymbolFixture(public []byte) DocumentModel {
	d := baseFixture(public)
	d.Documents = []Document{{DocumentID: "doc-1", URISHA256: "sha256:" + strings.Repeat("a", 64), DidOpenOperationID: "open", DidOpenComplete: true, DocumentSymbolOperationID: "symbols"}}
	addOperation(&d, Operation{"open", 2, "textDocument/didOpen", "doc-1"}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "RESPONSE_BODY", "RESPONSE_DECODED", "MATCHED", "DID_OPEN_COMPLETE")
	addOperation(&d, Operation{"symbols", 3, "textDocument/documentSymbol", "doc-1"}, "DISPATCH", "REQUEST_WRITE_COMPLETE", "RESPONSE_BODY", "RESPONSE_DECODED", "MATCHED")
	return d
}
func marshal(t *testing.T, d DocumentModel) []byte {
	t.Helper()
	b, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func clone(t *testing.T, d DocumentModel) DocumentModel {
	t.Helper()
	var c DocumentModel
	if e := json.Unmarshal(marshal(t, d), &c); e != nil {
		t.Fatal(e)
	}
	return c
}

func TestValidLifecycleFixtures(t *testing.T) {
	public := []byte(`{"schema_version":"lsp-trace.graph-provenance.v3"}`)
	fixtures := map[string]DocumentModel{"matched": matchedFixture(public), "timeout-late": timeoutLateFixture(public), "unmatched": unmatchedFixture(public), "framing-failure": framingFixture(public), "write-failure": writeFailureFixture(public), "process-exit": processExitFixture(public), "omitted": emptyRetentionFixture(public, "OMITTED"), "evicted": emptyRetentionFixture(public, "EVICTED"), "seed-documentSymbol": seedDocumentSymbolFixture(public)}
	for name, d := range fixtures {
		t.Run(name, func(t *testing.T) {
			raw := marshal(t, d)
			if _, e := Verify(raw, public); e != nil {
				t.Fatalf("%s/%s: %v", assertSemantic, name, e)
			}
		})
	}
}

func TestLifecycleClosedAndSemanticMutations(t *testing.T) {
	public := []byte(`{"schema_version":"lsp-trace.graph-provenance.v3"}`)
	good := seedDocumentSymbolFixture(public)
	mutations := map[string]func(*DocumentModel){
		"adapted-id":          func(d *DocumentModel) { d.Operations[0].OperationID = "../init" },
		"duplicate-document":  func(d *DocumentModel) { d.Documents = append(d.Documents, d.Documents[0]); finishRetention(d) },
		"duplicate-operation": func(d *DocumentModel) { d.Operations = append(d.Operations, d.Operations[0]); finishRetention(d) },
		"duplicate-handle":    func(d *DocumentModel) { d.Operations[1].Handle = d.Operations[0].Handle },
		"non-strict-sequence": func(d *DocumentModel) { d.Events[1].Sequence = d.Events[0].Sequence },
		"unknown-operation":   func(d *DocumentModel) { d.Events[1].OperationID = "missing" },
		"wrong-handle":        func(d *DocumentModel) { d.Events[1].Handle = 99 },
		"two-terminals": func(d *DocumentModel) {
			i := len(d.Events) - 1
			d.Events = append(d.Events, ev(d.Events[i].Sequence+1, "symbols", 3, "TERMINAL_TIMEOUT"))
			finishRetention(d)
		},
		"matched-missing-write":             func(d *DocumentModel) { d.Events = append(d.Events[:1], d.Events[2:]...); finishRetention(d) },
		"late-without-prior-terminal":       func(d *DocumentModel) { d.Events[len(d.Events)-1].Kind = "LATE" },
		"unmatched-before-decode":           func(d *DocumentModel) { d.Events[len(d.Events)-2].Kind = "UNMATCHED" },
		"didOpen-incomplete":                func(d *DocumentModel) { d.Documents[0].DidOpenComplete = false },
		"documentSymbol-without-capability": func(d *DocumentModel) { d.Initialize.DocumentSymbolCapability = false },
		"capability-without-matched-init":   func(d *DocumentModel) { d.Initialize.Status = "FAILED" },
		"exit-without-cleanup": func(d *DocumentModel) {
			n := d.Events[len(d.Events)-1].Sequence + 1
			d.Events = append(d.Events, Event{Sequence: n, Kind: "PROCESS_EXIT"})
			finishRetention(d)
		},
		"retention-count":     func(d *DocumentModel) { d.Retention.RetainedRecords++ },
		"truncation-mismatch": func(d *DocumentModel) { d.Retention.Truncated = true },
		"eviction-mismatch":   func(d *DocumentModel) { d.Retention.Status = "EVICTED"; d.Retention.EvictedRecords = 1 },
		"string-limit":        func(d *DocumentModel) { d.Attempt.ManagerID = strings.Repeat("x", MaxStringBytes+1) },
		"record-limit": func(d *DocumentModel) {
			for len(d.Documents)+len(d.Operations)+len(d.Events) <= MaxRecords {
				d.Events = append(d.Events, Event{Sequence: uint64(len(d.Events) + 1), Kind: "PROCESS_EXIT"})
			}
			finishRetention(d)
		},
		"known-field-method-smuggling": func(d *DocumentModel) { d.Operations[0].Method = "file:///private" },
		"known-field-digest-smuggling": func(d *DocumentModel) { d.Documents[0].URISHA256 = "sha256:../../secret" },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			d := clone(t, good)
			change(&d)
			if _, e := Verify(marshal(t, d), public); e == nil {
				t.Fatalf("%s/%s accepted", assertSemantic, name)
			}
		})
	}

	raw := marshal(t, good)
	closed := map[string][]byte{
		"unknown-top":    bytes.Replace(raw, []byte(`{"schema_version"`), []byte(`{"secret":"x","schema_version"`), 1),
		"unknown-nested": bytes.Replace(raw, []byte(`"attempt_id"`), []byte(`"secret":"x","attempt_id"`), 1),
		"duplicate-key":  bytes.Replace(raw, []byte(`{"schema_version"`), []byte(`{"schema_version":"x","schema_version"`), 1),
		"trailing":       append(append([]byte{}, raw...), []byte(` {}`)...),
	}
	for name, candidate := range closed {
		t.Run(name, func(t *testing.T) {
			if _, e := Verify(candidate, public); e == nil {
				t.Fatalf("%s/%s accepted", assertClosed, name)
			}
		})
	}
}

func TestBindingVerifiedBeforeSemantics(t *testing.T) {
	public := []byte("public-v3")
	d := matchedFixture(public)
	d.Events[1].Sequence = d.Events[0].Sequence
	raw := marshal(t, d)
	if _, e := Verify(raw, []byte("wrong")); e == nil || !strings.Contains(e.Error(), "length mismatch") {
		t.Fatalf("%s/order: %v", assertBinding, e)
	}
	if _, e := Verify(raw, public); e == nil || !strings.Contains(e.Error(), "sequence") {
		t.Fatalf("%s/semantics: %v", assertBinding, e)
	}
}

func TestCertifiedSnapshotProjectionIsDefensive(t *testing.T) {
	public := []byte("public-v3")
	d := matchedFixture(public)
	v := snapshotValue{attempt: d.Attempt, initialize: d.Initialize, documents: d.Documents, operations: d.Operations, events: d.Events, retention: d.Retention, artifact: d.Artifact}
	s, e := certifyRuntimeSnapshot(v)
	if e != nil {
		t.Fatal(e)
	}
	v.events[0].Kind = "SECRET"
	raw, e := Project(s)
	if e != nil {
		t.Fatalf("%s: %v", assertProjection, e)
	}
	if _, e = Verify(raw, public); e != nil {
		t.Fatalf("%s/verify: %v", assertProjection, e)
	}
	if _, e = Project(CertifiedRuntimeSnapshot{}); e == nil {
		t.Fatalf("%s/uncertified", assertProjection)
	}
}
