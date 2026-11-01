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
	"syscall"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// None_Model / SCRIPTED_CONTRACT. The actual CLI receives a physical nonempty
// reply, then its observer refuses exact READY identity. Inspect immediate stdout,
// not a later LatestCohort rejection. The original stdio producer is controlled.
func TestHostCohortRestartRefusalNoZeroDTO(t *testing.T) {
	for _, invalid := range []string{"config", "output-path"} {
		t.Run(invalid, func(t *testing.T) {
			source, sourcePath, base := hostProductCLIRequestFixture(t)
			sourceRaw, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			c := product.CohortConfig{RolePromptSHA256: [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}, Requested: 20, Profile: product.CohortProfile, Protocol: product.CohortProtocol, SourceConfigSHA256: product.ConfigDigest(source.Configuration), ScopeSHA256: source.Configuration.ScopeSHA256, Store: filepath.Join(base, "refused-cohort"), Grant: pilot.Digest([]byte("controlled refusal grant")), MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}
			if invalid == "config" {
				c.Store = "relative-untrusted"
			} else if err := os.WriteFile(c.Store, []byte("not a trusted directory"), 0600); err != nil {
				t.Fatal(err)
			}
			r := hostCohortRequest{Schema: "private-existing-host-cohort-request.v1", SourceRequest: sourcePath, SourceRequestSHA256: pilot.Digest(sourceRaw), Configuration: c, BackendTimeoutMS: 180000}
			r.RuntimeGrantSHA256 = hostCohortDigest(r)
			raw, _ := json.Marshal(r)
			path := filepath.Join(base, "refused-request.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := runPrivateHostCohort([]string{"fail", path}, bytes.NewReader(nil), &stdout, &stderr); code == 0 || stdout.Len() != 0 {
				t.Fatalf("ASSERT_RESTART_CLI_NO_ZERO_DTO: code=%d stdout=%q", code, stdout.String())
			}
		})
	}
}

func TestHostCohortPinRepairImmediateFailure(t *testing.T) {
	for _, nav := range []bool{false, true} {
		t.Run(fmt.Sprint(nav), func(t *testing.T) {
			source, sourcePath, base := hostProductCLIRequestFixture(t)
			source.Configuration.MaxCalls = 16
			source.Configuration.ApprovalSHA256 = product.ConfigDigest(source.Configuration)
			source.RuntimeGrantSHA256 = hostProductRequestDigestV3(source)
			raw, _ := json.Marshal(source)
			if e := os.WriteFile(sourcePath, raw, 0600); e != nil {
				t.Fatal(e)
			}
			c := product.CohortConfig{RolePromptSHA256: [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}, Requested: 20, Navigation: nav, Profile: product.CohortProfile, Protocol: product.CohortProtocol, SourceConfigSHA256: product.ConfigDigest(source.Configuration), ScopeSHA256: source.Configuration.ScopeSHA256, Store: filepath.Join(base, "pinrepair-cohort"), Grant: pilot.Digest([]byte("controlled pin repair grant")), MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}
			request := hostCohortRequest{Schema: "private-existing-host-cohort-request.v1", SourceRequest: sourcePath, SourceRequestSHA256: pilot.Digest(raw), Configuration: c, BackendTimeoutMS: 180000}
			request.RuntimeGrantSHA256 = hostCohortDigest(request)
			path := filepath.Join(base, "pinrepair-request.json")
			raw, _ = json.Marshal(request)
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			tool := filepath.Join(base, "lsp-trace-pinrepair")
			if out, e := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); e != nil {
				t.Fatalf("fixture build %v %s", e, out)
			}
			if code := runPrivateHostCohort([]string{"setup", path}, bytes.NewReader(nil), io.Discard, io.Discard); code != 0 {
				t.Fatal("setup", code)
			}
			m := hostCohortMailbox(request, source)
			for _, role := range []string{"interpreter", "reviewer"} {
				if e := product.MarkCohortReady(m.Directory, product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: "pinrepair-" + role, PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(role)))}); e != nil {
					t.Fatal(e)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, tool, "private-host-cohort", "run", path)
			cmd.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if e := cmd.Start(); e != nil {
				t.Fatal(e)
			}
			waited := false
			defer func() {
				if !waited {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			stop := 3
			if nav {
				stop = 5
			}
			known := 0
			hist := map[string][]product.CohortDecision{}
			for seq := 1; seq <= stop; seq++ {
				role := "interpreter"
				if seq%2 == 0 {
					role = "reviewer"
				}
				w, e := product.WaitCohort(ctx, m.Directory, role, uint64(seq-1))
				if e != nil || w.Bundle == nil {
					t.Fatal("fixture wait", e)
				}
				var reply []byte
				if seq == stop {
					if e = cmd.Process.Signal(syscall.SIGSTOP); e != nil {
						t.Fatal(e)
					}
					reply = []byte(" {\"Protocol\":\"WRONG\"} \n")
					if e = product.SubmitCohort(m.Directory, role, uint64(seq), reply); e == nil {
						t.Fatal("bad protocol fixture accepted")
					}
					readyPath := filepath.Join(m.Directory, "ready-interpreter.json")
					readyRaw, e := os.ReadFile(readyPath)
					if e != nil {
						t.Fatal(e)
					}
					var ready product.Ready
					if json.Unmarshal(readyRaw, &ready) != nil {
						t.Fatal("ready fixture")
					}
					ready.LaunchID = "substituted-after-reply"
					readyRaw, _ = json.Marshal(ready)
					if e = os.WriteFile(readyPath, readyRaw, 0600); e != nil {
						t.Fatal(e)
					}
					if e = cmd.Process.Signal(syscall.SIGCONT); e != nil {
						t.Fatal(e)
					}
				} else {
					task, e := product.ReadCohortBundle(*w.Bundle)
					if e != nil {
						t.Fatal(e)
					}
					response := product.CohortResponse{Protocol: product.CohortProtocol, RequestDigest: task.RequestDigest, Items: []product.CohortDecision{}}
					for _, i := range task.Items {
						d := product.CohortDecision{ItemID: i.ItemID, InputID: i.InputID, Target: i.Target, Attempt: i.Attempt, Predecessor: i.Predecessor, OwnBodySHA256: i.OwnBodySHA256, Revision: i.Revision, Disposition: "SUPPORTED", Label: "Controlled lead", Explanation: "Bounded own body", Alternative: "Scaffolding", Falsifier: "Caller contradicts", Quote: i.Body, Start: 0, End: len(i.Body)}
						response.Items = append(response.Items, d)
						hist[i.ItemID] = append(hist[i.ItemID], d)
					}
					reply, _ = json.Marshal(response)
					if e = product.SubmitCohort(m.Directory, role, uint64(seq), reply); e != nil {
						t.Fatal(e)
					}
				}
				known += len(reply)
			}
			e := cmd.Wait()
			waited = true
			var report product.CohortReport
			if json.Unmarshal(stdout.Bytes(), &report) != nil || e == nil || report.ResponseBytes != known {
				t.Fatalf("ASSERT_PIN_CLI_IMMEDIATE_KNOWN_RX: got=%d want=%d err=%v stderr=%s", report.ResponseBytes, known, e, stderr.String())
			}
			if report.AccountingValidation != "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES" || report.MachinePaths != [2]string{} || report.HumanPaths != [2]string{} {
				t.Fatal("ASSERT_PIN_CLI_UNKNOWN_NO_FAKE_PAIRS", report.AccountingValidation, report.MachinePaths)
			}
			wantReviewed := 10
			if nav {
				wantReviewed = 20
			}
			if report.Reviewed != wantReviewed {
				t.Fatal("ASSERT_PIN_CLI_PRIOR_REVIEWED", report.Reviewed)
			}
			for n := 0; n < wantReviewed; n++ {
				o := report.Outcomes[n]
				got, _ := json.Marshal(o.History)
				want, _ := json.Marshal(hist[o.Item.ItemID])
				if !bytes.Equal(got, want) {
					t.Fatal("ASSERT_PIN_CLI_PRIOR_BYTE_EXACT", n)
				}
			}
			receiptRaw, e := os.ReadFile(filepath.Join(c.Store, fmt.Sprintf("host/claim-%08d.json", stop)))
			if e != nil || !bytes.Contains(receiptRaw, []byte(`"Bytes":23`)) {
				t.Fatal("ASSERT_PIN_CLI_RETURN_RECEIPT", string(receiptRaw), e)
			}
			if !product.AdmitCohortReport(report) || encodeHostProductV3(cohortBrokenWriter{}, report) == 0 || report.ResponseBytes != known {
				t.Fatal("ASSERT_PIN_CLI_OUTPUT_FAILURE_NOT_SPEND_REFUND")
			}
			t.Logf("ASSERT_PIN_CLI_IMMEDIATE_KNOWN_RX: PASS nav=%t known=%d accounting=%s no advertised pairs", nav, known, report.AccountingValidation)
			// Restart without repairing READY. Neither API nor CLI may turn this
			// durable local observation into zero spend or a verified publication.
			originals := map[string][]byte{}
			for _, dir := range []string{filepath.Join(c.Store, "claim"), filepath.Join(c.Store, "host"), filepath.Join(c.Store, "pages"), m.Directory} {
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					p := filepath.Join(dir, entry.Name())
					b, err := os.ReadFile(p)
					if err != nil {
						t.Fatal(err)
					}
					originals[p] = b
				}
			}
			check := func(label string, got product.CohortReport) {
				t.Helper()
				if got.ResponseBytes != known || got.RequestBytes != report.RequestBytes || got.Calls != report.Calls {
					t.Errorf("ASSERT_RESTART_%s_KNOWN_RX: got=%d want=%d calls=%d request=%d", label, got.ResponseBytes, known, got.Calls, got.RequestBytes)
				} else {
					t.Logf("ASSERT_RESTART_%s_KNOWN_RX: PASS known=%d", label, known)
				}
				if got.Schema == "" || got.AccountingValidation != "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES" || got.MachinePaths != [2]string{} || got.HumanPaths != [2]string{} || got.Accepted || got.Authority != 0 || got.FeatureIdentity != "UNRESOLVED" || got.SourceCompleteness != "UNKNOWN" || got.AbsenceInference != "PROHIBITED" {
					t.Errorf("ASSERT_RESTART_%s_UNKNOWN_NO_PAIRS: accounting=%q schema=%q", label, got.AccountingValidation, got.Schema)
				}
				if got.Reviewed != wantReviewed {
					t.Errorf("ASSERT_RESTART_%s_PRIOR_REVIEWS: got=%d want=%d", label, got.Reviewed, wantReviewed)
				}
				wantHistory, _ := json.Marshal(report.Outcomes)
				gotHistory, _ := json.Marshal(got.Outcomes)
				if !bytes.Equal(wantHistory, gotHistory) {
					t.Errorf("ASSERT_RESTART_%s_PRIOR_HISTORY_BYTE_EXACT", label)
				}
			}
			api, apiErr := product.DeliverCohortFailure(c, c.Requested, "HOST_TOOL_UNAVAILABLE")
			if apiErr == nil {
				t.Error("ASSERT_RESTART_API_REFUSES_HISTORY")
			}
			check("API", api)
			for attempt := 0; attempt < 2; attempt++ {
				failCmd := exec.CommandContext(ctx, tool, "private-host-cohort", "fail", path)
				var out, errout bytes.Buffer
				failCmd.Stdout, failCmd.Stderr = &out, &errout
				if err := failCmd.Run(); err == nil {
					t.Error("ASSERT_RESTART_CLI_EXIT_FAILURE")
				}
				var got product.CohortReport
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Errorf("ASSERT_RESTART_CLI_DTO: %v", err)
				}
				check("CLI", got)
			}
			for p, before := range originals {
				after, err := os.ReadFile(p)
				if err != nil || !bytes.Equal(before, after) {
					t.Errorf("ASSERT_RESTART_OLD_BYTES_UNCHANGED: %s", filepath.Base(p))
				}
			}
			if _, err := product.LatestCohort(c); err == nil {
				t.Error("ASSERT_RESTART_LATEST_REFUSES_UNKNOWN")
			}
			if _, err := product.FinalizeCohortHost(c, m.Directory, [2]product.CohortRoleReturn{}); err == nil {
				t.Error("ASSERT_RESTART_FINALIZE_REFUSES_UNKNOWN")
			}
		})
	}
}
