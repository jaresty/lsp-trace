package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
)

func TestV2CarrierSupplyParameterStates(t *testing.T) {
	for _, state := range []string{"SUCCESS", "FAILED", "CAPTURE_FAILED", "CAPTURE_INCOMPLETE", "BUDGET_BLOCKED"} {
		t.Run(state, func(t *testing.T) {
			base, root := coordinatorV2Fixture(t, nil)
			req := base.Request
			req.RequiredTargets = nil
			if state == "CAPTURE_INCOMPLETE" {
				req.Limits.MaxResponseBytes = 10
			}
			if state == "BUDGET_BLOCKED" {
				req.Limits.MaxEvidenceBytes = 0
			}
			wire := acquisition.NewWireClient(func(context.Context, acquisition.WireRequest) (json.RawMessage, error) {
				return json.RawMessage(`[]`), nil
			})
			client := acquisition.WithDocumentSupply(wire, func(_ context.Context, _ acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
				s := acquisition.Supply{URI: l.URI, LanguageID: strings.Repeat("x", 100)}
				if state == "FAILED" {
					return s, fmt.Errorf("supply failed")
				}
				if state == "CAPTURE_FAILED" {
					s.Observation = json.RawMessage(`{`)
				}
				return s, nil
			})
			r, err := acquisition.Acquire(context.Background(), client, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Requests) == 0 || r.Requests[0].Outcome != state {
				t.Fatalf("fixture: %+v", r.Requests)
			}
			retained := state != "BUDGET_BLOCKED"
			if (len(r.Requests[0].Params) > 0) != retained {
				t.Fatal("fixture parameter retention")
			}
			raw, err := CaptureV2(context.Background(), r, root)
			if err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			if err = json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			found := -1
			for i, b := range e.Bindings {
				if b.Pointer == "/acquisition/requests/0/params" {
					found = i
					if b.AnchorStatus != "VALID_COORDINATES" || b.Attribution != "SOURCE" || b.URI != req.Root.Locator.URI {
						t.Fatal("parameter tuple", b)
					}
				}
			}
			if (found >= 0) != retained {
				t.Fatalf("actual-parameter census: retained=%v found=%d", retained, found)
			}
			if found >= 0 {
				altered := e
				altered.Bindings = append(append([]BindingV2{}, e.Bindings[:found]...), e.Bindings[found+1:]...)
				if err = ValidateV2(altered); err == nil {
					t.Fatal("missing parameter tuple admitted")
				}
			}
			if err = os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if _, err = ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal("source-free admission", err)
			}
			// Invalid retained parameter encodings must never synthesize a tuple. Native
			// acquisition admission may reject before census, which is also fail-closed.
			for _, bad := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"line":"bad"}`), json.RawMessage(`{`)} {
				copy := r
				copy.Requests = append([]acquisition.RequestRecord{}, r.Requests...)
				copy.Requests[0].Params = bad
				bs, err := CensusV2(copy)
				if err != nil {
					continue
				}
				for _, b := range bs {
					if b.Pointer == "/acquisition/requests/0/params" {
						t.Fatal("synthetic missing/unparseable parameter tuple", b)
					}
				}
			}
		})
	}
}
