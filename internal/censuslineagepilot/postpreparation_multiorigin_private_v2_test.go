//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
	prep "lsp-trace/internal/privatedescribeprep"
)

func TestPostPreparationPrivateV2OwnedSeparateOrigins(t *testing.T) {
	pilot.WithOwnedMatchedProducerPacketPrivateV2(t, 2, 1, false, func(a pilot.DerivedProducerPacketPrivateV2, _ func() int) {
		pilot.WithOwnedMatchedProducerPacketPrivateV2(t, 2, 1, false, func(b pilot.DerivedProducerPacketPrivateV2, _ func() int) {
			ctx := context.Background()
			aa, bb := inputsV2(t, a), inputsV2(t, b)
			inputs := append(slices.Clone(aa), bb...)
			r, err := prep.PreparePrivateV2(ctx, inputs, prepLimitsV2())
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Index) != 6 || len(r.Packets) != 6 || len(r.Coverage) != 4 {
				t.Fatal("ASSERT_V2_SEPARATE_ORIGIN_COVERAGE")
			}
			if r.Coverage[0].PrivateV2.OriginID != r.Coverage[1].PrivateV2.OriginID || r.Coverage[2].PrivateV2.OriginID != r.Coverage[3].PrivateV2.OriginID || r.Coverage[0].PrivateV2.OriginID == r.Coverage[2].PrivateV2.OriginID {
				t.Fatal("ASSERT_V2_BATCHES_NOT_INDEPENDENT_ORIGINS")
			}
			seen := map[string]bool{}
			for _, c := range r.Coverage {
				if seen[c.InputSHA256] {
					t.Fatal("ASSERT_V2_NO_CROSS_ORIGIN_NAMESPACE_COLLISION")
				}
				seen[c.InputSHA256] = true
			}
			// Reversed caller selection still emits original acquisition order.
			e, _ := aa[0].Projection2.PreparationEvidencePrivateV2()
			source, err := graphprovenance.ParseSourceWithholdingPrivateV2(e.SourceArtifact)
			if err != nil {
				t.Fatal(err)
			}
			selected := slices.Clone(aa)
			selected[0].Targets = []string{source.Evidence.Acquisition.Targets[1].Requested.ID, source.Evidence.Acquisition.Targets[0].Requested.ID}
			ordered, err := prep.PreparePrivateV2(ctx, selected, prepLimitsV2())
			if err != nil || ordered.Index[0].OriginalOrdinal != 0 || ordered.Index[1].OriginalOrdinal != 1 {
				t.Fatal("ASSERT_V2_CALLER_ORDER_NOT_ACQUISITION_ORDER", err)
			}
			// Every exposed inspection clone is mutable without changing sealed bytes.
			before := r.CanonicalBytes()
			for _, in := range inputs {
				detached, err := in.Projection2.PreparationEvidencePrivateV2()
				if err != nil {
					t.Fatal(err)
				}
				for _, raw := range [][]byte{detached.Packet, detached.QualificationReport, detached.CanonicalSeeds, detached.CaptureOrigin, detached.SourceArtifact, detached.OriginalNativeV3, detached.DerivedCanonicalV5} {
					raw[0] = '!'
				}
				math := in.Projection2.Math()
				math.NodeIdentities[0] = "foreign"
				report := in.Assessment2.Report()
				report.Runs[0].Status = "FAILED"
				report.Pairs = nil
				raw := in.Assessment2.Bytes()
				raw[0] = '!'
			}
			again, err := prep.PreparePrivateV2(ctx, inputs, prepLimitsV2())
			if err != nil || !bytes.Equal(before, again.CanonicalBytes()) {
				t.Fatal("ASSERT_V2_READONLY_CLONES_CANNOT_SUBSTITUTE", err)
			}
			h, err := prep.NewPostPreparationPrivateV2(ctx, inputs, prepLimitsV2(), postLimitsV2())
			if err != nil {
				t.Fatal(err)
			}
			subs, reviews := submissionsV2(t, h, r)
			// Same quote text is insufficient when its handle is from another packet.
			foreign := subs[0]
			foreign.Claims = []prep.TechnicalClaim{{Text: "cross origin", Citations: subs[len(aa[0].Targets)+3].Claims[0].Citations}}
			ledger, err := h.Reconcile(ctx, []prep.CallerSubmission{foreign}, nil)
			if err != nil || ledger.Targets[0].SubmissionStatus != "REJECTED" {
				t.Fatal("ASSERT_V2_FOREIGN_PACKET_CITATION_REFUSED", err)
			}
			outputs, protected, paths := rootsV2(t, inputs)
			if len(protected) != 4 {
				t.Fatal("ASSERT_V2_ALL_PACKET_ROOTS_REQUIRED")
			}
			out, err := prep.RetainPostPreparationPrivateV2(ctx, h, subs, reviews, outputs, protected)
			if err != nil {
				t.Fatal(err)
			}
			verifyPublicationV2(t, h, subs, reviews, out, paths)
			// Detach inspection-only JSON; neither Result nor ledger can mint an owner.
			raw, _ := json.Marshal(ledger)
			var forged prep.PostPreparationPrivateV2
			_ = json.Unmarshal(raw, &forged)
			if _, err := forged.PreparedBytes(); err == nil {
				t.Fatal("ASSERT_V2_LEDGER_CANNOT_MINT_OWNER")
			}
			t.Log("ASSERT_V2_MULTIORIGIN_EXACT_NAMESPACES_ROOTS: PASS")
		})
	})
}
