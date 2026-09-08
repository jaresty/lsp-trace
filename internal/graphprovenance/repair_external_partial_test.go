package graphprovenance

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/acquisition"
	"strings"
	"testing"
)

func TestRepairExternalIncompleteSupplySelector(t *testing.T) {
	base, root := coordinatorV2Fixture(t, nil)
	req := base.Request
	req.RequiredTargets = nil
	req.Limits.MaxResponseBytes = 10
	wire := acquisition.NewWireClient(func(context.Context, acquisition.WireRequest) (json.RawMessage, error) {
		return json.RawMessage(`[]`), nil
	})
	client := acquisition.WithDocumentSupply(wire, func(_ context.Context, _ acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
		return acquisition.Supply{URI: l.URI, LanguageID: strings.Repeat("x", 100)}, nil
	})
	r, err := acquisition.Acquire(context.Background(), client, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Requests) != 1 || r.Requests[0].Outcome != "CAPTURE_INCOMPLETE" || len(r.Requests[0].Params) == 0 {
		t.Fatalf("fixture %+v", r.Requests)
	}
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	for _, b := range e.Bindings {
		if b.Pointer == "/acquisition/requests/0/params" {
			if b.Attribution != "SOURCE" || b.AnchorStatus != "VALID_COORDINATES" {
				t.Fatal(b)
			}
			return
		}
	}
	t.Fatalf("retained complete position params lack tuple binding: outcome=%s params=%s", r.Requests[0].Outcome, r.Requests[0].Params)
}
