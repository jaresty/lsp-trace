package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/custodyevidence"
)

// LoadHostTrustStore is for trusted process startup only, never operation input.
// File contents model independently approved host grants; loading a file does
// not itself perform the cryptographic verification named in receipt metadata.
func LoadHostTrustStore(path string) (*custodyevidence.HostTrustStore, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("host custody config: %w", err)
	}
	var config struct {
		Grants []custodyevidence.HostTrustGrant `json:"grants"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("host custody config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("host custody config must contain one JSON value")
	}
	return custodyevidence.NewHostTrustStore(config.Grants)
}
