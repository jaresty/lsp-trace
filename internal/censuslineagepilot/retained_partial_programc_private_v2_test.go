//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programc"
	derivedc "lsp-trace/internal/retainedpartialprogramc"
)

func TestRetainedPartialProgramCPrivateV2OwnedScopes(t *testing.T) {
	for _, scope := range [][2]int{{1, 0}, {2, 1}, {5, 2}} {
		t.Run(fmt.Sprint(scope), func(t *testing.T) {
			pilot.WithOwnedProducerPacketPrivateV2(t, scope[0], scope[1], func(proof pilot.DerivedProducerPacketPrivateV2, traffic func() int) {
				ctx := context.Background()
				before := traffic()
				view, err := pilot.ReadDerivedProducerPacketPrivateV2(ctx, proof)
				if err != nil {
					t.Fatal(err)
				}
				packet, err := pilot.PureParseProducerPacketPrivateV2(view.Packet)
				if err != nil {
					t.Fatal(err)
				}
				var previous derivedc.ProjectionPrivateV2
				for i, batch := range view.Batches {
					p, f := derivedc.ProjectPrivateV2(ctx, proof, i)
					if f != nil {
						t.Fatal("ASSERT_V2_PROJECT", i, f)
					}
					o, f := derivedc.ComputeProjectionPrivateV2(p, 42)
					if f != nil {
						t.Fatal(f)
					}
					again, f := derivedc.ComputeProjectionPrivateV2(p, 42)
					if f != nil || !reflect.DeepEqual(o, again) {
						t.Fatal("ASSERT_V2_SEED42_DETERMINISM", f)
					}
					e, err := p.PreparationEvidencePrivateV2()
					if err != nil {
						t.Fatal(err)
					}
					source, err := graphprovenance.ParseSourceWithholdingPrivateV2(batch.SourceArtifact)
					if err != nil {
						t.Fatal(err)
					}
					s := p.Source()
					badPolicy := source
					badPolicy.Policy.Generation = 0
					badSource, _ := json.Marshal(badPolicy)
					if _, err := graphprovenance.ParseSourceWithholdingPrivateV2(badSource); err == nil {
						t.Fatal("ASSERT_V2_INVALID_SOURCE_POLICY_REFUSED")
					}
					var dtoProof pilot.DerivedProducerPacketPrivateV2
					_ = json.Unmarshal(view.Packet, &dtoProof)
					if _, f := derivedc.ProjectPrivateV2(ctx, dtoProof, i); f == nil {
						t.Fatal("ASSERT_V2_CONSISTENT_PACKET_DTO_NOT_PROOF")
					}
					want, err := graph.ProjectRetainedV3ToV5Private(batch.NativeGraph, source.Policy.WorkspaceURI, s.ProviderCommand)
					if err != nil || !bytes.Equal(want, e.DerivedCanonicalV5) || !bytes.Equal(e.OriginalNativeV3, batch.NativeGraph) || !bytes.Equal(e.SourceArtifact, batch.SourceArtifact) || !bytes.Equal(e.CanonicalSeeds, batch.CanonicalSeeds) || !bytes.Equal(e.CaptureOrigin, batch.CaptureOrigin) || !bytes.Equal(e.Packet, view.Packet) || !bytes.Equal(e.QualificationReport, view.QualificationReport) {
						t.Fatal("ASSERT_V2_EXACT_ORIGINAL_AND_DERIVED_BYTES", err)
					}
					math, f := programc.ProjectNativeCallsMathPrivateV2(want)
					if f != nil || !reflect.DeepEqual(math, p.Math()) {
						t.Fatal("ASSERT_V2_EXACT_MATH_HELPER", f)
					}
					if s.Profile != derivedc.ProfilePrivateV2 || s.Profile == derivedc.ProfileID || s.ProfileDigest != derivedc.ProfileDigestPrivateV2() || s.ProfileDigest == derivedc.ProfileDigest || s.BatchOrdinal != i || s.PacketSHA256 != view.PacketSHA256 || s.QualificationReportSHA256 != view.ReportSHA256 || s.OriginalNativeV3SHA256 != pilot.Digest(batch.NativeGraph) || s.DerivedCanonicalV5SHA256 != pilot.Digest(want) || s.OriginalNativeV3SHA256 == s.DerivedCanonicalV5SHA256 || s.SourceArtifactSHA256 != pilot.Digest(batch.SourceArtifact) || s.CanonicalSeedsSHA256 != pilot.Digest(batch.CanonicalSeeds) || s.CaptureOriginSHA256 != pilot.Digest(batch.CaptureOrigin) || s.RuntimeMetadataSHA256 != pilot.Digest(packet.RuntimeMetadata) {
						t.Fatal("ASSERT_V2_SOURCE_PINS")
					}
					for _, target := range source.Evidence.Acquisition.Targets {
						role := ""
						for _, file := range packet.Files {
							if file.Record.URI == target.Requested.Locator.URI {
								role = file.Record.Role
							}
						}
						if role != "NOMINATION" {
							t.Fatal("ASSERT_V2_CONTEXT_NOT_NOMINATION", role)
						}
					}
					if s.SourceMemberCount != scope[0]+scope[1] || s.BatchTargetCount != len(source.Evidence.Acquisition.Targets) || s.SnapshotTargetCount < scope[0] || s.NodeCount != len(math.NodeIdentities) || s.RelationCount != len(source.Evidence.Acquisition.Graph.Edges) || s.OccurrenceCount != len(math.Occurrences) || !slices.IsSorted(math.NodeIdentities) {
						t.Fatal("ASSERT_V2_VARIABLE_COUNTS", s)
					}
					if s.Authority != 0 || s.Accepted || !s.Partial || s.CanonicalComplete || s.ReplayAdmitted || s.OriginalNativeV5IdentityClaimed || s.SourceCompleteness != "UNKNOWN" || s.FeatureIdentity != "UNRESOLVED" || s.AbsenceInference != "PROHIBITED" || s.GraphWideCustodyStatus != "UNAVAILABLE" || s.OrdinaryQualificationReceiptStatus != "UNAVAILABLE" || s.OriginalHistoryStatus != "ORIGINAL_OBSERVATION_NOT_AUTHORITY" || s.DerivedV5SemanticReceipt.SemanticCommitmentDigest == "" || s.OriginalV3SemanticReceipt.SemanticCommitmentDigest == "" || s.DerivedV5SemanticReceipt.SemanticCommitmentDigest == s.OriginalV3SemanticReceipt.SemanticCommitmentDigest || s.PublicClaims != 0 {
						t.Fatal("ASSERT_V2_CLAIM_CEILING")
					}
					for _, occ := range math.Occurrences {
						if occ.Weight != 1 || occ.Provider != s.ProviderCommand {
							t.Fatal("ASSERT_V2_UNIT_DIRECTED_OCCURRENCES")
						}
					}
					members := 0
					for _, community := range o.Math.Communities {
						members += len(community.Members)
					}
					if members != len(math.NodeIdentities) || o.Math.Seed != 42 || o.Math.Resolution != 1 || o.ProfileID != derivedc.ProfilePrivateV2 {
						t.Fatal("ASSERT_V2_ACTUAL_PARTITION_COUNTS")
					}
					for _, seeds := range [][]uint64{{42, 43}, {42, 43, 44, 45, 46, 47}} {
						a, r, err := derivedc.InstabilityComputePrivateV2(ctx, p, seeds)
						if err != nil {
							t.Fatal(err)
						}
						if r.ExpectedRuns != len(seeds) || r.ExpectedPairs != len(seeds)*(len(seeds)-1)/2 || len(r.Runs) != r.ExpectedRuns || len(r.Pairs) != r.ExpectedPairs || r.IndependentSources != 1 || r.Profile != derivedc.InstabilityProfilePrivateV2 || r.PolicySHA256 != derivedc.InstabilityPolicyDigestPrivateV2() || r.Source != s {
							t.Fatal("ASSERT_V2_ALL_RUNS_AND_PAIRS")
						}
						if err = derivedc.ValidateInstabilityPrivateV2(ctx, p, a, a.Bytes()); err != nil {
							t.Fatal(err)
						}
						if i > 0 && derivedc.ValidateInstabilityPrivateV2(ctx, previous, a, a.Bytes()) == nil {
							t.Fatal("ASSERT_V2_FOREIGN_BATCH_REFUSED")
						}
						for _, mutate := range []func(*derivedc.InstabilityReportPrivateV2){
							func(r *derivedc.InstabilityReportPrivateV2) { r.Source.BatchOrdinal++ },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Source.PacketSHA256 = "wrong" },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Source.QualificationReportSHA256 = "wrong" },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Source.OriginalNativeV3SHA256 = "wrong" },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Source.DerivedCanonicalV5SHA256 = "wrong" },
							func(r *derivedc.InstabilityReportPrivateV2) { r.PolicySHA256 = "wrong" },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Runs = r.Runs[:1] },
							func(r *derivedc.InstabilityReportPrivateV2) { r.Pairs = nil },
						} {
							changed := a.Report()
							mutate(&changed)
							raw, _ := json.Marshal(changed)
							if derivedc.ValidateInstabilityPrivateV2(ctx, p, a, raw) == nil {
								t.Fatal("ASSERT_V2_TAMPERED_REPORT_REFUSED")
							}
						}
						raw := a.Bytes()
						raw[0] = 'x'
						r.Runs[0].Partition.Communities[0].Members[0] = "changed"
						if derivedc.ValidateInstabilityPrivateV2(ctx, p, a, a.Bytes()) != nil {
							t.Fatal("ASSERT_V2_REPORT_CLONES")
						}
						var forged derivedc.AssessmentPrivateV2
						_ = json.Unmarshal(a.Bytes(), &forged)
						if derivedc.ValidateInstabilityPrivateV2(ctx, p, forged, a.Bytes()) == nil {
							t.Fatal("ASSERT_V2_DTO_CANNOT_SEAL")
						}
					}
					e.Packet[0] = 'x'
					e.QualificationReport[0] = 'x'
					e.SourceArtifact[0] = 'x'
					e.OriginalNativeV3[0] = 'x'
					e.DerivedCanonicalV5[0] = 'x'
					e.CanonicalSeeds[0] = 'x'
					e.CaptureOrigin[0] = 'x'
					clone := p.Math()
					clone.NodeIdentities[0] = "changed"
					for pair := range clone.PairWeights {
						clone.PairWeights[pair] = 999
					}
					after, f := derivedc.ComputeProjectionPrivateV2(p, 42)
					if f != nil || !reflect.DeepEqual(o, after) {
						t.Fatal("ASSERT_V2_PROJECTION_CLONES", f)
					}
					for _, raw := range [][]byte{view.Packet, batch.SourceArtifact, want} {
						if _, f := programc.Project(raw); f == nil {
							t.Fatal("ASSERT_V2_ORDINARY_ADMISSION_CLOSED")
						}
					}
					// Captured Callable0/range(0..1) metadata intentionally does not identify real
					// A/B declarations in the fixture file. Evidence extraction is NOT AST qualification.
					for _, node := range source.Evidence.Acquisition.Graph.Nodes {
						if node.Name == "Callable0" && node.Range.Start.Line != 0 {
							t.Fatal("ASSERT_V2_NATIVE_METADATA_UNCHANGED")
						}
					}
					t.Logf("V2 owned nom=%d context=%d batch=%d targets=%d nodes=%d relations=%d occurrences=%d communities=%d", scope[0], scope[1], i, s.BatchTargetCount, s.NodeCount, s.RelationCount, s.OccurrenceCount, len(o.Math.Communities))
					previous = p
				}
				for _, ordinal := range []int{-1, 64, len(view.Batches)} {
					if _, f := derivedc.ProjectPrivateV2(ctx, proof, ordinal); f == nil {
						t.Fatal("ASSERT_V2_BAD_ORDINAL_REFUSED", ordinal)
					}
				}
				if traffic() != before {
					t.Fatal("ASSERT_V2_NO_PROVIDER_AFTER_CAPTURE")
				}
			})
		})
	}
}

func TestRetainedPartialProgramCPrivateV2OwnedCodec(t *testing.T) {
	pilot.WithOwnedProducerPacketCodecPrivateV2(t, func(proof pilot.DerivedProducerPacketPrivateV2, originals [][]byte, traffic func() int) {
		before := traffic()
		for i, original := range originals {
			p, f := derivedc.ProjectPrivateV2(context.Background(), proof, i)
			if f != nil {
				t.Fatal(f)
			}
			e, err := p.PreparationEvidencePrivateV2()
			if err != nil || !bytes.Equal(e.OriginalNativeV3, original) {
				t.Fatal("ASSERT_V2_CODEC_ORIGINAL_PROCESS_UNCHANGED", err)
			}
			var old, derived struct {
				Process json.RawMessage `json:"process_context"`
			}
			if json.Unmarshal(original, &old) != nil || json.Unmarshal(e.DerivedCanonicalV5, &derived) != nil || len(old.Process) == 0 || !bytes.Equal(old.Process, derived.Process) {
				t.Fatal("ASSERT_V2_CODEC_DERIVED_PROCESS_UNCHANGED")
			}
			if _, f := derivedc.ComputeProjectionPrivateV2(p, 42); f != nil {
				t.Fatal(f)
			}
			a, _, err := derivedc.InstabilityComputePrivateV2(context.Background(), p, []uint64{42, 43})
			if err != nil || derivedc.ValidateInstabilityPrivateV2(context.Background(), p, a, a.Bytes()) != nil {
				t.Fatal("ASSERT_V2_CODEC_ASSESSMENT", err)
			}
		}
		if traffic() != before {
			t.Fatal("ASSERT_V2_CODEC_NO_PROVIDER_AFTER_CAPTURE")
		}
	})
}

func TestRetainedPartialProgramCPrivateV2ZeroSeals(t *testing.T) {
	ctx := context.Background()
	var proof pilot.DerivedProducerPacketPrivateV2
	for _, raw := range []string{`{}`, `{"state":{},"Packet":"pretend"}`} {
		_ = json.Unmarshal([]byte(raw), &proof)
		if _, f := derivedc.ProjectPrivateV2(ctx, proof, 0); f == nil {
			t.Fatal("ASSERT_V2_ZERO_PRODUCER_REFUSED")
		}
	}
	var p derivedc.ProjectionPrivateV2
	_ = json.Unmarshal([]byte(`{}`), &p)
	if _, f := derivedc.ComputeProjectionPrivateV2(p, 42); f == nil {
		t.Fatal("ASSERT_V2_ZERO_MATH_DTO_REFUSED")
	}
	if _, err := p.PreparationEvidencePrivateV2(); err == nil {
		t.Fatal("ASSERT_V2_ZERO_EVIDENCE_REFUSED")
	}
}
