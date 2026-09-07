package boundedanalysis

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"lsp-trace/internal/graphprovenance"
)

// preflightAdmission is only for the bounded family. Follow the fixed JSON
// evidence chain, never source Content, canonical receipt bytes or notification
// text. Each layer is bounded before any historical recursive validator sees it.
func preflightAdmission(raw []byte, analytical bool) error {
	if analytical {
		if err := Preflight(raw, MaxBytes); err != nil {
			return err
		}
		var err error
		raw, err = decodedCarrier(raw, "input_bytes", MaxInputBytes)
		if err != nil {
			return err
		}
	}
	if err := Preflight(raw, MaxInputBytes); err != nil {
		return err
	}
	provenance, err := decodedCarrier(raw, "input_bytes", graphprovenance.MaxEnvelopeBytes)
	if err != nil {
		return err
	}
	if err = Preflight(provenance, graphprovenance.MaxEnvelopeBytes); err != nil {
		return fmt.Errorf("provenance: %w", err)
	}
	graph, err := decodedCarrier(provenance, "graph_bytes", graphprovenance.MaxGraphBytes)
	if err != nil {
		return err
	}
	if err = Preflight(graph, graphprovenance.MaxGraphBytes); err != nil {
		return fmt.Errorf("graph: %w", err)
	}
	return nil
}

// carrierValue rejects repeated typed-field assignments, including case-folded
// aliases. Otherwise an earlier malformed carrier could be hidden by the last
// value here, while the historical []byte decoder still processes both values.
// Exact duplicate keys have already been rejected by Preflight.
type carrierValue struct {
	raw  json.RawMessage
	seen bool
}

func (v *carrierValue) UnmarshalJSON(raw []byte) error {
	if v.seen {
		return fmt.Errorf("duplicate JSON carrier assignment")
	}
	v.seen = true
	v.raw = append(v.raw[:0], raw...)
	return nil
}

// The caller has preflighted this layer. Typed RawMessages skip unrelated
// values without materializing an object tree. Bound decoded allocation before
// decoding, preserving encoding/json's standard base64 behavior (including CR/LF).
func decodedCarrier(raw []byte, field string, limit int) ([]byte, error) {
	var carrier struct {
		Input carrierValue `json:"input_bytes"`
		Graph carrierValue `json:"graph_bytes"`
	}
	if err := json.Unmarshal(raw, &carrier); err != nil {
		return nil, fmt.Errorf("%s carrier: %w", field, err)
	}
	value := carrier.Input.raw
	if field == "graph_bytes" {
		value = carrier.Graph.raw
	}
	value = bytes.TrimSpace(value)
	if len(value) == 0 || value[0] != '"' {
		return nil, fmt.Errorf("%s carrier must be a base64 JSON string", field)
	}
	var encoded string
	if err := json.Unmarshal(value, &encoded); err != nil {
		return nil, fmt.Errorf("%s carrier: %w", field, err)
	}
	encoded = strings.ReplaceAll(strings.ReplaceAll(encoded, "\r", ""), "\n", "")
	n := base64.StdEncoding.DecodedLen(len(encoded))
	if strings.HasSuffix(encoded, "=") {
		n--
	}
	if strings.HasSuffix(encoded, "==") {
		n--
	}
	if n > limit {
		return nil, fmt.Errorf("%s carrier byte LIMIT", field)
	}
	// DecodeString allocates DecodedLen bytes: at most limit+2 after the
	// padding adjustment above, including malformed padded inputs. Encoded
	// strings and JSON unescaping are bounded by the preflighted parent bytes.
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%s carrier base64: %w", field, err)
	}
	return decoded, nil
}
