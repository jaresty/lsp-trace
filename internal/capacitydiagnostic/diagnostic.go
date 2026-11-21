package capacitydiagnostic

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/strictjson"
)

// V3 is exclusively a Start capacity refusal. V1/V2 stay closed and unchanged.
const CapacityDiagnosticSchemaVersion = "lsp-trace.census-operation-diagnostic.v3"

type CapacityDiagnostic struct {
	SchemaVersion string                     `json:"schema_version"`
	Operation     string                     `json:"operation"`
	Reason        string                     `json:"reason"`
	Action        string                     `json:"action"`
	Retry         bool                       `json:"retry"`
	Metrics       *CapacityDiagnosticMetrics `json:"metrics,omitempty"`
}

type CapacityDiagnosticMetrics struct {
	Unit      string `json:"unit"`
	Spent     uint64 `json:"spent"`
	Protected uint64 `json:"protected"`
	Limit     uint64 `json:"limit"`
	Available uint64 `json:"available"`
	Deficit   uint64 `json:"deficit"`
}

func ValidateCapacityDiagnostic(d CapacityDiagnostic) error {
	bad := errors.New("invalid census capacity diagnostic")
	if d.SchemaVersion != CapacityDiagnosticSchemaVersion || d.Operation != "start" || d.Action != "STOP" || d.Retry {
		return bad
	}
	if d.Reason == "CAPACITY_INVALID_QUOTE" {
		if d.Metrics != nil {
			return bad
		}
		return nil
	}
	m := d.Metrics
	if m == nil {
		return bad
	}
	switch d.Reason {
	case "CAPACITY_BYTES":
		if m.Unit != "bytes" || m.Limit > 64<<20 || m.Protected > 64<<20 {
			return bad
		}
	case "CAPACITY_WORK":
		if m.Unit != "work_units" || m.Limit != 1<<32 || m.Protected > 32*(64<<20) || m.Protected%32 != 0 {
			return bad
		}
	case "CAPACITY_PACKETS":
		if m.Unit != "packets" || m.Limit != 64 || m.Spent != 0 || m.Protected > 423 {
			return bad
		}
	default:
		return bad
	}
	if m.Spent > m.Limit || m.Available != m.Limit-m.Spent || m.Protected <= m.Available || m.Deficit != m.Protected-m.Available {
		return bad
	}
	return nil
}

func MarshalCapacityDiagnostic(d CapacityDiagnostic) ([]byte, error) {
	if e := ValidateCapacityDiagnostic(d); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(d)
	if e != nil {
		return nil, e
	}
	return append(raw, '\n'), nil
}

func DecodeCapacityDiagnostic(raw []byte) (CapacityDiagnostic, error) {
	bad := errors.New("invalid census capacity diagnostic")
	if len(raw) > 4096 || strictjson.RejectDuplicates(raw) != nil {
		return CapacityDiagnostic{}, bad
	}
	if !validCapacityShape(raw) {
		return CapacityDiagnostic{}, bad
	}
	var d CapacityDiagnostic
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF {
		return CapacityDiagnostic{}, bad
	}
	if e := ValidateCapacityDiagnostic(d); e != nil {
		return CapacityDiagnostic{}, e
	}
	return d, nil
}
