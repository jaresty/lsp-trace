//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

func prepLimitsV2() prep.Limits {
	return prep.Limits{MaxTargets: 64, MaxPartBytes: 2 << 20, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
}
func postLimitsV2() prep.PostPreparationLimits {
	return prep.PostPreparationLimits{MaxConstituents: 64, MaxLedgerTargets: 4096, MaxSubmissions: 256, MaxReviews: 256, MaxClaimsPerItem: 16, MaxCitationsPerClaim: 32, MaxStringBytes: 64 << 10, MaxItemBytes: 1 << 20, MaxInputBytes: 16 << 20, MaxOutputBytes: 16 << 20, MaxEvidenceRecords: 262144, MaxWork: 10000000}
}
func inputsV2(t *testing.T, proof pilot.DerivedProducerPacketPrivateV2) []prep.InputPrivateV2 {
	t.Helper()
	ctx := context.Background()
	view, err := pilot.ReadDerivedProducerPacketPrivateV2(ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	var inputs []prep.InputPrivateV2
	for i := range view.Batches {
		p, f := derived.ProjectPrivateV2(ctx, proof, i)
		if f != nil {
			t.Fatal(f)
		}
		a, _, err := derived.InstabilityComputePrivateV2(ctx, p, []uint64{42, 43, 44, 45, 46, 47})
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, prep.InputPrivateV2{Projection2: p, Assessment2: a})
	}
	return inputs
}
func rootsV2(t *testing.T, inputs []prep.InputPrivateV2) ([2]*publication.ContentRetentionRoot, []*publication.ObservedRoot, [2]string) {
	t.Helper()
	ctx := context.Background()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(home, ".private-v2-post-owned-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	var outputs [2]*publication.ContentRetentionRoot
	var paths [2]string
	for i := range outputs {
		paths[i] = filepath.Join(base, fmt.Sprint(i))
		if err := os.Mkdir(paths[i], 0700); err != nil {
			t.Fatal(err)
		}
		r, err := publication.OpenContentRetentionRoot(ctx, paths[i])
		if err != nil {
			t.Fatal(err)
		}
		outputs[i] = r
		t.Cleanup(func() {
			if err := r.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	seen := map[string]bool{}
	var protected []*publication.ObservedRoot
	for _, in := range inputs {
		e, err := in.Projection2.PreparationEvidencePrivateV2()
		if err != nil {
			t.Fatal(err)
		}
		var report pilot.ProducerPacketQualificationReportPrivateV2
		if err := json.Unmarshal(e.QualificationReport, &report); err != nil {
			t.Fatal(err)
		}
		for _, o := range report.CopyObservations {
			if seen[o.RootPath] {
				continue
			}
			seen[o.RootPath] = true
			r, err := publication.OpenObservedRoot(ctx, o.RootPath)
			if err != nil {
				t.Fatal(err)
			}
			protected = append(protected, r)
			t.Cleanup(func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			})
		}
	}
	return outputs, protected, paths
}
func ceilingsV2() prep.PrivateCeilings {
	return prep.PrivateCeilings{FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", OrdinaryNative: "NOT_ATTEMPTED", Describe: "NOT_ATTEMPTED", Workflow: "NOT_ATTEMPTED"}
}
func submissionsV2(t *testing.T, h prep.PostPreparationPrivateV2, r prep.Result) ([]prep.CallerSubmission, []prep.CallerReview) {
	t.Helper()
	sha, err := h.PreparationSHA256()
	if err != nil {
		t.Fatal(err)
	}
	var subs []prep.CallerSubmission
	for _, p := range r.Packets {
		var handle, quote string
		for _, part := range p.Parts {
			for _, rec := range part.Records {
				if rec.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
					var d prep.Declaration
					if err := json.Unmarshal(rec.Value, &d); err != nil {
						t.Fatal(err)
					}
					handle, quote = rec.Handle, d.Body
				}
			}
		}
		if handle == "" {
			t.Fatal("ASSERT_V2_OWN_AST_BODY_REQUIRED")
		}
		subs = append(subs, prep.CallerSubmission{PrivateCeilings: ceilingsV2(), PreparationSHA256: sha, InputSHA256: p.InputSHA256, Target: p.Target, PacketID: p.ID, OrderedPartSHA256: slices.Clone(p.PartSHA256), AuthorTag: "caller-A", Disposition: "DESCRIBED", WorkingLabel: "owned technical assertion", TechnicalDescription: "Caller-authored assertion; not semantic qualification", Claims: []prep.TechnicalClaim{{Text: "This exact retained declaration was mechanically associated", Citations: []prep.Citation{{Handle: handle, SourceQuote: quote}}}}, Limitations: "No runtime/feature identity or full definition claim", Alternatives: "Could be a utility rather than a feature", RejectionCondition: "Reject if own retained identifier association fails"})
	}
	first, err := h.Reconcile(context.Background(), subs, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reviews []prep.CallerReview
	for _, row := range first.Targets {
		if row.SubmissionStatus == "SUBMITTED" {
			reviews = append(reviews, prep.CallerReview{PrivateCeilings: ceilingsV2(), PreparationSHA256: sha, InputSHA256: row.InputSHA256, Target: row.Target, SubmissionSHA256: row.SubmissionSHA256, ReviewerAuthorTag: "caller-B", Verdict: "SUPPORT_WITH_LIMITS", Findings: "Caller asserts support only; unauthenticated independence, not adjudication"})
		}
	}
	return subs, reviews
}
func verifyPublicationV2(t *testing.T, h prep.PostPreparationPrivateV2, subs []prep.CallerSubmission, reviews []prep.CallerReview, out prep.PostPublicationOutcomePrivateV2, paths [2]string) {
	t.Helper()
	if out.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" || out.Phase != "PAIR_VERIFIED" || out.Authority != 0 || out.Accepted || out.Profile != prep.PostPublicationProfilePrivateV2 {
		t.Fatal("ASSERT_V2_PRIVATE_PAIR", out.Status, out.Phase)
	}
	prepared, _ := h.PreparedBytes()
	ledger, err := h.Reconcile(context.Background(), subs, reviews)
	if err != nil {
		t.Fatal(err)
	}
	raw := out.EnvelopeBytes()
	if pilot.Digest(raw) != out.EnvelopeSHA256 {
		t.Fatal("ASSERT_V2_ENVELOPE_HASH")
	}
	var env struct {
		SchemaVersion, Profile, PreparationSHA256 string
		Preparation                               json.RawMessage
		Ledger                                    prep.StageLedgerPrivateV2
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.SchemaVersion != "post-preparation-publication.private.v2" || env.Profile != prep.PostPublicationProfilePrivateV2 || !bytes.Equal(env.Preparation, prepared) || !reflect.DeepEqual(env.Ledger, ledger) {
		t.Fatal("ASSERT_V2_EXACT_EMBEDDED_PREPARATION_LEDGER")
	}
	for _, path := range paths {
		r, err := publication.OpenContentRetentionRoot(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.ReadPostPreparationPrivateV2(context.Background(), out.EnvelopeSHA256, int64(len(raw)))
		closeErr := r.Close()
		if err != nil || closeErr != nil || !bytes.Equal(got, raw) || pilot.Digest(got) != out.EnvelopeSHA256 {
			t.Fatal("ASSERT_V2_INDEPENDENT_REOPEN", err, closeErr)
		}
	}
	raw[0] = '!'
	if out.EnvelopeBytes()[0] == '!' {
		t.Fatal("ASSERT_V2_DETACHED_ENVELOPE")
	}
}

func TestPostPreparationPrivateV2OwnedConnected(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			pilot.WithOwnedMatchedProducerPacketPrivateV2(t, counts[0], counts[1], true, func(proof pilot.DerivedProducerPacketPrivateV2, traffic func() int) {
				before := traffic()
				ctx := context.Background()
				inputs := inputsV2(t, proof)
				r, err := prep.PreparePrivateV2(ctx, inputs, prepLimitsV2())
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				for i := 0; i < counts[0]; i++ {
					want += 1 + i%3
				}
				if len(r.Packets) != want || len(r.Index) != want || len(r.Coverage) != len(inputs) || r.RequestType != prep.RequestTypePrivateV2 {
					t.Fatal("ASSERT_V2_ALL_TARGETS_PREPARED", len(r.Packets), want, r.Coverage)
				}
				seen := map[string]bool{}
				for i, c := range r.Coverage {
					if c.Requested != c.Prepared+c.SourceGap+c.ResourceGap || c.PrivateV2 == nil || c.PrivateV2.BatchTargetCount != c.OriginalTargets || c.OriginalSymbols != c.OriginalTargets || c.PrivateV2.SnapshotSymbolCount != want || c.PrivateV2.SnapshotTargetCount != want || c.PrivateV2.BatchSelectedTargetCount != c.Requested || c.PrivateV2.Source.BatchOrdinal != i || c.PrivateV2.Source.SourceMemberCount != counts[0]+counts[1] || seen[c.InputSHA256] {
						t.Fatal("ASSERT_V2_BATCH_COVERAGE_NAMESPACES", c)
					}
					seen[c.InputSHA256] = true
				}
				for _, p := range r.Packets {
					cv := r.Coverage[0]
					for _, c := range r.Coverage {
						if c.InputSHA256 == p.InputSHA256 {
							cv = c
						}
					}
					input := inputs[cv.PrivateV2.Source.BatchOrdinal]
					e, _ := input.Projection2.PreparationEvidencePrivateV2()
					source, err := graphprovenance.ParseSourceWithholdingPrivateV2(e.SourceArtifact)
					if err != nil {
						t.Fatal(err)
					}
					assessmentFound, declFound, targetFound := false, false, false
					for _, part := range p.Parts {
						if part.RequestType != prep.RequestTypePrivateV2 {
							t.Fatal("ASSERT_V2_PART_VERSION")
						}
						for _, rec := range part.Records {
							if !strings.HasPrefix(rec.Handle, p.InputSHA256+"/") {
								t.Fatal("ASSERT_V2_HANDLE_NAMESPACE")
							}
							switch rec.Kind {
							case "ASSESSMENT_PRIVATE_V2":
								var a derived.InstabilityReportPrivateV2
								if json.Unmarshal(rec.Value, &a) != nil || !reflect.DeepEqual(a, input.Assessment2.Report()) || len(a.Runs) != 6 || len(a.Pairs) != 15 || a.ExpectedRuns != 6 || a.ExpectedPairs != 15 {
									t.Fatal("ASSERT_V2_ALL_SIX_RUNS_FIFTEEN_PAIRS")
								}
								assessmentFound = true
							case "TARGET":
								var target struct{ Node graph.Node }
								if json.Unmarshal(rec.Value, &target) != nil {
									t.Fatal("target")
								}
								for _, original := range source.Evidence.Acquisition.Graph.Nodes {
									if original.ID == target.Node.ID {
										if !reflect.DeepEqual(original, target.Node) {
											t.Fatal("ASSERT_V2_ORIGINAL_NATIVE_IDENTITY")
										}
										targetFound = true
									}
								}
							case "RETAINED_SOURCE_AST_DECLARATION":
								var d prep.Declaration
								if json.Unmarshal(rec.Value, &d) != nil || !strings.HasPrefix(d.Body, "func ") || d.Kind != "RETAINED_SOURCE_AST_DECLARATION" || d.SelectionRange.Start.Line < 2 {
									t.Fatal("ASSERT_V2_MATCHED_AST_NOT_NATIVE_FULL_DEFINITION")
								}
								declFound = true
							}
						}
					}
					if !assessmentFound || !declFound || !targetFound {
						t.Fatal("ASSERT_V2_REQUIRED_RECORDS")
					}
				}
				h, err := prep.NewPostPreparationPrivateV2(ctx, inputs, prepLimitsV2(), postLimitsV2())
				if err != nil {
					t.Fatal(err)
				}
				b, _ := h.PreparedBytes()
				if !bytes.Equal(b, r.CanonicalBytes()) {
					t.Fatal("ASSERT_V2_SEALED_PREPARATION_EXACT")
				}
				b[0] = '!'
				again, _ := h.PreparedBytes()
				if again[0] == '!' {
					t.Fatal("ASSERT_V2_PREP_CLONE")
				}
				manifest, _ := h.Manifest()
				manifest[0].OrderedPartSHA256[0] = "changed"
				fresh, _ := h.Manifest()
				if fresh[0].OrderedPartSHA256[0] == "changed" {
					t.Fatal("ASSERT_V2_MANIFEST_CLONE")
				}
				subs, reviews := submissionsV2(t, h, r)
				ledger, err := h.Reconcile(ctx, subs, reviews)
				if err != nil || len(ledger.Leads) != want || ledger.GroupingPrecondition != "UNRESOLVED_NOT_CHECKED" || ledger.NominationCensus != "NOT_ADMITTED" {
					t.Fatal("ASSERT_V2_PROVISIONAL_LEDGER", err)
				}
				for _, lead := range ledger.Leads {
					if lead.Status != "PROVISIONAL" || lead.Authority != 0 || lead.Accepted || lead.Independence != "CALLER_ASSERTED_INDEPENDENCE_NOT_AUTHENTICATED" {
						t.Fatal("ASSERT_V2_NO_SEMANTIC_QUALIFICATION")
					}
				}
				outputs, protected, paths := rootsV2(t, inputs)
				// Named counterfactual: the current guard must reject a missing actual root.
				bad, err := prep.RetainPostPreparationPrivateV2(ctx, h, subs, reviews, outputs, protected[:len(protected)-1])
				if err == nil || bad.Copies[0].Outcome != "NOT_ATTEMPTED" {
					t.Fatal("ASSERT_V2_MISSING_ACTUAL_PRODUCER_ROOT_REFUSED", err)
				}
				t.Log("ASSERT_V2_MISSING_ACTUAL_PRODUCER_ROOT_REFUSED: PASS")
				out, err := prep.RetainPostPreparationPrivateV2(ctx, h, subs, reviews, outputs, protected)
				if err != nil {
					t.Fatal("publish", err, out.Phase)
				}
				verifyPublicationV2(t, h, subs, reviews, out, paths)
				if traffic() != before {
					t.Fatal("ASSERT_V2_CONNECTED_NO_PROVIDER_AFTER_CAPTURE")
				}
				t.Log("ASSERT_V2_CONNECTED_RETAINED_AST_POST_PAIR: PASS")
			})
		})
	}
}

func TestPostPreparationPrivateV2OwnedRefusalsAndGaps(t *testing.T) {
	ctx := context.Background()
	pilot.WithOwnedMatchedProducerPacketPrivateV2(t, 5, 2, false, func(proof pilot.DerivedProducerPacketPrivateV2, traffic func() int) {
		inputs := inputsV2(t, proof)
		r, err := prep.PreparePrivateV2(ctx, inputs, prepLimitsV2())
		if err != nil {
			t.Fatal(err)
		}
		foreign, f := derived.ProjectPrivateV2(ctx, proof, 0)
		if f != nil {
			t.Fatal(f)
		}
		bad := slices.Clone(inputs)
		bad[0].Projection2 = foreign
		if _, err := prep.PreparePrivateV2(ctx, bad, prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_V2_SAME_BYTES_FOREIGN_STATE_REFUSED")
		}
		bad = slices.Clone(inputs)
		bad[0].Assessment2 = derived.AssessmentPrivateV2{}
		if _, err := prep.PreparePrivateV2(ctx, bad, prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_V2_ZERO_ASSESSMENT_REFUSED")
		}
		if _, err := prep.PreparePrivateV2(ctx, append(slices.Clone(inputs), inputs[0]), prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_V2_DUPLICATE_NAMESPACE_REFUSED")
		}
		limited := prepLimitsV2()
		limited.MaxTargets = 1
		small, err := prep.PreparePrivateV2(ctx, inputs, limited)
		if err != nil || len(small.Index) != len(r.Index) || len(small.Packets) != 1 {
			t.Fatal("ASSERT_V2_OVER_TARGET_RETAINED_AS_RESOURCE_GAPS", err)
		}
		for _, ix := range small.Index[1:] {
			if ix.Status != "RESOURCE_GAP" {
				t.Fatal("ASSERT_V2_NO_DROPPED_TARGET")
			}
		}
		limited = prepLimitsV2()
		limited.MaxSourceBytes = 1
		gaps, err := prep.PreparePrivateV2(ctx, inputs, limited)
		if err != nil || len(gaps.Packets) != 0 || len(gaps.Index) != len(r.Index) {
			t.Fatal("ASSERT_V2_SOURCE_RESOURCE_NOT_CLIPPED", err)
		}
		limited = prepLimitsV2()
		limited.MaxPartBytes = 2048
		limited.MaxParts = 1
		gaps, err = prep.PreparePrivateV2(ctx, inputs, limited)
		if err != nil || len(gaps.Packets) != 0 || len(gaps.Index) != len(r.Index) {
			t.Fatal("ASSERT_V2_INDIVISIBLE_NOT_CLIPPED", err)
		}
		for _, limit := range []prep.Limits{{}, {MaxTargets: 65, MaxPartBytes: 2048, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}, {MaxTargets: 64, MaxPartBytes: 2047, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}, {MaxTargets: 64, MaxPartBytes: 2<<20 + 1, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}, {MaxTargets: 64, MaxPartBytes: 2048, MaxTotalBytes: 8<<20 + 1, MaxParts: 256, MaxSourceBytes: 2 << 20}, {MaxTargets: 64, MaxPartBytes: 2048, MaxTotalBytes: 8 << 20, MaxParts: 257, MaxSourceBytes: 2 << 20}} {
			if _, err := prep.PreparePrivateV2(ctx, inputs, limit); err == nil {
				t.Fatal("ASSERT_V2_EXPLICIT_LIMITS", limit)
			}
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := prep.PreparePrivateV2(canceled, inputs, prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_V2_CANCELED_BEFORE_AST")
		}
		bad = slices.Clone(inputs)
		bad[0].Targets = []string{"not-an-own-target"}
		if _, err := prep.PreparePrivateV2(ctx, bad, prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_V2_FOREIGN_SELECTION")
		}
		h, err := prep.NewPostPreparationPrivateV2(ctx, inputs, prepLimitsV2(), postLimitsV2())
		if err != nil {
			t.Fatal(err)
		}
		subs, reviews := submissionsV2(t, h, r)
		outputs, protected, _ := rootsV2(t, inputs)
		for _, kind := range []string{"pin", "parts", "quote", "foreign-handle", "same-author", "review-pin", "oversized-sub", "oversized-review", "duplicate", "duplicate-review", "foreign-root", "duplicate-root"} {
			t.Run(kind, func(t *testing.T) {
				ss := slices.Clone(subs)
				rr := slices.Clone(reviews)
				roots := slices.Clone(protected)
				switch kind {
				case "pin":
					ss[0].PreparationSHA256 = "wrong"
				case "parts":
					ss[0].OrderedPartSHA256 = []string{"wrong"}
				case "quote":
					ss[0].Claims = []prep.TechnicalClaim{{Text: "invalid", Citations: []prep.Citation{{Handle: subs[0].Claims[0].Citations[0].Handle, SourceQuote: "not present"}}}}
				case "foreign-handle":
					ss[0].Claims = []prep.TechnicalClaim{{Text: "invalid", Citations: []prep.Citation{{Handle: subs[len(subs)-1].Claims[0].Citations[0].Handle, SourceQuote: subs[len(subs)-1].Claims[0].Citations[0].SourceQuote}}}}
				case "same-author":
					rr[0].ReviewerAuthorTag = ss[0].AuthorTag
				case "review-pin":
					rr[0].SubmissionSHA256 = "wrong"
				case "oversized-sub":
					ss[0].TechnicalDescription = strings.Repeat("x", 64<<10+1)
				case "oversized-review":
					rr[0].Findings = strings.Repeat("x", 64<<10+1)
				case "duplicate":
					ss = append(ss, ss[0])
				case "duplicate-review":
					rr = append(rr, rr[0])
				case "foreign-root":
					_, _, otherPaths := rootsV2(t, inputs)
					foreign, e := publication.OpenObservedRoot(ctx, otherPaths[0])
					if e != nil {
						t.Fatal(e)
					}
					t.Cleanup(func() {
						if e := foreign.Close(); e != nil {
							t.Error(e)
						}
					})
					roots[0] = foreign
				case "duplicate-root":
					roots[1] = roots[0]
				}
				out, err := prep.RetainPostPreparationPrivateV2(ctx, h, ss, rr, outputs, roots)
				if err == nil || out.Copies[0].Outcome != "NOT_ATTEMPTED" {
					t.Fatal("ASSERT_V2_BAD_CALLER_OR_ROOT_NO_WRITES", kind, err)
				}
			})
		}
		// Pure reconciliation reports pending/abstained/rejected, without acceptance.
		abstain := subs[0]
		abstain.Disposition = "ABSTAINED"
		abstain.Claims = nil
		rejected := subs[1]
		rejected.PreparationSHA256 = "wrong"
		mixed, err := h.Reconcile(ctx, []prep.CallerSubmission{abstain, rejected}, nil)
		if err != nil {
			t.Fatal(err)
		}
		pending, abstained, rejects := 0, 0, 0
		for _, c := range mixed.Coverage {
			pending += c.Pending
			abstained += c.Abstained
			rejects += c.Rejected
		}
		if pending != len(r.Packets)-2 || abstained != 1 || rejects != 1 || len(mixed.Leads) != 0 {
			t.Fatal("ASSERT_V2_ALL_TERMINAL_ACCOUNTING")
		}
		if _, err := prep.RetainPostPreparationPrivateV2(ctx, prep.PostPreparationPrivateV2{}, nil, nil, outputs, protected); err == nil {
			t.Fatal("ASSERT_V2_ZERO_POST_REFUSED")
		}
		t.Log("ASSERT_V2_SEALED_PAIR_BOUNDS_QUOTES_ACCOUNTING: PASS")
	})
	pilot.WithOwnedProducerPacketPrivateV2(t, 2, 1, func(proof pilot.DerivedProducerPacketPrivateV2, _ func() int) {
		inputs := inputsV2(t, proof)
		r, err := prep.PreparePrivateV2(ctx, inputs, prepLimitsV2())
		if err != nil || len(r.Packets) != 0 || len(r.Index) != 3 {
			t.Fatal("ASSERT_V2_UNMATCHED_NATIVE_SOURCE_GAP", err, r.Coverage)
		}
		for _, ix := range r.Index {
			if ix.Status != "SOURCE_GAP" {
				t.Fatal("ASSERT_V2_UNMATCHED_NOT_QUALIFIED", ix)
			}
		}
		h, err := prep.NewPostPreparationPrivateV2(ctx, inputs, prepLimitsV2(), postLimitsV2())
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := h.Reconcile(ctx, nil, nil)
		if err != nil || len(ledger.Leads) != 0 {
			t.Fatal("ASSERT_V2_ZERO_MATCHING_NO_LEADS", err)
		}
	})
}
