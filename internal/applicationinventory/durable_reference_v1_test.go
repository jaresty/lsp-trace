//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"encoding/json"
	ms "lsp-trace/internal/adr0011methodsyntax"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInventoryReferencePublication(t *testing.T) { runConnectedPublication(t, true) }
func TestInventoryReferenceIncomplete(t *testing.T)  { runConnectedIncomplete(t, true) }
func checkOriginalReferenceInput(t *testing.T, task DurableTask, input []byte) {
	t.Helper()
	if !strings.Contains(task.Contract.Rules, "$.OriginalReferences") || !strings.Contains(task.Contract.Rules, "NOT_CAPTURED_BY_THIS_WITNESS") || strings.Contains(task.Contract.Rules, "$.Request.Envelope.Prompt") || task.Phase == "Describe" && !strings.Contains(task.Contract.Rules, "$.Request.envelope.prompt") {
		t.Fatal("REFERENCE_TASK_METADATA")
	}
	var ws []is.InventoryReferenceWitnessV1
	if task.Phase == "Describe" {
		w, e := is.ValidateInventoryReferenceEvidenceV1(input, task.RunID, task.RunID+"-"+map[int]string{1: "000000", 2: "000001"}[task.Sequence])
		if e != nil {
			t.Fatal("ORIGINAL_REFERENCE_INPUT", e)
		}
		ws = append(ws, w)
		var original map[string]json.RawMessage
		if json.Unmarshal(input, &original) != nil {
			t.Fatal("input")
		}
		for _, field := range []string{"method", "request", "source", "target", "digest", "missing", "cross-job", "old-schema", "truncated", "count", "overbound", "duplicate-target", "read-key", "occurrence", "silently-truncated"} {
			copy := map[string]json.RawMessage{}
			for k, v := range original {
				copy[k] = v
			}
			var bad is.InventoryReferenceWitnessV1
			wr, _ := json.Marshal(w)
			_ = json.Unmarshal(wr, &bad)
			switch field {
			case "silently-truncated":
				var items []json.RawMessage
				_ = json.Unmarshal(w.Pair.Result, &items)
				nine := make([]json.RawMessage, 9)
				for i := range nine {
					nine[i] = items[0]
				}
				bad.Pair.Result, _ = json.Marshal(nine)
				bad.Targets = make([]ms.Item, 8)
				for i := range bad.Targets {
					bad.Targets[i] = w.Targets[0]
					bad.Targets[i].Ordinal = i
				}
				bad.ReturnedCount = 8
			case "truncated":
				bad.Truncated = true
			case "count":
				bad.ReturnedCount++
			case "overbound":
				bad.Pair.Result = json.RawMessage(`["` + strings.Repeat("x", 5000) + `"]`)
			case "duplicate-target":
				bad.Targets = append(bad.Targets, bad.Targets[0])
				bad.ReturnedCount++
			case "read-key":
				bad.Pair.Read.Key.ID++
			case "occurrence":
				bad.Query.OccurrenceID = "sha256:" + strings.Repeat("f", 64)
			case "method":
				bad.Pair.Method = "textDocument/definition"
			case "request":
				bad.Pair.Key.ID++
			case "source":
				bad.Pair.Source.SHA256 = "sha256:" + strings.Repeat("0", 64)
			case "target":
				bad.Targets[0].URI += "-altered"
			case "digest":
				bad.Digest = "sha256:" + strings.Repeat("0", 64)
			case "cross-job":
				bad.RunID = strings.Repeat("1", 64)
			case "old-schema":
				copy["Schema"] = json.RawMessage(`"lsp-trace.inventory-evidence.v1"`)
			}
			if field != "digest" {
				bad.Digest = ""
				b, _ := json.Marshal(bad)
				bad.Digest = cat.NativeDigestV3(b)
			}
			copy["OriginalReferences"], _ = json.Marshal(bad)
			if field == "missing" {
				delete(copy, "OriginalReferences")
			}
			raw, _ := json.Marshal(copy)
			if _, e := is.ValidateInventoryReferenceEvidenceV1(raw, w.RunID, w.LeadID); e == nil {
				t.Fatal("ALTERED_REFERENCE_ACCEPTED", field)
			}
		}
	} else {
		var c cat.NativeCatalogWireV3
		if json.Unmarshal(input, &c) != nil || len(c.Evidence) != 2 {
			t.Fatal("catalog")
		}
		for _, e := range c.Evidence {
			var w is.InventoryReferenceWitnessV1
			if json.Unmarshal(e.OriginalReferences, &w) != nil || is.ValidateInventoryReferencesV1(w, task.RunID, w.LeadID, e.Unit, e.Request.RecordID, e.OriginalInputID) != nil {
				t.Fatal("CATALOG_ORIGINAL_REFERENCE_MISSING")
			}
			m := e.InventoryAdmission
			if m == nil || m.RunID != w.RunID || m.LeadID != w.LeadID || m.WitnessSHA256 != w.Digest || len(m.EvidenceSHA256) != 71 || len(m.AdmissionSHA256) != 71 {
				t.Fatal("CATALOG_ADMISSION_BINDING_MISSING")
			}
			ws = append(ws, w)
		}
	}
	for _, w := range ws {
		if len(w.Targets) != 1 || w.Null || w.Pair.Method != "textDocument/references" || w.Direction != "QUERY_TO_REPORTED_REFERENCE_LOCATION" || w.TargetSourceAvailability != "NOT_CAPTURED_BY_THIS_WITNESS" {
			t.Fatal("ORIGINAL_REFERENCE_SHAPE")
		}
		target := "/query-17.go"
		if strings.HasSuffix(w.Query.URI, "/query-17.go") {
			target = "/query-01.go"
		}
		if !strings.HasSuffix(w.Targets[0].URI, target) {
			t.Fatal("ORIGINAL_CROSS_BATCH_EDGE_MISSING")
		}
		t.Logf("REFERENCE_WITNESS phase=%s digest=%s method=%s request=%d session=%s query=%s target=%s range=%+v source=%s", task.Phase, w.Digest, w.Pair.Method, w.Pair.Key.ID, w.Pair.SessionID, w.Query.URI, w.Targets[0].URI, w.Targets[0].Range, w.TargetSourceAvailability)
	}
}
func checkReferenceResult(t *testing.T, r Report) {
	t.Helper()
	headers, targets := 0, 0
	uris := map[string]bool{}
	for _, g := range r.UntouchedGaps {
		if strings.HasPrefix(g, "ORIGINAL_REFERENCES_ADMISSION:") {
			headers++
			if !strings.Contains(g, "METHOD:textDocument/references;") || !strings.Contains(g, "TARGET_SOURCE:NOT_CAPTURED_BY_THIS_WITNESS;") {
				t.Fatal("REFERENCE_HEADER_MISSING")
			}
		}
		if strings.HasPrefix(g, "ORIGINAL_REFERENCE_TARGET;") {
			targets++
			parts := strings.SplitN(g, ";LOCATION:", 2)
			var item ms.Item
			if len(parts) != 2 || json.Unmarshal([]byte(parts[1]), &item) != nil || !strings.Contains(parts[0], "DIRECTION:QUERY_TO_REPORTED_REFERENCE_LOCATION") || item.Ordinal != 0 || item.Kind != ms.Location || item.Range.Start != (ms.Position{Line: 2, Character: 12}) || item.Range.End != (ms.Position{Line: 2, Character: 19}) {
				t.Fatal("REFERENCE_PUBLICATION_TARGET_CHANGED")
			}
			uris[filepath.Base(item.URI)] = true
		}
	}
	if headers != 2 || targets != 2 || !uris["query-01.go"] || !uris["query-17.go"] {
		t.Fatal("REFERENCE_PROVENANCE_MISSING", headers, targets, uris)
	}
	raw, _ := json.Marshal(r)
	t.Log("REFERENCE_RESULT " + string(raw))
}

func TestInventoryReferenceRefusedShapes(t *testing.T) {
	for _, shape := range []string{"duplicate", "overflow", "empty", "null"} {
		t.Run(shape, func(t *testing.T) {
			base, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			dir := filepath.Join(base, "store")
			for _, p := range []string{dir, filepath.Join(base, "source")} {
				if e = os.Mkdir(p, 0700); e != nil {
					t.Fatal(e)
				}
			}
			result, log := runResumeProcess(t, dir, "", "references-"+shape+"-first")
			if !strings.Contains(log, "REFERENCE_SHAPE_REFUSED "+shape) || result.Status.Admissions != 0 || result.Status.Status != "PARTIAL_UNCERTAIN" || result.Symbols != 16 || result.Starts != 1 {
				t.Fatal("SHAPE_REFUSAL", result)
			}
			files, _ := json.Marshal(retentionPreflightFiles(t, dir))
			t.Log("CONNECTED_FILES_JSON " + string(files))
		})
	}
}

func TestInventoryReferenceQuote(t *testing.T) {
	n, p := connectedCandidateQuote(t, cat.InventoryConnectedEvidenceV1, cat.InventoryConnectedPayloadV1, cat.InventoryConnectedStateV1, cat.InventoryReferenceManifestV1)
	if p != 19 || n != cat.InventoryConnectedDeliveryBytesV1+cat.InventoryReferenceManifestExtraV1 {
		t.Fatal("whole-object quote")
	}
	plan := durableCompletionPlan{Stages: make([]uint64, 5)}
	h := durableInventoryHead{Schema: is.InventoryReferenceHeadVersionV1}
	h.referenceCompletion(&plan)
	var extra uint64
	for _, v := range plan.Stages {
		extra += v
	}
	if extra+cat.InventoryReferenceManifestExtraV1 != cat.InventoryReferenceSemanticExtraV1 || cat.InventoryReferenceLimitV1 != cat.InventoryReferenceAcquisitionV1+cat.InventoryReferenceSealV1+n+extra {
		t.Fatal("REFERENCE_QUOTE")
	}
	b, e := cat.NewInventoryReferenceBudgetV1(time.Now().Add(time.Second), cat.InventoryReferenceAdmissionV1, cat.InventoryResumeLimitV1, 0)
	if e != nil {
		t.Fatal(e)
	}
	used, _ := b.Usage()
	if used != cat.InventoryResumeLimitV1 {
		t.Fatal("PRIOR_SPEND_RESET")
	}
	t.Logf("REFERENCE_PROFILE retention=%d acquisition=%d seal=%d semantic=%d H=%d packets=28", cat.InventoryReferenceRetentionV1, cat.InventoryReferenceAcquisitionV1, cat.InventoryReferenceSealV1, n+extra, cat.InventoryReferenceLimitV1)
}
