package censusresult

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/strictjson"
)

// OperationDiagnostic is intentionally separate from the legacy census diagnostic.
// Retry means permission to repeat the SAME failed operation unchanged. Version 1
// grants no such permission: charged reads and uncertain submits are not replayable.
const OperationDiagnosticSchemaVersion = "lsp-trace.census-operation-diagnostic.v1"

// V2 is a separate closed stored-version refusal, not an extension of v1's enum.
const StoredVersionDiagnosticSchemaVersion = "lsp-trace.census-operation-diagnostic.v2"
const UnsupportedStoredVersion OperationReason = "UNSUPPORTED_STORED_VERSION"

type OperationReason string

const (
	StoreRootMissing     OperationReason = "STORE_ROOT_MISSING"
	StorePathInvalid     OperationReason = "STORE_PATH_INVALID"
	StoreAliasRefused    OperationReason = "STORE_ALIAS_REFUSED"
	StorePermissions     OperationReason = "STORE_PERMISSIONS"
	StoreSourceOverlap   OperationReason = "STORE_SOURCE_OVERLAP"
	StoreIO              OperationReason = "STORE_IO"
	ConfigUnclassified   OperationReason = "CONFIG_UNCLASSIFIED"
	InvalidOperation     OperationReason = "INVALID_OPERATION"
	RunUnavailable       OperationReason = "RUN_UNAVAILABLE"
	OperationUnconfirmed OperationReason = "OPERATION_UNCONFIRMED"
)

type OperationRecovery struct {
	RunID        string `json:"run_id"`
	StoreContext string `json:"store_context"`
}
type OperationDiagnostic struct {
	SchemaVersion string             `json:"schema_version"`
	Operation     string             `json:"operation"`
	Reason        OperationReason    `json:"reason"`
	Action        string             `json:"action"`
	Retry         bool               `json:"retry"`
	Recovery      *OperationRecovery `json:"recovery,omitempty"`
}

func IsOperationConfigReason(r OperationReason) bool {
	switch r {
	case StoreRootMissing, StorePathInvalid, StoreAliasRefused, StorePermissions, StoreSourceOverlap, StoreIO, ConfigUnclassified:
		return true
	}
	return false
}

func ValidateOperationDiagnostic(d OperationDiagnostic) error {
	bad := errors.New("invalid census operation diagnostic")
	if d.SchemaVersion == StoredVersionDiagnosticSchemaVersion {
		if d.Reason != UnsupportedStoredVersion || d.Action != "STOP" || d.Retry || d.Recovery != nil {
			return bad
		}
		switch d.Operation {
		case "task", "read", "submit", "result":
			return nil
		default:
			return bad
		}
	}
	if d.SchemaVersion != OperationDiagnosticSchemaVersion || d.Retry {
		return bad
	}
	switch d.Operation {
	case "start", "task", "read", "submit", "result", "unknown":
	default:
		return bad
	}
	if IsOperationConfigReason(d.Reason) {
		if d.Operation == "unknown" || d.Action != "CORRECT_CONFIG" || d.Recovery != nil {
			return bad
		}
		return nil
	}
	switch d.Reason {
	case InvalidOperation:
		if d.Action != "STOP" || d.Recovery != nil {
			return bad
		}
	case RunUnavailable:
		if d.Action != "STOP" || d.Recovery != nil || d.Operation == "unknown" {
			return bad
		}
	case OperationUnconfirmed:
		if d.Action != "OBSERVE_CURRENT_TASK" || d.Recovery == nil || d.Operation == "start" || d.Operation == "unknown" {
			return bad
		}
		b, e := hex.DecodeString(d.Recovery.RunID)
		if e != nil || len(b) != 32 || hex.EncodeToString(b) != d.Recovery.RunID || d.Recovery.StoreContext != "SAME_SELECTION" {
			return bad
		}
	default:
		return bad
	}
	return nil
}
func MarshalOperationDiagnostic(d OperationDiagnostic) ([]byte, error) {
	if e := ValidateOperationDiagnostic(d); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(d)
	if e != nil {
		return nil, e
	}
	return append(raw, '\n'), nil
}
func DecodeOperationDiagnostic(raw []byte) (OperationDiagnostic, error) {
	bad := errors.New("invalid census operation diagnostic")
	if len(raw) > 4096 || strictjson.RejectDuplicates(raw) != nil {
		return OperationDiagnostic{}, bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return OperationDiagnostic{}, bad
	}
	for _, key := range []string{"schema_version", "operation", "reason", "action", "retry"} {
		v, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return OperationDiagnostic{}, bad
		}
	}
	if v, ok := fields["recovery"]; ok && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return OperationDiagnostic{}, bad
	}
	var d OperationDiagnostic
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF {
		return OperationDiagnostic{}, bad
	}
	if e := ValidateOperationDiagnostic(d); e != nil {
		return OperationDiagnostic{}, e
	}
	return d, nil
}
