// Package requestlifecycle defines the private, offline request-lifecycle
// diagnostic document. It deliberately contains no runtime or CLI wiring.
package requestlifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"lsp-trace/internal/strictjson"
)

const (
	SchemaVersion  = "lsp-trace.private-request-lifecycle-diagnostics.v1"
	PublicV3Schema = "lsp-trace.graph-provenance.v3"
	MaxRecords     = 64
	MaxBytes       = 65536
	MaxStringBytes = 4096
)

var (
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)
)

type Attempt struct {
	AttemptID  string `json:"attempt_id"`
	ManagerID  string `json:"manager_id"`
	ProcessID  string `json:"process_id"`
	Generation uint64 `json:"generation"`
}

type Initialize struct {
	OperationID              string `json:"operation_id"`
	Status                   string `json:"status"`
	DocumentSymbolCapability bool   `json:"document_symbol_capability"`
}

type Document struct {
	DocumentID                string `json:"document_id"`
	URISHA256                 string `json:"uri_sha256"`
	DidOpenOperationID        string `json:"did_open_operation_id"`
	DidOpenComplete           bool   `json:"did_open_complete"`
	DocumentSymbolOperationID string `json:"document_symbol_operation_id,omitempty"`
}

type Operation struct {
	OperationID string `json:"operation_id"`
	Handle      uint64 `json:"handle"`
	Method      string `json:"method"`
	DocumentID  string `json:"document_id,omitempty"`
}

type Event struct {
	Sequence    uint64 `json:"sequence"`
	OperationID string `json:"operation_id,omitempty"`
	Handle      uint64 `json:"handle,omitempty"`
	Kind        string `json:"kind"`
}

type Retention struct {
	Status          string `json:"status"`
	RetainedRecords int    `json:"retained_records"`
	OmittedRecords  int    `json:"omitted_records"`
	EvictedRecords  int    `json:"evicted_records"`
	Truncated       bool   `json:"truncated"`
	MaxRecords      int    `json:"max_records"`
	MaxBytes        int    `json:"max_bytes"`
	MaxStringBytes  int    `json:"max_string_bytes"`
}

type ArtifactBinding struct {
	Schema string `json:"schema"`
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
}

type DocumentModel struct {
	SchemaVersion string          `json:"schema_version"`
	Attempt       Attempt         `json:"attempt"`
	Initialize    Initialize      `json:"initialize"`
	Documents     []Document      `json:"documents"`
	Operations    []Operation     `json:"operations"`
	Events        []Event         `json:"events"`
	Retention     Retention       `json:"retention"`
	Artifact      ArtifactBinding `json:"artifact"`
}

var methods = map[string]bool{
	"initialize":                        true,
	"textDocument/didOpen":              true,
	"textDocument/documentSymbol":       true,
	"textDocument/prepareCallHierarchy": true,
	"callHierarchy/incomingCalls":       true,
	"callHierarchy/outgoingCalls":       true,
}
var eventKinds = map[string]bool{
	"DISPATCH":                 true,
	"REQUEST_WRITE_COMPLETE":   true,
	"RESPONSE_BODY":            true,
	"RESPONSE_DECODED":         true,
	"MATCHED":                  true,
	"TERMINAL_TIMEOUT":         true,
	"TERMINAL_WRITE_FAILURE":   true,
	"TERMINAL_FRAMING_FAILURE": true,
	"TERMINAL_PROCESS_EXIT":    true,
	"LATE":                     true,
	"UNMATCHED":                true,
	"DID_OPEN_COMPLETE":        true,
	"PROCESS_EXIT":             true,
	"CLEANUP_COMPLETE":         true,
}
var terminalKinds = map[string]bool{
	"MATCHED":                  true,
	"TERMINAL_TIMEOUT":         true,
	"TERMINAL_WRITE_FAILURE":   true,
	"TERMINAL_FRAMING_FAILURE": true,
	"TERMINAL_PROCESS_EXIT":    true,
	"UNMATCHED":                true,
}

// CertifiedRuntimeSnapshot is immutable to callers: all state is held in an
// unexported value and this package exposes no constructor from event slices.
type CertifiedRuntimeSnapshot struct{ value snapshotValue }
type snapshotValue struct {
	attempt    Attempt
	initialize Initialize
	documents  []Document
	operations []Operation
	events     []Event
	retention  Retention
	artifact   ArtifactBinding
	certified  bool
}

// certifyRuntimeSnapshot is intentionally package-private. A later runtime
// integration may call it only through code placed in this internal package.
func certifyRuntimeSnapshot(v snapshotValue) (CertifiedRuntimeSnapshot, error) {
	v.certified = true
	s := CertifiedRuntimeSnapshot{value: cloneSnapshot(v)}
	if err := validateModel(modelFromSnapshot(s)); err != nil {
		return CertifiedRuntimeSnapshot{}, err
	}
	return s, nil
}
func cloneSnapshot(v snapshotValue) snapshotValue {
	v.documents = append([]Document(nil), v.documents...)
	v.operations = append([]Operation(nil), v.operations...)
	v.events = append([]Event(nil), v.events...)
	return v
}
func modelFromSnapshot(s CertifiedRuntimeSnapshot) DocumentModel {
	v := cloneSnapshot(s.value)
	return DocumentModel{SchemaVersion: SchemaVersion, Attempt: v.attempt, Initialize: v.initialize, Documents: v.documents, Operations: v.operations, Events: v.events, Retention: v.retention, Artifact: v.artifact}
}

// Project serializes a certified immutable runtime snapshot. It cannot accept
// caller-created events, operations, or a generic interface value.
func Project(s CertifiedRuntimeSnapshot) ([]byte, error) {
	if !s.value.certified {
		return nil, errors.New("uncertified runtime snapshot")
	}
	d := modelFromSnapshot(s)
	if err := validateModel(d); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxBytes {
		return nil, errors.New("lifecycle diagnostic byte limit")
	}
	return append(raw, '\n'), nil
}

// Verify checks the external public artifact binding before decoding or
// evaluating lifecycle semantics.
func Verify(raw, publicV3 []byte) (DocumentModel, error) {
	var envelope struct {
		SchemaVersion string          `json:"schema_version"`
		Artifact      ArtifactBinding `json:"artifact"`
	}
	if len(raw) == 0 || len(raw) > MaxBytes {
		return DocumentModel{}, errors.New("lifecycle diagnostic byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return DocumentModel{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	// Decode only the binding with a raw map so semantic fields are not trusted yet.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return DocumentModel{}, err
	}
	if b, ok := top["schema_version"]; ok {
		_ = json.Unmarshal(b, &envelope.SchemaVersion)
	}
	if b, ok := top["artifact"]; ok {
		ad := json.NewDecoder(bytes.NewReader(b))
		ad.DisallowUnknownFields()
		if err := ad.Decode(&envelope.Artifact); err != nil {
			return DocumentModel{}, err
		}
	}
	if envelope.SchemaVersion != SchemaVersion {
		return DocumentModel{}, errors.New("lifecycle diagnostic schema")
	}
	if envelope.Artifact.Schema != PublicV3Schema {
		return DocumentModel{}, errors.New("public V3 schema binding")
	}
	if envelope.Artifact.Length != len(publicV3) {
		return DocumentModel{}, errors.New("public V3 length mismatch")
	}
	sum := sha256.Sum256(publicV3)
	if envelope.Artifact.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		return DocumentModel{}, errors.New("public V3 digest mismatch")
	}
	return decodeSemantics(raw)
}

func decodeSemantics(raw []byte) (DocumentModel, error) {
	var d DocumentModel
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return d, errors.New("one lifecycle diagnostic object required")
	}
	if err := validateModel(d); err != nil {
		return d, err
	}
	return d, nil
}

func validID(s string) bool     { return idPattern.MatchString(s) }
func validDigest(s string) bool { return digestPattern.MatchString(s) }
func stringsBounded(d DocumentModel) bool {
	values := []string{d.SchemaVersion, d.Attempt.AttemptID, d.Attempt.ManagerID, d.Attempt.ProcessID, d.Initialize.OperationID, d.Initialize.Status, d.Retention.Status, d.Artifact.Schema, d.Artifact.SHA256}
	for _, x := range d.Documents {
		values = append(values, x.DocumentID, x.URISHA256, x.DidOpenOperationID, x.DocumentSymbolOperationID)
	}
	for _, x := range d.Operations {
		values = append(values, x.OperationID, x.Method, x.DocumentID)
	}
	for _, x := range d.Events {
		values = append(values, x.OperationID, x.Kind)
	}
	for _, s := range values {
		if len(s) > MaxStringBytes {
			return false
		}
	}
	return true
}

func validateModel(d DocumentModel) error {
	if d.SchemaVersion != SchemaVersion || !stringsBounded(d) {
		return errors.New("lifecycle schema or string bound")
	}
	if !validID(d.Attempt.AttemptID) || !validID(d.Attempt.ManagerID) || !validID(d.Attempt.ProcessID) || d.Attempt.Generation == 0 {
		return errors.New("attempt identity")
	}
	if d.Initialize.Status != "MATCHED" && d.Initialize.Status != "UNMATCHED" && d.Initialize.Status != "FAILED" && d.Initialize.Status != "OMITTED" {
		return errors.New("initialize status")
	}
	if len(d.Documents)+len(d.Operations)+len(d.Events) > MaxRecords {
		return errors.New("lifecycle record limit")
	}
	if d.Retention.MaxRecords != MaxRecords || d.Retention.MaxBytes != MaxBytes || d.Retention.MaxStringBytes != MaxStringBytes || d.Retention.RetainedRecords != len(d.Documents)+len(d.Operations)+len(d.Events) || d.Retention.OmittedRecords < 0 || d.Retention.EvictedRecords < 0 {
		return errors.New("retention accounting")
	}
	switch d.Retention.Status {
	case "AVAILABLE":
		if d.Retention.EvictedRecords != 0 || d.Retention.Truncated != (d.Retention.OmittedRecords > 0) {
			return errors.New("available retention semantics")
		}
	case "OMITTED":
		if d.Retention.RetainedRecords != 0 || d.Retention.OmittedRecords == 0 || d.Retention.EvictedRecords != 0 || !d.Retention.Truncated {
			return errors.New("omitted retention semantics")
		}
	case "EVICTED":
		if d.Retention.RetainedRecords != 0 || d.Retention.OmittedRecords != 0 || d.Retention.EvictedRecords == 0 || d.Retention.Truncated {
			return errors.New("evicted retention semantics")
		}
	default:
		return errors.New("retention status")
	}
	if d.Artifact.Schema != PublicV3Schema || d.Artifact.Length < 0 || !validDigest(d.Artifact.SHA256) {
		return errors.New("artifact binding")
	}

	docs := map[string]Document{}
	for _, x := range d.Documents {
		if !validID(x.DocumentID) || !validDigest(x.URISHA256) || !validID(x.DidOpenOperationID) {
			return errors.New("document identity")
		}
		if _, exists := docs[x.DocumentID]; exists {
			return errors.New("duplicate document id")
		}
		docs[x.DocumentID] = x
	}
	ops := map[string]Operation{}
	handles := map[uint64]bool{}
	for _, x := range d.Operations {
		if !validID(x.OperationID) || x.Handle == 0 || !methods[x.Method] {
			return errors.New("operation identity")
		}
		if _, exists := ops[x.OperationID]; exists || handles[x.Handle] {
			return errors.New("duplicate operation identity")
		}
		if x.DocumentID != "" {
			if _, ok := docs[x.DocumentID]; !ok {
				return errors.New("operation document foreign key")
			}
		}
		ops[x.OperationID] = x
		handles[x.Handle] = true
	}
	initOp, ok := ops[d.Initialize.OperationID]
	if d.Initialize.Status == "OMITTED" {
		if d.Initialize.OperationID != "" || d.Initialize.DocumentSymbolCapability || d.Retention.Status == "AVAILABLE" {
			return errors.New("omitted initialize detail")
		}
	} else if !ok || initOp.Method != "initialize" {
		return errors.New("initialize operation")
	}
	if d.Initialize.DocumentSymbolCapability && d.Initialize.Status != "MATCHED" {
		return errors.New("initialize capability without matched initialize")
	}
	for _, x := range d.Documents {
		op, ok := ops[x.DidOpenOperationID]
		if !ok || op.Method != "textDocument/didOpen" || op.DocumentID != x.DocumentID {
			return errors.New("didOpen operation")
		}
		if x.DocumentSymbolOperationID != "" {
			op, ok = ops[x.DocumentSymbolOperationID]
			if !ok || op.Method != "textDocument/documentSymbol" || op.DocumentID != x.DocumentID {
				return errors.New("documentSymbol operation")
			}
			if !x.DidOpenComplete {
				return errors.New("documentSymbol before didOpen completion")
			}
		}
	}

	type state struct {
		dispatch, write, body, decoded bool
		terminal                       string
		terminalSeq                    uint64
	}
	states := map[string]*state{}
	var previous uint64
	var processExit, cleanup uint64
	for _, e := range d.Events {
		if e.Sequence == 0 || e.Sequence <= previous || !eventKinds[e.Kind] {
			return errors.New("event sequence or kind")
		}
		previous = e.Sequence
		if e.Kind == "PROCESS_EXIT" || e.Kind == "CLEANUP_COMPLETE" {
			if e.OperationID != "" || e.Handle != 0 {
				return errors.New("process event carries operation")
			}
			if e.Kind == "PROCESS_EXIT" {
				if processExit != 0 {
					return errors.New("duplicate process exit")
				}
				processExit = e.Sequence
			} else {
				if processExit == 0 || cleanup != 0 {
					return errors.New("cleanup chronology")
				}
				cleanup = e.Sequence
			}
			continue
		}
		op, ok := ops[e.OperationID]
		if !ok || e.Handle != op.Handle {
			return errors.New("event operation or handle foreign key")
		}
		s := states[e.OperationID]
		if s == nil {
			s = &state{}
			states[e.OperationID] = s
		}
		switch e.Kind {
		case "DISPATCH":
			if s.dispatch || s.terminal != "" {
				return errors.New("dispatch chronology")
			}
			s.dispatch = true
		case "REQUEST_WRITE_COMPLETE":
			if !s.dispatch || s.write || s.terminal != "" {
				return errors.New("write chronology")
			}
			s.write = true
		case "RESPONSE_BODY":
			if !s.write || s.body || s.terminal != "" {
				return errors.New("response body chronology")
			}
			s.body = true
		case "RESPONSE_DECODED":
			if !s.body || s.decoded || s.terminal != "" {
				return errors.New("decode chronology")
			}
			s.decoded = true
		case "MATCHED":
			if !s.dispatch || !s.write || !s.body || !s.decoded || s.terminal != "" {
				return errors.New("matched chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "TERMINAL_TIMEOUT":
			if !s.dispatch || s.terminal != "" {
				return errors.New("timeout chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "TERMINAL_WRITE_FAILURE":
			if !s.dispatch || s.write || s.terminal != "" {
				return errors.New("terminal write chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "TERMINAL_FRAMING_FAILURE":
			if !s.write || s.body || s.terminal != "" {
				return errors.New("framing chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "TERMINAL_PROCESS_EXIT":
			if !s.dispatch || processExit == 0 || processExit >= e.Sequence || s.terminal != "" {
				return errors.New("process exit terminal chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "UNMATCHED":
			if !s.body || !s.decoded || s.terminal != "" {
				return errors.New("unmatched chronology")
			}
			s.terminal = e.Kind
			s.terminalSeq = e.Sequence
		case "LATE":
			if s.terminal == "" || e.Sequence <= s.terminalSeq || s.terminal == "MATCHED" || s.terminal == "UNMATCHED" {
				return errors.New("late chronology")
			}
		case "DID_OPEN_COMPLETE":
			if op.Method != "textDocument/didOpen" || !s.write || s.terminal != "MATCHED" {
				return errors.New("didOpen completion chronology")
			}
		}
	}
	for id, op := range ops {
		s := states[id]
		if s == nil || !s.dispatch || !terminalKinds[s.terminal] {
			return fmt.Errorf("operation %s lacks exactly one terminal", id)
		}
		if op.Method == "textDocument/didOpen" {
			doc := docs[op.DocumentID]
			if doc.DidOpenComplete != hasEvent(d.Events, id, "DID_OPEN_COMPLETE") {
				return errors.New("didOpen completion mismatch")
			}
		}
		if op.Method == "textDocument/documentSymbol" && !d.Initialize.DocumentSymbolCapability {
			return errors.New("documentSymbol without capability")
		}
	}
	if d.Initialize.Status != "OMITTED" {
		terminal := states[d.Initialize.OperationID].terminal
		matched := d.Initialize.Status == "MATCHED" && terminal == "MATCHED"
		unmatched := d.Initialize.Status == "UNMATCHED" && terminal == "UNMATCHED"
		failed := d.Initialize.Status == "FAILED" && terminal != "MATCHED" && terminal != "UNMATCHED"
		if !matched && !unmatched && !failed {
			return errors.New("initialize terminal mismatch")
		}
	}
	if processExit != 0 && cleanup == 0 {
		return errors.New("process exit lacks cleanup")
	}
	if cleanup != 0 && cleanup <= processExit {
		return errors.New("process cleanup order")
	}
	return nil
}
func hasEvent(events []Event, id, kind string) bool {
	for _, e := range events {
		if e.OperationID == id && e.Kind == kind {
			return true
		}
	}
	return false
}
