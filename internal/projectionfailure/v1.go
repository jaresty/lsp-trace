// Package projectionfailure defines a private, bounded diagnostic carrier for
// Graph Provenance V5 projection failures that occur before public artifact bytes exist.
package projectionfailure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"

	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const (
	SchemaVersion  = "lsp-trace.private-graph-provenance-v5-projection-failure.v1"
	MaxRecords     = 64
	MaxBytes       = 65536
	MaxStringBytes = 4096
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var allowedMethods = map[string]bool{"initialize": true, "textDocument/didOpen": true, "textDocument/documentSymbol": true, "textDocument/prepareCallHierarchy": true, "callHierarchy/incomingCalls": true, "callHierarchy/outgoingCalls": true}
var allowedDispositions = map[string]bool{"MATCHED": true, "UNMATCHED": true, "DEADLINE_EXCEEDED": true, "WRITE_FAILED": true, "FRAMING_FAILED": true, "PROCESS_EXITED": true}

type Identity struct {
	AttemptID           string `json:"attempt_id"`
	SessionID           string `json:"session_id"`
	Generation          uint64 `json:"generation"`
	Operation           string `json:"operation"`
	RequestPolicySHA256 string `json:"request_policy_sha256"`
}
type Failure struct {
	Stage             string `json:"stage"`
	Code              string `json:"code"`
	MismatchReason    string `json:"mismatch_reason"`
	PublicArtifact    string `json:"public_artifact"`
	PublicationStatus string `json:"publication_status"`
}
type Record struct {
	Method      string `json:"method"`
	Disposition string `json:"disposition"`
}
type Retention struct {
	RetainedRecords int `json:"retained_records"`
	OmittedRecords  int `json:"omitted_records"`
	MaxRecords      int `json:"max_records"`
	MaxBytes        int `json:"max_bytes"`
	MaxStringBytes  int `json:"max_string_bytes"`
}
type Document struct {
	SchemaVersion string    `json:"schema_version"`
	Identity      Identity  `json:"identity"`
	Failure       Failure   `json:"failure"`
	Records       []Record  `json:"records"`
	Retention     Retention `json:"retention"`
}

type Request struct {
	Operation      string
	RequestPolicy  []byte
	Stage          string
	Code           string
	MismatchReason string
}

func Project(source sessionruntime.DiagnosticSnapshotSet, req Request) ([]byte, error) {
	if !source.Certified() || source.AttemptID() == "" || source.SessionID() == "" || source.Generation() == 0 {
		return nil, errors.New("uncertified projection failure source")
	}
	if req.Operation != "slice-v3" && req.Operation != "incoming-v3" {
		return nil, errors.New("projection failure operation")
	}
	if req.Stage != "GRAPH_PROVENANCE_V5_PROJECTION" || req.Code != "OUTPUT_VALIDATION_FAILED" || req.MismatchReason != "NO_EXACT_TOPMOST_SIBLING_RELATIONS" {
		return nil, errors.New("projection failure identity")
	}
	if len(req.RequestPolicy) == 0 {
		return nil, errors.New("request policy unavailable")
	}
	sum := sha256.Sum256(req.RequestPolicy)
	ops := source.Operations()
	if len(ops) == 0 || len(ops) > MaxRecords {
		return nil, errors.New("projection failure source bound")
	}
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].Handle() < ops[j].Handle() })
	d := Document{SchemaVersion: SchemaVersion, Identity: Identity{AttemptID: string(source.AttemptID()), SessionID: source.SessionID(), Generation: source.Generation(), Operation: req.Operation, RequestPolicySHA256: "sha256:" + hex.EncodeToString(sum[:])}, Failure: Failure{Stage: req.Stage, Code: req.Code, MismatchReason: req.MismatchReason, PublicArtifact: "UNAVAILABLE", PublicationStatus: "NOT_PUBLISHED"}, Records: []Record{}}
	for _, op := range ops {
		disposition, ok := terminalDisposition(op)
		if !ok || !allowedMethods[op.Method] {
			return nil, errors.New("projection failure record")
		}
		d.Records = append(d.Records, Record{Method: op.Method, Disposition: disposition})
	}
	d.Retention = Retention{RetainedRecords: len(d.Records), OmittedRecords: int(source.Omitted()), MaxRecords: MaxRecords, MaxBytes: MaxBytes, MaxStringBytes: MaxStringBytes}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if err := Validate(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func terminalDisposition(op sessionruntime.DiagnosticSnapshot) (string, bool) {
	var out string
	for _, e := range op.Events.Events {
		switch sessionruntime.DiagnosticEventName(e.Code) {
		case "SEMANTIC_MATCHED", "TERMINAL_RESPONSE", "INITIALIZED":
			out = "MATCHED"
		case "SEMANTIC_UNMATCHED":
			out = "UNMATCHED"
		case "TERMINAL_DEADLINE":
			out = "DEADLINE_EXCEEDED"
		case "TERMINAL_FAILURE":
			out = "FRAMING_FAILED"
		}
	}
	return out, out != ""
}

func Validate(raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxBytes {
		return errors.New("projection failure byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var d Document
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one projection failure object required")
	}
	if d.SchemaVersion != SchemaVersion || d.Failure.Stage != "GRAPH_PROVENANCE_V5_PROJECTION" || d.Failure.Code != "OUTPUT_VALIDATION_FAILED" || d.Failure.MismatchReason != "NO_EXACT_TOPMOST_SIBLING_RELATIONS" || d.Failure.PublicArtifact != "UNAVAILABLE" || d.Failure.PublicationStatus != "NOT_PUBLISHED" {
		return errors.New("projection failure schema or identity")
	}
	if d.Identity.AttemptID == "" || d.Identity.SessionID == "" || d.Identity.Generation == 0 || (d.Identity.Operation != "slice-v3" && d.Identity.Operation != "incoming-v3") || !digestPattern.MatchString(d.Identity.RequestPolicySHA256) {
		return errors.New("projection failure invocation identity")
	}
	if len(d.Records) == 0 || len(d.Records) > MaxRecords || d.Retention.RetainedRecords != len(d.Records) || d.Retention.OmittedRecords < 0 || d.Retention.MaxRecords != MaxRecords || d.Retention.MaxBytes != MaxBytes || d.Retention.MaxStringBytes != MaxStringBytes {
		return errors.New("projection failure retention")
	}
	strings := []string{d.SchemaVersion, d.Identity.AttemptID, d.Identity.SessionID, d.Identity.Operation, d.Identity.RequestPolicySHA256, d.Failure.Stage, d.Failure.Code, d.Failure.MismatchReason, d.Failure.PublicArtifact, d.Failure.PublicationStatus}
	for _, r := range d.Records {
		if !allowedMethods[r.Method] || !allowedDispositions[r.Disposition] {
			return errors.New("projection failure disposition")
		}
		strings = append(strings, r.Method, r.Disposition)
	}
	for _, s := range strings {
		if len(s) == 0 || len(s) > MaxStringBytes {
			return errors.New("projection failure string bound")
		}
	}
	return nil
}
