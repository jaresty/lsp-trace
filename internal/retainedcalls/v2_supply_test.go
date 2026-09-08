package retainedcalls

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

type supplyClientV2 struct {
	acquisition.Client
	versions map[string]int
}

func (c *supplyClientV2) PrepareDocument(_ context.Context, ctx acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
	c.versions[l.URI]++
	version := c.versions[l.URI]
	content := []byte("package p\n// supplied version " + stringID(version) + "\r\n")
	method := "textDocument/didOpen"
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": l.URI, "languageId": "go", "version": version, "text": string(content)}})
	if version > 1 {
		method = "textDocument/didChange"
		params, _ = json.Marshal(map[string]any{"textDocument": map[string]any{"uri": l.URI, "version": version}, "contentChanges": []any{map[string]any{"text": string(content)}}})
	}
	observation, err := json.Marshal(sessionruntime.DocumentSupply{Classification: graphprovenance.Supplied, SessionID: ctx.SessionID, Generation: ctx.Generation, URI: l.URI, DocumentVersion: version, Method: method, Content: content, Params: params})
	return acquisition.Supply{URI: l.URI, LanguageID: "go", Observation: observation}, err
}
func TestRetainedV2SupplyVersionsAndIncompleteTargets(t *testing.T) {
	for _, variant := range []string{"supplies", "partial", "ambiguous", "unadmitted", "failed", "budget"} {
		t.Run(variant, func(t *testing.T) {
			input, original := fixtureV2(t, acquisition.Slice, variant)
			e := readExportV2(t, input)
			e.InputBytes = nil
			p, err := ReconstructV2(e.Tables)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Result.Targets) != len(original.Targets) {
				t.Fatal("ASSERT_ALL_TARGETS")
			}
			if variant == "supplies" {
				versions := map[string]map[int]bool{}
				for _, s := range p.Supplies {
					if s.Receipt != nil {
						r := s.Receipt
						if versions[r.URI] == nil {
							versions[r.URI] = map[int]bool{}
						}
						versions[r.URI][r.Supply.Version] = true
					}
				}
				found := false
				for _, v := range versions {
					if len(v) > 1 {
						found = true
					}
				}
				if !found {
					t.Fatal("ASSERT_SAME_URI_VERSIONS")
				}
				for i := range e.Tables.Supplies {
					bad := cloneTablesV2(t, e.Tables)
					bad.Supplies = append(bad.Supplies[:i], bad.Supplies[i+1:]...)
					if _, err := ReconstructV2(bad); err == nil {
						t.Fatal("ASSERT_SUPPLY_ROW_REQUIRED")
					}
				}
			}
			if variant == "partial" {
				found := false
				for _, b := range p.Bindings {
					if b.AnchorStatus == "INVALID_COORDINATES" {
						found = true
					}
				}
				if !found || p.Result.AcquisitionComplete {
					t.Fatal("ASSERT_INVALID_ANCHORS_STAY_INVALID")
				}
			}
			if variant == "ambiguous" && p.Result.Targets[0].Resolution.Status != acquisition.Ambiguous {
				t.Fatal("ASSERT_AMBIGUOUS_ROOT")
			}
			if variant == "unadmitted" {
				found := false
				for _, x := range p.Result.Targets {
					if x.Admission == acquisition.AdmissionBlocked {
						found = true
					}
				}
				if !found {
					t.Fatal("ASSERT_UNADMITTED_TARGET")
				}
			}
			if variant == "failed" && p.Result.Targets[0].Resolution.Status != acquisition.ResolutionFailed {
				t.Fatal("ASSERT_FAILED_ROOT")
			}
			if variant == "budget" && p.Result.Targets[0].Resolution.Status != acquisition.ResolutionBlocked {
				t.Fatal("ASSERT_BUDGET_ROOT")
			}
			t.Log("ASSERT_V2_SUPPLY_TARGET_COVERAGE: PASS")
		})
	}
}
