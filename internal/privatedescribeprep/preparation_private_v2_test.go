package privatedescribeprep

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programc"
)

func TestPrivateV2NeutralSourceRequiresOwnReadablePolicyReceipt(t *testing.T) {
	body := []byte("package p\nfunc F() {}\n")
	n := graph.Node{ID: "native", Item: graph.Item{Name: "F", Kind: 12, URI: "file:///retained.go", Range: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 6}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 6}}}}
	target := acquisition.TargetResult{Requested: acquisition.Target{ID: "target"}, Resolution: acquisition.Resolution{Identity: &n}}
	a := acquisition.Result{Graph: graph.Result{Nodes: []graph.Node{n}, Seeds: []graph.SeedResult{{Label: "target", PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: []string{n.ID}}}}}
	a.Request.Context.PositionEncoding = "utf-16"
	for _, kind := range []string{"readable", "missing", "withheld", "wrong-hash", "duplicate", "unmatched"} {
		t.Run(kind, func(t *testing.T) {
			v := preparationView{captures: []graphprovenance.Receipt{{ID: "receipt", URI: n.URI, Status: "READABLE", Content: body}}, members: []sourceMember{{n.URI, pilot.Digest(body), "NOMINATION"}}, math: programc.CallsMathPrivate{NodeIdentities: []string{n.ID}}}
			tt := target
			nn := n
			tt.Resolution.Identity = &nn
			switch kind {
			case "missing":
				v.captures = nil
			case "withheld":
				v.captures[0].Status = "POLICY_WITHHELD"
			case "wrong-hash":
				v.members[0].SHA256 = pilot.Digest([]byte("foreign"))
			case "duplicate":
				v.captures = append(v.captures, v.captures[0])
			case "unmatched":
				nn.Name = "Callable0"
			}
			rows, _, gap := targetRecordsPrivate("input", "scope", 0, tt, a, v, Limits{64, 64 << 10, 8 << 20, 256, 2 << 20})
			if kind != "readable" {
				if gap == "" || len(rows) != 0 {
					t.Fatal("ASSERT_V2_NO_SOURCE_FALLBACK", kind, gap)
				}
				return
			}
			if gap != "" {
				t.Fatal(gap)
			}
			found := false
			for _, r := range rows {
				if r.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
					var d Declaration
					if json.Unmarshal(r.Value, &d) != nil || d.Body != "func F() {}" || d.NativeRange != n.Range || d.DeclarationRange == n.Range {
						t.Fatal("ASSERT_V2_AST_NATIVE_BOUNDARY")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("ASSERT_V2_OWN_READABLE_AST")
			}
		})
	}
}
func TestPrivateV2PostShapeAndCanonicalWithdrawal(t *testing.T) {
	// Package-private mechanical DTOs exercise shape only, never sealed admission.
	r := mechanicalPreparation(4)
	r.RequestType = RequestTypePrivateV2
	for i := range r.Packets {
		for j := range r.Packets[i].Parts {
			r.Packets[i].Parts[j].RequestType = RequestTypePrivateV2
			b, _ := json.Marshal(r.Packets[i].Parts[j])
			r.PartBytes -= lenMustPart(mechanicalPreparation(4).Packets[i].Parts[j])
			r.PartBytes += len(b)
			r.Packets[i].PartSHA256[j] = pilot.Digest(b)
		}
	}
	if _, err := postFromPrepared(context.Background(), r, mechanicalLimits()); err == nil {
		t.Fatal("ASSERT_V2_NOT_V1_ADMISSION")
	}
	if _, err := postFromPreparedProfile(context.Background(), r, mechanicalLimits(), RequestTypePrivateV2); err != nil {
		t.Fatal(err)
	}
	r.Index = make([]TargetIndex, 4097)
	if _, err := postFromPreparedProfile(context.Background(), r, mechanicalLimits(), RequestTypePrivateV2); err == nil {
		t.Fatal("ASSERT_V2_LEDGER_4097_REFUSED")
	}
	r = mechanicalPreparation(4)
	limits := r.Limits
	limits.MaxTotalBytes = len(r.CanonicalBytes()) - 1
	bounded, err := boundCanonicalResult(r, limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded.Packets) >= 4 || len(bounded.Index) != 4 || bounded.Coverage[0].Requested != bounded.Coverage[0].Prepared+bounded.Coverage[0].ResourceGap+bounded.Coverage[0].SourceGap {
		t.Fatal("ASSERT_V2_WHOLE_PACKET_WITHDRAWAL")
	}
	for _, ix := range bounded.Index {
		if ix.Status == "RESOURCE_GAP" && ix.PacketID != "" {
			t.Fatal("ASSERT_V2_WITHDRAWAL_INDEX_REPAIR")
		}
	}
	r = mechanicalPreparation(1)
	r.Packets = nil
	r.Coverage[0].Prepared = 0
	r.Coverage[0].SourceGap = 1
	r.Index[0].Status = "SOURCE_GAP"
	r.Index[0].Gap = strings.Repeat("x", 3000)
	r.Index[0].PacketID = ""
	limits = r.Limits
	limits.MaxTotalBytes = 2048
	if _, err := boundCanonicalResult(r, limits); err == nil {
		t.Fatal("ASSERT_V2_INDEX_ALONE_REFUSED_NOT_TRUNCATED")
	}
}
func lenMustPart(p Part) int { b, _ := json.Marshal(p); return len(b) }
func TestPrivateV2OpaqueAPISurface(t *testing.T) {
	limits := Limits{64, 64 << 10, 8 << 20, 256, 2 << 20}
	if _, err := PreparePrivateV2(context.Background(), make([]InputPrivateV2, 65), limits); err == nil {
		t.Fatal("ASSERT_V2_65_CONSTITUENTS_REFUSED")
	}
	if _, err := PreparePrivateV2(nil, []InputPrivateV2{{}}, limits); err == nil {
		t.Fatal("ASSERT_V2_NIL_CONTEXT_REFUSED")
	}
	if _, err := PreparePrivateV2(context.Background(), []InputPrivateV2{{Targets: make([]string, 65)}}, limits); err == nil {
		t.Fatal("ASSERT_V2_65_SELECTIONS_REFUSED")
	}
	limits.MaxSourceBytes = 2<<20 + 1
	if _, err := PreparePrivateV2(context.Background(), []InputPrivateV2{{}}, limits); err == nil {
		t.Fatal("ASSERT_V2_SOURCE_LIMIT_ONE_OVER_REFUSED")
	}
	typ := reflect.TypeOf(RetainPostPreparationPrivateV2)
	if typ.NumIn() != 6 || typ.In(1) != reflect.TypeOf(PostPreparationPrivateV2{}) {
		t.Fatal("ASSERT_V2_SEALED_PUBLICATION_ARGUMENT")
	}
	for i := 0; i < typ.NumIn(); i++ {
		if typ.In(i) == reflect.TypeOf(Result{}) || typ.In(i) == reflect.TypeOf(StageLedgerPrivateV2{}) {
			t.Fatal("ASSERT_V2_NO_DTO_TO_SEAL")
		}
	}
	var forged PostPreparationPrivateV2
	if json.Unmarshal([]byte(`{"state":{},"prepared":"forged"}`), &forged) != nil {
		t.Fatal("json diagnostic decode")
	}
	if _, err := forged.PreparedBytes(); err == nil {
		t.Fatal("ASSERT_V2_JSON_CANNOT_MINT_POST")
	}
	if _, err := json.Marshal(forged); err == nil {
		t.Fatal("ASSERT_V2_OPAQUE_POST_NO_JSON")
	}
}
