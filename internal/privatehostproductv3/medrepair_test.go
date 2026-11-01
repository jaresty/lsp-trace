//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostProductContinueEarlyRefusalRejectsFabricatedCanonicalReport(t *testing.T) {
	c := testConfig(t)
	if e := Initialize(c); e != nil {
		t.Fatal(e)
	}
	s, e := openStore(c)
	if e != nil {
		t.Fatal(e)
	}
	prior := Report{Status: "PAUSED_BUDGET", Application: c.Application, Requested: 20, Completed: 1, Gaps: 2, Pending: 17, Candidates: []Candidate{{ID: "old", Revision: 3, EvidenceRefs: []string{"old-immutable-ref"}}}, TargetGaps: []TargetGap{{Target: "gap", Phase: "old", Class: "old"}}, LedgerGeneration: 9, LedgerTip: "old-tip", Spend: Spend{Calls: 2}}
	if e = s.deliver(&prior); e != nil {
		s.close()
		t.Fatal(e)
	}
	s.close()
	// Mutable-looking historical alias must not select the retained observation.
	if e = os.WriteFile(filepath.Join(c.Store, "result/report.json"), []byte(`{}`), 0600); e != nil {
		t.Fatal(e)
	}
	r, e := Continue(context.Background(), c, Transfer{}, nil, nil)
	if e == nil || r.Status != "FAILED" || r.Admission == nil || r.Admission.Coverage != "UNKNOWN" || r.Completed != 0 || r.Gaps != 0 || r.Pending != c.Requested || len(r.Candidates) != 0 || len(r.TargetGaps) != 0 || r.Admission.PriorMachinePaths != [2]string{} || r.Admission.PriorHumanPaths != [2]string{} {
		t.Fatal("ASSERT_CONTINUE_EARLY_QUALIFIED_CANONICAL_OBSERVATION", e)
	}
	for _, p := range r.HumanPaths {
		raw, e := os.ReadFile(filepath.Join(c.Store, p))
		if e != nil || !bytes.Contains(raw, []byte("not current-source AST ownership")) || !bytes.Contains(raw, []byte("Refusal resource counts: UNKNOWN")) {
			t.Fatal("ASSERT_FAILURE_HUMAN_QUALIFIED", e)
		}
	}
	bad := c
	bad.ApprovalSHA256 = "invalid"
	r, e = Continue(context.Background(), bad, Transfer{}, nil, nil)
	if e == nil || r.Profile != Profile || r.Status != "FAILED" || r.Admission == nil || r.Admission.Counts != "UNKNOWN" {
		t.Fatal("ASSERT_INVALID_CONFIG_SAFE_UNKNOWN_REPORT")
	}
}

// None_Model: predetermined payloads exercise the public physical IPC boundary.
func TestHostProductSubmitUnambiguousJSON(t *testing.T) {
	for _, kind := range []string{"duplicate_protocol", "case_alias", "escaped_duplicate", "duplicate_digest", "duplicate_quote", "duplicate_supported", "alias_only", "nested_review", "nested_citation", "nested_quote", "trailing", "legitimate", "legitimate_null"} {
		t.Run(kind, func(t *testing.T) {
			dir, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			for _, role := range []string{"interpreter", "reviewer"} {
				if e = MarkReady(dir, Ready{Protocol: Protocol, Role: role, LaunchID: role, PromptSHA256: PromptDigest(role)}); e != nil {
					t.Fatal(e)
				}
			}
			body := "func Own() { /* 尾 &<> \"quoted\" */ }"
			task := Task{Protocol: Protocol, Role: "reviewer", Sequence: 1, RequestDigest: hash([]byte("own request")), Body: body}
			m := Mailbox{Directory: dir, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, ExchangeTimeout: time.Second}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				r, e := m.Exchange(ctx, task)
				if e == nil && r.Quote != body {
					e = fmt.Errorf("body changed")
				}
				done <- e
			}()
			if _, e = WaitBundle(ctx, dir, "reviewer", 0); e != nil {
				t.Fatal(e)
			}
			response := Response{Protocol: Protocol, RequestDigest: task.RequestDigest, Label: "Own", Explanation: "own body", Alternative: "infrastructure", RejectionCondition: "contrary source", Quote: body, End: len(body), Supported: true}
			canonical, _ := json.Marshal(response)
			payload := canonical
			switch kind {
			case "duplicate_protocol":
				payload = append([]byte(`{"Protocol":"OLD",`), canonical[1:]...)
			case "case_alias":
				payload = append([]byte(`{"protocol":"OLD",`), canonical[1:]...)
			case "escaped_duplicate":
				payload = append([]byte(`{"Protoc\u006fl":"OLD",`), canonical[1:]...)
			case "duplicate_digest":
				payload = append([]byte(`{"RequestDigest":"foreign",`), canonical[1:]...)
			case "duplicate_quote":
				payload = append([]byte(`{"Quote":"foreign",`), canonical[1:]...)
			case "duplicate_supported":
				payload = append([]byte(`{"Supported":false,`), canonical[1:]...)
			case "nested_quote":
				payload = bytes.Replace(canonical, []byte(`"Quote":`), []byte(`"Quote":{"Digest":"old","Digest":"new"},"Ignored":`), 1)
			case "legitimate_null":
				payload = bytes.Replace(canonical, []byte(`"Label":"Own"`), []byte(`"Label":null`), 1)
				response.Label = ""
				canonical, _ = json.Marshal(response)
			case "alias_only":
				payload = bytes.Replace(canonical, []byte(`"Protocol"`), []byte(`"protocol"`), 1)
			case "nested_review":
				payload = append([]byte(`{"Review":{"Supported":false,"Supported":true},`), canonical[1:]...)
			case "nested_citation":
				payload = append([]byte(`{"Citation":{"Digest":"old","digest":"new"},`), canonical[1:]...)
			case "trailing":
				payload = append(append([]byte{}, canonical...), []byte(` null`)...)
			case "legitimate":
				// Different key order, whitespace and unescaped Unicode/HTML remain legal.
				payload = []byte(fmt.Sprintf(" { \"Supported\": true, \"Quote\": %q, \"End\": %d, \"Start\": 0, \"RejectionCondition\": \"contrary source\", \"Alternative\": \"infrastructure\", \"Explanation\": \"own body\", \"Label\": \"Own\", \"RequestDigest\": %q, \"Protocol\": %q } \n", body, len(body), task.RequestDigest, Protocol))
			}
			e = Submit(dir, "reviewer", 1, payload)
			if kind != "legitimate" && kind != "legitimate_null" {
				if e == nil {
					t.Errorf("ASSERT_SUBMIT_AMBIGUITY_REFUSED: accepted %s", kind)
				}
				if _, statErr := os.Stat(filepath.Join(dir, "reply-00000001.json")); !os.IsNotExist(statErr) {
					t.Error("ASSERT_SUBMIT_NO_LAUNDERED_REPLY")
				}
				cancel()
				<-done
			} else {
				if e != nil {
					t.Fatal("ASSERT_SUBMIT_LEGITIMATE_JSON", e)
				}
				if e = <-done; e != nil {
					t.Fatal(e)
				}
				raw, e := os.ReadFile(filepath.Join(dir, "reply-00000001.json"))
				if e != nil || !bytes.Equal(raw, canonical) {
					t.Fatal("ASSERT_SUBMIT_CANONICAL_PAYLOAD_EXACT", e)
				}
				if kind == "legitimate" && !strings.Contains(string(payload), "尾 &<>") {
					t.Fatal("fixture Unicode")
				}
			}
		})
	}
}
