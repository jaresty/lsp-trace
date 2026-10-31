//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

func cohortHandoffTest(t *testing.T, counts ...int) (PostPreparationPrivateV2, *cohortHandoffBackend, *cohortHandoffBackend) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".owned-cohort-handoff-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	count := 2
	if len(counts) == 1 {
		count = counts[0]
	}
	h := automaticTestOwner(t, count)
	cfg := CohortHandoffConfigPrivateV2{Enabled: true, Mode: CohortModePrivateV2, Protocol: CohortHandoffProtocolPrivateV2, MaxItems: 6, Directory: dir, Invocation: "owned-cohort", InterpreterRole: "same-model/session-i", ReviewerRole: "same-model/session-r", ScopeSHA256: pilot.Digest([]byte("owned-scope")), TimeoutMS: 100, OverallTimeoutMS: 1000, PollMS: 1, Limits: cohortTestLimits()}
	if count == 6 {
		cfg.TimeoutMS = 1000
		cfg.OverallTimeoutMS = 5000
	}
	cfg.ApprovalSHA256 = CohortHandoffApprovalPrivateV2(cfg)
	i, v, e := NewCohortHandoffPrivateV2(cfg, cfg.ScopeSHA256, h)
	if e != nil {
		t.Fatal(e)
	}
	b := i.(*cohortHandoffBackend)
	t.Cleanup(func() {
		if !b.shared.closed {
			FinishCohortHandoffPrivateV2(i, nil, errors.New("controlled test complete"))
		}
	})
	return h, b, v.(*cohortHandoffBackend)
}
func cohortHandoffRequestTest(h PostPreparationPrivateV2) CohortRequestPrivateV2 {
	return cohortRequest("INTERPRET_COHORT", h.state.sha, []CohortItemPrivateV2{{InputSHA256: "owned-input", Target: "target-0", Bodies: []OwnBodyPrivateV2{{"h", "own body"}}}}, h.state.limits)
}
func TestCohortHandoffBindingsAndBytes(t *testing.T) {
	for _, kind := range []string{"valid", "reordered", "request", "owner", "sequence", "role", "predecessor", "early", "late", "oversize", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			h, i, _ := cohortHandoffTest(t)
			req := cohortHandoffRequestTest(h)
			cap := 4096
			frame := i.frame(req, cap)
			size, e := i.cohortWireSize(req, cap)
			encoded, _ := json.Marshal(frame)
			if e != nil || size != len(encoded) {
				t.Fatal("ASSERT_COHORT_PHYSICAL_SIZE: FAIL")
			}
			response := CohortHandoffResponsePrivateV2{Binding: frame.Binding, PayloadSHA256: pilot.Digest([]byte("[]")), PayloadByteLength: 2, Payload: []byte("[]")}
			switch kind {
			case "reordered":
				response.Binding.Items = append(response.Binding.Items, CohortItemBindingPrivateV2{Target: "extra"})
			case "request":
				response.Binding.RequestSHA256 = "foreign"
			case "owner":
				response.Binding.OwnerNonce = "foreign"
			case "sequence":
				response.Binding.Sequence++
			case "role":
				response.Binding.RoleID = "foreign"
			case "predecessor":
				response.Binding.Items[0].PredecessorDigest = "foreign"
			case "legacy":
				response.Binding.Protocol = AutomaticHandoffProtocolPrivateV2
			case "late":
				i.shared.exchangeDeadline = time.Now().Add(-time.Second)
			}
			raw, _ := json.Marshal(response)
			if kind == "oversize" {
				raw = make([]byte, cap+100)
			}
			host := make(chan error, 1)
			if kind == "early" {
				host <- i.shared.publish("response-000001.json", raw)
			} else if kind != "late" {
				go func() {
					deadline := time.Now().Add(time.Second)
					for time.Now().Before(deadline) {
						if _, e := i.shared.root.Lstat("request-000001.json"); e == nil {
							host <- i.shared.publish("response-000001.json", raw)
							return
						}
						time.Sleep(time.Millisecond)
					}
					host <- errors.New("controlled request wait bound")
				}()
			}
			got, e := i.ExchangeCohort(context.Background(), req, cap)
			if kind != "late" {
				if err := <-host; err != nil {
					t.Fatal(err)
				}
			}
			if kind == "valid" {
				if e != nil || string(got) != "[]" || i.lastResponse != len(raw) || i.physicalSHA != pilot.Digest(raw) {
					t.Fatal("ASSERT_COHORT_HANDOFF_BINDING: FAIL", e)
				}
				actual, e := i.shared.read("request-000001.json", 1<<20)
				if e != nil || !bytes.Equal(actual, encoded) {
					t.Fatal("ASSERT_COHORT_PHYSICAL_SIZE: FAIL emitted", e)
				}
			} else {
				var terminal interface{ TerminalHandoffFailure() }
				if e == nil || !errors.As(e, &terminal) || !i.shared.failed {
					t.Fatal("ASSERT_COHORT_HANDOFF_BINDING: FAIL refusal", kind, e)
				}
			}
			t.Log("ASSERT_COHORT_HANDOFF_BINDING: PASS")
			t.Log("ASSERT_COHORT_PHYSICAL_SIZE: PASS")
		})
	}
}
func TestCohortForeignOwnerAndClosedReuse(t *testing.T) {
	_, i, v := cohortHandoffTest(t)
	foreign := automaticTestOwner(t, 2)
	a, e := AutomateCohortPrivateV2(context.Background(), foreign, cohortTestLimits(), 6, i, v)
	if e == nil || a.Inventory().Failure != "HANDOFF_FOREIGN_OWNER" || a.owner != nil {
		t.Fatal("ASSERT_COHORT_FOREIGN_OWNER: FAIL")
	}
	if e = FinishCohortHandoffPrivateV2(i, nil, fmt.Errorf("controlled failure")); e != nil {
		t.Fatal(e)
	}
	if e = FinishCohortHandoffPrivateV2(i, nil, fmt.Errorf("controlled failure")); e == nil {
		t.Fatal("ASSERT_COHORT_CLOSED_REUSE: FAIL")
	}
	if _, e = i.ExchangeCohort(context.Background(), cohortHandoffRequestTest(foreign), 4096); e == nil {
		t.Fatal("ASSERT_COHORT_CLOSED_REUSE: FAIL exchange")
	}
	t.Log("ASSERT_COHORT_FOREIGN_OWNER: PASS")
	t.Log("ASSERT_COHORT_CLOSED_REUSE: PASS")
}
func TestCohortSixteenPhysicalCalls(t *testing.T) {
	for _, n := range []int{7, 8} {
		h := automaticTestOwner(t, n)
		i, v := cohortTestPair()
		a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 1, i, v)
		if a.Inventory().Calls != 16 {
			t.Fatal("ASSERT_COHORT_GLOBAL_SIXTEEN: FAIL", n, a.Inventory().Calls)
		}
		if n == 7 {
			if e != nil || len(a.Inventory().Candidates) != 7 {
				t.Fatal("ASSERT_COHORT_GLOBAL_SIXTEEN: FAIL success", e)
			}
		} else if e == nil || a.owner != nil || len(a.Inventory().Candidates) != 0 {
			t.Fatal("ASSERT_COHORT_GLOBAL_SIXTEEN: FAIL refusal", e)
		}
	}
	t.Log("ASSERT_COHORT_GLOBAL_SIXTEEN: PASS")
}
func TestCohortDeadlineAndTotalPrefix(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := cohortTestPair()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	i.fn = func(CohortRequestPrivateV2) ([]byte, error) {
		<-ctx.Done()
		return []byte("consumed prefix"), ctx.Err()
	}
	a, e := AutomateCohortPrivateV2(ctx, h, cohortTestLimits(), 6, i, v)
	if e == nil || a.Inventory().Failure != "CANCELLED" || a.Inventory().Calls != 1 || a.Inventory().Bytes < len("consumed prefix") || a.owner != nil {
		t.Fatal("ASSERT_COHORT_DEADLINE_PREFIX: FAIL", e, a.Inventory())
	}
	t.Log("ASSERT_COHORT_DEADLINE_PREFIX: PASS")
}
func TestLegacyHandoffApprovalWireFrozen(t *testing.T) {
	c := AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: "/owned/mailbox", Invocation: "fixed", InterpreterRole: "i", ReviewerRole: "r", ScopeSHA256: "scope", TimeoutMS: 100, OverallTimeoutMS: 1000, PollMS: 1, Limits: AutomaticLimitsPrivateV2{1, 16, 1048576, 65536, 8388608, 4194304}}
	expected := `{"Enabled":true,"Directory":"/owned/mailbox","Invocation":"fixed","InterpreterRole":"i","ReviewerRole":"r","ScopeSHA256":"scope","TimeoutMS":100,"OverallTimeoutMS":1000,"PollMS":1,"Limits":{"MaxCorrections":1,"MaxCalls":16,"MaxRequestBytes":1048576,"MaxResponseBytes":65536,"MaxTotalBytes":8388608,"MaxOutputBytes":4194304},"ApprovalSHA256":""}`
	raw, _ := json.Marshal(c)
	if string(raw) != expected || AutomaticHandoffApprovalPrivateV2(c) != pilot.Digest([]byte(expected)) {
		t.Fatal("ASSERT_LEGACY_APPROVAL_FROZEN: FAIL")
	}
	t.Log("ASSERT_LEGACY_APPROVAL_FROZEN: PASS")
}

func TestCohortSixPhysicalMailbox(t *testing.T) {
	h, i, v := cohortHandoffTest(t, 6)
	fi, fv := cohortTestPair()
	done := make(chan error, 1)
	go func() {
		for sequence := 1; sequence <= 4; sequence++ {
			name := fmt.Sprintf("request-%06d.json", sequence)
			var raw []byte
			var err error
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				raw, err = os.ReadFile(i.cfg.Directory + "/" + name)
				if err == nil {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if err != nil {
				done <- err
				return
			}
			var frame CohortHandoffRequestPrivateV2
			var req CohortRequestPrivateV2
			if json.Unmarshal(raw, &frame) != nil || json.Unmarshal(frame.Request, &req) != nil {
				done <- errors.New("controlled cohort frame parse")
				return
			}
			if (sequence <= 2 && len(req.Items) != 6) || (sequence > 2 && (req.Navigation == nil || len(req.Navigation.Candidates) != 6)) {
				done <- errors.New("controlled six-item physical denominator")
				return
			}
			backend := fi
			if frame.Binding.RoleID == v.role {
				backend = fv
			}
			payload, err := backend.fn(req)
			if err != nil {
				done <- err
				return
			}
			response, _ := json.Marshal(CohortHandoffResponsePrivateV2{frame.Binding, pilot.Digest(payload), len(payload), payload})
			if len(raw) > i.cfg.Limits.MaxRequestBytes || len(response) > frame.Binding.ResponseAllowance {
				done <- errors.New("controlled physical frame budget")
				return
			}
			if err = i.shared.publish(fmt.Sprintf("response-%06d.json", sequence), response); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 6, i, v)
	hostErr := <-done
	if e != nil || hostErr != nil || len(a.Inventory().Candidates) != 6 || a.Inventory().Calls != 4 || i.shared.sequence != 4 {
		t.Fatal("ASSERT_COHORT_SIX_PHYSICAL: FAIL", e, hostErr, a.Inventory())
	}
	for _, frame := range a.Inventory().Cohort.Frames {
		if frame.PhysicalResponseByteLength == 0 || frame.PhysicalResponseSHA256 == "" {
			t.Fatal("ASSERT_COHORT_FRAME_EVIDENCE: FAIL")
		}
	}
	t.Logf("ASSERT_COHORT_SIX_PHYSICAL: PASS calls=%d physical_budget_bytes=%d", a.Inventory().Calls, a.Inventory().Bytes)
}

func TestCohortExistingRetentionOwnerAndIdempotence(t *testing.T) {
	outs, ins := postPublicationRoots(t)
	_, otherIns := postPublicationRoots(t)
	ins = append(ins, otherIns[0])
	h := automaticTestOwner(t, 2)
	for _, root := range ins {
		path, id, e := root.CanonicalIdentityPrivateV2()
		if e != nil {
			t.Fatal(e)
		}
		h.roots = append(h.roots, producerRootBindingPrivateV2{path, id.Device, id.Inode})
	}
	i, v := cohortTestPair()
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 6, i, v)
	if e != nil {
		t.Fatal(e)
	}
	foreign := automaticTestOwner(t, 2)
	if _, e = RetainAutomaticPrivateV2(context.Background(), foreign, a, outs, ins); e == nil {
		t.Fatal("ASSERT_COHORT_RETENTION_OWNER: FAIL")
	}
	out, e := RetainAutomaticPrivateV2(context.Background(), h, a, outs, ins)
	if e != nil || out.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
		t.Fatal("ASSERT_COHORT_RETAIN_EXISTING_PAIR: FAIL", e)
	}
	for _, root := range outs {
		raw, e := root.ReadPostPreparationPrivateV2(context.Background(), out.EnvelopeSHA256, int64(out.ByteLength))
		if e != nil || !bytes.Equal(raw, out.EnvelopeBytes()) {
			t.Fatal("ASSERT_COHORT_RETAIN_EXISTING_PAIR: FAIL readback", e)
		}
	}
	again, e := RetainAutomaticPrivateV2(context.Background(), h, a, outs, ins)
	if e != nil || !bytes.Equal(again.EnvelopeBytes(), out.EnvelopeBytes()) {
		t.Fatal("ASSERT_COHORT_RETENTION_IDEMPOTENT: FAIL", e)
	}
	t.Log("ASSERT_COHORT_RETENTION_OWNER: PASS")
	t.Log("ASSERT_COHORT_RETAIN_EXISTING_PAIR: PASS")
	t.Log("ASSERT_COHORT_RETENTION_IDEMPOTENT: PASS")
}
