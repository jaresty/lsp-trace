//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

type variableAutomaticOwnedBackend struct {
	id string
	fn func(prep.AutomaticRequestPrivateV2) ([]byte, error)
}

func (b variableAutomaticOwnedBackend) Identity() string { return b.id }
func (b variableAutomaticOwnedBackend) Exchange(ctx context.Context, r prep.AutomaticRequestPrivateV2, limit int) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return b.fn(r)
}
func variableAutomaticBytes(v any) []byte { b, _ := json.Marshal(v); return b }
func variableAutomaticConfig(t *testing.T, n int) (variableCallerConfig, string, variableGitRunner) {
	t.Helper()
	c, path, git := variableOwnedConfig(t, n, 0)
	for i := range c.Scope.Members {
		body := []byte("package owned\n\nfunc A0(xs []int) int { total := 0; for _, x := range xs { if x > 0 { total += x } }; return total }\n")
		if e := os.WriteFile(filepath.Join(c.Scope.Root, c.Scope.Members[i].CanonicalRelativePath), body, 0600); e != nil {
			t.Fatal(e)
		}
		c.Scope.Members[i].ExpectedSHA256 = pilot.Digest(body)
		c.Scope.Members[i].ByteLength = uint64(len(body))
	}
	frozen, e := pilot.FreezeScopePrivateV2(context.Background(), c.Scope)
	if e != nil {
		t.Fatal(e)
	}
	c.Schema = "private-variable-scope-v2-caller.v3"
	c.Purpose = "PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL"
	cfg := prep.AutomaticHTTPConfigPrivateV2{Enabled: true, Endpoint: "https://owned.invalid/v1/chat/completions", InterpreterModel: "owned-interpreter", ReviewerModel: "owned-reviewer", ScopeSHA256: pilot.Digest(frozen.SpecBytes()), TimeoutMS: 1000, Limits: prep.AutomaticLimitsPrivateV2{MaxCorrections: 1, MaxCalls: 32, MaxRequestBytes: 8 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 32 << 20, MaxOutputBytes: 8 << 20}}
	cfg.ApprovalSHA256 = prep.AutomaticHTTPApprovalPrivateV2(cfg)
	c.Automation = &cfg
	variableSave(t, path, c)
	return c, path, git
}
func variableAutomaticOwnedFactory(t *testing.T, afterCleanup func(), kind string) variableAutomaticFactory {
	t.Helper()
	return func(prep.AutomaticHTTPConfigPrivateV2, string) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
		afterCleanup()
		i := variableAutomaticOwnedBackend{"interpreter", func(r prep.AutomaticRequestPrivateV2) ([]byte, error) {
			afterCleanup()
			if kind == "unavailable" {
				return nil, prep.AutomaticBackendErrorPrivateV2{Class: "UNAVAILABLE"}
			}
			switch r.Stage {
			case "INTERPRET":
				if len(r.Bodies) != 1 || !strings.Contains(r.Bodies[0].Body, "if x > 0") {
					t.Fatal("own body missing")
				}
				q := r.Bodies[0]
				return variableAutomaticBytes(prep.AutomaticInterpretationPrivateV2{Disposition: "DESCRIBED", Label: "positive accumulation", Description: "Starts total at zero, iterates input values, adds positive values, and returns total.", Limitations: "Own AST only; production invocation unknown", Alternatives: "May not be invoked", RejectionCondition: "Own body lacks positive filter", Claims: []prep.AutomaticClaimPrivateV2{{Text: "Accumulates only positive values", Citations: []prep.ExactQuotePrivateV2{{Handle: q.Handle, Start: 0, End: len(q.Body), Quote: q.Body}}}}}), nil
			case "NAVIGATE":
				ids := []string{}
				for _, c := range r.Candidates {
					ids = append(ids, c.ID)
				}
				return variableAutomaticBytes([]prep.NavigationPrivateV2{{ID: "arithmetic-question", Candidates: ids, Rationale: "Possible arithmetic relation, not a feature identity", WithdrawalCondition: "Contrary domain feedback"}}), nil
			}
			return nil, errors.New("unexpected owned stage")
		}}
		v := variableAutomaticOwnedBackend{"reviewer", func(r prep.AutomaticRequestPrivateV2) ([]byte, error) {
			afterCleanup()
			return variableAutomaticBytes(prep.AutomaticReviewPrivateV2{Digest: r.RevisionDigest, Verdict: "APPROVE", Findings: "Owned fixture checks the loop, predicate and accumulator; runtime unknown"}), nil
		}}
		return i, v, nil
	}
}
func TestVariableCallerAutomaticOneInvocation(t *testing.T) {
	for _, n := range []int{1, 3} {
		t.Run(strings.Repeat("batch", n), func(t *testing.T) {
			c, path, git := variableAutomaticConfig(t, n)
			factory, starts, child, _ := variableOwnedFactory(t, "")
			var transportSnapshot *int
			after := func() {
				if transportSnapshot == nil {
					snapshot := int(child.traffic.Load())
					transportSnapshot = &snapshot
				} else if *transportSnapshot != int(child.traffic.Load()) {
					t.Fatal("ASSERT_AUTO_NO_POST_SHUTDOWN_REQUEST: FAIL")
				}
				if child.shutdownRequests.Load() != 1 || child.teardowns.Load() != 1 {
					t.Fatal("ASSERT_AUTO_AFTER_CLEANUP: FAIL")
				}
			}
			r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, variableAutomaticOwnedFactory(t, after, ""))
			if e != nil || r.Phase != "PROVISIONAL_COMPLETE" || r.Inventory == nil || len(r.Inventory.Candidates) != n || starts.Load() != 1 {
				t.Fatal("ASSERT_CALLER_AUTO_FINAL: FAIL", e, r.Phase, r.Inventory)
			}
			qualificationCheck(t, "report/acquisition/complete", r.Acquisition.Complete, r.Acquisition.Complete)
			qualificationCheck(t, "report/acquisition/expected", r.Acquisition.ExpectedBatches == n, r.Acquisition.ExpectedBatches)
			qualificationCheck(t, "report/acquisition/captures", r.Acquisition.CompletedCaptures == n, r.Acquisition.CompletedCaptures)
			qualificationCheck(t, "report/cleanup/status", r.Cleanup.Status == "JOINED_STOP_VERIFIED", r.Cleanup.Status)
			qualificationCheck(t, "report/cleanup/stop", r.Cleanup.StopCalls == 1, r.Cleanup.StopCalls)
			qualificationCheck(t, "report/cleanup/shutdown", r.Cleanup.ShutdownCalls == 1, r.Cleanup.ShutdownCalls)
			qualificationCheck(t, "report/cleanup/operation", r.Cleanup.TerminalReads == 1, r.Cleanup.TerminalReads)
			if r.Outcome == nil || r.Outcome.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" || r.Outcome.Accepted || r.Outcome.Authority != 0 {
				t.Fatal("ASSERT_CALLER_AUTO_PAIR: FAIL")
			}
			raw := r.Outcome.EnvelopeBytes()
			var envelope struct{ Preparation prep.Result }
			if json.Unmarshal(raw, &envelope) != nil || len(envelope.Preparation.Coverage) != n {
				t.Fatal("ASSERT_AUTO_ASSESSMENTS: FAIL")
			}
			batchDigests := map[string]bool{}
			for _, packet := range envelope.Preparation.Packets {
				found := false
				for _, part := range packet.Parts {
					for _, rec := range part.Records {
						if rec.Kind == "ASSESSMENT_PRIVATE_V2" {
							var assessment derived.InstabilityReportPrivateV2
							if json.Unmarshal(rec.Value, &assessment) != nil || len(assessment.Runs) != len(c.Seeds) || len(assessment.Pairs) != len(c.Seeds)*(len(c.Seeds)-1)/2 || assessment.IndependentSources != 1 {
								t.Fatal("ASSERT_AUTO_ASSESSMENTS: FAIL")
							}
							if batchDigests[assessment.Source.SourceArtifactSHA256] {
								t.Fatal("ASSERT_AUTO_ASSESSMENTS: FAIL")
							}
							batchDigests[assessment.Source.SourceArtifactSHA256] = true
							found = true
						}
					}
				}
				if !found {
					t.Fatal("ASSERT_AUTO_ASSESSMENTS: FAIL")
				}
			}
			if len(batchDigests) != n {
				t.Fatal("ASSERT_AUTO_ASSESSMENTS: FAIL")
			}
			t.Log("ASSERT_AUTO_ASSESSMENTS: PASS")
			t.Log("ASSERT_AUTO_NO_POST_SHUTDOWN_REQUEST: PASS")
			if !bytes.Contains(raw, []byte(`"HumanInventory"`)) || !bytes.Contains(raw, []byte(`"Inventory"`)) || !bytes.Contains(raw, []byte("Accumulates only positive values")) {
				t.Fatal("ASSERT_CALLER_AUTO_PAIR: FAIL")
			}
			for _, path := range c.PostOutputs {
				root, e := publication.OpenContentRetentionRoot(context.Background(), path)
				if e != nil {
					t.Fatal(e)
				}
				b, e := root.ReadPostPreparationPrivateV2(context.Background(), r.Outcome.EnvelopeSHA256, int64(r.Outcome.ByteLength))
				ce := root.Close()
				if e != nil || ce != nil || !bytes.Equal(b, raw) {
					t.Fatal("ASSERT_CALLER_AUTO_PAIR: FAIL", e, ce)
				}
			}
			traffic := child.traffic.Load()
			if _, e = variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, variableAutomaticOwnedFactory(t, after, "")); e == nil || starts.Load() != 1 || child.traffic.Load() != traffic {
				t.Fatal("ASSERT_CALLER_AUTO_NO_REPLAY: FAIL")
			}
			t.Log("procedure: variableRunWithAutomatic -> real scope/acquisition/retention/computation/preparation/automatic/review/paired publication; owned process and models only")
			t.Log("ASSERT_CALLER_AUTO_FINAL: PASS")
			t.Log("ASSERT_CALLER_AUTO_PAIR: PASS")
			t.Log("ASSERT_CALLER_AUTO_NO_REPLAY: PASS")
			t.Log("ASSERT_AUTO_AFTER_CLEANUP: PASS")
		})
	}
}
func TestVariableCallerAutomaticUnavailable(t *testing.T) {
	c, path, git := variableAutomaticConfig(t, 2)
	factory, _, _, _ := variableOwnedFactory(t, "")
	r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, variableAutomaticOwnedFactory(t, func() {}, "unavailable"))
	if e != nil || r.Phase != "PROVISIONAL_WITH_UNRESOLVED" || r.Inventory == nil || len(r.Inventory.Rows) != 2 || len(r.Inventory.Candidates) != 0 {
		t.Fatal("ASSERT_CALLER_AUTO_UNAVAILABLE: FAIL", e, r.Phase, r.Inventory)
	}
	for _, row := range r.Inventory.Rows {
		if row.Status != "UNAVAILABLE" {
			t.Fatal(row)
		}
	}
	if _, e = os.Stat(c.ReportPath); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_CALLER_AUTO_UNAVAILABLE: PASS")
}
func TestVariableCallerAutomaticBindingUnavailable(t *testing.T) {
	_, path, git := variableAutomaticConfig(t, 2)
	factory, _, _, _ := variableOwnedFactory(t, "")
	r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, func(prep.AutomaticHTTPConfigPrivateV2, string) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
		return nil, nil, prep.AutomaticBackendErrorPrivateV2{Class: "UNAVAILABLE"}
	})
	if e == nil || r.Inventory == nil || len(r.Inventory.Rows) != 2 || r.Phase != "FAILED_SPENT" {
		t.Fatal("ASSERT_AUTO_BINDING_ACCOUNTING: FAIL", e)
	}
	for _, row := range r.Inventory.Rows {
		if row.Status != "UNAVAILABLE" {
			t.Fatal("ASSERT_AUTO_BINDING_ACCOUNTING: FAIL")
		}
	}
	t.Log("ASSERT_AUTO_BINDING_ACCOUNTING: PASS")
}
func TestVariableCallerAutomaticReportCloseFailure(t *testing.T) {
	_, path, git := variableAutomaticConfig(t, 1)
	factory, _, _, _ := variableOwnedFactory(t, "")
	closeReport := func(f *os.File) error { return errors.Join(f.Close(), errors.New("owned report close failure")) }
	r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, closeReport, variableOwnedCodeInputs, variableAutomaticOwnedFactory(t, func() {}, ""))
	if e == nil || r.Phase != "FAILED_SPENT" || r.Outcome == nil || r.Outcome.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
		t.Fatal("ASSERT_AUTO_REPORT_FAILURE_TERMINAL: FAIL", e, r.Phase)
	}
	t.Log("ASSERT_AUTO_REPORT_FAILURE_TERMINAL: PASS")
}
func TestVariableCallerAutomaticApprovalBeforeClaim(t *testing.T) {
	for _, kind := range []string{"off", "scope", "digest"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableAutomaticConfig(t, 1)
			switch kind {
			case "off":
				c.Automation.Enabled = false
			case "scope":
				c.Automation.ScopeSHA256 = pilot.Digest([]byte("foreign"))
				c.Automation.ApprovalSHA256 = prep.AutomaticHTTPApprovalPrivateV2(*c.Automation)
			case "digest":
				c.Automation.ApprovalSHA256 = "foreign"
			}
			variableSave(t, path, c)
			factory, starts, _, _ := variableOwnedFactory(t, "")
			calls := 0
			r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, func(prep.AutomaticHTTPConfigPrivateV2, string) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
				calls++
				return nil, nil, nil
			})
			if e == nil || starts.Load() != 0 || calls != 0 || r.Inventory != nil {
				t.Fatal("ASSERT_CALLER_AUTO_APPROVAL: FAIL")
			}
			variableEmpty(t, c)
			t.Log("ASSERT_CALLER_AUTO_APPROVAL: PASS")
		})
	}
}
