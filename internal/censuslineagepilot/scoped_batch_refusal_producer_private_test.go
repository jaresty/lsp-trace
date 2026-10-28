//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	orchestration "lsp-trace/internal/acquisitionorchestration"
	pilot "lsp-trace/internal/censuslineagepilot"
	"testing"
)

// This is the actual reservation/authority/Manager producer with six genuinely
// supplied frozen fixture files. It is NOT a reproduction of the spent live
// request's 25 positions or a claim that its unrecorded wire DTO was identical.
func TestScopedProducerEndpointRefusalPrivate(t *testing.T) {
	for _, kind := range []string{"nomination", "context", "external", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
				f.ProducerEndpointPrivate(kind)
				req := batchFixtureRequest(t, f)
				selection, _ := mint(t, ctx, f, req)
				_, prepared := f.Traffic()
				out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, req, selection)
				events, overflow := f.DiagnosticsPrivate()
				messages := f.RequestsPrivate()
				t.Logf("case=%s failure=%v complete=%v requests=%d diagnostics=%+v overflow=%v", kind, failure, out.BoundedTraversalComplete, len(messages), events, overflow)
				if kind == "nomination" || kind == "context" {
					if failure != nil || !out.BoundedTraversalComplete {
						t.Fatal("frozen endpoint producer refused", failure)
					}
				} else {
					if failure == nil || len(out.RawV5) != 0 {
						t.Fatal("refusal produced retained graph")
					}
				}
				if kind == "context" {
					if len(messages) != 3 || messages[2].Method != "callHierarchy/incomingCalls" {
						t.Fatal("context query missing")
					}
					var p struct {
						Item struct {
							URI string `json:"uri"`
						} `json:"item"`
					}
					if json.Unmarshal(messages[2].Params, &p) != nil || p.Item.URI != f.ContextURI {
						t.Fatal("wrong context query URI")
					}
				}
				if kind == "external" || kind == "malformed" {
					if failure.Code != "OUTPUT_VALIDATION_FAILED" {
						t.Fatal("public failure code changed", failure.Code)
					}
				}
				if kind == "external" {
					found := false
					for _, event := range events {
						if event.Reason == "URI_OUTSIDE_FROZEN" && event.URIClass == "FROZEN_EXTERNAL" {
							found = true
						}
					}
					if !found {
						t.Fatal("missing external predicate witness")
					}
				}
				_, afterPrepared := f.Traffic()
				if prepared != afterPrepared {
					t.Fatal("Prepare after bind")
				}
				assertReleased(t, f, true)
				before, _ := f.Traffic()
				if _, failure = orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, req, selection); failure == nil {
					t.Fatal("spent selection reused")
				}
				after, _ := f.Traffic()
				if after != before {
					t.Fatal("spent retry wrote wire")
				}
			})
		})
	}
}
