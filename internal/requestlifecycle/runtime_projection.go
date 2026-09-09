package requestlifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/sessionruntime"
)

// projectRuntime is the package-private Stage2 adapter. Its input can only be
// certified by a sessionruntime.Manager from exact opaque attempt/generation/
// operation handles; callers never supply lifecycle events.
func projectRuntime(source sessionruntime.DiagnosticSnapshotSet) ([]byte, error) {
	if !source.Certified() || source.AttemptID() == "" || source.SessionID() == "" || source.Generation() == 0 {
		return nil, errors.New("uncertified runtime projection source")
	}
	operations := source.Operations()
	if len(operations) == 0 || len(operations) > MaxRecords {
		return nil, errors.New("runtime projection source bound")
	}
	sort.SliceStable(operations, func(i, j int) bool { return operations[i].Handle() < operations[j].Handle() })
	for i := 1; i < len(operations); i++ {
		if operations[i-1].Handle() == operations[i].Handle() {
			return nil, errors.New("duplicate runtime operation")
		}
	}

	attempt := string(source.AttemptID())
	managerDigest := sha256.Sum256([]byte("lsp-trace/request-lifecycle/manager/v1\x00" + source.SessionID()))
	processDigest := operations[0].ProcessIdentity.ExecutableBytes
	artifactDigest := sha256.Sum256(nil)
	d := DocumentModel{
		SchemaVersion: SchemaVersion,
		Attempt:       Attempt{AttemptID: attempt, ManagerID: "manager-" + hex.EncodeToString(managerDigest[:8]), ProcessID: "process-" + hex.EncodeToString(processDigest[:8]), Generation: source.Generation()},
		Artifact:      ArtifactBinding{Schema: PublicV3Schema, Length: 0, SHA256: "sha256:" + hex.EncodeToString(artifactDigest[:])},
	}

	documentIDs := map[string]string{}
	for _, snapshot := range operations {
		if snapshot.DocumentURI == "" {
			continue
		}
		if _, ok := documentIDs[snapshot.DocumentURI]; !ok {
			sum := sha256.Sum256([]byte(snapshot.DocumentURI))
			documentIDs[snapshot.DocumentURI] = "doc-" + hex.EncodeToString(sum[:8])
		}
	}

	var sequence uint64
	var omitted uint64 = source.Omitted()
	for _, snapshot := range operations {
		if snapshot.Method == "" || !methods[snapshot.Method] {
			return nil, errors.New("runtime operation method")
		}
		handle := snapshot.Handle()
		opID := "op-" + hex.EncodeToString(uint64Bytes(handle))
		docID := documentIDs[snapshot.DocumentURI]
		d.Operations = append(d.Operations, Operation{OperationID: opID, Handle: handle, Method: snapshot.Method, DocumentID: docID})
		projected, dropped, err := projectEvents(snapshot, opID, handle)
		if err != nil {
			return nil, err
		}
		omitted += dropped
		for i := range projected {
			sequence++
			projected[i].Sequence = sequence
		}
		d.Events = append(d.Events, projected...)
		if snapshot.Method == "initialize" {
			status := "FAILED"
			if hasEvent(projected, opID, "MATCHED") {
				status = "MATCHED"
			}
			if hasEvent(projected, opID, "UNMATCHED") {
				status = "UNMATCHED"
			}
			d.Initialize = Initialize{OperationID: opID, Status: status, DocumentSymbolCapability: status == "MATCHED" && snapshot.Initialize.DocumentSymbolSupport}
		}
		if snapshot.Method == "textDocument/didOpen" {
			sum := sha256.Sum256([]byte(snapshot.DocumentURI))
			d.Documents = append(d.Documents, Document{DocumentID: docID, URISHA256: "sha256:" + hex.EncodeToString(sum[:]), DidOpenOperationID: opID, DidOpenComplete: hasEvent(projected, opID, "DID_OPEN_COMPLETE")})
		}
		if snapshot.Method == "textDocument/documentSymbol" {
			for i := range d.Documents {
				if d.Documents[i].DocumentID == docID {
					d.Documents[i].DocumentSymbolOperationID = opID
				}
			}
		}
	}
	if d.Initialize.OperationID == "" {
		return nil, errors.New("runtime projection missing initialize")
	}
	retained := len(d.Documents) + len(d.Operations) + len(d.Events)
	if retained > MaxRecords {
		return nil, errors.New("runtime projection record bound")
	}
	d.Retention = Retention{Status: "AVAILABLE", RetainedRecords: retained, OmittedRecords: int(omitted), Truncated: omitted > 0, MaxRecords: MaxRecords, MaxBytes: MaxBytes, MaxStringBytes: MaxStringBytes}
	snapshot, err := certifyRuntimeSnapshot(snapshotValue{attempt: d.Attempt, initialize: d.Initialize, documents: d.Documents, operations: d.Operations, events: d.Events, retention: d.Retention, artifact: d.Artifact})
	if err != nil {
		return nil, err
	}
	return Project(snapshot)
}

func projectEvents(snapshot sessionruntime.DiagnosticSnapshot, opID string, handle uint64) ([]Event, uint64, error) {
	out := []Event{}
	var dropped uint64
	terminal := false
	appendKind := func(kind string) {
		out = append(out, Event{OperationID: opID, Handle: handle, Kind: kind})
		if terminalKinds[kind] {
			terminal = true
		}
	}
	for _, event := range snapshot.Events.Events {
		name := sessionruntime.DiagnosticEventName(event.Code)
		switch name {
		case "BEGIN":
			appendKind("DISPATCH")
		case "WRITE_ATTEMPT":
			dropped++
		case "WRITE_COMPLETE":
			appendKind("REQUEST_WRITE_COMPLETE")
		case "READ_COMPLETE", "RESPONSE_DECODED", "CORRELATED", "UNCORRELATED":
			dropped++
		case "INITIALIZED":
			appendKind("RESPONSE_BODY")
			appendKind("RESPONSE_DECODED")
		case "SEMANTIC_MATCHED":
			appendKind("RESPONSE_BODY")
			appendKind("RESPONSE_DECODED")
			appendKind("MATCHED")
		case "SEMANTIC_UNMATCHED":
			appendKind("RESPONSE_BODY")
			appendKind("RESPONSE_DECODED")
			appendKind("UNMATCHED")
		case "LATE":
			if terminal {
				appendKind("LATE")
			} else {
				dropped++
			}
		case "TERMINAL_DEADLINE":
			appendKind("TERMINAL_TIMEOUT")
		case "TERMINAL_FAILURE":
			wrote := false
			for _, prior := range out {
				if prior.Kind == "REQUEST_WRITE_COMPLETE" {
					wrote = true
				}
			}
			if wrote {
				appendKind("TERMINAL_FRAMING_FAILURE")
			} else {
				appendKind("TERMINAL_WRITE_FAILURE")
			}
		case "TERMINAL_RESPONSE":
			if terminal {
				dropped++
				continue
			}
			if snapshot.Method == "textDocument/didOpen" {
				appendKind("RESPONSE_BODY")
				appendKind("RESPONSE_DECODED")
				appendKind("MATCHED")
				appendKind("DID_OPEN_COMPLETE")
			} else {
				if snapshot.Method != "initialize" {
					appendKind("RESPONSE_BODY")
					appendKind("RESPONSE_DECODED")
				}
				appendKind("MATCHED")
			}
		default:
			if event.Kind == manageddiagnostic.EventTerminal {
				return nil, 0, errors.New("unknown runtime terminal event")
			}
			dropped++
		}
	}
	if !terminal {
		return nil, 0, errors.New("runtime operation missing terminal")
	}
	return out, dropped, nil
}

func uint64Bytes(v uint64) []byte {
	const digits = "0123456789abcdef"
	out := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		out[i] = digits[v&15]
		v >>= 4
	}
	return out
}
