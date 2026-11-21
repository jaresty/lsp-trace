//go:build darwin || linux

package sourcecontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"strings"
	"testing"
	"time"
)

func quoteTestHost(s string) dw.ResponseHostV2 {
	digest := "sha256:" + strings.Repeat("a", 64)
	return dw.ResponseHostV2{RequestRecordID: s, MessageID: s, AttemptID: s, Consumer: dw.ConsumerIdentityV2{Resolution: "UNRESOLVED", AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: dw.HostPinsV2{WorkerSHA256: digest, ModelSHA256: digest, GrammarSHA256: digest, PromptSHA256: digest}, Provenance: dw.HostProvenanceV2{PacketID: s, RequestLineageIdentity: s}, Custody: dw.HostCustodyV2{GraphDigest: digest, CaptureID: s}}
}

// Encoding-only upper-bound checks do not fabricate native grants or qualify
// an application. Actual genuine-owner qualification is the process fixture.
func TestInventoryCompletionOwnedEncodingBounds(t *testing.T) {
	handles := []string{"C1", "C2", "PACKET_SCOPE", "UNRESOLVED_CUSTODY"}
	for _, s := range []string{"x", strings.Repeat("a", 4096), strings.Repeat("<", 4096), strings.Repeat("\x00\\\"\u2028", 256), strings.Repeat("雪", 1024)} {
		t.Run(string([]rune(s)[:1]), func(t *testing.T) {
			sem := dw.SemanticResponseV2{Verdict: "COMPLETE", TargetRole: s, ConsumerNeed: dw.ConsumerNeedV2{Status: "UNRESOLVED"}, ProvidedBehavior: dw.ProvidedBehaviorV2{Value: s}, BoundaryContribution: "boundary", Limitations: []string{"a", "b", "c", "d", "e", "f", "g", "h"}, CitationSuggestions: &dw.CitationSuggestionsV2{TargetRole: handles, ConsumerNeed: handles, ProvidedBehavior: handles, BoundaryContribution: handles, Limitations: handles}}
			var text bytes.Buffer
			enc := json.NewEncoder(&text)
			enc.SetEscapeHTML(false)
			if e := enc.Encode(sem); e != nil {
				t.Fatal(e)
			}
			adapter := dw.EncodeAdapterResultV2(strings.TrimSpace(text.String()))
			candidate, e := dw.ParseSuppliedTerminalResponse(adapter, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768})
			if e != nil {
				t.Fatal("supported boundary rejected", len(adapter), e)
			}
			h := quoteTestHost(strings.Repeat("<\\\"\u2028\xff", 64))
			b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
			upper, e := inventoryRecordUpper(context.Background(), h, len(adapter), b)
			if e != nil {
				t.Fatal(e)
			}
			record, e := dw.NewResponseRecordV2(candidate.CanonicalSemantic(), h)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := record.Bytes()
			if e != nil {
				t.Fatal(e)
			}
			if uint64(len(raw)) > upper {
				t.Fatalf("response record underquote: actual=%d quote=%d", len(raw), upper)
			}
			hostJSON, _ := json.Marshal(h)
			if len(raw) > len(candidate.CanonicalSemantic())+len(hostJSON)+2048 {
				t.Fatal("closed response overhead exceeds2048")
			}
			for _, bodySize := range []int{0, 32767, 32768, 32769, 1 << 20} {
				p := retainedInventoryEvidence{Schema: "lsp-trace.inventory-evidence.v1", RunID: strings.Repeat("a", 64), LeadID: strings.Repeat("a", 71), Handoff: strings.Repeat("a", 71), Checkpoint: strings.Repeat("a", 71), Interpreter: "main", Host: h}
				p.Unit.Body = strings.Repeat("<", bodySize)
				p.Request.Envelope.Prompt = strings.Repeat("\\\"\u2028", 100)
				p.Selected.Intent.InputID = "sha256:" + strings.Repeat("a", 64)
				// Retained proof/accounting and long escaped metadata are not discarded.
				p.Accounting.URI = strings.Repeat("<", 4096)
				p.Accounting.Head = strings.Repeat("a", 71)
				e := SelectedEvidenceV3{ID: record.ID(), HandoffID: p.Handoff, CheckpointID: p.Checkpoint, OriginalInputID: p.Selected.Intent.InputID, Interpreter: p.Interpreter, Origin: dw.SuppliedTerminalResponseOnly, Execution: dw.NoProcessOrHostExecutionVerified, Selected: cat.NativeSelectedRowV3{RowPosition: p.Selected.RowPosition, SlotOrdinal: p.Selected.Slot.Ordinal, Mode: p.Selected.Slot.Mode, RunID: p.Selected.Slot.RunID, SeedNamespace: p.Selected.Slot.SeedNamespace, Seed: p.Selected.Slot.Seed, Intent: p.Selected.Intent}, Unit: p.Unit, Request: p.Request, Record: raw, RawAdapterDigest: "sha256:" + strings.Repeat("a", 64)}
				input, _ := json.Marshal(p)
				output, _ := json.Marshal(e)
				if len(output) > len(input)+len(adapter)+2048 {
					t.Fatalf("description underquote body=%d input=%d output=%d", bodySize, len(input), len(output))
				}
				payload := uint64(len(p.Unit.Body)+len(p.Request.Envelope.Prompt)) + upper
				if uint64(len(e.Unit.Body)+len(e.Request.Envelope.Prompt)+len(e.Record)) > payload {
					t.Fatal("seal operand underquote")
				}
			}
			t.Logf("adapter=%d canonical=%d record=%d quote=%d host=%d closed_overhead=%d", len(adapter), len(candidate.CanonicalSemantic()), len(raw), upper, len(hostJSON), len(raw)-len(candidate.CanonicalSemantic())-len(hostJSON))
		})
	}
}

func TestInventoryCompletionCanonicalDefaults(t *testing.T) {
	raw := `{"verdict":"COMPLETE","target_role":"x","consumer_need":{"status":"UNRESOLVED"},"provided_behavior":{"value":"x"},"boundary_contribution":"x","limitations":[],"citation_suggestions":{}}`
	adapter := dw.EncodeAdapterResultV2(raw)
	candidate, e := dw.ParseSuppliedTerminalResponse(adapter, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768})
	if e != nil {
		t.Fatal(e)
	}
	host := quoteTestHost("x")
	b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	upper, e := inventoryRecordUpper(context.Background(), host, len(adapter), b)
	if e != nil {
		t.Fatal(e)
	}
	record, e := dw.NewResponseRecordV2(candidate.CanonicalSemantic(), host)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := record.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	if uint64(len(encoded)) > upper {
		t.Fatal("canonical defaults underquoted", len(encoded), upper)
	}
	t.Logf("omitted nested fields: adapter=%d canonical=%d record=%d upper=%d", len(adapter), len(candidate.CanonicalSemantic()), len(encoded), upper)
}

func TestInventoryCompletionInvalidUTF8HostBound(t *testing.T) {
	raw := `{"verdict":"COMPLETE","target_role":"x","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"x","consumer_relative":false},"boundary_contribution":"x","limitations":[]}`
	adapter := dw.EncodeAdapterResultV2(raw)
	h := quoteTestHost(strings.Repeat("\xff", 4096))
	b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	upper, e := inventoryRecordUpper(context.Background(), h, len(adapter), b)
	if e != nil {
		t.Fatal(e)
	}
	record, e := dw.NewResponseRecordV2([]byte(raw), h)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := record.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	if uint64(len(encoded)) > upper {
		t.Fatalf("invalid UTF-8 host underquote: actual=%d quote=%d", len(encoded), upper)
	}
}

func TestInventoryCompletionQuoteChargesBeforeScan(t *testing.T) {
	b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 1)
	if _, e := inventoryRecordUpper(context.Background(), quoteTestHost(strings.Repeat("x", 4096)), 65536, b); e == nil {
		t.Fatal("unfunded quote accepted")
	}
	used, _ := b.Usage()
	if used != 0 {
		t.Fatal("failed quote charged partial bytes", used)
	}
}
