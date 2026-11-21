//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

// Historical whole-object RED is retained in input-delivery-probe-20261119-18d.
// This unchanged legal semantic payload now demands the full usable trajectory.
func TestCensusLargeRequiredInputDeliveryCharacterization(t *testing.T) {
	if runCensusLargeRequiredInputDeliveryProbe(t) {
		t.Fatal("ASSERT_REQUIRED_INPUT_TRAJECTORY: FAIL; required input delivery denied")
	}
}

type deliveryProbeHead struct {
	Schema, Phase, Status, InputPrefix string
	Sequence, CompletionCursor         int
	Used, Work, Packets, Limit         uint64
	SemanticNS, Deadline, RunDeadline  int64
	Input, State                       publication.StoredObject
	Delivery                           struct {
		Index publication.StoredObject
		Next  int
	}
	DeliverySpent                      uint64
	SuccessorReserve, SuccessorPackets uint64
	Completion                         struct {
		Version                     string
		Stages                      []uint64
		StateUpper, PreparationUsed uint64
	}
}

func deliveryProbeSnapshot(t *testing.T, p *contractProcess, run string) ([]byte, deliveryProbeHead) {
	t.Helper()
	raw := p.head(run)
	var env struct {
		Digest  string
		Content json.RawMessage
	}
	var h deliveryProbeHead
	if json.Unmarshal(raw, &env) != nil || cat.NativeDigestV3(env.Content) != env.Digest || json.Unmarshal(env.Content, &h) != nil {
		t.Fatal("invalid authoritative head snapshot")
	}
	if h.Limit != 64<<20 || h.Work != 32*h.Used || h.Work > 1<<32 || h.Packets > 64 || h.SemanticNS > int64(120*time.Second) || h.CompletionCursor > len(h.Completion.Stages) {
		t.Fatal("accounting/cap mismatch", h)
	}
	protected := deliveryProbeProtected(h)
	if h.Used+protected > h.Limit {
		t.Fatal("protected capacity exceeded")
	}
	t.Logf("DELIVERY_HEAD phase=%s seq=%d status=%s input=%d prep=%d used=%d work=%d packets=%d cursor=%d protected=%d protectedPackets=%d headroom=%d semanticNS=%d stages=%v", h.Phase, h.Sequence, h.Status, h.Input.Bytes, h.Completion.PreparationUsed, h.Used, h.Work, h.Packets, h.CompletionCursor, protected, deliveryProbePackets(h), h.Limit-h.Used-protected, h.SemanticNS, h.Completion.Stages)
	return raw, h
}
func deliveryProbeProtected(h deliveryProbeHead) uint64 {
	var n uint64
	for _, q := range h.Completion.Stages[h.CompletionCursor:] {
		n += q
	}
	if h.Delivery.Index.Valid() {
		for at := h.Delivery.Next; at < h.Input.Bytes; at += 32768 {
			n += deliveryProbeQuote(h, at, min(32768, h.Input.Bytes-at))
		}
	}
	return n + h.SuccessorReserve
}
func deliveryProbePackets(h deliveryProbeHead) uint64 {
	n := uint64(len(h.Completion.Stages)-h.CompletionCursor) + h.SuccessorPackets
	if h.Delivery.Index.Valid() {
		n += uint64((h.Input.Bytes - h.Delivery.Next + 32767) / 32768)
	}
	return n
}
func deliveryProbeQuote(h deliveryProbeHead, offset, count int) uint64 {
	if !h.Delivery.Index.Valid() {
		return 6*uint64(h.Input.Bytes) + 2*uint64(count) + 16384
	}
	first, last := offset/32768, (offset+count-1)/32768
	verified := min(h.Input.Bytes, (last+1)*32768) - first*32768
	return 8*uint64(h.Delivery.Index.Bytes) + 6*uint64(verified) + 2*uint64(count) + 16384
}

// These direct reads are independent test instrumentation only. Their bytes
// must never become an answer or replace any returned caller input read.
func deliveryProbeObject(t *testing.T, p *contractProcess, prefix string, ref publication.StoredObject) []byte {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(p.home, "Library", "Application Support", "lsp-trace", "census", "durable", prefix+ref.ID+".json"))
	if e != nil || len(raw) != ref.Bytes || cat.NativeDigestV3(raw) != "sha256:"+ref.ID {
		t.Fatal("observer object integrity", e)
	}
	return raw
}

func runCensusLargeRequiredInputDeliveryProbe(t *testing.T) bool {
	t.Helper()
	// Unchanged child checks exactly three genuine M/P/S completions and original
	// method/params/result identities once, then exits and removes its source.
	p, task := newContractProcess(t, "three-item")
	return runCensusLargeRequiredInputDeliveryForProcess(t, p, task, false)
}

func runCensusLargeRequiredInputDeliveryForProcess(t *testing.T, p *contractProcess, task ai.DurableTask, scale bool) bool {
	t.Helper()
	_, initial := deliveryProbeSnapshot(t, p, task.RunID)
	if initial.Schema != "lsp-trace.durable-inventory-head.v2" || initial.Completion.Version != "lsp-trace.inventory-completion-quote.v2" {
		t.Fatal("fresh run did not emit explicit v2 formats")
	}
	if len(initial.Completion.Stages) != 6 {
		t.Fatal("three-item completion denominator lost")
	}
	t.Log("ASSERT_ORIGINAL_THREE_MPS_ONCE: PASS (unchanged acquisition child)")
	var initialState struct {
		Prepared []publication.StoredObject
		Report   ai.Report
	}
	if json.Unmarshal(deliveryProbeObject(t, p, "inventory-state-", initial.State), &initialState) != nil || len(initialState.Prepared) != 3 || len(initialState.Report.Census) != 3 || len(initialState.Report.Dispositions) != 3 {
		t.Fatal("initial denominator")
	}
	for _, r := range initialState.Prepared {
		if !r.Valid() {
			t.Fatal("silent unprepared subset")
		}
	}
	described := 0
	var groupTask, groupRevision string
	var readSpent uint64
	for task.Status != "PUBLISHED" {
		_, h := deliveryProbeSnapshot(t, p, task.RunID)
		if h.Input.Bytes != task.Artifacts[0].Bytes {
			t.Fatal("returned object length mismatch")
		}
		total := h.Input.Bytes
		pages := (total + 32767) / 32768
		var onePass uint64
		for at := 0; at < total; at += 32768 {
			onePass += deliveryProbeQuote(h, at, min(32768, total-at))
		}
		t.Logf("DELIVERY_OBJECT phase=%s seq=%d L=%d pages=%d indexBytes=%d onePassQuote=%d", task.Phase, task.Sequence, total, pages, h.Delivery.Index.Bytes, onePass)
		var input []byte
		for offset := 0; offset < total; {
			beforeRaw, before := deliveryProbeSnapshot(t, p, task.RunID)
			count := min(32768, total-offset)
			quote := deliveryProbeQuote(before, offset, count)
			headroom := before.Limit - before.Used - deliveryProbeProtected(before)
			if before.Delivery.Index.Valid() && before.Delivery.Next == offset {
				headroom += quote
			}
			argv := append([]string(nil), task.Contract.Operations["read"]...)
			argv[7] = fmtInt(offset)
			t.Logf("DELIVERY_READ_BEFORE phase=%s seq=%d offset=%d count=%d quote=%d headroom=%d priorReadSpent=%d", task.Phase, task.Sequence, offset, count, quote, headroom, readSpent)
			if quote > headroom {
				// Invoke the real CLI once and require an actual refusal, not merely the
				// arithmetic prediction. Never retry, change caps or continue this run.
				diagnostic := p.call("reject-detail", nil, argv)
				afterRaw, after := deliveryProbeSnapshot(t, p, task.RunID)
				if !bytes.Equal(beforeRaw, afterRaw) || !reflect.DeepEqual(before, after) {
					t.Fatal("ASSERT_DENIED_DELIVERY_UNCHANGED_HEAD: FAIL")
				}
				if time.Now().UnixNano() >= min(before.Deadline, before.RunDeadline) {
					t.Fatal("denial confounded by expired deadline")
				}
				if before.Used != initial.Used+readSpent+sumDeliveryStages(before.Completion.Stages[:before.CompletionCursor]) {
					t.Fatal("spent operations refunded or unexplained charge")
				}
				t.Logf("FIRST_DENIAL phase=%s seq=%d described=%d offset=%d delivered=%d total=%d quote=%d headroom=%d deficit=%d diagnostic=%s", task.Phase, task.Sequence, described, offset, len(input), total, quote, headroom, quote-headroom, diagnostic)
				t.Log("ASSERT_DENIED_DELIVERY_UNCHANGED_HEAD: PASS; prior spending retained; no retry")
				return true
			}
			raw := p.call("call", nil, argv)
			var page sc.DurableDescribePageV1
			if json.Unmarshal(raw, &page) != nil || page.RunID != task.RunID || page.TaskID != task.TaskID || page.ArtifactID != "input" || page.Offset != offset || page.Total != total || len(page.Data) != count {
				t.Fatal("returned page mismatch")
			}
			_, after := deliveryProbeSnapshot(t, p, task.RunID)
			if after.Used != before.Used+quote || after.Work != before.Work+32*quote || after.Packets != before.Packets+1 || after.CompletionCursor != before.CompletionCursor || after.SemanticNS != before.SemanticNS || after.Deadline != before.Deadline || after.RunDeadline != before.RunDeadline || !reflect.DeepEqual(after.Completion, before.Completion) {
				t.Fatal("ASSERT_EXACT_DELIVERY_CHARGE: FAIL")
			}
			readSpent += quote
			t.Logf("ASSERT_EXACT_DELIVERY_CHARGE: PASS pageWireBytes=%d decodedPageBytes=%d", len(raw), len(page.Data))
			input = append(input, page.Data...)
			offset += count
		}
		if len(input) != h.Input.Bytes || cat.NativeDigestV3(input) != "sha256:"+h.Input.ID {
			t.Fatal("ASSERT_EXACT_RECONSTRUCTION: FAIL")
		}
		t.Log("ASSERT_EXACT_RECONSTRUCTION: PASS")
		var answer []byte
		var retained struct {
			RunID, LeadID string
			Unit, Request json.RawMessage
			Host          dw.ResponseHostV2
			Accounting    struct{ Windows uint64 }
		}
		switch task.Phase {
		case "Describe":
			if json.Unmarshal(input, &retained) != nil || retained.RunID != task.RunID || retained.Accounting.Windows != 3 {
				t.Fatal("genuine retained input missing")
			}
			var unit struct{ Workspace, Body string }
			if json.Unmarshal(retained.Unit, &unit) != nil || unit.Body == "" {
				t.Fatal("source body absent")
			}
			if _, e := os.Stat(unit.Workspace); !os.IsNotExist(e) {
				t.Fatal("source still present", e)
			}
			candidate, e := dw.ParseSuppliedTerminalResponse([]byte(task.Contract.Example), dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768})
			if e != nil {
				t.Fatal("original example grammar", e)
			}
			semantic := candidate.Semantic()
			if semantic.Verdict != "COMPLETE" || semantic.ConsumerNeed.Status != "UNRESOLVED" || semantic.ConsumerNeed.Value != "" || semantic.ProvidedBehavior.ConsumerRelative {
				t.Fatal("source-only example admission changed")
			}
			semantic.TargetRole = strings.Repeat("r", 4096)
			semantic.ProvidedBehavior.Value = strings.Repeat("b", 4096)
			semantic.BoundaryContribution = strings.Repeat("c", 4096)
			semantic.Limitations = []string{strings.Repeat("l", 4096), strings.Repeat("m", 4096), strings.Repeat("n", 4096), strings.Repeat("o", 4096)}
			semanticRaw, e := json.Marshal(semantic)
			if e != nil {
				t.Fatal(e)
			}
			answer = dw.EncodeAdapterResultV2(string(semanticRaw))
			parsed, e := dw.ParseSuppliedTerminalResponse(answer, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768})
			if e != nil || !reflect.DeepEqual(parsed.Semantic(), semantic) {
				t.Fatal("large fixture strict parser", e)
			}
			t.Logf("ASSERT_LARGE_LEGAL_RESPONSE: PASS contentBytes=28672 semanticBytes=%d adapterBytes=%d sourceAbsent=true", len(semanticRaw), len(answer))
		case "GroupDecision", "Reviewer":
			var catalog cat.NativeCatalogWireV3
			if cat.DecodeNativeV3(input, &catalog) != nil || len(catalog.Evidence) != 3 {
				t.Fatal("catalog denominator")
			}
			d := cat.NativeDecisionV3{Predecessor: catalog.Revision, Actor: task.Actor, Reason: "controlled provisional judgment", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "checkout", Label: "checkout proposal", Reason: "first evidence only; remaining membership unresolved", Members: []string{catalog.Evidence[0].ID}}}}
			if task.Phase == "GroupDecision" {
				groupTask, groupRevision = task.TaskID, catalog.Revision
			}
			if task.Phase == "Reviewer" {
				if task.Role != "reviewer" || task.TaskID == groupTask || task.PreviousTaskID != groupTask || catalog.Revision == groupRevision || len(catalog.Groups) != 1 || catalog.Groups[0].Label != "checkout proposal" {
					t.Fatal("ASSERT_DISTINCT_SEQUENTIAL_REVIEW: FAIL")
				}
				t.Log("ASSERT_DISTINCT_SEQUENTIAL_REVIEW: PASS")
				d.Affected = []string{"checkout"}
				d.Groups[0].Label = "reviewed checkout proposal"
			}
			answer, _ = json.Marshal(d)
		default:
			t.Fatal("unexpected phase", task.Phase)
		}
		previous := task
		_, beforeSubmit := deliveryProbeSnapshot(t, p, task.RunID)
		submitStarted := time.Now()
		var raw []byte
		if scale {
			wire := p.call("probe-detail", answer, task.Contract.Operations["submit"])
			var response struct {
				Code       int
				Output     json.RawMessage
				Diagnostic string
			}
			if json.Unmarshal(wire, &response) != nil {
				t.Fatal("submit observation")
			}
			if response.Code != 0 {
				_, after := deliveryProbeSnapshot(t, p, task.RunID)
				base := beforeSubmit.Completion.Stages[beforeSubmit.CompletionCursor]
				if after.Status != "CLAIMED" || after.Used != beforeSubmit.Used+base || after.Work != beforeSubmit.Work+32*base || after.Packets != beforeSubmit.Packets+1 || after.CompletionCursor != beforeSubmit.CompletionCursor+1 || after.State != beforeSubmit.State || after.Input != beforeSubmit.Input || after.Sequence != beforeSubmit.Sequence || !reflect.DeepEqual(after.Completion, beforeSubmit.Completion) || time.Now().UnixNano() >= beforeSubmit.Deadline {
					t.Fatal("unexpected refusal: not an unexpired claim-only admission boundary", response.Diagnostic)
				}
				t.Logf("SCALE_STAGED_REFUSAL phase=%s seq=%d baseSpent=%d elapsed=%s diagnostic=%s", task.Phase, task.Sequence, base, time.Since(submitStarted), response.Diagnostic)
				return true
			}
			raw = response.Output
		} else {
			raw = p.call("call", answer, task.Contract.Operations["submit"])
		}
		t.Logf("DELIVERY_SUBMIT_TIMING phase=%s seq=%d elapsed=%s", task.Phase, task.Sequence, time.Since(submitStarted))
		if json.Unmarshal(raw, &task) != nil || task.PreviousTaskID != previous.TaskID || task.Sequence != previous.Sequence+1 || task.RunID != previous.RunID {
			t.Fatal("consumer did not advance", string(raw))
		}
		_, next := deliveryProbeSnapshot(t, p, task.RunID)
		stage := next.Completion.Stages[beforeSubmit.CompletionCursor]
		if next.Used != beforeSubmit.Used+stage || next.CompletionCursor != beforeSubmit.CompletionCursor+1 || !reflect.DeepEqual(next.Completion.Stages[:beforeSubmit.CompletionCursor], beforeSubmit.Completion.Stages[:beforeSubmit.CompletionCursor]) {
			t.Fatal("ASSERT_SUBMIT_CONSERVATION: FAIL")
		}
		t.Logf("ASSERT_SUBMIT_CONSERVATION: PASS semanticSlice=%d successorBuildSlice=%d total=%d", beforeSubmit.Completion.Stages[beforeSubmit.CompletionCursor], stage-beforeSubmit.Completion.Stages[beforeSubmit.CompletionCursor], stage)
		if previous.Phase == "Describe" {
			described++
			var state struct {
				Descriptions, Prepared []publication.StoredObject
				Report                 ai.Report
			}
			if json.Unmarshal(deliveryProbeObject(t, p, "inventory-state-", next.State), &state) != nil || len(state.Descriptions) != described || len(state.Prepared) != 3 || len(state.Report.Census) != 3 || len(state.Report.Dispositions) != 3 {
				t.Fatal("consumer denominator changed")
			}
			description := deliveryProbeObject(t, p, "inventory-description-", state.Descriptions[described-1])
			var evidence struct {
				Unit, Request json.RawMessage
				Record        json.RawMessage
			}
			if json.Unmarshal(description, &evidence) != nil || !bytes.Equal(evidence.Unit, retained.Unit) || !bytes.Equal(evidence.Request, retained.Request) {
				t.Fatal("source/query request pins changed")
			}
			record, e := dw.ParseResponseRecordV2(evidence.Record)
			if e != nil || record.Host() != retained.Host {
				t.Fatal("host pins changed", e)
			}
			t.Logf("ASSERT_ADMITTED_DESCRIPTION_PINS: PASS index=%d descriptionBytes=%d recordBytes=%d retainedInputBytes=%d", described, len(description), len(evidence.Record), len(input))
		}
	}
	raw := p.call("call", nil, task.Contract.Operations["result"])
	var report ai.Report
	if json.Unmarshal(raw, &report) != nil || !report.Published || report.SuccessfulDescriptions != 3 || report.WithMembership != 1 || report.WithoutMembership != 2 || len(report.Census) != 3 || len(report.Dispositions) != 3 || report.Authority != 0 || report.Accepted || report.Completeness != "UNKNOWN" {
		t.Fatal("publication denominator/custody")
	}
	deliveryProbeSnapshot(t, p, task.RunID)
	t.Log("ASSERT_REQUIRED_INPUT_TRAJECTORY: PASS")
	return false
}
func sumDeliveryStages(stages []uint64) uint64 {
	var n uint64
	for _, q := range stages {
		n += q
	}
	return n
}
