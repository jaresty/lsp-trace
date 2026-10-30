//go:build darwin || linux

package privatedescribeprep

import (
	"context"
	"encoding/json"
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func handoffTest(t *testing.T, n int) (PostPreparationPrivateV2, *automaticHandoffBackend, *automaticHandoffBackend) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".owned-handoff-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	h := automaticTestOwner(t, n)
	c := AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: dir, Invocation: "owned-invocation", InterpreterRole: "same-model/interpreter/session-a", ReviewerRole: "same-model/reviewer/session-b", ScopeSHA256: pilot.Digest([]byte("scope")), TimeoutMS: 100, OverallTimeoutMS: 1000, PollMS: 1, Limits: automaticTestLimits()}
	c.ApprovalSHA256 = AutomaticHandoffApprovalPrivateV2(c)
	i, v, e := NewAutomaticHandoffPrivateV2(c, c.ScopeSHA256, h)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if !i.(*automaticHandoffBackend).shared.closed {
			FinishAutomaticHandoffPrivateV2(i, nil, fmt.Errorf("test done"))
		}
	})
	return h, i.(*automaticHandoffBackend), v.(*automaticHandoffBackend)
}
func handoffFixtureResponse(b *automaticHandoffBackend, r AutomaticRequestPrivateV2, cap int, payload []byte) []byte {
	raw, _ := json.Marshal(AutomaticHandoffResponsePrivateV2{b.frame(r, cap).Binding, pilot.Digest(payload), len(payload), payload})
	return raw
}
func handoffHostOnce(t *testing.T, b *automaticHandoffBackend, raw []byte) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	s := b.shared
	seq := s.sequence + 1
	go func() {
		deadline := time.Now().Add(time.Second)
		for {
			if _, e := os.Stat(filepath.Join(s.cfg.Directory, fmt.Sprintf("request-%06d.json", seq))); e == nil {
				break
			}
			if time.Now().After(deadline) {
				done <- fmt.Errorf("request not published")
				return
			}
			time.Sleep(time.Millisecond)
		}
		done <- s.publish(fmt.Sprintf("response-%06d.json", seq), raw)
	}()
	return done
}
func TestAutomaticHandoffBindings(t *testing.T) {
	for _, kind := range []string{"valid", "role", "request", "predecessor", "invocation", "owner", "sequence", "partial", "oversized", "missing", "late"} {
		t.Run(kind, func(t *testing.T) {
			h, i, _ := handoffTest(t, 1)
			r := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: h.state.sha, PredecessorDigest: "previous"}
			cap := 4096
			raw := handoffFixtureResponse(i, r, cap, []byte("[]"))
			var response AutomaticHandoffResponsePrivateV2
			json.Unmarshal(raw, &response)
			switch kind {
			case "role":
				response.Binding.RoleID = "foreign"
			case "request":
				response.Binding.RequestSHA256 = pilot.Digest([]byte("other"))
			case "predecessor":
				response.Binding.PredecessorDigest = "other"
			case "invocation":
				response.Binding.Invocation = "other"
			case "owner":
				response.Binding.OwnerNonce = strings.Repeat("0", 64)
			case "sequence":
				response.Binding.Sequence++
			case "late":
				i.shared.cfg.TimeoutMS = 1
			}
			raw, _ = json.Marshal(response)
			if kind == "partial" {
				raw = []byte(`{"Binding":`)
			}
			if kind == "oversized" {
				raw = []byte(strings.Repeat("x", cap+100))
			}
			var done <-chan error
			if kind != "missing" && kind != "late" {
				done = handoffHostOnce(t, i, raw)
			}
			payload, e := i.Exchange(context.Background(), r, cap)
			if done != nil {
				if he := <-done; he != nil {
					t.Fatal(he)
				}
			}
			if kind == "valid" {
				if e != nil || string(payload) != "[]" {
					t.Fatalf("HANDOFF_EXACT_BINDING: FAIL %v", e)
				}
			} else {
				if e == nil || !i.shared.failed {
					t.Fatal("HANDOFF_EXACT_BINDING: FAIL", kind)
				}
				seq := i.shared.sequence
				if _, e = i.Exchange(context.Background(), r, cap); e == nil || i.shared.sequence != seq {
					t.Fatal("HANDOFF_TERMINAL_NO_SEND: FAIL")
				}
			}
			t.Log("HANDOFF_EXACT_BINDING: PASS", kind)
		})
	}
}
func TestAutomaticHandoffBudget(t *testing.T) {
	h, i, _ := handoffTest(t, 1)
	r := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: h.state.sha}
	l := automaticTestLimits()
	l.MaxResponseBytes = 4096
	q, e := i.wireSize(r, l.MaxResponseBytes)
	if e != nil {
		t.Fatal(e)
	}
	l.MaxTotalBytes = q
	b := automaticBudget{limits: l}
	_, e = b.call(context.Background(), i, r)
	if e == nil || b.calls != 0 || b.bytes != 0 || i.shared.sequence != 0 {
		t.Fatal("HANDOFF_ZERO_CAPACITY_NO_SEND: FAIL")
	}
	t.Log("HANDOFF_ZERO_CAPACITY_NO_SEND: PASS")
	l.MaxTotalBytes = q + 4096
	b = automaticBudget{limits: l}
	done := handoffHostOnce(t, i, []byte(strings.Repeat("x", 8192)))
	_, e = b.call(context.Background(), i, r)
	if he := <-done; he != nil {
		t.Fatal(he)
	}
	if e == nil || b.bytes != l.MaxTotalBytes || i.lastResponse != 4097 || b.calls != 1 {
		t.Fatalf("HANDOFF_PREFIX_CHARGED: FAIL bytes=%d cap=%d read=%d", b.bytes, l.MaxTotalBytes, i.lastResponse)
	}
	_, e = b.call(context.Background(), i, r)
	if e == nil || b.calls != 1 || i.shared.sequence != 1 {
		t.Fatal("HANDOFF_EXHAUSTION_NO_SEND: FAIL")
	}
	t.Log("HANDOFF_PREFIX_CHARGED: PASS final lookahead=1; no further send")
}
func TestAutomaticHandoffTerminalReceiptBinding(t *testing.T) {
	for _, kind := range []string{"expired", "detached-outcome"} {
		t.Run(kind, func(t *testing.T) {
			_, i, _ := handoffTest(t, 1)
			if kind == "expired" {
				i.shared.deadline = time.Now().Add(-time.Second)
			}
			outcome := &PostPublicationOutcomePrivateV2{Status: "VERIFIED_PAIR_PRIVATE_WITH_LIMITS"}
			if kind == "expired" {
				// A package-owned, digest-consistent test observation isolates the deadline
				// from detached-outcome rejection. Actual pair retention is tested separately.
				content, _ := json.Marshal(postPublicationEnvelopePrivateV2{PreparationSHA256: i.shared.owner.sha, Inventory: &AutomaticInventoryPrivateV2{}})
				outcome.envelope = content
				outcome.EnvelopeSHA256 = pilot.Digest(content)
				outcome.ByteLength = uint64(len(content))
			}
			if e := FinishAutomaticHandoffPrivateV2(i, outcome, nil); e == nil {
				t.Fatal("HANDOFF_TERMINAL_RECEIPT_BINDING: FAIL", kind)
			}
			raw, e := os.ReadFile(filepath.Join(i.shared.cfg.Directory, "terminal.json"))
			if e != nil {
				t.Fatal(e)
			}
			var receipt struct{ Status string }
			if json.Unmarshal(raw, &receipt) != nil || receipt.Status != "FAILED_CLOSED" {
				t.Fatal("HANDOFF_TERMINAL_RECEIPT_BINDING: FAIL", kind)
			}
			t.Log("HANDOFF_TERMINAL_RECEIPT_BINDING: PASS", kind)
		})
	}
}
func TestAutomaticHandoffTerminalAccounting(t *testing.T) {
	h, i, v := handoffTest(t, 3)
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e == nil || a.Inventory().Status != "FAILED" || len(a.Inventory().Rows) != 3 || a.Inventory().Calls != 1 || i.shared.sequence != 1 {
		t.Fatalf("HANDOFF_TERMINAL_ACCOUNTING: FAIL error=%v inventory=%+v", e, a.Inventory())
	}
	for _, row := range a.Inventory().Rows {
		if row.Status == "PREPARED" || row.Status == "PROVISIONAL" {
			t.Fatal("HANDOFF_TERMINAL_ACCOUNTING: FAIL unaccounted row")
		}
	}
	t.Log("HANDOFF_TERMINAL_ACCOUNTING: PASS all selected targets accounted, one request only")
}
func TestAutomaticHandoffForeignOwner(t *testing.T) {
	h, i, v := handoffTest(t, 1)
	foreign := h
	state := *h.state
	foreign.state = &state
	a, e := AutomatePrivateV2(context.Background(), foreign, automaticTestLimits(), i, v)
	if e == nil || a.Inventory().Failure != "HANDOFF_FOREIGN_OWNER" || i.shared.sequence != 0 {
		t.Fatal("HANDOFF_SAME_BYTES_FOREIGN_OWNER: FAIL")
	}
	t.Log("HANDOFF_SAME_BYTES_FOREIGN_OWNER: PASS")
}
func TestAutomaticHandoffUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "empty", "mode", "substitute"} {
		t.Run(kind, func(t *testing.T) {
			h, i, _ := handoffTest(t, 1)
			r := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: h.state.sha}
			s := i.shared
			name := filepath.Join(s.cfg.Directory, "response-000001.json")
			raw := handoffFixtureResponse(i, r, 4096, []byte("[]"))
			if kind == "substitute" {
				old := s.cfg.Directory + "-old"
				if e := os.Rename(s.cfg.Directory, old); e != nil {
					t.Fatal(e)
				}
				defer os.RemoveAll(old)
				os.Mkdir(s.cfg.Directory, 0700)
			} else {
				target := filepath.Join(s.cfg.Directory, "foreign")
				os.WriteFile(target, raw, 0600)
				switch kind {
				case "symlink":
					os.Symlink(target, name)
				case "hardlink":
					os.Link(target, name)
				case "empty":
					os.WriteFile(name, nil, 0600)
				case "mode":
					os.WriteFile(name, raw, 0644)
				}
			}
			if _, e := i.Exchange(context.Background(), r, 4096); e == nil || s.sequence != 0 {
				t.Fatal("HANDOFF_UNSAFE_BEFORE_SEND: FAIL")
			}
			t.Log("HANDOFF_UNSAFE_BEFORE_SEND: PASS", kind)
		})
	}
}

// Generic consumed-byte defenses remain independent of the file transport.
type automaticBudgetNumericBackend struct{ request, response, calls int }

func (*automaticBudgetNumericBackend) Identity() string { return "owned-numeric" }
func (b *automaticBudgetNumericBackend) wireSize(AutomaticRequestPrivateV2, int) (int, error) {
	return b.request, nil
}
func (b *automaticBudgetNumericBackend) wireResponseBytes() int { return b.response }
func (b *automaticBudgetNumericBackend) Exchange(context.Context, AutomaticRequestPrivateV2, int) ([]byte, error) {
	b.calls++
	return nil, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
}
func TestAutomaticConsumedPrefixNumeric(t *testing.T) {
	for _, tc := range []struct {
		q, r, want, calls int
		class             string
	}{{-1, 0, 0, 0, "MALFORMED"}, {1, -1, 1, 1, "MALFORMED"}, {1, 8192, 8193, 1, "UNAVAILABLE"}, {1, int(^uint(0) >> 1), 3 << 20, 1, "BUDGET_EXHAUSTED"}} {
		b := automaticBudget{limits: automaticTestLimits()}
		b.limits.MaxTotalBytes = 3 << 20
		x := &automaticBudgetNumericBackend{request: tc.q, response: tc.r}
		_, e := b.call(context.Background(), x, AutomaticRequestPrivateV2{})
		if e == nil || automaticClass(e) != tc.class || b.bytes != tc.want || b.calls != tc.calls {
			t.Fatal("GENERIC_PREFIX_CHARGE: FAIL", b.bytes, tc)
		}
	}
	t.Log("GENERIC_PREFIX_CHARGE: PASS")
}
