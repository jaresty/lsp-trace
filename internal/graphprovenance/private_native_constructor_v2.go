package graphprovenance

import (
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/acquisition"
)

var ErrPrivateNativeOrdinaryCaptureV2 = errors.New("private native constructor ordinary source coverage refused")

// This helper performs real ordinary post-traversal IO, once PER VIEW. Existing
// CaptureV2 caps remain 64 files, 1MiB/file, 4MiB/pass. Two views cost at most
// 128 read attempts and 8MiB charged source bytes, plus two workspace opens.
// CaptureBudgetV2 is historical declared accounting, not authenticated FS events.
func CaptureNativeConstructorSourcePrivateV2(ctx context.Context, r acquisition.Result, workspace string) (EvidenceV2, error) {
	// Restrict the constructor before ordinary marshaling/census/copying.
	if len(r.Graph.Nodes) > 128 || len(r.Graph.Edges) > 128 || len(r.Requests) > 3 || r.Usage.EvidenceBytes > 1<<20 {
		return EvidenceV2{}, ErrPrivateNativeOrdinaryCaptureV2
	}
	raw, err := CaptureV2(ctx, r, workspace)
	if err != nil {
		return EvidenceV2{}, err
	}
	var e EvidenceV2
	if err = json.Unmarshal(raw, &e); err != nil {
		return EvidenceV2{}, err
	}
	if err = ValidateNativeConstructorSourcePrivateV2(e); err != nil {
		return EvidenceV2{}, err
	}
	return e, nil
}

// Validation only: no IO, receipt synthesis, budget rewrite or private-profile
// relabeling. Missing captures retain the existing ordinary typed branch.
func ValidateNativeConstructorSourcePrivateV2(e EvidenceV2) error {
	if err := ValidateV2(e); err != nil {
		return err
	}
	for _, c := range e.Captures {
		if c.Status != "READABLE" {
			return ErrPrivateNativeOrdinaryCaptureV2
		}
	}
	return nil
}
