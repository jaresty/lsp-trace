//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Full one-private-CLI workflow. Existing stdio producer is CONTROLLED, not gopls.
// Both role services and their return records are scripted None_Model contract
// fixtures; they do not claim actual Agent execution or semantic independence.
func TestHostCohortCLIActualTwentyOneInvocation(t *testing.T) {
	source, sourcePath, base := hostProductCLIRequestFixture(t)
	source.Configuration.MaxCalls = 16
	source.Configuration.ApprovalSHA256 = product.ConfigDigest(source.Configuration)
	source.RuntimeGrantSHA256 = hostProductRequestDigestV3(source)
	raw, _ := json.Marshal(source)
	if e := os.WriteFile(sourcePath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	c := product.CohortConfig{RolePromptSHA256: [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}, Requested: 20, Navigation: true, Profile: product.CohortProfile, Protocol: product.CohortProtocol, SourceConfigSHA256: product.ConfigDigest(source.Configuration), ScopeSHA256: source.Configuration.ScopeSHA256, Store: filepath.Join(base, "reviewed-cohort"), Grant: pilot.Digest([]byte("caller-supplied controlled fixture cohort grant")), MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}
	request := hostCohortRequest{Schema: "private-existing-host-cohort-request.v1", SourceRequest: sourcePath, SourceRequestSHA256: pilot.Digest(raw), Configuration: c, BackendTimeoutMS: 180000}
	request.RuntimeGrantSHA256 = hostCohortDigest(request)
	path := filepath.Join(base, "cohort-request.json")
	raw, _ = json.Marshal(request)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	tool := filepath.Join(base, "lsp-trace")
	build := exec.Command("go", "build", "-o", tool, ".")
	if raw, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build %v %s", e, raw)
	}
	var setup, diagnostic bytes.Buffer
	if runPrivateHostCohort([]string{"setup", path}, bytes.NewReader(nil), &setup, &diagnostic) != 0 {
		t.Fatal("setup", diagnostic.String())
	}
	m := hostCohortMailbox(request, source)
	ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
	defer cancel()
	type observed struct {
		role                     string
		ret                      product.CohortRoleReturn
		calls, request, response int
		err                      error
	}
	completed := make(chan observed, 2)
	notifications := make(chan string, 2)
	var toolMu sync.Mutex
	toolResults := map[string]observed{}
	notified := map[string]bool{}
	getResult := func(role string) (observed, error) {
		toolMu.Lock()
		defer toolMu.Unlock()
		result, ok := toolResults[role]
		if !ok || !notified[role] {
			return observed{}, fmt.Errorf("scripted result unavailable before completion notification")
		}
		return result, nil
	}
	if _, e := getResult("interpreter"); e == nil {
		t.Fatal("ASSERT_COHORT_NO_RESULT_BEFORE_COMPLETION_NOTIFICATION")
	}
	// Predetermined tool-contract adapter only; no Agent/model/controller runtime.
	go func() {
		for n := 0; n < 2; n++ {
			result := <-completed
			toolMu.Lock()
			toolResults[result.role] = result
			toolMu.Unlock()
			notifications <- result.role
		}
	}()
	for _, role := range []string{"interpreter", "reviewer"} {
		if e := product.MarkCohortReady(m.Directory, product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: "scripted-hosttool-" + role, PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(role)))}); e != nil {
			t.Fatal(e)
		}
		go func(role string) {
			var after uint64
			seen := observed{role: role}
			correctID := ""
			for {
				wait, e := product.WaitCohort(ctx, m.Directory, role, after)
				if e != nil {
					seen.err = e
					completed <- seen
					return
				}
				if wait.Event == "TERMINAL" {
					seen.ret = product.CohortRoleReturn{Kind: "SCRIPTED_CONTRACT", Role: role, LaunchID: "scripted-hosttool-" + role, TerminalSHA256: wait.TerminalSHA256, Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("predetermined return observation")), Sequence: after}
					completed <- seen
					return
				}
				if wait.Bundle.Protocol == product.CohortNavigationProtocol {
					task, e := product.ReadCohortNavigationBundle(*wait.Bundle)
					if e != nil {
						seen.err = e
						completed <- seen
						return
					}
					groups := task.Proposal
					if role == "interpreter" {
						groups = []product.CohortNavigation{{ID: "scripted-nav-1", Label: "Reversible grouping", Explanation: "Controlled navigation hypothesis, not feature identity", Alternative: "Keep separate", Falsifier: "Responsibilities differ", Withdrawal: "Withdraw on contradiction", Revisit: "Revisit after correction", Members: []product.CohortRevision{task.Candidates[0].Revision, task.Candidates[1].Revision}, Supported: true}}
					}
					reply, _ := json.Marshal(product.CohortNavigationResponse{Protocol: product.CohortNavigationProtocol, RequestDigest: task.RequestDigest, Groups: groups})
					seen.calls++
					seen.request += wait.Bundle.Bytes
					seen.response += len(reply)
					if e = product.SubmitCohort(m.Directory, role, wait.Bundle.Sequence, reply); e != nil {
						seen.err = e
						completed <- seen
						return
					}
					after = wait.Bundle.Sequence
					continue
				}
				task, e := product.ReadCohortBundle(*wait.Bundle)
				if e != nil {
					seen.err = e
					completed <- seen
					return
				}
				seen.calls++
				seen.request += wait.Bundle.Bytes
				response := product.CohortResponse{Protocol: product.CohortProtocol, RequestDigest: task.RequestDigest, Items: []product.CohortDecision{}}
				for _, item := range task.Items {
					if item.ConfigSHA256 != product.ConfigDigest(source.Configuration) || item.ScopeSHA256 != source.Configuration.ScopeSHA256 || item.Body == "" {
						seen.err = fmt.Errorf("own-source binding")
						completed <- seen
						return
					}
					d := product.CohortDecision{ItemID: item.ItemID, InputID: item.InputID, Target: item.Target, Attempt: item.Attempt, Predecessor: item.Predecessor, OwnBodySHA256: item.OwnBodySHA256, Revision: item.Revision, Disposition: "SUPPORTED", Label: "Controlled own declaration", Explanation: "Predetermined technical lead; product purpose unresolved", Alternative: "Scaffolding rather than capability", Falsifier: "Own declaration does not support description", Quote: item.Body, Start: 0, End: len(item.Body)}
					// One correction and re-review from the SAME sixteen-call grant.
					if role == "reviewer" && correctID == "" {
						correctID = item.ItemID
					}
					if role == "reviewer" && item.ItemID == correctID && item.Revision == 1 {
						d.Disposition = "UNSUPPORTED"
					}
					response.Items = append(response.Items, d)
				}
				reply, _ := json.Marshal(response)
				seen.response += len(reply)
				if e = product.SubmitCohort(m.Directory, role, wait.Bundle.Sequence, reply); e != nil {
					seen.err = e
					completed <- seen
					return
				}
				after = wait.Bundle.Sequence
			}
		}(role)
	}
	cmd := exec.CommandContext(ctx, tool, "private-host-cohort", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	e := cmd.Run()
	if e != nil {
		t.Fatalf("ASSERT_COHORT_CLI_TWENTY_PIPELINE: %v %s stdout=%s", e, errout.String(), out.String())
	}
	var report product.CohortReport
	if json.Unmarshal(out.Bytes(), &report) != nil || report.Reviewed != 20 || report.Pending != 0 || report.Calls > 16 || report.Calls != 8 || len(report.NavigationHistory) != 2 || report.Authority != 0 || report.Accepted {
		t.Fatalf("ASSERT_COHORT_CLI_TWENTY_COVERAGE: reviewed=%d calls=%d pending=%d", report.Reviewed, report.Calls, report.Pending)
	}
	calls, requests, responses := 0, 0, 0
	var returns [2]product.CohortRoleReturn
	for n := 0; n < 2; n++ {
		select {
		case role := <-notifications:
			toolMu.Lock()
			notified[role] = true
			toolMu.Unlock()
			o, retrievalErr := getResult(role)
			if retrievalErr != nil {
				t.Fatal(retrievalErr)
			}
			if o.err != nil {
				t.Fatal(o.err)
			}
			idx := 0
			if o.ret.Role == "reviewer" {
				idx = 1
			}
			returns[idx] = o.ret
			calls += o.calls
			requests += o.request
			responses += o.response
		case <-ctx.Done():
			t.Fatal("ASSERT_COHORT_ROLE_RETURN_DEADLINE", ctx.Err())
		}
	}
	if report.Calls != calls || report.RequestBytes != requests || report.ResponseBytes != responses {
		t.Fatal("ASSERT_COHORT_CLI_PHYSICAL_BYTES_EXACT")
	}
	if !report.OriginalCleanup.InnerGraceful || !report.OriginalCleanup.OuterOriginalProcessWaited {
		t.Fatal("ASSERT_COHORT_THREE_CLEANUP_FIELDS", report.OriginalCleanup)
	}
	finalInput, _ := json.Marshal(returns)
	var final bytes.Buffer
	if runPrivateHostCohort([]string{"finalize", path}, bytes.NewReader(finalInput), &final, io.Discard) != 0 {
		t.Fatal("ASSERT_COHORT_CLI_TERMINAL_RETURN_VIEW")
	}
	var view product.CohortHostView
	if json.Unmarshal(final.Bytes(), &view) != nil || view.Status != "SCRIPTED_CONTRACT_RETURNS_RECORDED" {
		t.Fatal("ASSERT_COHORT_SCRIPTED_NOT_AUTHENTICATED_AGENT")
	}
	if prior, e := product.LatestCohort(c); e != nil || prior.Reviewed != 20 {
		t.Fatal("ASSERT_COHORT_CLI_VERIFIED_HISTORY", e)
	}
	t.Logf("ASSERT_COHORT_CLI_TWENTY_PIPELINE: PASS; %d exchanges; request=%d response=%d; twenty own bodies; None_Model scripted host returns", calls, requests, responses)
}
